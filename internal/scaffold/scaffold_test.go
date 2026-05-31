package scaffold

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateFromSchema_AllTypes(t *testing.T) {
	schema := []byte(`{
		"properties": {
			"name":   {"type": "string"},
			"age":    {"type": "integer"},
			"score":  {"type": "number"},
			"active": {"type": "boolean"},
			"tags":   {"type": "array"},
			"meta":   {"type": "object"}
		}
	}`)
	cases, err := GenerateFromSchema(schema)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want := map[string]int{
		"name":   8, // empty, whitespace, very long, unicode, sql, xss, path-traversal, null-byte
		"age":    5,
		"score":  5,
		"active": 2,
		"tags":   3,
		"meta":   1,
	}
	counts := map[string]int{}
	for _, c := range cases {
		counts[c.Field]++
	}
	for field, n := range want {
		if counts[field] != n {
			t.Errorf("field %q: got %d cases, want %d", field, counts[field], n)
		}
	}
}

func TestGenerateFromSchema_NullableType(t *testing.T) {
	// Schema fields of the form `type: ["string", "null"]` should be
	// treated as the non-null type. The null value is exercised via
	// the null boundary case if that type were null itself.
	schema := []byte(`{"properties": {"name": {"type": ["string", "null"]}}}`)
	cases, err := GenerateFromSchema(schema)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("expected string boundary cases for nullable string")
	}
	// All cases should be string-shaped boundaries.
	for _, c := range cases {
		if c.Field != "name" {
			t.Errorf("unexpected field %q", c.Field)
		}
	}
}

func TestGenerateFromSchema_Deterministic(t *testing.T) {
	schema := []byte(`{"properties": {"b": {"type": "integer"}, "a": {"type": "string"}}}`)
	first, _ := GenerateFromSchema(schema)
	second, _ := GenerateFromSchema(schema)
	if len(first) != len(second) {
		t.Fatalf("non-deterministic length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Field != second[i].Field || first[i].Why != second[i].Why {
			t.Errorf("non-deterministic order at %d: %+v vs %+v", i, first[i], second[i])
		}
	}
	// First field should be "a" (alphabetical).
	if first[0].Field != "a" {
		t.Errorf("expected first field to be 'a' (alphabetical), got %q", first[0].Field)
	}
}

func TestGenerateFromSchema_InvalidJSON(t *testing.T) {
	_, err := GenerateFromSchema([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestEmit_Python(t *testing.T) {
	cases := []BoundaryCase{
		{"prompt", "", "empty string"},
		{"prompt", "héllo", "unicode"},
	}
	out := Emit(cases, EmitOptions{
		SchemaPath: "schemas/input.json",
		PromptPath: "prompts/main.md",
		Language:   "python",
	})
	if !strings.Contains(out, "import pytest") {
		t.Errorf("python output missing pytest import: %s", out)
	}
	if !strings.Contains(out, "def test_boundary_prompt") {
		t.Errorf("python output missing parametrized test: %s", out)
	}
	if !strings.Contains(out, "schemas/input.json") {
		t.Errorf("python output missing schema header: %s", out)
	}
	if !strings.Contains(out, "your_prompt_invoke") {
		t.Errorf("python output missing adopter placeholder: %s", out)
	}
}

func TestEmit_TypeScript(t *testing.T) {
	cases := []BoundaryCase{{"name", "", "empty string"}}
	out := Emit(cases, EmitOptions{Language: "typescript"})
	if !strings.Contains(out, "import { describe, it, expect } from 'vitest';") {
		t.Errorf("ts output missing vitest import: %s", out)
	}
	if !strings.Contains(out, "yourPromptInvoke") {
		t.Errorf("ts output missing adopter placeholder: %s", out)
	}
}

func TestEmit_JSON(t *testing.T) {
	cases := []BoundaryCase{{"x", 1, "one"}}
	out := Emit(cases, EmitOptions{Language: "json"})
	var parsed struct {
		Cases []BoundaryCase `json:"cases"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json output not parseable: %v\n%s", err, out)
	}
	if len(parsed.Cases) != 1 {
		t.Errorf("expected 1 case, got %d", len(parsed.Cases))
	}
}

func TestEmit_DefaultsToPython(t *testing.T) {
	out := Emit([]BoundaryCase{{"x", "y", "z"}}, EmitOptions{})
	if !strings.Contains(out, "import pytest") {
		t.Errorf("default emit should be python; got: %s", out)
	}
}

func TestSafePythonIdent(t *testing.T) {
	cases := map[string]string{
		"prompt":     "prompt",
		"user-input": "user_input",
		"User.Email": "user_email",
		"2nd_choice": "_2nd_choice",
		"!!!":        "___",
		"":           "field",
	}
	for in, want := range cases {
		if got := safePythonIdent(in); got != want {
			t.Errorf("safePythonIdent(%q): got %q want %q", in, got, want)
		}
	}
}

func TestPythonRepr_BoolAndNil(t *testing.T) {
	cases := map[any]string{
		true:  "True",
		false: "False",
		nil:   "None",
		"abc": `"abc"`,
		42:    "42",
	}
	for in, want := range cases {
		if got := pythonRepr(in); got != want {
			t.Errorf("pythonRepr(%v): got %q want %q", in, got, want)
		}
	}
}
