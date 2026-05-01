package main

import (
	"strings"
	"testing"
)

// TestConfigNamespace_FeedbackPrintsURL verifies `terrain config
// feedback` runs without error and renders a URL. Pure side-effect
// command — we just verify it doesn't blow up.
func TestConfigNamespace_FeedbackPrintsURL(t *testing.T) {
	t.Parallel()

	out, err := captureRun(func() error {
		return runConfigNamespaceCLI([]string{"feedback"})
	})
	if err != nil {
		t.Fatalf("config feedback: %v", err)
	}
	if !strings.Contains(string(out), "github.com/pmclSF/terrain") {
		t.Errorf("expected GitHub URL in output, got:\n%s", string(out))
	}
}

// TestConfigNamespace_TelemetryStatusReports verifies `terrain config
// telemetry --status` produces output without error.
func TestConfigNamespace_TelemetryStatusReports(t *testing.T) {
	t.Parallel()

	out, err := captureRun(func() error {
		return runConfigNamespaceCLI([]string{"telemetry", "--status"})
	})
	if err != nil {
		t.Fatalf("config telemetry --status: %v", err)
	}
	if !strings.Contains(string(out), "Telemetry") {
		t.Errorf("expected 'Telemetry' in status output, got:\n%s", string(out))
	}
}

// TestConfigNamespace_UnknownVerbReturnsError verifies an unknown verb
// returns a hard error.
func TestConfigNamespace_UnknownVerbReturnsError(t *testing.T) {
	t.Parallel()

	err := runCaptured(func() error {
		return runConfigNamespaceCLI([]string{"not-a-real-verb"})
	})
	if err == nil {
		t.Fatal("expected error for unknown verb, got nil")
	}
}

// TestConfigNamespace_EmptyArgsReturnsError verifies bare `terrain
// config` returns an error so CI scripts that omit the verb fail
// loudly.
func TestConfigNamespace_EmptyArgsReturnsError(t *testing.T) {
	t.Parallel()

	err := runCaptured(func() error {
		return runConfigNamespaceCLI(nil)
	})
	if err == nil {
		t.Fatal("expected error for missing verb, got nil")
	}
}
