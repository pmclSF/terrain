package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/aliases"
)

// captureEmit runs emitAliasNotesTo against a local bytes.Buffer and
// returns what would have been written. Avoids the global os.Stderr
// swap that previously raced with other parallel engine tests.
func captureEmit(reg *aliases.Registry, hits map[string]bool) string {
	var buf bytes.Buffer
	emitAliasNotesTo(&buf, reg, hits)
	return buf.String()
}

// testRegistry returns a small in-memory alias registry with three
// entries the tests below exercise.
func testRegistry(t *testing.T) *aliases.Registry {
	t.Helper()
	yaml := []byte(`
version: 1
aliases:
  oldRuleA:
    replaces_with: [newRuleA1, newRuleA2]
    why: "Structurally-motivated split."
  oldRuleB:
    replaces_with: [newRuleB]
  oldRuleC:
    replaces_with: [newRuleC]
    why: "Renamed for clarity."
`)
	reg, err := aliases.LoadFromBytes(yaml)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}
	return reg
}

// TestEmitAliasNotes_NilOrEmpty: the helper must be no-op safe for
// the nil-registry and empty-hits cases. The pipeline calls it
// unconditionally.
func TestEmitAliasNotes_NilOrEmpty(t *testing.T) {
	ResetAliasNotesForTesting()
	if out := captureEmit(nil, map[string]bool{"oldRuleA": true}); out != "" {
		t.Errorf("nil registry produced output: %q", out)
	}
	reg := testRegistry(t)
	if out := captureEmit(reg, nil); out != "" {
		t.Errorf("nil hits produced output: %q", out)
	}
	if out := captureEmit(reg, map[string]bool{}); out != "" {
		t.Errorf("empty hits produced output: %q", out)
	}
}

// TestEmitAliasNotes_TerrainQuietSuppresses confirms the TERRAIN_QUIET=1
// env var silences all NOTEs. Critical for CI use where the migration
// hints would clutter logs.
func TestEmitAliasNotes_TerrainQuietSuppresses(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "1")
	reg := testRegistry(t)
	if out := captureEmit(reg, map[string]bool{"oldRuleA": true}); out != "" {
		t.Errorf("TERRAIN_QUIET=1 still emitted: %q", out)
	}
}

// TestEmitAliasNotes_FirstHitEmits validates the NOTE shape: includes
// the old rule_id, the expanded list, and the alias's why text.
func TestEmitAliasNotes_FirstHitEmits(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "")
	reg := testRegistry(t)
	out := captureEmit(reg, map[string]bool{"oldRuleA": true})
	if !strings.Contains(out, `"oldRuleA" is a deprecated rule_id`) {
		t.Errorf("missing deprecated-rule-id line; got:\n%s", out)
	}
	if !strings.Contains(out, "newRuleA1") || !strings.Contains(out, "newRuleA2") {
		t.Errorf("missing replacement IDs; got:\n%s", out)
	}
	if !strings.Contains(out, "Structurally-motivated split.") {
		t.Errorf("missing why text; got:\n%s", out)
	}
	if !strings.Contains(out, "TERRAIN_QUIET=1") {
		t.Errorf("missing quiet-hint line; got:\n%s", out)
	}
}

// TestEmitAliasNotes_OncePerSession: a second call on the same
// rule_id within the same process must stay silent. The user gets
// one migration hint per session, not one per pipeline invocation.
func TestEmitAliasNotes_OncePerSession(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "")
	reg := testRegistry(t)
	first := captureEmit(reg, map[string]bool{"oldRuleA": true})
	if first == "" {
		t.Fatal("first call produced no output")
	}
	second := captureEmit(reg, map[string]bool{"oldRuleA": true})
	if second != "" {
		t.Errorf("second call on same rule_id emitted: %q", second)
	}
}

// TestEmitAliasNotes_ResetForTestingClearsMemo: after a Reset,
// the same rule_id should emit again. Tests rely on this to
// exercise the emit-once contract in isolation.
func TestEmitAliasNotes_ResetForTestingClearsMemo(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "")
	reg := testRegistry(t)
	first := captureEmit(reg, map[string]bool{"oldRuleB": true})
	if first == "" {
		t.Fatal("first call produced no output")
	}
	ResetAliasNotesForTesting()
	second := captureEmit(reg, map[string]bool{"oldRuleB": true})
	if second == "" {
		t.Error("after Reset, second call still suppressed")
	}
}

// TestEmitAliasNotes_DeterministicOrder: when multiple hits are passed
// in one call, the NOTEs come out in sorted rule_id order so output
// is reproducible across runs (Go map iteration order is randomized).
func TestEmitAliasNotes_DeterministicOrder(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "")
	reg := testRegistry(t)
	out := captureEmit(reg, map[string]bool{
		"oldRuleC": true,
		"oldRuleA": true,
		"oldRuleB": true,
	})
	idxA := strings.Index(out, "oldRuleA")
	idxB := strings.Index(out, "oldRuleB")
	idxC := strings.Index(out, "oldRuleC")
	if !(idxA < idxB && idxB < idxC) {
		t.Errorf("alias NOTEs not in sorted order: A=%d B=%d C=%d\nfull output:\n%s", idxA, idxB, idxC, out)
	}
}

// TestEmitAliasNotes_OnlyKnownIDsEmit: a hit on an old_id the registry
// doesn't know about must be silently ignored, not emit a malformed
// NOTE.
func TestEmitAliasNotes_OnlyKnownIDsEmit(t *testing.T) {
	ResetAliasNotesForTesting()
	t.Setenv("TERRAIN_QUIET", "")
	reg := testRegistry(t)
	if out := captureEmit(reg, map[string]bool{"notInRegistry": true}); out != "" {
		t.Errorf("unknown rule_id emitted: %q", out)
	}
}
