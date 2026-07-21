package remediate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

const (
	scaffoldPath = "evals/promptfoo/handler.yaml"
	surfacePath  = "src/handler.ts"
)

// missingEvalFinding is the target: a surface with no eval, carrying a
// new_file Fix that materializes the covering scaffold.
func missingEvalFinding() findings.Finding {
	return findings.Finding{
		Version:      findings.SchemaVersion,
		RuleID:       "terrain/ai/surface-missing-eval",
		Severity:     findings.SeverityWarning,
		PrimaryLoc:   findings.Location{Path: surfacePath, Line: 12},
		ShortMessage: "surface has no eval",
		DocsURL:      "docs/rules/ai/surface-missing-eval.md",
		Suggestions: []findings.Suggestion{{
			Text: "Create the eval",
			Fix:  &findings.Fix{Kind: findings.FixNewFile, Path: scaffoldPath, Content: "prompts:\n  - file://" + surfacePath + "\n"},
		}},
	}
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

// rerunClosesOnScaffold models the real rule: the finding is present iff the
// scaffold file is absent. Applying the fix therefore clears it.
func rerunClosesOnScaffold(root string) ([]findings.Finding, error) {
	if exists(root, scaffoldPath) {
		return nil, nil
	}
	return []findings.Finding{missingEvalFinding()}, nil
}

// TestValidate_ClosesTheLoop: the valid path — fix clears the finding and
// introduces nothing new.
func TestValidate_ClosesTheLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := missingEvalFinding()
	before := []findings.Finding{target}

	v, err := Validate(root, target, before, rerunClosesOnScaffold)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !v.Valid {
		t.Errorf("Valid = false, want true (note: %s)", v.Note)
	}
	if !v.Cleared {
		t.Error("Cleared = false, want true")
	}
	if len(v.NewFindings) != 0 {
		t.Errorf("NewFindings = %v, want none", v.NewFindings)
	}
	// Non-destructive: the file the fix created must be reverted.
	if exists(root, scaffoldPath) {
		t.Error("scaffold file left on disk; Validate must revert its changes")
	}
}

// TestValidate_FindingDoesNotClear: a remediation that doesn't actually
// resolve the finding is not valid.
func TestValidate_FindingDoesNotClear(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := missingEvalFinding()
	before := []findings.Finding{target}

	// Re-run ignores the scaffold — the finding persists regardless.
	rerun := func(string) ([]findings.Finding, error) {
		return []findings.Finding{missingEvalFinding()}, nil
	}
	v, err := Validate(root, target, before, rerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if v.Valid || v.Cleared {
		t.Errorf("expected invalid/uncleared, got %+v", v)
	}
}

// TestValidate_RemediationRegresses: a fix that clears the target but
// introduces a NEW finding is invalid — this is the regression guard that
// keeps Terrain from recommending fixes that break something else.
func TestValidate_RemediationRegresses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := missingEvalFinding()
	before := []findings.Finding{target}

	regression := findings.Finding{
		Version: findings.SchemaVersion, RuleID: "terrain/quality/orphaned-test",
		Severity: findings.SeverityWarning, PrimaryLoc: findings.Location{Path: scaffoldPath, Line: 1},
		ShortMessage: "scaffold is an orphaned test", DocsURL: "docs/rules/quality/orphaned-test.md",
	}
	// After the scaffold lands the target clears, but the scaffold itself
	// trips a different rule.
	rerun := func(r string) ([]findings.Finding, error) {
		if exists(r, scaffoldPath) {
			return []findings.Finding{regression}, nil
		}
		return []findings.Finding{missingEvalFinding()}, nil
	}
	v, err := Validate(root, target, before, rerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if v.Valid {
		t.Error("Valid = true, want false (a regression was introduced)")
	}
	if !v.Cleared {
		t.Error("Cleared = false, want true (target did clear)")
	}
	if len(v.NewFindings) != 1 || v.NewFindings[0].RuleID != "terrain/quality/orphaned-test" {
		t.Errorf("NewFindings = %+v, want the orphaned-test regression", v.NewFindings)
	}
}

// TestValidate_JudgeOnly: a finding with no applicable Fix is out of scope
// for the closed loop and routes to the judge fallback.
func TestValidate_JudgeOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := findings.Finding{
		Version: findings.SchemaVersion, RuleID: "terrain/quality/untested-export",
		Severity: findings.SeverityWarning, PrimaryLoc: findings.Location{Path: "foo.go"},
		ShortMessage: "no test", DocsURL: "docs/rules/quality/untested-export.md",
		Suggestions: []findings.Suggestion{{Text: "Write a test for Foo"}}, // no Fix
	}
	v, err := Validate(root, target, []findings.Finding{target}, rerunClosesOnScaffold)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if v.Applicable {
		t.Errorf("Applicable = true, want false for a judge-only suggestion")
	}
}

func TestApplyFix_PreExistingFileIsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "evals/promptfoo"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("hand-written eval\n")
	if err := os.WriteFile(filepath.Join(root, scaffoldPath), original, 0o644); err != nil {
		t.Fatal(err)
	}
	revert, _, err := ApplyFix(root, findings.Fix{Kind: findings.FixNewFile, Path: scaffoldPath, Content: "generated"})
	if err != nil {
		t.Fatalf("ApplyFix: %v", err)
	}
	if err := revert(); err != nil {
		t.Fatalf("revert: %v", err)
	}
	// The pre-existing, hand-written file must be untouched by apply+revert.
	got, _ := os.ReadFile(filepath.Join(root, scaffoldPath))
	if string(got) != string(original) {
		t.Errorf("pre-existing file modified: got %q, want %q", got, original)
	}
}

func TestApplyFix_RejectsEscapingPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, bad := range []string{"../escape.yaml", "/etc/passwd", "a/../../b"} {
		if _, _, err := ApplyFix(root, findings.Fix{Kind: findings.FixNewFile, Path: bad}); err == nil {
			t.Errorf("ApplyFix(%q) = nil error, want rejection", bad)
		}
	}
}
