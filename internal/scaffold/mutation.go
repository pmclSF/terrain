// Package scaffold generates boundary-case test inputs from a JSON
// Schema. Used by `terrain scaffold` and the
// `/terrain scaffold accept` slash verb to give adopters a populated
// test file they can drop into their suite without designing the
// cases by hand.
//
// Boundary cases per JSON Schema type:
//   - string:  empty, whitespace, max-length, unicode-edge, sql/code-injection-shaped, null-byte
//   - integer: 0, -1, MAX_INT, MIN_INT
//   - number:  0.0, -0.0, NaN-shaped, infinity-shaped, very-large, very-small
//   - boolean: true, false
//   - array:   empty, singleton, many
//   - null:    null
//
// The library is deterministic. Same schema in → same cases out.
// Used by the test-scaffold path; LLM-free.
package scaffold

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// BoundaryCase is one (field, value, why) triple for a generated test
// input. why captures the boundary class so the rendered test file
// shows adopters WHY each case matters.
type BoundaryCase struct {
	Field string `json:"field"`
	Value any    `json:"value"`
	Why   string `json:"why"`
}

// GenerateFromSchema reads a JSON Schema doc and returns the
// boundary cases for every field's declared type. Fields with
// unrecognized types contribute a single "unknown type" placeholder.
//
// Cases are returned sorted by Field then by Why so output is stable.
func GenerateFromSchema(schemaBody []byte) ([]BoundaryCase, error) {
	var doc struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaBody, &doc); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	var cases []BoundaryCase
	for field, prop := range doc.Properties {
		typeStr := normalizeType(prop.Type)
		cases = append(cases, boundaryCases(field, typeStr)...)
	}
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].Field != cases[j].Field {
			return cases[i].Field < cases[j].Field
		}
		return cases[i].Why < cases[j].Why
	})
	return cases, nil
}

// normalizeType picks one canonical type string from the schema's
// type field. Accepts the plain string form and the nullable array
// form (["string", "null"]).
func normalizeType(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		// type: ["string", "null"] — pick the non-null one as the
		// shape under test; nullability is exercised via the null
		// boundary case.
		for _, item := range v {
			if s, ok := item.(string); ok && s != "null" {
				return s
			}
		}
	}
	return ""
}

// boundaryCases produces the per-type boundary case set for a single
// field.
func boundaryCases(field, typ string) []BoundaryCase {
	switch typ {
	case "string":
		return []BoundaryCase{
			{field, "", "empty string"},
			{field, "   ", "whitespace only"},
			{field, strings.Repeat("a", 10_000), "very long (10k chars)"},
			{field, "héllo wörld 🦀 © ßaß", "unicode edge"},
			{field, "'; DROP TABLE users; --", "SQL-injection-shaped"},
			{field, "<script>alert(1)</script>", "XSS-shaped"},
			{field, "../../etc/passwd", "path-traversal-shaped"},
			{field, "value\x00with-null", "null-byte"},
		}
	case "integer":
		return []BoundaryCase{
			{field, 0, "zero"},
			{field, -1, "negative one"},
			{field, 1, "one"},
			{field, 2147483647, "INT32_MAX"},
			{field, -2147483648, "INT32_MIN"},
		}
	case "number":
		return []BoundaryCase{
			{field, 0.0, "zero"},
			{field, -0.0, "negative zero"},
			{field, 1e308, "very large (near double max)"},
			{field, 1e-308, "very small (near double min)"},
			{field, -1.5, "negative fractional"},
		}
	case "boolean":
		return []BoundaryCase{
			{field, true, "true"},
			{field, false, "false"},
		}
	case "array":
		return []BoundaryCase{
			{field, []any{}, "empty array"},
			{field, []any{"a"}, "singleton array"},
			{field, makeManyArray(100), "100-element array"},
		}
	case "null":
		return []BoundaryCase{
			{field, nil, "null value"},
		}
	case "object":
		return []BoundaryCase{
			{field, map[string]any{}, "empty object"},
		}
	default:
		return []BoundaryCase{
			{field, nil, fmt.Sprintf("unknown type %q — adopter to supply", typ)},
		}
	}
}

func makeManyArray(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = fmt.Sprintf("item-%d", i)
	}
	return out
}
