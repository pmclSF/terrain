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
