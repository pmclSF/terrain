package schemadiff

import "testing"

func TestDiffJSONSchema_FieldTypeChanged(t *testing.T) {
	oldDoc := []byte(`{"properties": {"age": {"type": "integer"}}}`)
	newDoc := []byte(`{"properties": {"age": {"type": "string"}}}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d changes, want 1: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != ChangeTypeChanged {
		t.Errorf("Kind = %v, want ChangeTypeChanged", c.Kind)
	}
	if c.Field != "age" {
		t.Errorf("Field = %q, want %q", c.Field, "age")
	}
	if c.OldType != "integer" {
		t.Errorf("OldType = %q, want %q", c.OldType, "integer")
	}
	if c.NewType != "string" {
		t.Errorf("NewType = %q, want %q", c.NewType, "string")
	}
}

func TestDiffJSONSchema_NoChanges(t *testing.T) {
	doc := []byte(`{"properties": {"name": {"type": "string"}, "age": {"type": "integer"}}}`)
	got, err := DiffJSONSchema(doc, doc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d changes, want 0: %+v", len(got), got)
	}
}

func TestDiffJSONSchema_MultipleChangesAreOrderedByField(t *testing.T) {
	oldDoc := []byte(`{"properties": {
		"name":     {"type": "string"},
		"age":      {"type": "integer"},
		"old_only": {"type": "boolean"}
	}}`)
	newDoc := []byte(`{"properties": {
		"name":     {"type": "string"},
		"age":      {"type": "string"},
		"new_only": {"type": "number"}
	}}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(got), got)
	}
	// Expect alphabetical by Field: age, new_only, old_only.
	wantFields := []string{"age", "new_only", "old_only"}
	wantKinds := []ChangeKind{ChangeTypeChanged, ChangeAdded, ChangeRemoved}
	for i, c := range got {
		if c.Field != wantFields[i] {
			t.Errorf("got[%d].Field = %q, want %q", i, c.Field, wantFields[i])
		}
		if c.Kind != wantKinds[i] {
			t.Errorf("got[%d].Kind = %v, want %v", i, c.Kind, wantKinds[i])
		}
	}
}

func TestDiffJSONSchema_InvalidJSONReturnsError(t *testing.T) {
	cases := []struct {
		name           string
		oldDoc, newDoc []byte
	}{
		{"old is garbage", []byte(`not json at all`), []byte(`{"properties":{}}`)},
		{"new is garbage", []byte(`{"properties":{}}`), []byte(`not json at all`)},
		{"both empty", []byte(``), []byte(``)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DiffJSONSchema(c.oldDoc, c.newDoc)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestChangeKind_String(t *testing.T) {
	cases := []struct {
		k    ChangeKind
		want string
	}{
		{ChangeUnknown, "unknown"},
		{ChangeAdded, "added"},
		{ChangeRemoved, "removed"},
		{ChangeTypeChanged, "type-changed"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.k.String(); got != c.want {
				t.Errorf("ChangeKind(%d).String() = %q, want %q", int(c.k), got, c.want)
			}
		})
	}
}

func TestDiffJSONSchema_NullableTypeArrayHandled(t *testing.T) {
	// type as an array form (nullable field). Both schemas have the
	// field with the same type; no change should be reported.
	doc := []byte(`{
		"properties": {
			"score": {"type": ["string", "null"]}
		}
	}`)
	got, err := DiffJSONSchema(doc, doc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d changes for identical nullable schemas, want 0: %+v", len(got), got)
	}
}

func TestDiffJSONSchema_NullableTypeArrayOrderInsensitive(t *testing.T) {
	oldDoc := []byte(`{"properties": {"x": {"type": ["string", "null"]}}}`)
	newDoc := []byte(`{"properties": {"x": {"type": ["null", "string"]}}}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("reordered nullable shouldn't be a type change; got %+v", got)
	}
}

func TestDiffJSONSchema_NullableTypeArrayChangeDetected(t *testing.T) {
	// Going from `["string", "null"]` to `["integer", "null"]` IS a
	// real type change and should fire.
	oldDoc := []byte(`{"properties": {"x": {"type": ["string", "null"]}}}`)
	newDoc := []byte(`{"properties": {"x": {"type": ["integer", "null"]}}}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 1 || got[0].Kind != ChangeTypeChanged {
		t.Fatalf("expected one type-changed entry, got %+v", got)
	}
	if got[0].OldType != "null|string" {
		t.Errorf("OldType = %q, want %q", got[0].OldType, "null|string")
	}
	if got[0].NewType != "integer|null" {
		t.Errorf("NewType = %q, want %q", got[0].NewType, "integer|null")
	}
}

func TestDiffJSONSchema_TypeAbsentTreatedAsEmpty(t *testing.T) {
	// A field without an explicit `type` (only `description`) used to
	// crash on unmarshal. Now it normalizes to empty.
	doc := []byte(`{"properties": {"x": {"description": "no type field here"}}}`)
	got, err := DiffJSONSchema(doc, doc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("identical schemas without type produced changes: %+v", got)
	}
}

func TestDiffJSONSchema_FieldRemoved(t *testing.T) {
	oldDoc := []byte(`{
		"properties": {
			"name": {"type": "string"},
			"age":  {"type": "integer"}
		}
	}`)
	newDoc := []byte(`{
		"properties": {
			"name": {"type": "string"}
		}
	}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d changes, want 1: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != ChangeRemoved {
		t.Errorf("Kind = %v, want ChangeRemoved", c.Kind)
	}
	if c.Field != "age" {
		t.Errorf("Field = %q, want %q", c.Field, "age")
	}
	if c.OldType != "integer" {
		t.Errorf("OldType = %q, want %q", c.OldType, "integer")
	}
}

func TestDiffJSONSchema_FieldAdded(t *testing.T) {
	oldDoc := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`)
	newDoc := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age":  {"type": "integer"}
		}
	}`)
	got, err := DiffJSONSchema(oldDoc, newDoc)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d changes, want 1: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != ChangeAdded {
		t.Errorf("Kind = %v, want ChangeAdded", c.Kind)
	}
	if c.Field != "age" {
		t.Errorf("Field = %q, want %q", c.Field, "age")
	}
	if c.NewType != "integer" {
		t.Errorf("NewType = %q, want %q", c.NewType, "integer")
	}
}
