package sysml

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFixtureCollab(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod")
		}
		dir = parent
	}
	nodes, err := ParseFile(context.Background(), filepath.Join(dir, "testdata", "loop", "collab.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	collab := ParseCollab(nodes)
	if len(collab.Proposals) != 1 {
		t.Fatalf("proposals = %+v", collab.Proposals)
	}
	proposal := collab.Proposals[0]
	if proposal.State != "open" || len(proposal.Touches) != 1 || proposal.Touches[0] != "ingest.input" {
		t.Fatalf("proposal = %+v", proposal)
	}
}
