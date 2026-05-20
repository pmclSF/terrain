package engine

import (
	"os"
	"strings"
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

func TestExpandSuppressionAliases_FindingIDOnAliasedDetectorWarnsOnce(t *testing.T) {
	// Capture stderr.
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
	})

	ResetSuppressionFindingIDWarningsForTesting()
	reg, _ := aliases.Load()

	// A FindingID that embeds the deprecated rule_id should trigger a warn.
	in := []suppression.Entry{
		{FindingID: "aiHardcodedAPIKey@config/eval.yaml:line=42#deadbeef", Reason: "old"},
		// Duplicate of the same aliased detector: should NOT re-warn.
		{FindingID: "aiHardcodedAPIKey@other.yaml:line=1#cafebabe", Reason: "old2"},
	}
	out := expandSuppressionAliases(in, reg)
	if len(out) != 2 {
		t.Errorf("FindingID entries should pass through unmodified; got %d", len(out))
	}

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	if !strings.Contains(stderr, "[NOTE]") {
		t.Errorf("expected stderr NOTE; got %q", stderr)
	}
	if !strings.Contains(stderr, "aiHardcodedAPIKey") {
		t.Errorf("NOTE should mention the deprecated rule_id; got %q", stderr)
	}
	// Should fire exactly once for the same aliased detector.
	noteCount := strings.Count(stderr, "[NOTE]")
	if noteCount != 1 {
		t.Errorf("expected 1 NOTE for two same-detector entries (dedup); got %d", noteCount)
	}
}

func TestExpandSuppressionAliases_FindingIDOnNonAliasedDetectorSilent(t *testing.T) {
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	ResetSuppressionFindingIDWarningsForTesting()
	reg, _ := aliases.Load()

	in := []suppression.Entry{
		{FindingID: "untestedExport@src/foo.ts:line=1#abc12345", Reason: "fp"},
	}
	expandSuppressionAliases(in, reg)

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Errorf("non-aliased detector should not emit NOTE; got %q", string(buf[:n]))
	}
}

func TestExpandSuppressionAliases_TerrainQuietSilencesFindingIDWarning(t *testing.T) {
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	t.Setenv("TERRAIN_QUIET", "1")
	ResetSuppressionFindingIDWarningsForTesting()
	reg, _ := aliases.Load()

	in := []suppression.Entry{
		{FindingID: "aiHardcodedAPIKey@x:line=1#abc12345"},
	}
	expandSuppressionAliases(in, reg)

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Errorf("TERRAIN_QUIET should silence NOTE; got %q", string(buf[:n]))
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
