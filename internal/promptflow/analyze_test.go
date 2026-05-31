package promptflow

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/prompttemplate"
	"github.com/pmclSF/terrain/internal/schemadiff"
)

func TestAnalyze_RemovedFieldProducesFinding(t *testing.T) {
	after := Discoveries{
		Templates: []TemplateFile{{
			Path: "prompts/welcome.md",
			Tpl: prompttemplate.Template{
				Kind: prompttemplate.KindMustache,
				Body: "Hi {{user_id}}, your balance is {{balance}}.",
			},
		}},
		Schemas: []SchemaFile{{
			Path: "schemas/user.json",
			Body: []byte(`{"properties": {"balance": {"type": "number"}}}`),
		}},
	}
	before := map[string][]byte{
		"schemas/user.json": []byte(`{"properties": {
			"user_id": {"type": "string"},
			"balance": {"type": "number"}
		}}`),
	}

	findings, err := Analyze(after, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.TemplatePath != "prompts/welcome.md" {
		t.Errorf("TemplatePath = %q", f.TemplatePath)
	}
	if f.SchemaPath != "schemas/user.json" {
		t.Errorf("SchemaPath = %q", f.SchemaPath)
	}
	if f.Risk.Variable != "user_id" {
		t.Errorf("Risk.Variable = %q, want user_id", f.Risk.Variable)
	}
	if f.Risk.Change.Kind != schemadiff.ChangeRemoved {
		t.Errorf("Risk.Change.Kind = %v, want ChangeRemoved", f.Risk.Change.Kind)
	}
	if !strings.Contains(f.RenderedBefore, "Hi example_string,") {
		t.Errorf("RenderedBefore missing synthesized user_id: %q", f.RenderedBefore)
	}
	if !strings.Contains(f.RenderedBefore, "balance is 3.14") {
		t.Errorf("RenderedBefore missing synthesized balance: %q", f.RenderedBefore)
	}
	if !strings.Contains(f.RenderedAfter, "MISSING(user_id)") {
		t.Errorf("RenderedAfter should mark user_id MISSING, got: %q", f.RenderedAfter)
	}
}

func TestAnalyze_TemplateWithoutChangedVarsProducesNoFinding(t *testing.T) {
	after := Discoveries{
		Templates: []TemplateFile{{
			Path: "prompts/calm.md",
			Tpl: prompttemplate.Template{
				Kind: prompttemplate.KindMustache,
				Body: "Greetings {{balance}}",
			},
		}},
		Schemas: []SchemaFile{{
			Path: "schemas/user.json",
			Body: []byte(`{"properties": {"balance": {"type": "number"}}}`),
		}},
	}
	before := map[string][]byte{
		"schemas/user.json": []byte(`{"properties": {
			"user_id": {"type": "string"},
			"balance": {"type": "number"}
		}}`),
	}
	findings, err := Analyze(after, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0: %+v", len(findings), findings)
	}
}

func TestAnalyze_TypeChangedProducesFinding(t *testing.T) {
	after := Discoveries{
		Templates: []TemplateFile{{
			Path: "prompts/score.md",
			Tpl: prompttemplate.Template{
				Kind: prompttemplate.KindMustache,
				Body: "Score: {{score}}",
			},
		}},
		Schemas: []SchemaFile{{
			Path: "schemas/result.json",
			Body: []byte(`{"properties": {"score": {"type": "string"}}}`),
		}},
	}
	before := map[string][]byte{
		"schemas/result.json": []byte(`{"properties": {"score": {"type": "integer"}}}`),
	}
	findings, err := Analyze(after, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Risk.Change.Kind != schemadiff.ChangeTypeChanged {
		t.Errorf("Change.Kind = %v, want ChangeTypeChanged", f.Risk.Change.Kind)
	}
	if !strings.Contains(f.RenderedBefore, "Score: 42") {
		t.Errorf("RenderedBefore wrong type: %q", f.RenderedBefore)
	}
	if !strings.Contains(f.RenderedAfter, "Score: example_string") {
		t.Errorf("RenderedAfter wrong type: %q", f.RenderedAfter)
	}
}

func TestAnalyze_SchemaMissingFromBeforeProducesNoFinding(t *testing.T) {
	after := Discoveries{
		Templates: []TemplateFile{{
			Path: "prompts/welcome.md",
			Tpl: prompttemplate.Template{
				Kind: prompttemplate.KindMustache,
				Body: "Hi {{user_id}}",
			},
		}},
		Schemas: []SchemaFile{{
			Path: "schemas/new.json",
			Body: []byte(`{"properties": {"user_id": {"type": "string"}}}`),
		}},
	}
	before := map[string][]byte{}
	findings, err := Analyze(after, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for brand-new schema, got %d: %+v", len(findings), findings)
	}
}

func TestAnalyze_FindingsSortedByTemplateThenVariable(t *testing.T) {
	after := Discoveries{
		Templates: []TemplateFile{
			{
				Path: "prompts/b.md",
				Tpl: prompttemplate.Template{
					Kind: prompttemplate.KindMustache,
					Body: "{{zebra}} {{apple}}",
				},
			},
			{
				Path: "prompts/a.md",
				Tpl: prompttemplate.Template{
					Kind: prompttemplate.KindMustache,
					Body: "{{user_id}}",
				},
			},
		},
		Schemas: []SchemaFile{{
			Path: "schemas/x.json",
			Body: []byte(`{"properties": {}}`),
		}},
	}
	before := map[string][]byte{
		"schemas/x.json": []byte(`{"properties": {
			"user_id": {"type": "string"},
			"zebra":   {"type": "string"},
			"apple":   {"type": "string"}
		}}`),
	}
	findings, err := Analyze(after, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("got %d findings, want 3: %+v", len(findings), findings)
	}
	wantOrder := []struct{ tpl, varName string }{
		{"prompts/a.md", "user_id"},
		{"prompts/b.md", "apple"},
		{"prompts/b.md", "zebra"},
	}
	for i, w := range wantOrder {
		if findings[i].TemplatePath != w.tpl || findings[i].Risk.Variable != w.varName {
			t.Errorf("findings[%d] = (%q, %q), want (%q, %q)",
				i, findings[i].TemplatePath, findings[i].Risk.Variable, w.tpl, w.varName)
		}
	}
}

func TestRenderFinding_IncludesTitleAndPathsAndRender(t *testing.T) {
	f := Finding{
		TemplatePath:   "prompts/welcome.md",
		SchemaPath:     "schemas/user.json",
		Risk:           Risk{Variable: "user_id", Change: schemadiff.Change{Kind: schemadiff.ChangeRemoved, Field: "user_id", OldType: "string"}},
		RenderedBefore: "Hi example_string!",
		RenderedAfter:  "Hi MISSING(user_id)!",
	}
	got := RenderFinding(f)
	wantSubstrings := []string{
		"prompts/welcome.md",
		"schemas/user.json",
		"user_id",
		"Hi example_string!",
		"Hi MISSING(user_id)!",
		"/dismiss",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(got, s) {
			t.Errorf("RenderFinding output missing %q\n---OUTPUT---\n%s", s, got)
		}
	}
}
