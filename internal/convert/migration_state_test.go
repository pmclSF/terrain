package convert

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationStateManager_InitCreatesCurrentStateFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manager := NewMigrationStateManager(root)
	if err := manager.Init("jest", "vitest", ""); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	statePath := filepath.Join(root, ".terrain", "migration", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	var state migrationState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode state file: %v", err)
	}
	if state.Version != migrationStateVersion {
		t.Fatalf("state version = %d, want %d", state.Version, migrationStateVersion)
	}
	if state.Source != "jest" || state.Target != "vitest" {
		t.Fatalf("state direction = %s -> %s, want jest -> vitest", state.Source, state.Target)
	}
	if len(state.Files) != 0 {
		t.Fatalf("state files = %d, want 0", len(state.Files))
	}
}

func TestMigrationStateManager_LoadsLegacyStatePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacyDir := filepath.Join(root, ".terrain")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	legacy := migrationState{
		Version:   migrationStateVersion,
		StartedAt: "2026-04-04T00:00:00Z",
		UpdatedAt: "2026-04-04T00:00:00Z",
		Source:    "cypress",
		Target:    "playwright",
		Files: map[string]migrationStateFile{
			"auth.cy.js": {Status: "converted", Confidence: 95, FileType: "test"},
		},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "state.json"), data, 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	manager := NewMigrationStateManager(root)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !manager.IsConverted("auth.cy.js") {
		t.Fatal("expected converted entry from legacy state file")
	}
}

func TestMigrationStateManager_ResumeRetryAndRoundTripSemantics(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	manager := NewMigrationStateManager(root)
	if err := manager.Init("jest", "vitest", ""); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	manager.MarkConverted("a.test.js", "converted/a.test.js", "test", 95, 0)
	manager.MarkConverted("b.test.js", "converted/b.test.js", "test", 88, 0)
	manager.MarkFailed("c.test.js", "test", "converted/c.test.js", errors.New("parse error"))
	manager.MarkSkipped("d.png", "asset", "binary file")
	if err := manager.Save(); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	resumed := NewMigrationStateManager(root)
	if err := resumed.Load(); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !resumed.IsConverted("a.test.js") || !resumed.IsConverted("b.test.js") {
		t.Fatal("expected converted files to survive resume")
	}
	if resumed.IsConverted("c.test.js") {
		t.Fatal("failed file should not be marked converted")
	}
	if !resumed.IsFailed("c.test.js") {
		t.Fatal("expected failed file to be marked failed")
	}
	if resumed.IsFailed("d.png") {
		t.Fatal("skipped file should not be marked failed")
	}

	statusBeforeRetry := resumed.Status()
	if statusBeforeRetry.Converted != 2 || statusBeforeRetry.Failed != 1 || statusBeforeRetry.Skipped != 1 || statusBeforeRetry.Total != 4 {
		t.Fatalf("unexpected pre-retry status: %+v", statusBeforeRetry)
	}

	resumed.MarkConverted("c.test.js", "converted/c.test.js", "test", 75, 1)
	if err := resumed.Save(); err != nil {
		t.Fatalf("Save after retry returned error: %v", err)
	}

	reloaded := NewMigrationStateManager(root)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load after retry returned error: %v", err)
	}
	if !reloaded.IsConverted("c.test.js") {
		t.Fatal("expected retried file to be marked converted")
	}
	if reloaded.IsFailed("c.test.js") {
		t.Fatal("retried file should no longer be marked failed")
	}

	statusAfterRetry := reloaded.Status()
	if statusAfterRetry.Converted != 3 || statusAfterRetry.Failed != 0 || statusAfterRetry.Skipped != 1 || statusAfterRetry.Total != 4 {
		t.Fatalf("unexpected post-retry status: %+v", statusAfterRetry)
	}

	records := reloaded.Records()
	if len(records) != 4 {
		t.Fatalf("records = %d, want 4", len(records))
	}
	if records[0].InputPath != "a.test.js" || records[3].InputPath != "d.png" {
		t.Fatalf("records not sorted by path: %+v", records)
	}
}

func TestMigrationStateManager_ResetAndReplayIsIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	buildStatus := func() (MigrationStatus, error) {
		manager := NewMigrationStateManager(root)
		if err := manager.Init("cypress", "playwright", "converted"); err != nil {
			return MigrationStatus{}, err
		}
		manager.MarkConverted("auth.cy.js", "converted/auth.spec.js", "test", 85, 0)
		manager.MarkFailed("broken.cy.js", "test", "converted/broken.spec.js", errors.New("unsupported"))
		manager.MarkSkipped("fixtures/data.json", "fixture", "non-convertible type")
		if err := manager.Save(); err != nil {
			return MigrationStatus{}, err
		}
		return manager.Status(), nil
	}

	first, err := buildStatus()
	if err != nil {
		t.Fatalf("buildStatus first run: %v", err)
	}

	manager := NewMigrationStateManager(root)
	if err := manager.Reset(); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}

	second, err := buildStatus()
	if err != nil {
		t.Fatalf("buildStatus second run: %v", err)
	}

	if first.Total != second.Total ||
		first.Converted != second.Converted ||
		first.Failed != second.Failed ||
		first.Skipped != second.Skipped ||
		first.Source != second.Source ||
		first.Target != second.Target ||
		first.OutputRoot != second.OutputRoot {
		t.Fatalf("status mismatch after replay:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}
