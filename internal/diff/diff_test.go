package diff_test

import (
	"testing"
	"time"

	"github.com/realnedsanders/symphony/internal/diff"
	"github.com/realnedsanders/symphony/internal/model"
)

func TestDiffImpactAndContract(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	goal := model.Goal{
		Parts: []model.Part{
			{Name: "catalog", Def: "Component"},
			{Name: "ingest", Def: "Component"},
			{Name: "serve", Def: "Component"},
			{Name: "analytics", Def: "Component"},
		},
		Interfaces: []model.Interface{{Name: "DataContract", Shape: "v2"}},
		Places: []model.Place{
			{Part: "ingest", Port: "input", Interface: "DataContract"},
			{Part: "serve", Port: "input", Interface: "DataContract"},
		},
		Configurations: []model.Configuration{{Part: "catalog", Key: "replicas", Value: "2"}},
		Sensors: []model.SensorContract{{
			Name: "gitSensor", Observes: "git", Aim: "repos", Outside: "bodies",
		}},
	}
	readings := []model.Reading{
		{
			ID: "manifest:catalog", Source: "manifest", Subject: "catalog", Freshness: model.Fresh, Timestamp: now,
			Coverage: []string{"replicas"}, Observation: map[string]string{"replicas": "1"},
		},
		{
			ID: "cluster:ingest", Source: "cluster", Subject: "ingest.input", Freshness: model.Fresh, Timestamp: now,
			Coverage: []string{"shape"}, Observation: map[string]string{"shape": "v1"},
		},
		{
			ID: "cluster:serve", Source: "cluster", Subject: "serve.input", Freshness: model.Fresh, Timestamp: now,
			Coverage: []string{"shape"}, Observation: map[string]string{"shape": "v2"},
		},
		{
			ID: "claim:gitSensor", Source: "claim", Subject: "gitSensor", Freshness: model.Fresh, Timestamp: now,
			Coverage:    []string{"aim", "observes", "outside"},
			Observation: map[string]string{"observes": "git", "aim": "repos", "outside": "bodies"},
		},
		{
			ID: "git:archive", Source: "git", Subject: "archive", Freshness: model.Stale, Timestamp: now,
			Coverage: []string{"replicas"}, Observation: map[string]string{"replicas": "9"},
		},
	}
	proposals := []model.Proposal{{
		ID: "propIngest", State: "open", Touches: []string{"ingest.input"},
	}}

	report := diff.Compare(goal, readings, proposals)
	if len(report.Gaps) != 1 || report.Gaps[0].Subject != "analytics" {
		t.Fatalf("gaps = %+v", report.Gaps)
	}
	if len(report.Drifts) != 1 || report.Drifts[0].Property != "replicas" || report.Drifts[0].Goal != "2" || report.Drifts[0].Observed != "1" {
		t.Fatalf("drifts = %+v", report.Drifts)
	}
	if len(report.Impacts) != 1 || report.Impacts[0].Interface != "DataContract" {
		t.Fatalf("impacts = %+v", report.Impacts)
	}
	places := map[string]model.ImpactPlace{}
	for _, place := range report.Impacts[0].Places {
		places[place.ID] = place
	}
	if len(places) != 2 {
		t.Fatalf("impact places = %+v", report.Impacts[0].Places)
	}
	ingest := places["ingest.input"]
	if !ingest.SeesOldShape || ingest.ObservedShape != "v1" || len(ingest.Proposals) != 1 || ingest.Proposals[0] != "propIngest" {
		t.Fatalf("ingest impact = %+v", ingest)
	}
	serve := places["serve.input"]
	if serve.SeesOldShape || serve.ObservedShape != "v2" || len(serve.Proposals) != 0 {
		t.Fatalf("serve impact = %+v", serve)
	}
	if len(report.ContractChanges) != 0 {
		t.Fatalf("contract changes before edit = %+v", report.ContractChanges)
	}

	goal.Sensors[0].Observes = "git,cluster"
	changed := diff.Compare(goal, readings, proposals)
	if len(changed.ContractChanges) != 1 {
		t.Fatalf("contract changes = %+v", changed.ContractChanges)
	}
	change := changed.ContractChanges[0]
	if change.Sensor != "gitSensor" || change.Field != "observes" || change.Goal != "git,cluster" || change.Observed != "git" {
		t.Fatalf("contract change = %+v", change)
	}
}

func TestNestedReadingDoesNotHidePartConfig(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	goal := model.Goal{
		Parts:          []model.Part{{Name: "ingest", Def: "Component"}},
		Configurations: []model.Configuration{{Part: "ingest", Key: "replicas", Value: "2"}},
	}
	readings := []model.Reading{{
		ID: "cluster:ingest", Source: "cluster", Subject: "ingest.input", Freshness: model.Fresh, Timestamp: now,
		Coverage: []string{"shape"}, Observation: map[string]string{"shape": "v1"},
	}}

	report := diff.Compare(goal, readings, nil)
	var found bool
	for _, gap := range report.Gaps {
		if gap.Subject == "ingest" && gap.Property == "replicas" && gap.Summary == "missing replicas" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps = %+v, want a gap for missing ingest replicas", report.Gaps)
	}
	for _, drift := range report.Drifts {
		if drift.Subject == "ingest" && drift.Property == "replicas" {
			t.Fatalf("replicas drift = %+v, want a gap", drift)
		}
	}
}

func TestFreshContradictionIsNotOverwritten(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	goal := model.Goal{
		Parts:          []model.Part{{Name: "catalog", Def: "Component"}},
		Configurations: []model.Configuration{{Part: "catalog", Key: "replicas", Value: "2"}},
	}
	reading := func(id, replicas string) model.Reading {
		return model.Reading{
			ID: id, Subject: "catalog", Freshness: model.Fresh, Timestamp: now,
			Coverage: []string{"replicas"}, Observation: map[string]string{"replicas": replicas},
		}
	}

	report := diff.Compare(goal, []model.Reading{reading("a:git", "1"), reading("m:manifest", "2")}, nil)
	if len(report.Drifts) != 1 || report.Drifts[0].Observed != "1" || report.Drifts[0].ReadingID != "a:git" || report.Drifts[0].Goal != "2" {
		t.Fatalf("drifts = %+v, want observed=1 reading=a:git", report.Drifts)
	}

	swapped := diff.Compare(goal, []model.Reading{reading("a:git", "2"), reading("m:manifest", "1")}, nil)
	if len(swapped.Drifts) != 1 || swapped.Drifts[0].Observed != "1" || swapped.Drifts[0].ReadingID != "m:manifest" {
		t.Fatalf("swapped drifts = %+v, want observed=1 reading=m:manifest", swapped.Drifts)
	}
}
