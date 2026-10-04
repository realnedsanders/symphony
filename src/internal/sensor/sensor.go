// Package sensor projects git repositories, Kubernetes manifests, Zarf
// packages, and cluster resources into stamped readings.
package sensor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"symphony/internal/gitx"
	"symphony/internal/model"
	"gopkg.in/yaml.v3"
)

// ObserveGit reads one git repository. A repository with no commit is silent
// and has empty coverage. A commit older than staleBefore is stale.
func ObserveGit(path string, now, staleBefore time.Time) (model.Reading, error) {
	base := filepath.Base(path)
	reading := model.Reading{
		ID:          "git:" + base,
		Source:      "git",
		Subject:     base,
		Observation: map[string]string{},
		Timestamp:   now.UTC(),
		Freshness:   model.Silent,
	}
	if _, err := gitx.Run(path, "rev-parse", "--is-inside-work-tree"); err != nil {
		return reading, fmt.Errorf("sensor: git %s: %w", path, err)
	}
	head, err := gitx.Run(path, "rev-parse", "HEAD")
	if err != nil {
		return reading, nil
	}
	branch, err := gitx.Run(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return reading, err
	}
	committedAt, err := gitx.Run(path, "log", "-1", "--format=%cI")
	if err != nil {
		return reading, err
	}
	stamp, err := time.Parse(time.RFC3339, committedAt)
	if err != nil {
		return reading, fmt.Errorf("sensor: commit time %q: %w", committedAt, err)
	}
	reading.Observation["head"] = head
	reading.Observation["branch"] = branch
	reading.Coverage = []string{"branch", "head"}
	reading.Freshness = model.Fresh
	if stamp.Before(staleBefore) {
		reading.Freshness = model.Stale
	}
	observed, err := readProperties(filepath.Join(path, "symphony.observe"))
	if err != nil {
		return reading, err
	}
	if subject := observed["subject"]; subject != "" {
		reading.Subject = subject
		delete(observed, "subject")
	}
	for key, value := range observed {
		reading.Observation[key] = value
		reading.Coverage = append(reading.Coverage, key)
	}
	sort.Strings(reading.Coverage)
	return reading, nil
}

// ObserveManifests reads Kubernetes YAML documents under dir.
func ObserveManifests(dir string, now time.Time) ([]model.Reading, error) {
	var readings []model.Reading
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".yaml", ".yml":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		docs, err := decodeYAML(data)
		if err != nil {
			return fmt.Errorf("sensor: manifest %s: %w", path, err)
		}
		for _, doc := range docs {
			readings = append(readings, MapResource(doc, "manifest", now))
		}
		return nil
	})
	return readings, err
}

