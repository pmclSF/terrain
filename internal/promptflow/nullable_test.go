package promptflow

import "testing"

// TestParsePropertyTypes_NullableUnion: a nullable/union schema type
// (`["string","null"]`) must parse to a concrete type string, not be dropped,
// so the drift detector's before/after preview shows a real value, not MISSING.
func TestParsePropertyTypes_NullableUnion(t *testing.T) {
	t.Parallel()
	body := []byte(`{"properties":{"user_id":{"type":["string","null"]}}}`)
	got := parsePropertyTypes(body)
	if got["user_id"] != "null|string" {
		t.Fatalf("parsePropertyTypes user_id = %q, want null|string", got["user_id"])
	}
}

// TestSynthesize_NullableRendersConcreteValue: a nullable union synthesizes
// from its first non-null member rather than falling back to the generic
// "example" value.
func TestSynthesize_NullableRendersConcreteValue(t *testing.T) {
	t.Parallel()
	if got := synthesize("null|string"); got != "example_string" {
		t.Fatalf("synthesize(null|string) = %q, want example_string", got)
	}
	if got := synthesize("integer|null"); got != "42" {
		t.Fatalf("synthesize(integer|null) = %q, want 42", got)
	}
}
