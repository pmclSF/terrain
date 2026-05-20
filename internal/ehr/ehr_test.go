package ehr

import (
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// ── Recognize ───────────────────────────────────────────────────────

func TestRecognize_PromptfooBasic(t *testing.T) {
	body := []byte(`prompts:
  - prompts/customer-support.txt
  - prompts/billing.txt
providers:
  - id: openai:gpt-4o-mini
  - id: anthropic:claude-3-haiku
tests:
  - vars:
      prompt: prompts/customer-support.txt
      dataset: data/cs-cases.csv
`)
	r, err := RecognizeBytes(body, "promptfooconfig.yaml")
	if err != nil {
		t.Fatalf("RecognizeBytes: %v", err)
	}
	if r.Format != "promptfoo" {
		t.Errorf("Format = %q, want promptfoo", r.Format)
	}
	if len(r.SurfacesCovered) < 4 {
		t.Errorf("expected ≥4 surfaces, got %d: %+v", len(r.SurfacesCovered), r.SurfacesCovered)
	}
	want := map[string]SurfaceKind{
		"prompts/customer-support.txt": SurfacePrompt,
		"prompts/billing.txt":          SurfacePrompt,
		"openai:gpt-4o-mini":           SurfaceModel,
		"anthropic:claude-3-haiku":     SurfaceModel,
		"data/cs-cases.csv":            SurfaceDataset,
	}
	for _, s := range r.SurfacesCovered {
		if got := want[s.Value]; got != "" && got != s.Kind {
			t.Errorf("surface %q kind = %q, want %q", s.Value, s.Kind, got)
		}
	}
}

func TestRecognize_DeepevalGeneric(t *testing.T) {
	body := []byte(`model: gpt-4o
datasets:
  - data/sample.jsonl
`)
	r, _ := RecognizeBytes(body, "deepeval.yaml")
	if r.Format != "deepeval" {
		t.Errorf("Format = %q", r.Format)
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "gpt-4o") {
		t.Errorf("expected gpt-4o model surface, got %+v", r.SurfacesCovered)
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceDataset, "data/sample.jsonl") {
		t.Errorf("expected dataset surface, got %+v", r.SurfacesCovered)
	}
}

func TestRecognize_GenericFallback(t *testing.T) {
	body := []byte(`model: claude-3-opus
prompts:
  - templates/answer.txt
`)
	r, _ := RecognizeBytes(body, "custom-eval-config.yaml")
	if r.Format != "generic" {
		t.Errorf("Format = %q, want generic", r.Format)
	}
	if len(r.SurfacesCovered) < 2 {
		t.Errorf("expected ≥2 surfaces, got %d", len(r.SurfacesCovered))
	}
}

func TestRecognize_MalformedYAMLDoesNotPanic(t *testing.T) {
	body := []byte(`this is: not: valid: yaml: ---{}{`)
	r, err := RecognizeBytes(body, "promptfooconfig.yaml")
	if err != nil {
		t.Fatalf("RecognizeBytes should not error on malformed YAML: %v", err)
	}
	if len(r.SurfacesCovered) != 0 {
		t.Errorf("malformed YAML should yield no surfaces, got %d", len(r.SurfacesCovered))
	}
}

func TestRecognize_ProviderShapes(t *testing.T) {
	body := []byte(`providers:
  - bare-string-provider
  - id: with-id
  - name: ignored-name-only
`)
	r, _ := RecognizeBytes(body, "promptfooconfig.yaml")
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "bare-string-provider") {
		t.Errorf("bare string provider not captured")
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "with-id") {
		t.Errorf("id-keyed provider not captured")
	}
}

func TestRecognize_MultiDocYAML(t *testing.T) {
	body := []byte(`prompts:
  - prompts/dev.txt
---
prompts:
  - prompts/prod.txt
model: gpt-4o
`)
	r, err := RecognizeBytes(body, "promptfooconfig.yaml")
	if err != nil {
		t.Fatalf("RecognizeBytes: %v", err)
	}
	if !surfaceContains(r.SurfacesCovered, SurfacePrompt, "prompts/dev.txt") {
		t.Errorf("multi-doc YAML doc 1 prompt missing")
	}
	if !surfaceContains(r.SurfacesCovered, SurfacePrompt, "prompts/prod.txt") {
		t.Errorf("multi-doc YAML doc 2 prompt missing")
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "gpt-4o") {
		t.Errorf("multi-doc YAML doc 2 model missing")
	}
}

func TestRecognize_DeepEvalNestedTestCases(t *testing.T) {
	body := []byte(`test_cases:
  - prompt: prompts/customer.txt
    model: gpt-4o
    input: hello
  - prompt: prompts/billing.txt
    model: claude-3-haiku
`)
	r, err := RecognizeBytes(body, "deepeval.yaml")
	if err != nil {
		t.Fatalf("RecognizeBytes: %v", err)
	}
	if !surfaceContains(r.SurfacesCovered, SurfacePrompt, "prompts/customer.txt") {
		t.Errorf("deepeval test_case prompt missing; got %+v", r.SurfacesCovered)
	}
	if !surfaceContains(r.SurfacesCovered, SurfacePrompt, "prompts/billing.txt") {
		t.Errorf("deepeval second test_case prompt missing")
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "gpt-4o") {
		t.Errorf("deepeval test_case model missing")
	}
	if !surfaceContains(r.SurfacesCovered, SurfaceModel, "claude-3-haiku") {
		t.Errorf("deepeval second test_case model missing")
	}
}

