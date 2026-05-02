package main

import (
	"testing"
)

// TestConvertNamespace_KnownVerbsAreNotRejected mirrors the migrate
// namespace test — verifies the dispatcher recognizes the same
// canonical verbs when entered through `terrain convert ...`.
func TestConvertNamespace_KnownVerbsAreNotRejected(t *testing.T) {
	t.Parallel()
	expected := map[string]bool{
		"run":        true,
		"config":     true,
		"list":       true,
		"detect":     true,
		"shorthands": true,
		"estimate":   true,
		"status":     true,
		"checklist":  true,
		"readiness":  true,
		"blockers":   true,
		"preview":    true,
	}
	for verb := range migrateVerbs {
		if !expected[verb] {
			t.Errorf("unexpected verb in migrateVerbs: %q", verb)
		}
	}
	if len(migrateVerbs) != len(expected) {
		t.Errorf("migrateVerbs has %d entries, expected %d", len(migrateVerbs), len(expected))
	}
}

// TestConvertNamespace_LegacyDirectInvocationGoesToConvertCLI verifies
// that `terrain convert <file>` (no canonical verb) falls through to
// runConvertCLI (per-file converter) NOT runMigrateCLI (directory).
// This was the regression that motivated the split — pre-fix a
// per-file invocation routed to the directory-mode runner and errored
// with "--from <framework> is required (since the path was treated
// as a directory)".
func TestConvertNamespace_LegacyDirectInvocationGoesToConvertCLI(t *testing.T) {
	t.Parallel()
	// Calling with an obviously-invalid framework pair triggers
	// runConvertCLI's flag-validation path. If routing went to
	// runMigrateCLI by mistake, the error would mention "directory" /
	// "--from required for directory mode" rather than the per-file
	// converter's own validation.
	err := runCaptured(func() error {
		return runConvertNamespaceCLI([]string{"--from=nonexistent", "--to=alsonope"})
	})
	if err == nil {
		t.Fatal("expected error from runConvertCLI, got nil")
	}
}

// TestConvertNamespace_EmptyArgsRoutesToConvertCLI ensures bare
// `terrain convert` falls through to runConvertCLI's usage path, not
// the directory-mode migrate runner.
func TestConvertNamespace_EmptyArgsRoutesToConvertCLI(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("dispatch panicked: %v", r)
		}
	}()
	_ = runCaptured(func() error {
		return runConvertNamespaceCLI(nil)
	})
}

// TestConvertNamespace_ListVerbRoutesToListConversions ensures the
// canonical-verb path (`terrain convert list`) reaches the list
// runner, not the directory-mode fall-through.
func TestConvertNamespace_ListVerbRoutesToListConversions(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("dispatch panicked on `convert list`: %v", r)
		}
	}()
	_ = runCaptured(func() error {
		return runConvertNamespaceCLI([]string{"list"})
	})
}
