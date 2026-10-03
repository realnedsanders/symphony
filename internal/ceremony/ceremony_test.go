package ceremony_test

import (
	"context"
	"testing"
	"time"

	"github.com/realnedsanders/symphony/internal/author"
	"github.com/realnedsanders/symphony/internal/ceremony"
	"github.com/realnedsanders/symphony/internal/collab"
	"github.com/realnedsanders/symphony/internal/model"
	"github.com/realnedsanders/symphony/internal/store"
)

func TestAcceptRejectStandupRetro(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	original := model.Goal{
		Parts:          []model.Part{{Name: "ingest", Def: "Component"}},
		Owners:         []model.Ownership{{Part: "ingest", Owner: "platform"}},
		Configurations: []model.Configuration{{Part: "ingest", Key: "replicas", Value: "1"}},
	}
	if _, err := author.Apply(ctx, dir, original); err != nil {
		t.Fatal(err)
	}
	if _, err := collab.AddGap(ctx, dir, model.WorkGap{ID: "gapReplicas", Subject: "ingest", Summary: "replicas", Priority: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := collab.AddGap(ctx, dir, model.WorkGap{ID: "gapOwner", Subject: "ingest", Summary: "owner", Priority: 1}); err != nil {
		t.Fatal(err)
	}
	accepted, err := collab.Take(ctx, dir, "gapReplicas", "propReplicas")
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := collab.Take(ctx, dir, "gapOwner", "propOwner")
	if err != nil {
		t.Fatal(err)
	}

	acceptedGoal, err := store.ReadGoal(ctx, accepted.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	acceptedGoal.Configurations = []model.Configuration{{Part: "ingest", Key: "replicas", Value: "3"}}
	if _, err := author.Apply(ctx, accepted.Worktree, acceptedGoal); err != nil {
		t.Fatal(err)
	}
	rejectedGoal, err := store.ReadGoal(ctx, rejected.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	rejectedGoal.Owners = []model.Ownership{{Part: "ingest", Owner: "nope"}}
	if _, err := author.Apply(ctx, rejected.Worktree, rejectedGoal); err != nil {
		t.Fatal(err)
	}

	if _, err := ceremony.Accept(ctx, dir, "propReplicas"); err != nil {
		t.Fatal(err)
	}
	if _, err := ceremony.Reject(ctx, dir, "propOwner", "ingest", "owner nope", "out of scope"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.ReadGoal(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var replicas string
	for _, cfg := range reloaded.Configurations {
		if cfg.Part == "ingest" && cfg.Key == "replicas" {
			replicas = cfg.Value
		}
	}
	if replicas != "3" {
		t.Fatalf("replicas = %q, goal = %+v", replicas, reloaded)
	}
	var owner string
	for _, item := range reloaded.Owners {
		if item.Part == "ingest" {
			owner = item.Owner
		}
	}
	if owner != "platform" {
		t.Fatalf("owner = %q, want platform", owner)
	}
	var decision model.Decision
	foundDecision := false
	for _, item := range reloaded.Decisions {
		if item.About == "ingest" && item.Status == "rejected" {
			decision = item
			foundDecision = true
		}
	}
	if !foundDecision || decision.Alternative != "owner nope" || decision.Rationale != "out of scope" {
		t.Fatalf("decision = %+v", reloaded.Decisions)
	}

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reading := model.Reading{
		ID: "manifest:ingest", Subject: "ingest", Freshness: model.Fresh,
		Timestamp: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
	}
	if _, err := ceremony.RecordStandup(ctx, dir, "standupMonday", since, []model.Reading{reading}, []string{"contract review"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ceremony.RecordRetro(ctx, dir, "retroMonday"); err != nil {
		t.Fatal(err)
	}
	records, err := store.ReadCeremonies(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records.Standups) != 1 {
		t.Fatalf("standups = %+v", records.Standups)
	}
	standup := records.Standups[0]
	if len(standup.CurrentMovement) != 1 || standup.CurrentMovement[0] == "" {
		t.Fatalf("movement = %+v", standup.CurrentMovement)
	}
	if len(standup.Blockers) != 1 || standup.Blockers[0] != "contract review" {
		t.Fatalf("blockers = %+v", standup.Blockers)
	}
	if len(records.Retros) != 1 {
		t.Fatalf("retros = %+v", records.Retros)
	}
	retro := records.Retros[0]
	shipped := false
	for _, item := range retro.Shipped {
		if item == "propReplicas" {
			shipped = true
		}
	}
	if !shipped {
		t.Fatalf("shipped = %+v", retro.Shipped)
	}
	rejectedNoted := false
	for _, item := range retro.Rejected {
		if item.Alternative == "owner nope" && item.Rationale == "out of scope" {
			rejectedNoted = true
		}
	}
	if !rejectedNoted {
		t.Fatalf("rejected = %+v", retro.Rejected)
	}
}
