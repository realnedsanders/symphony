package sensor_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/realnedsanders/symphony/src/internal/diff"
	"github.com/realnedsanders/symphony/src/internal/gitx"
	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/sensor"
)

func TestSensors(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	staleBefore := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	root := t.TempDir()

	platform := filepath.Join(root, "platform")
	billing := filepath.Join(root, "billing")
	archive := filepath.Join(root, "archive")
	unborn := filepath.Join(root, "unborn")
	commitRepo(t, platform, "2026-01-15T00:00:00Z", "subject=catalog\nreplicas=1\n")
	commitRepo(t, billing, "2026-02-01T00:00:00Z", "subject=billing\nrole=source\n")
	commitRepo(t, archive, "2020-01-01T00:00:00Z", "subject=archive\nreplicas=9\n")
	if err := gitx.EnsureRepo(unborn); err != nil {
		t.Fatal(err)
	}

	platformReading, err := sensor.ObserveGit(platform, now, staleBefore)
	if err != nil {
		t.Fatal(err)
	}
	billingReading, err := sensor.ObserveGit(billing, now, staleBefore)
	if err != nil {
		t.Fatal(err)
	}
	archiveReading, err := sensor.ObserveGit(archive, now, staleBefore)
	if err != nil {
		t.Fatal(err)
	}
	silentReading, err := sensor.ObserveGit(unborn, now, staleBefore)
	if err != nil {
		t.Fatal(err)
	}

	if platformReading.Subject != "catalog" || platformReading.Observation["replicas"] != "1" {
		t.Fatalf("platform reading = %+v", platformReading)
	}
	if !platformReading.Timestamp.Equal(now) || !platformReading.Covers("replicas") || !platformReading.Covers("head") {
		t.Fatalf("platform stamp/coverage = %+v", platformReading)
	}
	if billingReading.Freshness != model.Fresh || billingReading.Observation["head"] == "" || !billingReading.Timestamp.Equal(now) {
		t.Fatalf("billing reading = %+v", billingReading)
	}
	if archiveReading.Freshness != model.Stale || archiveReading.Observation["replicas"] != "9" || len(archiveReading.Coverage) == 0 || !archiveReading.Timestamp.Equal(now) {
		t.Fatalf("archive reading = %+v", archiveReading)
	}
	if silentReading.Freshness != model.Silent || len(silentReading.Coverage) != 0 || len(silentReading.Observation) != 0 || !silentReading.Timestamp.Equal(now) {
		t.Fatalf("silent reading = %+v", silentReading)
	}

	manifestDir := filepath.Join(root, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: catalog\n  namespace: data\nspec:\n  replicas: 1\n  template:\n    spec:\n      containers:\n        - name: catalog\n          image: catalog:1\n")
	if err := os.WriteFile(filepath.Join(manifestDir, "catalog.yaml"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestReadings, err := sensor.ObserveManifests(manifestDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifestReadings) != 1 || manifestReadings[0].Observation["replicas"] != "1" || manifestReadings[0].Observation["image"] != "catalog:1" || !manifestReadings[0].Covers("replicas") || !manifestReadings[0].Timestamp.Equal(now) {
		t.Fatalf("manifest = %+v", manifestReadings)
	}

	zarfPath := filepath.Join(root, "zarf.yaml")
	zarf := []byte("kind: ZarfPackageConfig\nmetadata:\n  name: platform-bundle\ncomponents:\n  - name: catalog\n    manifests:\n      - name: catalog\n        files:\n          - manifests/catalog.yaml\n")
	if err := os.WriteFile(zarfPath, zarf, 0o644); err != nil {
		t.Fatal(err)
	}
	zarfReading, err := sensor.ObserveZarf(zarfPath, now)
	if err != nil {
		t.Fatal(err)
	}
	if zarfReading.Observation["name"] != "platform-bundle" || zarfReading.Observation["components"] != "catalog" || !zarfReading.Covers("manifests") || !zarfReading.Timestamp.Equal(now) {
		t.Fatalf("zarf = %+v", zarfReading)
	}

	snapshot := []byte(`{"items":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"ingest","labels":{"symphony.io/subject":"ingest.input"}},"data":{"shape":"v1"}}]}`)
	objects, err := sensor.DecodeResources(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	clusterReadings := sensor.ObserveCluster(objects, now)
	if len(clusterReadings) != 1 || clusterReadings[0].Subject != "ingest.input" || clusterReadings[0].Observation["shape"] != "v1" || !clusterReadings[0].Covers("shape") || !clusterReadings[0].Timestamp.Equal(now) || clusterReadings[0].Source != "cluster" {
		t.Fatalf("cluster = %+v", clusterReadings)
	}

	goal := model.Goal{
		Parts: []model.Part{{Name: "catalog", Def: "Component"}, {Name: "archive", Def: "Component"}},
		Configurations: []model.Configuration{
			{Part: "catalog", Key: "replicas", Value: "2"},
			{Part: "archive", Key: "replicas", Value: "1"},
		},
	}
	report := diff.Compare(goal, []model.Reading{platformReading, billingReading, archiveReading, silentReading}, nil)
	if len(report.Drifts) != 1 || report.Drifts[0].Subject != "catalog" || report.Drifts[0].Observed != "1" || report.Drifts[0].Goal != "2" {
		t.Fatalf("drifts = %+v", report.Drifts)
	}
	if !contains(report.Stale, archiveReading.ID) || contains(report.Stale, platformReading.ID) {
		t.Fatalf("stale = %v", report.Stale)
	}
	if !contains(report.Silent, silentReading.ID) {
		t.Fatalf("silent = %v", report.Silent)
	}
	for _, drift := range report.Drifts {
		if drift.ReadingID == archiveReading.ID || drift.ReadingID == silentReading.ID {
			t.Fatalf("stale or silent reading classified as drift: %+v", drift)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func commitRepo(t *testing.T, dir, date, observe string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "symphony.observe"), []byte(observe), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitx.EnsureRepo(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.Run(dir, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "commit", "-m", "observe")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Symphony",
		"GIT_AUTHOR_EMAIL=symphony@local",
		"GIT_COMMITTER_NAME=Symphony",
		"GIT_COMMITTER_EMAIL=symphony@local",
		"GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_DATE="+date,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %s: %v", out, err)
	}
}
