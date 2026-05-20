package engine

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aliases"
	"github.com/pmclSF/terrain/internal/suppression"
)

func TestExpandSuppressionAliases_NilRegPassesThrough(t *testing.T) {
	in := []suppression.Entry{{SignalType: "aiHardcodedAPIKey"}}
	out := expandSuppressionAliases(in, nil)
	if len(out) != 1 || out[0].SignalType != "aiHardcodedAPIKey" {
		t.Errorf("nil registry should pass through; got %+v", out)
	}
}

func TestExpandSuppressionAliases_NoAliasPassesThrough(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{
		{SignalType: "untestedExport", File: "src/**", Reason: "deprecated module"},
	}
	out := expandSuppressionAliases(in, reg)
	if len(out) != 1 {
		t.Errorf("non-aliased entry should pass through; got %d", len(out))
	}
}

func TestExpandSuppressionAliases_HardcodedAPIKeyExpands(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{
		{SignalType: "aiHardcodedAPIKey", File: "config/**", Reason: "test fixtures"},
	}
	out := expandSuppressionAliases(in, reg)
	if len(out) < 2 {
		t.Fatalf("aiHardcodedAPIKey should expand to ≥2 entries; got %d", len(out))
	}
	seen := map[string]bool{}
	for _, e := range out {
		seen[e.SignalType] = true
		// Reason + File must be preserved on each copy.
		if e.Reason != "test fixtures" {
			t.Errorf("Reason not preserved: %+v", e)
		}
		if e.File != "config/**" {
			t.Errorf("File not preserved: %+v", e)
		}
	}
	for _, want := range []string{
		"aiHardcodedAPIKey",
		"aiHardcodedAPIKey-literal-shape",
		"secretScannerCoverageDegraded",
	} {
		if !seen[want] {
			t.Errorf("expansion missing %q; got %v", want, mapKeys(seen))
		}
	}
}

func TestExpandSuppressionAliases_StaticSkippedTestExpands(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{{SignalType: "staticSkippedTest"}}
	out := expandSuppressionAliases(in, reg)
	if len(out) < 2 {
		t.Fatalf("staticSkippedTest should expand; got %d", len(out))
	}
}

func TestExpandSuppressionAliases_DepsDriftRiskExpands(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{{SignalType: "depsDriftRisk"}}
	out := expandSuppressionAliases(in, reg)
	if len(out) < 2 {
		t.Fatalf("depsDriftRisk should expand; got %d", len(out))
	}
}

func TestExpandSuppressionAliases_EmptySignalTypePassesThrough(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{{FindingID: "abc123", Reason: "FP via exact-match"}}
	out := expandSuppressionAliases(in, reg)
	if len(out) != 1 || out[0].FindingID != "abc123" {
		t.Errorf("FindingID-only entry should pass through; got %+v", out)
	}
}

func TestExpandSuppressionAliases_PreservesOrderAndContent(t *testing.T) {
	reg, _ := aliases.Load()
	in := []suppression.Entry{
		{SignalType: "untestedExport"},
		{SignalType: "aiHardcodedAPIKey"},
		{SignalType: "weakAssertion"},
	}
	out := expandSuppressionAliases(in, reg)
	// Expect untestedExport (1), aiHardcodedAPIKey expansion (3), weakAssertion (1) → 5
	if len(out) < 5 {
		t.Errorf("expected ≥5 after expansion, got %d: %v", len(out), out)
	}
	if out[0].SignalType != "untestedExport" {
		t.Errorf("first entry should be untestedExport (order preserved); got %q", out[0].SignalType)
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