func TestRecognize_DedupsSurfaces(t *testing.T) {
	body := []byte(`prompts: [foo.txt, foo.txt, foo.txt]`)
	r, _ := RecognizeBytes(body, "x.yaml")
	count := 0
	for _, s := range r.SurfacesCovered {
		if s.Value == "foo.txt" && s.Kind == SurfacePrompt {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected dedup to 1 foo.txt, got %d", count)
	}
}

// ── Covers ──────────────────────────────────────────────────────────

func TestCovers_ExactValue(t *testing.T) {
	r := &Report{SurfacesCovered: []Surface{
		{Kind: SurfacePrompt, Value: "prompts/foo.txt"},
	}}
	if !Covers([]*Report{r}, SurfacePrompt, "prompts/foo.txt") {
		t.Errorf("exact match should cover")
	}
}

func TestCovers_PathSuffix(t *testing.T) {
	r := &Report{SurfacesCovered: []Surface{
		{Kind: SurfacePrompt, Value: "prompts/foo.txt"},
	}}
	if !Covers([]*Report{r}, SurfacePrompt, "foo.txt") {
		t.Errorf("suffix match should cover")
	}
}

func TestCovers_KindMismatch(t *testing.T) {
	r := &Report{SurfacesCovered: []Surface{
		{Kind: SurfaceModel, Value: "gpt-4"},
	}}
	if Covers([]*Report{r}, SurfacePrompt, "gpt-4") {
		t.Errorf("kind mismatch should not cover")
	}
}

func TestCovers_ModelExactOnly(t *testing.T) {
	// Models should NOT match by suffix — model names don't have path
	// semantics.
	r := &Report{SurfacesCovered: []Surface{
		{Kind: SurfaceModel, Value: "openai/gpt-4o"},
	}}
	if Covers([]*Report{r}, SurfaceModel, "gpt-4o") {
		t.Errorf("model surfaces should match exactly, not by suffix")
	}
}

// ── GateSuppression ─────────────────────────────────────────────────

func loadReg(t *testing.T, state mechanisms.State) *mechanisms.Registry {
	t.Helper()
	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Override(MechanismName, state); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestGateSuppression_Off_LegacyBehavior(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOff)
	// legacy: any eval config suppresses every finding.
	if got := GateSuppression(reg, nil, SurfacePrompt, "x", "r", "f", true); got != false {
		t.Errorf("state=off + legacyHadConfig=true should NOT keep (legacy behavior)")
	}
	if got := GateSuppression(reg, nil, SurfacePrompt, "x", "r", "f", false); got != true {
		t.Errorf("state=off + no legacy config should keep")
	}
}

func TestGateSuppression_On_OnlySuppressesCoveredSurfaces(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	reports := []*Report{{SurfacesCovered: []Surface{
		{Kind: SurfacePrompt, Value: "prompts/foo.txt"},
	}}}
	// Surface IS covered → suppress (keep=false).
	if got := GateSuppression(reg, reports, SurfacePrompt, "prompts/foo.txt", "r", "f", true); got != false {
		t.Errorf("covered surface should suppress")
	}
	// Surface NOT covered → keep firing (keep=true), even though legacy
	// would have suppressed.
	if got := GateSuppression(reg, reports, SurfacePrompt, "prompts/bar.txt", "r", "f", true); got != true {
		t.Errorf("uncovered surface should keep finding even with eval present")
	}
}

func TestGateSuppression_Shadow_EmitsWouldAdd(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadReg(t, mechanisms.StateShadow)
	reports := []*Report{{SurfacesCovered: []Surface{
		{Kind: SurfacePrompt, Value: "prompts/covered.txt"},
	}}}
	// Legacy suppresses (eval present); per-surface verdict says fire.
	got := GateSuppression(reg, reports, SurfacePrompt, "prompts/uncovered.txt", "promptFileMissingEval", "f.py", true)
	if got != false {
		t.Errorf("shadow should preserve legacy verdict (Keep=false), got Keep=%v", got)
	}
	if len(sink.Events()) != 1 {
		t.Errorf("expected 1 shadow event (would_add), got %d", len(sink.Events()))
	}
	if len(sink.Events()) == 1 && sink.Events()[0].Action != shadow.ActionAdd {
		t.Errorf("event action = %v, want would_add", sink.Events()[0].Action)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func surfaceContains(ss []Surface, kind SurfaceKind, value string) bool {
	for _, s := range ss {
		if s.Kind == kind && s.Value == value {
			return true
		}
	}
	return false
}
