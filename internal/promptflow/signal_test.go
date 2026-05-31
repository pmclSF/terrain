package promptflow

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/schemadiff"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestFinding_ToSignal_RemovedField(t *testing.T) {
	f := Finding{
		TemplatePath: "prompts/welcome.md",
		SchemaPath:   "schemas/user.json",
		Risk: Risk{
			Variable: "user_id",
			Change: schemadiff.Change{
				Kind:    schemadiff.ChangeRemoved,
				Field:   "user_id",
				OldType: "string",
			},
		},
		RenderedBefore: "Hi example_string!",
		RenderedAfter:  "Hi MISSING(user_id)!",
	}
	s := f.ToSignal()

	if s.Type != signals.SignalAIPromptSchemaDrift {
		t.Errorf("Type = %v, want %v", s.Type, signals.SignalAIPromptSchemaDrift)
	}
	if s.Category != models.CategoryAI {
		t.Errorf("Category = %v, want %v", s.Category, models.CategoryAI)
	}
	if s.Severity != models.SeverityHigh {
		t.Errorf("Severity = %v, want %v", s.Severity, models.SeverityHigh)
	}
	if s.Location.File != "prompts/welcome.md" {
		t.Errorf("Location.File = %q, want %q", s.Location.File, "prompts/welcome.md")
	}
	if !strings.Contains(s.Explanation, "user_id") {
		t.Errorf("Explanation missing variable name: %q", s.Explanation)
	}
	if !strings.Contains(s.Explanation, "removed") {
		t.Errorf("Explanation should mention removal: %q", s.Explanation)
	}
	if s.Metadata["renderedBefore"] != "Hi example_string!" {
		t.Errorf("Metadata[renderedBefore] = %v", s.Metadata["renderedBefore"])
	}
	if s.Metadata["renderedAfter"] != "Hi MISSING(user_id)!" {
		t.Errorf("Metadata[renderedAfter] = %v", s.Metadata["renderedAfter"])
	}
	if s.Metadata["changeKind"] != "removed" {
		t.Errorf("Metadata[changeKind] = %v", s.Metadata["changeKind"])
	}
}

func TestFinding_ToSignal_TypeChanged(t *testing.T) {
	f := Finding{
		TemplatePath: "prompts/score.md",
		SchemaPath:   "schemas/result.json",
		Risk: Risk{
			Variable: "score",
			Change: schemadiff.Change{
				Kind:    schemadiff.ChangeTypeChanged,
				Field:   "score",
				OldType: "integer",
				NewType: "string",
			},
		},
	}
	s := f.ToSignal()
	if !strings.Contains(s.Explanation, "integer to string") {
		t.Errorf("Explanation should describe type change: %q", s.Explanation)
	}
	if s.Metadata["oldType"] != "integer" || s.Metadata["newType"] != "string" {
		t.Errorf("Metadata types wrong: %v / %v", s.Metadata["oldType"], s.Metadata["newType"])
	}
}

func TestToSignals_ConvertsSlice(t *testing.T) {
	findings := []Finding{
		{TemplatePath: "a.md", SchemaPath: "x.json", Risk: Risk{Variable: "v1", Change: schemadiff.Change{Kind: schemadiff.ChangeRemoved, Field: "v1"}}},
		{TemplatePath: "b.md", SchemaPath: "x.json", Risk: Risk{Variable: "v2", Change: schemadiff.Change{Kind: schemadiff.ChangeRemoved, Field: "v2"}}},
	}
	got := ToSignals(findings)
	if len(got) != 2 {
		t.Fatalf("got %d signals, want 2", len(got))
	}
}
