package author_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"symphony/internal/author"
	"symphony/internal/httpapi"
	"symphony/internal/model"
	"symphony/internal/store"
	"symphony/internal/sysml"
)

func TestAuthoringRoundTrip(t *testing.T) {
	dir := t.TempDir()
	goal := model.Goal{
		Parts:      []model.Part{{Name: "catalog", Def: "Component"}, {Name: "ingest", Def: "Component"}},
		Interfaces: []model.Interface{{Name: "DataContract", Shape: "v2"}},
		Places:     []model.Place{{Part: "ingest", Port: "input", Interface: "DataContract"}},
		Configurations: []model.Configuration{
			{Part: "catalog", Key: "replicas", Value: "2"},
		},
		Owners: []model.Ownership{
			{Part: "catalog", Owner: "platform"},
			{Part: "ingest", Owner: "data"},
		},
		Decisions: []model.Decision{{
			Name: "keepShape", About: "DataContract", Choice: "v2",
			Alternative: "v3", Rationale: "consumers cannot migrate", Status: "rejected",
		}},
		Sensors: []model.SensorContract{{
			Name: "gitSensor", Observes: "git", Aim: "repos/platform", Outside: "commit message bodies",
		}},
	}

	commit, err := author.Apply(context.Background(), dir, goal)
	if err != nil {
		t.Fatalf("author: %v", err)
	}
	if commit == "" {
		t.Fatal("empty commit")
	}

	parsed, err := sysml.ParseGoalFile(context.Background(), filepath.Join(dir, store.GoalPath))
	if err != nil {
		t.Fatalf("parse committed goal: %v", err)
	}
	assertGoal(t, parsed, goal)

	web := filepath.Join(moduleRoot(t), "web")
	handler, err := httpapi.New(t.TempDir(), web)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	body, err := json.Marshal(map[string]string{
		"part":                "catalog",
		"owner":               "platform",
		"interface":           "DataContract",
		"shape":               "v2",
		"configKey":           "replicas",
		"configValue":         "2",
		"decisionName":        "keepShape",
		"decisionAbout":       "DataContract",
		"decisionChoice":      "v2",
		"decisionAlternative": "v3",
		"decisionRationale":   "consumers cannot migrate",
		"sensorName":          "gitSensor",
		"sensorObserves":      "git",
		"sensorAim":           "repos/platform",
		"sensorOutside":       "commit message bodies",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(ts.URL+"/api/goal", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var stored model.Goal
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Parts) != 1 || stored.Parts[0].Name != "catalog" {
		t.Fatalf("http goal parts = %+v", stored.Parts)
	}
	if len(stored.Sensors) != 1 || stored.Sensors[0].Observes != "git" || stored.Sensors[0].Aim != "repos/platform" {
		t.Fatalf("http sensor = %+v", stored.Sensors)
	}
	if len(stored.Decisions) != 1 || stored.Decisions[0].Rationale != "consumers cannot migrate" {
		t.Fatalf("http decision = %+v", stored.Decisions)
	}
}

func assertGoal(t *testing.T, got, want model.Goal) {
	t.Helper()
	if !sameParts(got.Parts, want.Parts) {
		t.Errorf("parts = %+v, want %+v", got.Parts, want.Parts)
	}
	if len(got.Interfaces) != len(want.Interfaces) || got.Interfaces[0].Name != "DataContract" || got.Interfaces[0].Shape != "v2" {
		t.Errorf("interfaces = %+v", got.Interfaces)
	}
	if len(got.Places) != 1 || got.Places[0].Part != "ingest" || got.Places[0].Port != "input" || got.Places[0].Interface != "DataContract" {
		t.Errorf("places = %+v", got.Places)
	}
	foundConfig := false
	for _, cfg := range got.Configurations {
		if cfg.Part == "catalog" && cfg.Key == "replicas" && cfg.Value == "2" {
			foundConfig = true
		}
	}
	if !foundConfig {
		t.Errorf("configurations = %+v", got.Configurations)
	}
	owners := map[string]string{}
	for _, owner := range got.Owners {
		owners[owner.Part] = owner.Owner
	}
	if owners["catalog"] != "platform" || owners["ingest"] != "data" {
		t.Errorf("owners = %+v", got.Owners)
	}
	if len(got.Decisions) != 1 {
		t.Fatalf("decisions = %+v", got.Decisions)
	}
	decision := got.Decisions[0]
	if decision.Name != "keepShape" || decision.About != "DataContract" || decision.Choice != "v2" || decision.Alternative != "v3" || decision.Rationale != "consumers cannot migrate" || decision.Status != "rejected" {
		t.Errorf("decision = %+v", decision)
	}
	if len(got.Sensors) != 1 {
		t.Fatalf("sensors = %+v", got.Sensors)
	}
	sensor := got.Sensors[0]
	if sensor.Name != "gitSensor" || sensor.Observes != "git" || sensor.Aim != "repos/platform" || sensor.Outside != "commit message bodies" {
		t.Errorf("sensor = %+v", sensor)
	}
}

func sameParts(got, want []model.Part) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]string{}
	for _, part := range got {
		seen[part.Name] = part.Def
	}
	for _, part := range want {
		if seen[part.Name] != part.Def {
			return false
		}
	}
	return true
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
