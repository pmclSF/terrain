package remediate_test

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/promptcontract"
	"github.com/pmclSF/terrain/internal/remediate"
)

// driftRerun re-runs the diff-free prompt↔schema drift detector on a repo and
// lands its output on canonical findings — the ReRunFunc the closed-loop
// validator drives.
func driftRerun(root string) ([]findings.Finding, error) {
	drift, err := promptcontract.AnalyzeInRepo(root)
	if err != nil {
		return nil, err
	}
	out := make([]findings.Finding, 0, len(drift))
	for _, s := range promptcontract.ToSignals(drift) {
		out = append(out, findings.FromSignal(s, "terrain/ai/prompt-schema-drift"))
	}
	return out, nil
}

// TestPromptSchemaDrift_RemediationClosesTheLoop exercises the full path end to
// end on the real drift detector: a prompt references a field the schema does not
// declare (a typo of an existing field), the correct-side producer rewrites the
// reference to the nearest field, and the closed-loop validator confirms that
// applying the fix and re-running the REAL detector clears the finding with no
// new findings. This passing test is what justifies the DefaultValidityRegistry
// entry for (terrain/ai/prompt-schema-drift, edit_in_place).
func TestPromptSchemaDrift_RemediationClosesTheLoop(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "models.py",
		"from pydantic import BaseModel\n\nclass UserProfile(BaseModel):\n    user_id: str\n    name: str\n")
	writeFile(t, root, "prompt.py",
		"import openai\nfrom models import UserProfile\n\n"+
			"def build(user: UserProfile) -> str:\n"+
			"    return f\"\"\"Hello {user.user_idx}, welcome back.\"\"\"\n")

	before, err := driftRerun(root)
	if err != nil {
		t.Fatalf("initial detect: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected exactly one drift finding, got %d: %+v", len(before), before)
	}

	// Attach the correct-side fix via the producer, as the pipeline does.
	fix := promptcontract.DriftFix(root, before[0])
	if fix == nil {
		t.Fatal("producer declined; expected a nearest-field correction (user_idx -> user_id)")
	}
	if fix.Kind != findings.FixEditInPlace {
		t.Errorf("Fix.Kind = %q, want edit_in_place", fix.Kind)
	}
	before[0].Suggestions = []findings.Suggestion{{Text: "Correct the field reference.", Fix: fix}}

	v, err := remediate.Validate(root, before[0], before, driftRerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !v.Valid {
		t.Errorf("remediation should close the loop; verdict: %s (new findings: %d)", v.Note, len(v.NewFindings))
	}
}

// TestPromptSchemaDrift_DeclinesWhenNoConfidentMatch verifies the producer does
// NOT fabricate a fix when the missing field has no near neighbour (the agno
// `step_input.message` shape: the schema has *_message fields, none within a
// typo's distance). Such a finding must stay judge-only.
func TestPromptSchemaDrift_DeclinesWhenNoConfidentMatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "models.py",
		"from pydantic import BaseModel\n\nclass StepInput(BaseModel):\n"+
			"    confirmation_message: str\n    user_input_message: str\n")
	writeFile(t, root, "prompt.py",
		"import openai\nfrom models import StepInput\n\n"+
			"def build(step_input: StepInput) -> str:\n"+
			"    return f\"\"\"Handling: {step_input.message}\"\"\"\n")

	before, err := driftRerun(root)
	if err != nil {
		t.Fatalf("initial detect: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected one drift finding, got %d", len(before))
	}
	if fix := promptcontract.DriftFix(root, before[0]); fix != nil {
		t.Errorf("producer should decline (no confident match), got fix: %+v", fix)
	}
}
