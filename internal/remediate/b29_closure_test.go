package remediate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestValidate_PreExistingTargetNotAttributable: a new_file fix whose target
// already exists is a no-op — Terrain did not create it. Even if detection then
// reports the finding "cleared" (e.g. a presence-only check), the remediation
// must NOT be reported valid, because Terrain performed no fix. This closes the
// closure-theater hole where a coincidentally-present file makes an unperformed
// fix look validated — exactly the kind of false "validated" claim that must
// never reach a shared finding.
func TestValidate_PreExistingTargetNotAttributable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// The fix's target already exists in the repo (Terrain did not create it).
	if err := os.WriteFile(filepath.Join(root, "eval.yaml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := findings.Finding{
		RuleID:       "terrain/ai/surface-missing-eval",
		PrimaryLoc:   findings.Location{Path: "surface.py"},
		ShortMessage: "surface has no eval",
		Suggestions: []findings.Suggestion{{
			Text: "add an eval",
			Fix:  &findings.Fix{Kind: findings.FixNewFile, Path: "eval.yaml", Content: "new"},
		}},
	}
	// A rerun that (wrongly) reports the finding cleared — simulating a
	// detector that only checks file presence.
	clearedRerun := func(string) ([]findings.Finding, error) { return nil, nil }

	v, err := Validate(root, target, []findings.Finding{target}, clearedRerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if v.Valid {
		t.Error("a fix over a pre-existing target must never be reported valid (closure theater)")
	}
	if v.Cleared {
		t.Error("Cleared must be false when the fix was not actually applied")
	}

	// The pre-existing file must be untouched.
	if b, _ := os.ReadFile(filepath.Join(root, "eval.yaml")); string(b) != "existing" {
		t.Error("a no-op new_file fix must not modify the pre-existing target")
	}
}

// TestValidate_RealNewFileFixIsValid: the control — a new_file fix that Terrain
// actually creates, which clears the finding with no new findings, IS valid.
func TestValidate_RealNewFileFixIsValid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := findings.Finding{
		RuleID:       "terrain/ai/surface-missing-eval",
		PrimaryLoc:   findings.Location{Path: "surface.py"},
		ShortMessage: "surface has no eval",
		Suggestions: []findings.Suggestion{{
			Text: "add an eval",
			Fix:  &findings.Fix{Kind: findings.FixNewFile, Path: "eval.yaml", Content: "new"},
		}},
	}
	clearedRerun := func(string) ([]findings.Finding, error) { return nil, nil }

	v, err := Validate(root, target, []findings.Finding{target}, clearedRerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !v.Valid {
		t.Errorf("a real, cleared new_file fix must be valid; got note %q", v.Note)
	}
}
