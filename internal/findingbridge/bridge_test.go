package findingbridge

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/findings"
)

// TestFromAIPipeline_ScaffoldBecomesStructuredFix is the core of the
// convergence seam: a finding carrying a fix scaffold must land on the
// canonical artifact as a Suggestion with a mechanically-applicable
// new_file Fix — the form the closed-loop validator applies and re-verifies.
func TestFromAIPipeline_ScaffoldBecomesStructuredFix(t *testing.T) {
	t.Parallel()
	in := aipipeline.Finding{
		Path:            "src/handler.ts",
		RuleID:          "ai.surface.missing_eval",
		Cohort:          "rag-app",
		Confidence:      0.82,
		LogOdds:         1.5,
		Severity:        aipipeline.SeverityHigh,
		Atoms:           []aipipeline.EvidenceAtom{{RuleID: "ai.callsite.openai.chat", Span: aipipeline.Span{Line: 12}}},
		FixScaffold:     "description: handler\nprompts:\n  - file://src/handler.ts\n",
		FixScaffoldPath: "evals/promptfoo/handler.yaml",
	}

	out := FromAIPipeline(in, "")

	if err := out.Validate(); err != nil {
		t.Fatalf("converted finding must be valid: %v", err)
	}
	// ruleID normalized from the dotted AI vocabulary.
	if out.RuleID != "terrain/ai/surface-missing-eval" {
		t.Errorf("RuleID = %q, want terrain/ai/surface-missing-eval", out.RuleID)
	}
	if out.Severity != findings.SeverityError {
		t.Errorf("Severity = %q, want error (high)", out.Severity)
	}
	// line recovered from the first atom span.
	if out.PrimaryLoc.Line != 12 {
		t.Errorf("PrimaryLoc.Line = %d, want 12", out.PrimaryLoc.Line)
	}
	if len(out.Suggestions) != 1 {
		t.Fatalf("Suggestions len = %d, want 1", len(out.Suggestions))
	}
	fix := out.Suggestions[0].Fix
	if fix == nil {
		t.Fatal("Suggestion.Fix must be present for a scaffold-bearing finding")
	}
	if fix.Kind != findings.FixNewFile {
		t.Errorf("Fix.Kind = %q, want new_file", fix.Kind)
	}
	if fix.Path != "evals/promptfoo/handler.yaml" {
		t.Errorf("Fix.Path = %q", fix.Path)
	}
	if fix.Content != in.FixScaffold {
		t.Errorf("Fix.Content must carry the scaffold body verbatim")
	}
	// calibration signal preserved.
	if out.Metadata["confidence"] != 0.82 || out.Metadata["cohort"] != "rag-app" {
		t.Errorf("Metadata lost calibration context: %+v", out.Metadata)
	}
}

// TestFromAIPipeline_NoScaffoldNoFix: a diagnostic-only finding (no
// scaffold) converts cleanly with no Suggestion.
func TestFromAIPipeline_NoScaffoldNoFix(t *testing.T) {
	t.Parallel()
	in := aipipeline.Finding{
		Path:     "src/agent.py",
		RuleID:   "ai.train.missing_tracker",
		Severity: aipipeline.SeverityMedium,
	}
	out := FromAIPipeline(in, "")
	if err := out.Validate(); err != nil {
		t.Fatalf("converted finding must be valid: %v", err)
	}
	if out.RuleID != "terrain/ai/train-missing-tracker" {
		t.Errorf("RuleID = %q, want terrain/ai/train-missing-tracker", out.RuleID)
	}
	if out.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want warning (medium)", out.Severity)
	}
	if out.Suggestions != nil {
		t.Errorf("Suggestions = %+v, want nil", out.Suggestions)
	}
}

// TestFromAIPipeline_ExplicitRuleIDWins: a caller-supplied canonical ruleID
// overrides the deterministic normalization.
func TestFromAIPipeline_ExplicitRuleIDWins(t *testing.T) {
	t.Parallel()
	in := aipipeline.Finding{Path: "x.go", RuleID: "ai.surface.missing_eval", Severity: aipipeline.SeverityLow}
	out := FromAIPipeline(in, "terrain/ai/surface-missing-eval-deepeval")
	if out.RuleID != "terrain/ai/surface-missing-eval-deepeval" {
		t.Errorf("RuleID = %q, want the caller-supplied id", out.RuleID)
	}
	if out.Severity != findings.SeverityNotice {
		t.Errorf("Severity = %q, want notice (low)", out.Severity)
	}
}

func TestNormalizeRuleID(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"ai.surface.missing_eval":          "terrain/ai/surface-missing-eval",
		"ai.train.missing_tracker":         "terrain/ai/train-missing-tracker",
		"ai.surface.missing_eval.deepeval": "terrain/ai/surface-missing-eval-deepeval",
		"uncovered_surface":                "terrain/ai/uncovered-surface",
	}
	for in, want := range cases {
		if got := NormalizeRuleID(in); got != want {
			t.Errorf("NormalizeRuleID(%q) = %q, want %q", in, got, want)
		}
	}
}
