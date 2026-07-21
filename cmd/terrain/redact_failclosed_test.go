package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRedactSourceForRoot_FailsClosedOnInvalidConfig: a malformed terrain.yaml
// must not silently disable source redaction. redactSourceForRoot returns true
// (redact) when the config cannot be parsed, so a config typo never leaks a
// repo's source into an emitted artifact.
func TestRedactSourceForRoot_FailsClosedOnInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Unclosed flow sequence → invalid YAML → LoadForRoot returns an error.
	if err := os.WriteFile(filepath.Join(dir, "terrain.yaml"), []byte("version: 1\nrules: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !redactSourceForRoot(dir) {
		t.Error("redactSourceForRoot must fail closed (return true) on an invalid config")
	}
}

// TestRedactSourceForRoot_NoConfigNoRedaction: with no config present, source
// redaction is off (the opt-in default) — the fail-closed guard must not
// over-redact when there is simply nothing to load.
func TestRedactSourceForRoot_NoConfigNoRedaction(t *testing.T) {
	if redactSourceForRoot(t.TempDir()) {
		t.Error("redactSourceForRoot must be false when no config is present")
	}
}
