package promptflow

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E_FixtureRepo exercises the full Discover → Analyze →
// RenderFinding pipeline against a temp directory laid out like a
// real repo with multiple templates and schemas.
func TestE2E_FixtureRepo(t *testing.T) {
	root := t.TempDir()

	// After-state files on disk.
	mustWrite(t, filepath.Join(root, "prompts", "welcome.md"),
		"Hi {{userId}}! Your balance is {{balance}}.")
	mustWrite(t, filepath.Join(root, "prompts", "scoreboard.md"),
		"Score for {{userId}}: {{score}}")
	mustWrite(t, filepath.Join(root, "prompts", "unaffected.md"),
		"This template references {{nothing_changed}}.")
	mustWrite(t, filepath.Join(root, "schemas", "user.json"),
		`{"type": "object", "properties": {"userId": {"type": "string"}, "balance": {"type": "number"}}}`)
	mustWrite(t, filepath.Join(root, "schemas", "result.json"),
		`{"type": "object", "properties": {"userId": {"type": "string"}, "score": {"type": "string"}}}`)
	mustWrite(t, filepath.Join(root, "package.json"),
		`{"name": "demo"}`) // non-schema; must be ignored

	// Before-state schema bodies (the PR renamed user_id → userId and
	// changed score: integer → string).
	before := map[string][]byte{
		"schemas/user.json":   []byte(`{"properties": {"user_id": {"type": "string"}, "balance": {"type": "number"}}}`),
		"schemas/result.json": []byte(`{"properties": {"userId": {"type": "string"}, "score": {"type": "integer"}}}`),
	}

	// Discover the after-state.
	disc, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(disc.Templates) != 3 {
		t.Fatalf("expected 3 templates, got %d: %+v", len(disc.Templates), disc.Templates)
	}
	if len(disc.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d: %+v", len(disc.Schemas), disc.Schemas)
	}

	// Analyze.
	findings, err := Analyze(disc, before)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Expected findings:
	//  - prompts/scoreboard.md references {{score}} → ChangeTypeChanged
	//  - prompts/welcome.md references {{userId}} but userId only got
	//    ADDED in user.json (was user_id before) → no finding on userId
	//    (added fields don't produce risks). HOWEVER, scoreboard
	//    also references {{userId}} on schema result.json where it's
	//    unchanged.
	//
	// Net: 1 finding — scoreboard.md / result.json / score type-changed.
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.TemplatePath != "prompts/scoreboard.md" {
		t.Errorf("TemplatePath = %q", f.TemplatePath)
	}
	if f.Risk.Variable != "score" {
		t.Errorf("Variable = %q", f.Risk.Variable)
	}

	// Render the markdown block.
	md := RenderFinding(f)
	for _, want := range []string{
		"Schema field type changed",
		"prompts/scoreboard.md",
		"schemas/result.json",
		"integer → string",
		"Score for example_string: 42",             // before render
		"Score for example_string: example_string", // after render
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown missing %q\n---OUTPUT---\n%s", want, md)
		}
	}
}
