package promptcontract

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestToSignal(t *testing.T) {
	d := Drift{
		PromptPath: "app/prompt.py",
		PromptLine: 12,
		SchemaName: "UserProfile",
		SchemaPath: "app/models.py",
		Variable:   "user_id",
		Kind:       "attribute",
		Message:    `Prompt references user.user_id, but UserProfile (app/models.py) declares no field "user_id"`,
	}
	s := d.ToSignal()

	if s.Type != signals.SignalAIPromptSchemaDrift {
		t.Errorf("Type = %q, want %q", s.Type, signals.SignalAIPromptSchemaDrift)
	}
	if s.Category != models.CategoryAI {
		t.Errorf("Category = %q, want %q", s.Category, models.CategoryAI)
	}
	// Severity must stay High. Drift is emitted at the cmd layer (see
	// appendPromptContractDriftSignals), bypassing the pipeline's
	// evidence-based severity cap — so High passes through verbatim. If drift is
	// ever moved into the engine detector registry it MUST gain an
	// evidence_data.json row, or the no-evidence gate-tier path would silently
	// cap it to Medium and weaken `--fail-on=high`. This assertion is the guard.
	if s.Severity != models.SeverityHigh {
		t.Errorf("Severity = %q, want High (gate detector; see the cap invariant above)", s.Severity)
	}
	if s.Location.File != "app/prompt.py" || s.Location.Line != 12 {
		t.Errorf("Location = %s:%d, want app/prompt.py:12", s.Location.File, s.Location.Line)
	}
	if !strings.Contains(s.Explanation, `declares no field "user_id"`) {
		t.Errorf("Explanation missing the drift detail: %q", s.Explanation)
	}
	if s.SuggestedAction == "" {
		t.Error("SuggestedAction is empty")
	}
	if s.Metadata["variable"] != "user_id" || s.Metadata["schemaName"] != "UserProfile" {
		t.Errorf("Metadata not carried through: %+v", s.Metadata)
	}
}

func TestToSignalsPreservesOrder(t *testing.T) {
	in := []Drift{
		{PromptPath: "a.py", PromptLine: 1, Variable: "x", Message: "m1"},
		{PromptPath: "b.py", PromptLine: 2, Variable: "y", Message: "m2"},
	}
	out := ToSignals(in)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if out[0].Location.File != "a.py" || out[1].Location.File != "b.py" {
		t.Errorf("order not preserved: %s, %s", out[0].Location.File, out[1].Location.File)
	}
}
