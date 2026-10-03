package collab_test

import (
	"context"
	"testing"

	"github.com/realnedsanders/symphony/internal/author"
	"github.com/realnedsanders/symphony/internal/collab"
	"github.com/realnedsanders/symphony/internal/gitx"
	"github.com/realnedsanders/symphony/internal/model"
	"github.com/realnedsanders/symphony/internal/store"
)

func TestGroomTakeAndResolve(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := author.Apply(ctx, dir, model.Goal{
		Parts: []model.Part{{Name: "ingest", Def: "Component"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := collab.AddGap(ctx, dir, model.WorkGap{ID: "gapIngest", Subject: "ingest.input", Summary: "old shape", Priority: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := collab.AddGap(ctx, dir, model.WorkGap{ID: "gapServe", Subject: "serve.input", Summary: "serve shape", Priority: 0}); err != nil {
		t.Fatal(err)
	}
	prioritized, err := collab.Prioritize(ctx, dir, "gapServe", 5)
	if err != nil {
		t.Fatal(err)
	}
	ordered := collab.Ordered(prioritized.Gaps)
	if ordered[0].ID != "gapServe" || ordered[0].Priority != 5 {
		t.Fatalf("priority order = %+v", ordered)
	}
	decomposed, err := collab.Decompose(ctx, dir, "gapServe", []model.WorkGap{
		{ID: "gapServePort", Subject: "serve.input", Summary: "port", Priority: 4},
		{ID: "gapServeClient", Subject: "serve.input", Summary: "client", Priority: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	children := 0
	for _, gap := range decomposed.Gaps {
		if gap.ParentID == "gapServe" {
			children++
		}
	}
	if children != 2 {
		t.Fatalf("children = %+v", decomposed.Gaps)
	}

	first, err := collab.Take(ctx, dir, "gapIngest", "propIngest")
	if err != nil {
		t.Fatal(err)
	}
	second, err := collab.Take(ctx, dir, "gapServe", "propServe")
	if err != nil {
		t.Fatal(err)
	}
	assertWorktree(t, dir, first)
	assertWorktree(t, dir, second)

	recorded, err := collab.RecordConflict(ctx, dir, model.Conflict{
		ID:          "clash",
		ProposalIDs: []string{"propIngest", "propServe"},
		Elements:    []string{"ingest.input", "DataContract"},
		Rationales:  []string{"keep v2"},
		ReadingIDs:  []string{"cluster:ingest"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if recorded.State != "open" || len(recorded.Elements) != 2 || recorded.Elements[0] != "ingest.input" || recorded.Elements[1] != "DataContract" || len(recorded.Rationales) != 1 || recorded.Rationales[0] != "keep v2" || len(recorded.ReadingIDs) != 1 || recorded.ReadingIDs[0] != "cluster:ingest" {
		t.Fatalf("conflict = %+v", recorded)
	}
	resolved, err := collab.Resolve(ctx, dir, "clash", "prefer ingest", "propIngest")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, proposal := range resolved.Proposals {
		states[proposal.ID] = proposal.State
	}
	if states["propIngest"] != "preferred" || states["propServe"] != "deferred" {
		t.Fatalf("states after resolve = %+v", resolved.Proposals)
	}
	var conflict model.Conflict
	for _, existing := range resolved.Conflicts {
		if existing.ID == "clash" {
			conflict = existing
		}
	}
	if conflict.State != "resolved" || conflict.Resolution != "prefer ingest" || conflict.Preferred != "propIngest" {
		t.Fatalf("resolved conflict = %+v", conflict)
	}
	reloaded, err := store.ReadCollab(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Conflicts) != 1 || reloaded.Conflicts[0].Resolution != "prefer ingest" {
		t.Fatalf("reloaded = %+v", reloaded)
	}
}

func assertWorktree(t *testing.T, repo string, proposal model.Proposal) {
	t.Helper()
	if proposal.Worktree == "" || proposal.Worktree == repo {
		t.Fatalf("worktree = %q", proposal.Worktree)
	}
	branch, err := gitx.Run(proposal.Worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if branch != proposal.Branch {
		t.Fatalf("HEAD = %s, want %s", branch, proposal.Branch)
	}
	if _, err := gitx.Run(repo, "rev-parse", "--verify", proposal.Branch); err != nil {
		t.Fatal(err)
	}
}
