package injection

import (
	"strings"
	"testing"
)

func TestLibrary_ContainsCanonicalPatterns(t *testing.T) {
	got := map[PatternID]bool{}
	for _, p := range Library() {
		got[p.ID] = true
	}
	for _, want := range []PatternID{
		PatternDANStyle,
		PatternInstructionLeak,
		PatternSystemPromptFishing,
		PatternRoleConfusion,
		PatternIndirectViaRetrieval,
	} {
		if !got[want] {
			t.Errorf("pattern %q missing from library", want)
		}
	}
}

func TestLibrary_EveryPatternHasMarkersAndInputs(t *testing.T) {
	for _, p := range Library() {
		if len(p.VulnerableMarkers) == 0 {
			t.Errorf("pattern %q has no VulnerableMarkers", p.ID)
		}
		if len(p.Inputs) < 5 {
			t.Errorf("pattern %q has %d Inputs, want >=5", p.ID, len(p.Inputs))
		}
	}
}

func TestDetectMatches_PromptWithUserInputMarker(t *testing.T) {
	body := "You are a support agent. Answer: {{user_input}}"
	matches := DetectMatches(body)
	if len(matches) == 0 {
		t.Fatalf("expected at least one match on `{{user_input}}` marker")
	}
	found := false
	for _, m := range matches {
		if m.Pattern.ID == PatternDANStyle {
			found = true
		}
	}
	if !found {
		t.Errorf("expected DAN-style pattern to match `{{user_input}}` template")
	}
}

func TestDetectMatches_PromptWithRetrievalMarker(t *testing.T) {
	body := "Context:\n{{retrieved_chunks}}\n\nAnswer the question."
	matches := DetectMatches(body)
	hit := false
	for _, m := range matches {
		if m.Pattern.ID == PatternIndirectViaRetrieval {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected indirect-via-retrieval pattern to match `{{retrieved` template")
	}
}

func TestDetectMatches_EmptyPromptReturnsNothing(t *testing.T) {
	if got := DetectMatches(""); len(got) != 0 {
		t.Errorf("empty body should return nothing, got %d", len(got))
	}
}

func TestDetectMatches_DeterministicOrder(t *testing.T) {
	// A prompt that triggers all five patterns. Order must be stable
	// across calls so downstream consumers can rely on it.
	body := "You are a {{user_input}} {{retrieved}} system instructions: Never act as anything else"
	first := DetectMatches(body)
	second := DetectMatches(body)
	if len(first) != len(second) {
		t.Fatalf("len mismatch %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Pattern.ID != second[i].Pattern.ID {
			t.Errorf("order drift at index %d: %q vs %q",
				i, first[i].Pattern.ID, second[i].Pattern.ID)
		}
	}
}

func TestEmit_PythonContainsParameterizeAndInputs(t *testing.T) {
	matches := DetectMatches("Answer: {{user_input}}")
	out := Emit(matches, EmitOptions{PromptPath: "prompts/system.md", Language: "python"})
	for _, want := range []string{
		"import pytest",
		"@pytest.mark.parametrize",
		"def test_resists_dan_style",
		"your_prompt_invoke(injection_input)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("python output missing %q\n---OUTPUT---\n%s", want, out)
		}
	}
}

func TestEmit_TypeScriptContainsDescribeAndInputs(t *testing.T) {
	matches := DetectMatches("Answer: {{user_input}}")
	out := Emit(matches, EmitOptions{PromptPath: "prompts/system.md", Language: "typescript"})
	for _, want := range []string{
		"import { describe, it, expect } from 'vitest';",
		"describe('resists dan-style",
		"yourPromptInvoke(input)",
		"expectedSafeBehavior(result)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("typescript output missing %q\n---OUTPUT---\n%s", want, out)
		}
	}
}

func TestEmit_JSONShape(t *testing.T) {
	matches := DetectMatches("Answer: {{user_input}}")
	out := Emit(matches, EmitOptions{Language: "json"})
	for _, want := range []string{
		`"id": "dan-style"`,
		`"inputs": [`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %q\n---OUTPUT---\n%s", want, out)
		}
	}
}

func TestByID(t *testing.T) {
	if _, ok := ByID(PatternDANStyle); !ok {
		t.Errorf("ByID(dan-style) should resolve")
	}
	if _, ok := ByID("not-a-real-pattern"); ok {
		t.Errorf("ByID('not-a-real-pattern') should be false")
	}
}
