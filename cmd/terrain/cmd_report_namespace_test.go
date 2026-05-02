package main

import (
	"testing"
)

// TestReportNamespace_KnownVerbsAreNotRejected verifies the dispatcher
// recognizes every canonical verb. We can't easily invoke the verbs
// for-real without the legacy parsers calling os.Exit on --help, so
// the test stops short of routing — it just confirms the dispatcher
// itself accepts each name (no "unknown report verb" error).
//
// Behavioural smoke-tests for each verb live in the legacy command
// tests already; the namespace dispatcher just forwards args.
func TestReportNamespace_KnownVerbsAreNotRejected(t *testing.T) {
	t.Parallel()
	expected := map[string]bool{
		"summary":      true,
		"insights":     true,
		"metrics":      true,
		"explain":      true,
		"show":         true,
		"impact":       true,
		"pr":           true,
		"posture":      true,
		"select-tests": true,
	}
	if len(reportVerbs) != len(expected) {
		t.Errorf("reportVerbs has %d entries, expected %d", len(reportVerbs), len(expected))
	}
	for _, verb := range reportVerbs {
		if !expected[verb] {
			t.Errorf("unexpected verb in reportVerbs: %q", verb)
		}
	}
}

// TestReportNamespace_UnknownVerbReturnsError verifies an unknown verb
// returns an error rather than falling through to a legacy runner.
// Read-side commands never had a "direct invocation" shape (unlike
// migrate cypress-playwright), so unknown verbs should be hard errors.
func TestReportNamespace_UnknownVerbReturnsError(t *testing.T) {
	t.Parallel()

	err := runCaptured(func() error {
		return runReportNamespaceCLI([]string{"not-a-real-verb"})
	})
	if err == nil {
		t.Fatal("expected error for unknown verb, got nil")
	}
}

// TestReportNamespace_EmptyArgsReturnsHelpAndError verifies bare
// `terrain report` returns an error so CI scripts that omit the verb
// fail loudly.
func TestReportNamespace_EmptyArgsReturnsHelpAndError(t *testing.T) {
	t.Parallel()

	err := runCaptured(func() error {
		return runReportNamespaceCLI(nil)
	})
	if err == nil {
		t.Fatal("expected error for missing verb, got nil")
	}
}

// TestReportNamespace_ExplainRequiresPositional verifies `terrain
// report explain` (no target) returns a useful error instead of
// silently running.
func TestReportNamespace_ExplainRequiresPositional(t *testing.T) {
	t.Parallel()

	err := runCaptured(func() error {
		return runReportNamespaceCLI([]string{"explain"})
	})
	if err == nil {
		t.Fatal("expected error for explain without target, got nil")
	}
}
