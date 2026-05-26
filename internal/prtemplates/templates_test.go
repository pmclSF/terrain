package prtemplates

import (
	"strings"
	"testing"
)

// TestLoad_EmbeddedYAML confirms the bundled templates.yaml parses
// cleanly and produces a usable Registry.
func TestLoad_EmbeddedYAML(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatalf("Load embedded: %v", err)
	}
	all := r.All()
	if len(all) == 0 {
		t.Fatal("no templates loaded")
	}
	// At minimum the 11-13 gate-tier detectors should have specimens.
	if len(all) < 11 {
		t.Errorf("expected ≥11 templates, got %d", len(all))
	}
}

// TestLoad_EveryTemplateHasRequiredFields validates the shape of every
// entry in templates.yaml. Missing fields would surface as empty
// fallbacks in the PR comment — the spec is explicit that copy must
// be real, not "TBD".
func TestLoad_EveryTemplateHasRequiredFields(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, tpl := range r.All() {
		if tpl.SignalType == "" {
			t.Errorf("template with empty signal_type: %+v", tpl)
		}
		if tpl.Title == "" {
			t.Errorf("%s: title is empty", tpl.SignalType)
		}
		if tpl.Summary == "" {
			t.Errorf("%s: summary is empty", tpl.SignalType)
		}
		if tpl.Action == "" {
			t.Errorf("%s: action is empty", tpl.SignalType)
		}
		if len(tpl.SlashHints) < 3 {
			t.Errorf("%s: expected at least 3 slash hints (show/explain/dismiss minimum), got %d",
				tpl.SignalType, len(tpl.SlashHints))
		}
	}
}

// TestLoad_EveryTemplateHasDismissHint enforces a contract: every
// template offers the user a /dismiss path. Phase 5e's slash-command
// surface treats /dismiss as the canonical "I've reviewed this" verb.
func TestLoad_EveryTemplateHasDismissHint(t *testing.T) {
	r, _ := Load()
	for _, tpl := range r.All() {
		var found bool
		for _, h := range tpl.SlashHints {
			if strings.HasPrefix(h.Command, "/dismiss") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: missing /dismiss slash hint", tpl.SignalType)
		}
	}
}

// TestLoad_EveryTemplateHasExplainHint enforces that the user can
// always reach the long-form explanation of a rule.
func TestLoad_EveryTemplateHasExplainHint(t *testing.T) {
	r, _ := Load()
	for _, tpl := range r.All() {
		var found bool
		for _, h := range tpl.SlashHints {
			if strings.HasPrefix(h.Command, "/terrain explain") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: missing /terrain explain slash hint", tpl.SignalType)
		}
	}
}

// TestGet_KnownType returns the template; unknown types return false.
func TestGet_KnownType(t *testing.T) {
	r, _ := Load()
	tpl, ok := r.Get("aiPromptInjectionRisk")
	if !ok {
		t.Fatal("expected aiPromptInjectionRisk to be registered")
	}
	if tpl.Title == "" {
		t.Error("expected non-empty title")
	}
	if !strings.Contains(strings.ToLower(tpl.Title), "prompt injection") {
		t.Errorf("title should describe prompt injection; got %q", tpl.Title)
	}
}

func TestGet_UnknownType(t *testing.T) {
	r, _ := Load()
	_, ok := r.Get("nonexistentSignalType")
	if ok {
		t.Error("expected ok=false for unknown signal_type")
	}
}

func TestGet_NilRegistry(t *testing.T) {
	var r *Registry
	_, ok := r.Get("anything")
	if ok {
		t.Error("nil registry should return ok=false")
	}
}

// TestLoadFromBytes_RejectsDuplicateSignalType pins the validator.
func TestLoadFromBytes_RejectsDuplicateSignalType(t *testing.T) {
	yaml := []byte(`
version: 1
templates:
  - signal_type: ruleA
    title: Title A
    summary: Summary A
    action: Action A
    slash_hints:
      - { label: "Show", command: "/terrain show <id>" }
      - { label: "Explain", command: "/terrain explain ruleA" }
      - { label: "Dismiss", command: "/dismiss reason:<x>" }
  - signal_type: ruleA
    title: Title A again
    summary: Summary
    action: Action
    slash_hints:
      - { label: "Show", command: "/terrain show <id>" }
      - { label: "Explain", command: "/terrain explain ruleA" }
      - { label: "Dismiss", command: "/dismiss reason:<x>" }
`)
	_, err := LoadFromBytes(yaml)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsMissingTitle(t *testing.T) {
	yaml := []byte(`
version: 1
templates:
  - signal_type: ruleA
    summary: Summary
    action: Action
    slash_hints:
      - { label: "x", command: "/x" }
`)
	_, err := LoadFromBytes(yaml)
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Errorf("expected title-required error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsBadVersion(t *testing.T) {
	yaml := []byte(`
version: 99
templates: []
`)
	_, err := LoadFromBytes(yaml)
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version error, got %v", err)
	}
}

// TestSignalTypes_Sorted confirms determinism.
func TestSignalTypes_Sorted(t *testing.T) {
	r, _ := Load()
	types := r.SignalTypes()
	for i := 1; i < len(types); i++ {
		if types[i] < types[i-1] {
			t.Errorf("not sorted: %q before %q", types[i-1], types[i])
		}
	}
}

// TestDefault_Singleton confirms Default returns the same registry
// across calls (sync.Once).
func TestDefault_Singleton(t *testing.T) {
	r1, err1 := Default()
	r2, err2 := Default()
	if err1 != nil || err2 != nil {
		t.Fatalf("Default err: %v / %v", err1, err2)
	}
	if r1 != r2 {
		t.Error("Default should return the same registry instance")
	}
}
