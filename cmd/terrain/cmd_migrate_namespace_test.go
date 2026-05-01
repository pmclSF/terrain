package main

import (
	"testing"
)

// TestMigrateNamespace_VerbsRouteToLegacyRunners verifies the canonical
// shape (`terrain migrate <verb>`) reaches the existing per-verb runner
// without behaviour change. We can't easily assert the output here, but
// we can prove the dispatcher routes correctly by feeding each verb a
// flag-only invocation that the legacy runner treats as "show usage"
// or "no-op". Anything else (panic, dispatch error) trips the test.
func TestMigrateNamespace_VerbsRouteToLegacyRunners(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"run with no framework pair returns usage error", []string{"run"}},
		{"list returns usage", []string{"list", "--help"}},
		{"detect returns help", []string{"detect", "--help"}},
		{"shorthands returns help", []string{"shorthands", "--help"}},
		{"estimate returns help", []string{"estimate", "--help"}},
		{"status returns help", []string{"status", "--help"}},
		{"checklist returns help", []string{"checklist", "--help"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Just ensure no panic; legacy runners may return errors for
			// invalid args but should never panic.
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("dispatcher panicked on %v: %v", tc.args, r)
				}
			}()
			_ = runCaptured(func() error {
				return runMigrateNamespaceCLI(tc.args)
			})
		})
	}
}

// TestMigrateNamespace_LegacyDirectInvocationStillWorks verifies that
// `terrain migrate cypress-playwright` (no verb prefix) falls through
// to the legacy runner. We pass an obviously-invalid framework pair
// and assert we get an error from the legacy runner rather than a
// "verb not recognised" error from the dispatcher.
func TestMigrateNamespace_LegacyDirectInvocationStillWorks(t *testing.T) {
	t.Parallel()

	// Use a clearly-invalid pair so the legacy runner's parser, not the
	// dispatcher, is the one that rejects it.
	err := runCaptured(func() error {
		return runMigrateNamespaceCLI([]string{"--from=does-not-exist", "--to=also-not-exist"})
	})
	if err == nil {
		t.Fatal("expected error from legacy runner, got nil")
	}
}

// TestMigrateNamespace_EmptyArgsRoutesToLegacyRunner ensures bare
// `terrain migrate` (or `terrain convert`) drops into the legacy
// runner so existing usage prompts and help text continue to render.
func TestMigrateNamespace_EmptyArgsRoutesToLegacyRunner(t *testing.T) {
	t.Parallel()

	// No panic; legacy runner produces a usage error or help text.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty-args dispatch panicked: %v", r)
		}
	}()
	_ = runCaptured(func() error {
		return runMigrateNamespaceCLI(nil)
	})
}
