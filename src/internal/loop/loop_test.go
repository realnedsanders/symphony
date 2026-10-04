package loop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/realnedsanders/symphony/src/internal/loop"
)

func TestLoopTwice(t *testing.T) {
	fixture := filepath.Join(moduleRoot(t), "testdata", "loop")
	first, err := loop.Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loop.Run(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("runs differ\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	for _, needle := range []string{
		"analytics no observation",
		"catalog replicas goal=2 observed=1",
		"DataContract shape=v2",
		"ingest.input observed=v1 old=true proposals=propIngest",
		"gitSensor observes goal=git,manifest,zarf,cluster observed=git",
	} {
		if !strings.Contains(first, needle) {
			t.Errorf("diff missing %q\n%s", needle, first)
		}
	}
	if strings.TrimSpace(first) == "" {
		t.Fatal("empty diff")
	}
	t.Logf("\n%s", first)
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