// ObserveZarf reads a Zarf package description. It does not deploy the package.
func ObserveZarf(path string, now time.Time) (model.Reading, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Reading{}, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return model.Reading{}, fmt.Errorf("sensor: zarf %s: %w", path, err)
	}
	meta, _ := doc["metadata"].(map[string]any)
	name, _ := scalar(meta["name"])
	if name == "" {
		name = filepath.Base(path)
	}
	var components, manifests []string
	switch list := doc["components"].(type) {
	case []any:
		for _, item := range list {
			comp, _ := item.(map[string]any)
			if compName, ok := scalar(comp["name"]); ok {
				components = append(components, compName)
			}
			switch mans := comp["manifests"].(type) {
			case []any:
				for _, man := range mans {
					body, _ := man.(map[string]any)
					switch files := body["files"].(type) {
					case []any:
						for _, file := range files {
							if text, ok := scalar(file); ok {
								manifests = append(manifests, text)
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(components)
	sort.Strings(manifests)
	reading := model.Reading{
		ID:      "zarf:" + name,
		Source:  "zarf",
		Subject: name,
		Observation: map[string]string{
			"name":       name,
			"components": strings.Join(components, ","),
			"manifests":  strings.Join(manifests, ","),
		},
		Timestamp: now.UTC(),
		Coverage:  []string{"components", "manifests", "name"},
		Freshness: model.Fresh,
	}
	return reading, nil
}

// DecodeResources parses a Kubernetes object or List, the JSON a live client
// receives after fetch.
func DecodeResources(data []byte) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("sensor: cluster json: %w", err)
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("sensor: cluster json root is %T", doc)
	}
	if items, ok := obj["items"].([]any); ok {
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			mapped, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("sensor: cluster item is %T", item)
			}
			out = append(out, mapped)
		}
		return out, nil
	}
	return []map[string]any{obj}, nil
}

// MapResource projects one Kubernetes API object into a reading. The live
// fetcher and the manifest reader both use it.
func MapResource(obj map[string]any, source string, now time.Time) model.Reading {
	meta, _ := obj["metadata"].(map[string]any)
	name, _ := scalar(meta["name"])
	namespace, _ := scalar(meta["namespace"])
	kind, _ := scalar(obj["kind"])
	subject := name
	if labels, ok := meta["labels"].(map[string]any); ok {
		if labeled, ok := scalar(labels["symphony.io/subject"]); ok && labeled != "" {
			subject = labeled
		}
	}
	obs := map[string]string{}
	var coverage []string
	add := func(key, value string, ok bool) {
		if !ok || value == "" {
			return
		}
		obs[key] = value
		coverage = append(coverage, key)
	}
	add("kind", kind, kind != "")
	add("name", name, name != "")
	add("namespace", namespace, namespace != "")
	if spec, ok := obj["spec"].(map[string]any); ok {
		if replicas, ok := scalar(spec["replicas"]); ok {
			add("replicas", replicas, true)
		}
		if template, ok := spec["template"].(map[string]any); ok {
			if podSpec, ok := template["spec"].(map[string]any); ok {
				if containers, ok := podSpec["containers"].([]any); ok && len(containers) > 0 {
					if container, ok := containers[0].(map[string]any); ok {
						if image, ok := scalar(container["image"]); ok {
							add("image", image, true)
						}
					}
				}
			}
		}
	}
	if data, ok := obj["data"].(map[string]any); ok {
		for key, value := range data {
			if text, ok := scalar(value); ok {
				add(key, text, true)
			}
		}
	}
	sort.Strings(coverage)
	if name == "" {
		name = subject
	}
	return model.Reading{
		ID:          source + ":" + name,
		Source:      source,
		Subject:     subject,
		Observation: obs,
		Timestamp:   now.UTC(),
		Coverage:    coverage,
		Freshness:   model.Fresh,
	}
}

// ObserveCluster maps API objects with MapResource.
func ObserveCluster(objects []map[string]any, now time.Time) []model.Reading {
	readings := make([]model.Reading, 0, len(objects))
	for _, obj := range objects {
		readings = append(readings, MapResource(obj, "cluster", now))
	}
	return readings
}

// FetchCluster requests current resources from a reachable cluster.
func FetchCluster(ctx context.Context) ([]map[string]any, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "configmaps,deployments,services", "-A", "-o", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return nil, fmt.Errorf("sensor: kubectl: %s: %w", msg, err)
	}
	return DecodeResources(stdout.Bytes())
}

// ObserveClaim reads a sensor contract claim (key=value lines).
func ObserveClaim(path string, now time.Time) (model.Reading, error) {
	props, err := readProperties(path)
	if err != nil {
		return model.Reading{}, err
	}
	subject := props["subject"]
	if subject == "" {
		subject = filepath.Base(path)
	}
	delete(props, "subject")
	coverage := make([]string, 0, len(props))
	for key := range props {
		coverage = append(coverage, key)
	}
	sort.Strings(coverage)
	return model.Reading{
		ID:          "claim:" + subject,
		Source:      "claim",
		Subject:     subject,
		Observation: props,
		Timestamp:   now.UTC(),
		Coverage:    coverage,
		Freshness:   model.Fresh,
	}, nil
}

func readProperties(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("sensor: %s: line %q is not key=value", path, line)
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return props, nil
}

func decodeYAML(data []byte) ([]map[string]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func scalar(value any) (string, bool) {
	switch n := value.(type) {
	case nil:
		return "", false
	case string:
		return n, true
	case int:
		return strconv.Itoa(n), true
	case int64:
		return strconv.FormatInt(n, 10), true
	case uint64:
		return strconv.FormatUint(n, 10), true
	case float64:
		if n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10), true
		}
		return strconv.FormatFloat(n, 'f', -1, 64), true
	case json.Number:
		return n.String(), true
	case bool:
		return strconv.FormatBool(n), true
	default:
		return "", false
	}
}
