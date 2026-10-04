package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntryPointTwice(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "symphony")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", out, err)
	}
	fixture := filepath.Join("..", "..", "..", "testdata", "loop")
	first := executeLoop(t, bin, fixture)
	second := executeLoop(t, bin, fixture)
	if first != second {
		t.Fatalf("runs differ\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	for _, needle := range []string{"gaps:", "drifts:", "impacts:", "analytics", "replicas", "DataContract"} {
		if !strings.Contains(first, needle) {
			t.Fatalf("output missing %q\n%s", needle, first)
		}
	}
}

func executeLoop(t *testing.T, bin, fixture string) string {
	t.Helper()
	cmd := exec.Command(bin, "loop", "-fixture", fixture)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loop: %s: %v", out, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("empty diff")
	}
	return string(out)
}

func TestAuthorCommand(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "symphony")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", out, err)
	}
	workspace := t.TempDir()
	cmd := exec.Command(bin, "author",
		"-workspace", workspace,
		"-part", "catalog",
		"-owner", "platform",
		"-interface", "DataContract",
		"-shape", "v2",
		"-sensor", "gitSensor",
		"-observes", "git",
		"-aim", "repos",
		"-outside", "bodies",
		"-decision", "keepShape",
		"-about", "DataContract",
		"-choice", "v2",
		"-alternative", "v3",
		"-rationale", "consumers cannot migrate",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("author: %s: %v", out, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("author printed no commit")
	}
	if _, err := os.Stat(filepath.Join(workspace, "model", "goal.sysml")); err != nil {
		t.Fatal(err)
	}
}
