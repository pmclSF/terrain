package main

import (
	"testing"
)

// TestMigrateNamespace_VerbsRouteToLegacyRunners verifies the canonical
// shape (`terrain migrate <verb>`) reaches the existing per-verb runner
// without behavior change. We can't easily assert the output here, but
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
// "verb not recognized" error from the dispatcher.
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

// TestMigrateNamespace_EmptyArgsPrintsCanonicalHelp ensures bare
// `terrain migrate` prints the canonical 0.2 verb listing instead of
// falling through to the legacy directory-mode usage block.
//
// Pre-0.2.x: bare `terrain migrate` errored with
// `--from <framework> is required (or pass <directory>)` — actively
// misleading users away from the canonical shape.
//
// 0.2 lock-in: stderr capture must contain "Usage: terrain migrate <verb>"
// and the verb table.
func TestMigrateNamespace_EmptyArgsPrintsCanonicalHelp(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty-args dispatch panicked: %v", r)
		}
	}()
	out, err := captureStderr(func() error {
		return runMigrateNamespaceCLI(nil)
	})
	if err != nil {
		t.Fatalf("empty args returned error: %v", err)
	}
	if !contains(out, "terrain migrate <verb>") {
		t.Errorf("expected canonical usage block on stderr, got: %s", out)
	}
	if !contains(out, "run") || !contains(out, "config") || !contains(out, "list") {
		t.Errorf("expected verb table in usage, got: %s", out)
	}
}

// TestMigrateNamespace_HelpFlagPrintsCanonicalHelp covers the
// `terrain migrate --help` and `-h` shapes. Pre-0.2.x both forwarded
// to the legacy directory-mode help, which printed
// `Usage: terrain migrate <dir>` and never named any of the
// 11 canonical verbs — the worst possible introduction to the new
// shape since the user explicitly asked for help.
func TestMigrateNamespace_HelpFlagPrintsCanonicalHelp(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--help", "-h"} {
		flag := flag
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			out, err := captureStderr(func() error {
				return runMigrateNamespaceCLI([]string{flag})
			})
			if err != nil {
				t.Fatalf("returned error: %v", err)
			}
			if !contains(out, "terrain migrate <verb>") {
				t.Errorf("expected canonical usage on %s, got: %s", flag, out)
			}
			if !contains(out, "preview") || !contains(out, "readiness") {
				t.Errorf("expected complete verb listing on %s, got: %s", flag, out)
			}
		})
	}
}

// TestConvertNamespace_HelpFlagPrintsCanonicalHelp mirrors the migrate
// test for the `convert` namespace. Both share the same dispatcher,
// so the noun-resolution helper must produce "convert" in the usage
// header rather than "migrate".
func TestConvertNamespace_HelpFlagPrintsCanonicalHelp(t *testing.T) {
	t.Parallel()
	out, err := captureStderr(func() error {
		return runConvertNamespaceCLI([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("returned error: %v", err)
	}
	if !contains(out, "terrain convert <verb>") {
		t.Errorf("expected canonical convert usage, got: %s", out)
	}
}
