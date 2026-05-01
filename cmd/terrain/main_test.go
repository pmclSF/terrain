package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/models"
)

func TestFindRecentSnapshots_SingleArchivePlusLatest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archive := filepath.Join(dir, "2026-03-09T10-00-00Z.json")
	latest := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(archive, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := os.WriteFile(latest, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write latest: %v", err)
	}

	gotLatest, gotPrevious, err := findRecentSnapshots(dir)
	if err != nil {
		t.Fatalf("findRecentSnapshots returned error: %v", err)
	}
	if gotLatest != latest {
		t.Fatalf("latest = %q, want %q", gotLatest, latest)
	}
	if gotPrevious != archive {
		t.Fatalf("previous = %q, want %q", gotPrevious, archive)
	}
}

func TestLoadSnapshot_MigratesLegacyFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.json")
	json := `{
  "repository": {
    "name": "repo",
    "snapshotTimestamp": "2026-03-01T00:00:00Z"
  },
  "codeUnits": [
    {"path":"src/auth.js","name":"login","kind":"function","exported":true}
  ]
}`
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatalf("write legacy snapshot: %v", err)
	}

	snap, err := loadSnapshot(path)
	if err != nil {
		t.Fatalf("loadSnapshot returned error: %v", err)
	}
	if snap.SnapshotMeta.SchemaVersion != models.LegacySnapshotSchemaVersion {
		t.Fatalf("schema version = %q, want %q", snap.SnapshotMeta.SchemaVersion, models.LegacySnapshotSchemaVersion)
	}
	if snap.GeneratedAt.IsZero() {
		t.Fatal("expected generatedAt to be backfilled")
	}
	want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !snap.GeneratedAt.Equal(want) {
		t.Fatalf("generatedAt = %s, want %s", snap.GeneratedAt.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if len(snap.CodeUnits) != 1 || snap.CodeUnits[0].UnitID != "src/auth.js:login" {
		t.Fatalf("expected backfilled code unit id, got %+v", snap.CodeUnits)
	}
}

func TestInitLogging_ParsesFlag(t *testing.T) {
	// Not parallel: modifies global logging state.
	initLogging([]string{"analyze", "--log-level=debug", "--root", "."})
	// Should not panic; logger is reconfigured.

	initLogging([]string{"analyze", "--log-level", "quiet"})
	// Should not panic.

	initLogging([]string{"analyze"})
	// No --log-level: keeps default.
}

func TestRunInit_InvalidRoot(t *testing.T) {
	t.Parallel()

	if err := runInit(filepath.Join(t.TempDir(), "missing"), false); err == nil {
		t.Fatal("expected error for missing root")
	}
}

// --- AI command tests ---

func TestIsEvalPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"eval/safety.yaml", true},
		{"evals/accuracy.py", true},
		{"evaluations/suite.js", true},
		{"__evals__/prompt_test.py", true},
		{"benchmarks/speed.go", true},
		{"src/eval/runner.ts", true},
		{"test/auth.test.js", false},
		{"src/utils/helper.ts", false},
		{"evaluate.py", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isEvalPath(tt.path)
		if got != tt.want {
			t.Errorf("isEvalPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestRunAI_CommandsRequireScenarioContext(t *testing.T) {
	t.Parallel()
	type subTest struct {
		name string
		fn   func() error
	}
	subs := []subTest{
		{"run", func() error { return runAIRun(".", false, "", false, false) }},
		{"record", func() error { return runAIRecord(".", false) }},
		{"baseline", func() error { return runAIBaseline(".", false) }},
	}
	for _, sub := range subs {
		// runCaptured serializes via captureRunMu so direct calls
		// don't race against other parallel tests that swap os.Stdout.
		if err := runCaptured(sub.fn); err == nil {
			t.Errorf("terrain ai %s should fail without runnable scenario context", sub.name)
		}
	}
}

func TestRunAIList_EmptyRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Empty directory — should produce zero counts without error.
	// Use captureRun to avoid racing on os.Stdout with other parallel tests.
	_, err := captureRun(func() error {
		return runAIList(root, false, false)
	})
	if err != nil {
		t.Fatalf("runAIList on empty dir: %v", err)
	}
}

func TestRunAIDoctor_EmptyRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Use captureRun to avoid racing on os.Stdout with other parallel tests.
	_, err := captureRun(func() error {
		return runAIDoctor(root, false)
	})
	if err != nil {
		t.Fatalf("runAIDoctor on empty dir: %v", err)
	}
}
