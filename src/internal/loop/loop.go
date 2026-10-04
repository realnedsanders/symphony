// Package loop runs the operating loop over a fixture workspace.
package loop

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/realnedsanders/symphony/src/internal/diff"
	"github.com/realnedsanders/symphony/src/internal/gitx"
	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/sensor"
	"github.com/realnedsanders/symphony/src/internal/sysml"
)

// Now is the clock the fixture loop observes with, so two runs share a diff.
var Now = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

// StaleBefore marks commits older than this instant as stale.
var StaleBefore = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// Run copies fixture, projects its sources, and returns the diff text.
func Run(fixture string) (string, error) {
	dir, err := os.MkdirTemp("", "symphony-loop-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	workspace := filepath.Join(dir, "workspace")
	if err := copyDir(fixture, workspace); err != nil {
		return "", err
	}
	if err := prepareRepos(filepath.Join(workspace, "repos")); err != nil {
		return "", err
	}
	report, err := Project(workspace)
	if err != nil {
		return "", err
	}
	return diff.Format(report), nil
}

// Project loads the workspace goal and collab, runs the sensors, and diffs.
func Project(workspace string) (model.Report, error) {
	ctx := context.Background()
	goal, err := sysml.ParseGoalFile(ctx, filepath.Join(workspace, "goal.sysml"))
	if err != nil {
		return model.Report{}, err
	}
	var proposals []model.Proposal
	collabPath := filepath.Join(workspace, "collab.sysml")
	if _, err := os.Stat(collabPath); err == nil {
		nodes, err := sysml.ParseFile(ctx, collabPath)
		if err != nil {
			return model.Report{}, err
		}
		proposals = sysml.ParseCollab(nodes).Proposals
	}
	readings, err := project(workspace)
	if err != nil {
		return model.Report{}, err
	}
	return diff.Compare(goal, readings, proposals), nil
}

func project(workspace string) ([]model.Reading, error) {
	var readings []model.Reading
	repoRoot := filepath.Join(workspace, "repos")
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		reading, err := sensor.ObserveGit(filepath.Join(repoRoot, entry.Name()), Now, StaleBefore)
		if err != nil {
			return nil, err
		}
		readings = append(readings, reading)
	}
	manifests, err := sensor.ObserveManifests(filepath.Join(workspace, "manifests"), Now)
	if err != nil {
		return nil, err
	}
	readings = append(readings, manifests...)
	zarfReading, err := sensor.ObserveZarf(filepath.Join(workspace, "zarf.yaml"), Now)
	if err != nil {
		return nil, err
	}
	readings = append(readings, zarfReading)
	clusterRaw, err := os.ReadFile(filepath.Join(workspace, "cluster", "resources.json"))
	if err != nil {
		return nil, err
	}
	objects, err := sensor.DecodeResources(clusterRaw)
	if err != nil {
		return nil, err
	}
	readings = append(readings, sensor.ObserveCluster(objects, Now)...)
	claimEntries, err := os.ReadDir(filepath.Join(workspace, "claims"))
	if err != nil {
		return nil, err
	}
	for _, entry := range claimEntries {
		if entry.IsDir() {
			continue
		}
		claim, err := sensor.ObserveClaim(filepath.Join(workspace, "claims", entry.Name()), Now)
		if err != nil {
			return nil, err
		}
		readings = append(readings, claim)
	}
	return readings, nil
}

func prepareRepos(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if err := gitx.EnsureRepo(path); err != nil {
			return err
		}
		if entry.Name() == "unborn" {
			continue
		}
		if _, err := gitx.Run(path, "add", "-A"); err != nil {
			return err
		}
		date := "2026-01-15T00:00:00Z"
		if entry.Name() == "archive" {
			date = "2020-01-01T00:00:00Z"
		}
		if err := commitAt(path, "observe", date); err != nil {
			return err
		}
	}
	return nil
}

func commitAt(dir, message, date string) error {
	cmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Symphony",
		"GIT_AUTHOR_EMAIL=symphony@local",
		"GIT_COMMITTER_NAME=Symphony",
		"GIT_COMMITTER_EMAIL=symphony@local",
		"GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_DATE="+date,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("loop: commit %s: %s: %w", dir, out, err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := in.Close(); err == nil {
			err = cerr
		}
	}()
	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}
