package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/terrainconfig"
)

// TestRunAnalyze_OnTerrainErrorFailOpen proves on_terrain_error controls
// whether an analysis *infrastructure* failure (here a forced 1ns timeout)
// blocks the run. Fail-open must happen ONLY for `pass` — never the default
// or explicit `block` (those are fail-closed, the safe default).
func TestRunAnalyze_OnTerrainErrorFailOpen(t *testing.T) {
	writeRepo := func(onErr string) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".terrain"), 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := "version: 1\n"
		if onErr != "" {
			cfg += "on_terrain_error: " + onErr + "\n"
		}
		if err := os.WriteFile(filepath.Join(root, ".terrain", "terrain.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "app.py"), []byte("def f():\n    return 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	run := func(root string) error {
		return runAnalyze(analyzeRunOpts{
			Root: root, JSONOutput: true, Format: "json", Timeout: time.Nanosecond,
		})
	}

	if err := run(writeRepo("")); err == nil {
		t.Error("default config: a forced timeout should fail the run (fail-closed); got nil")
	}
	if err := run(writeRepo("pass")); err != nil {
		t.Errorf("on_terrain_error=pass: forced timeout should fail open (nil); got: %v", err)
	}
	if err := run(writeRepo("block")); err == nil {
		t.Error("on_terrain_error=block: forced timeout should fail the run; got nil")
	}
}

// TestResolveBaselinePath covers ai.baselines_dir auto-discovery: explicit
// --baseline wins; otherwise {dir}/latest.json is used iff it exists as a
// file; missing / nil-config / directory-shaped cases yield no baseline.
func TestResolveBaselinePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := &terrainconfig.Config{AI: &terrainconfig.AISection{BaselinesDir: ".terrain/baselines"}}

	// baselines_dir set but no latest.json yet → no baseline (must not return
	// a path that doesn't exist).
	if got := resolveBaselinePath(root, "", cfg); got != "" {
		t.Errorf("no latest.json yet: want empty, got %q", got)
	}

	// Create the canonical baseline → it's auto-discovered.
	dir := filepath.Join(root, ".terrain", "baselines")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	latest := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(latest, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveBaselinePath(root, "", cfg); got != latest {
		t.Errorf("with latest.json present: want %q, got %q", latest, got)
	}

	// Explicit --baseline always wins, even when baselines_dir would resolve.
	if got := resolveBaselinePath(root, "explicit.json", cfg); got != "explicit.json" {
		t.Errorf("explicit --baseline must win over baselines_dir; got %q", got)
	}

	// No config → no baseline (nil-safe).
	if got := resolveBaselinePath(root, "", nil); got != "" {
		t.Errorf("nil config: want empty, got %q", got)
	}

	// latest.json existing as a directory must be ignored (it's not a snapshot).
	cfgDir := &terrainconfig.Config{AI: &terrainconfig.AISection{BaselinesDir: "bdir"}}
	if err := os.MkdirAll(filepath.Join(root, "bdir", "latest.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveBaselinePath(root, "", cfgDir); got != "" {
		t.Errorf("latest.json being a directory should be ignored; got %q", got)
	}
}
