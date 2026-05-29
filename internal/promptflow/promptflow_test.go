package promptflow

import (
	"errors"
	"testing"

	"github.com/pmclSF/terrain/internal/prompttemplate"
	"github.com/pmclSF/terrain/internal/schemadiff"
)

func TestCorrelateVars_VarMatchesTypeChanged(t *testing.T) {
	vars := []string{"age"}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeTypeChanged, Field: "age", OldType: "integer", NewType: "string"},
	}
	got := CorrelateVars(vars, changes)
	if len(got) != 1 {
		t.Fatalf("got %d risks, want 1: %+v", len(got), got)
	}
	if got[0].Change.Kind != schemadiff.ChangeTypeChanged {
		t.Errorf("Change.Kind = %v, want ChangeTypeChanged", got[0].Change.Kind)
	}
}

func TestCorrelateVars_AddedDoesNotProduceRisk(t *testing.T) {
	vars := []string{"new_field"}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeAdded, Field: "new_field", NewType: "string"},
	}
	got := CorrelateVars(vars, changes)
	if len(got) != 0 {
		t.Errorf("got %d risks, want 0: %+v", len(got), got)
	}
}

func TestCorrelateVars_NoOverlapMeansNoRisks(t *testing.T) {
	vars := []string{"foo", "bar"}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeRemoved, Field: "baz", OldType: "string"},
	}
	got := CorrelateVars(vars, changes)
	if len(got) != 0 {
		t.Errorf("got %d risks, want 0: %+v", len(got), got)
	}
}

func TestCorrelateVars_EmptyInputs(t *testing.T) {
	if got := CorrelateVars(nil, nil); len(got) != 0 {
		t.Errorf("nil/nil -> %d risks, want 0", len(got))
	}
	if got := CorrelateVars([]string{"x"}, nil); len(got) != 0 {
		t.Errorf("vars without changes -> %d risks, want 0", len(got))
	}
	if got := CorrelateVars(nil, []schemadiff.Change{{Kind: schemadiff.ChangeRemoved, Field: "x"}}); len(got) != 0 {
		t.Errorf("changes without vars -> %d risks, want 0", len(got))
	}
}

func TestCorrelateVars_MultipleRisksSortedByVariable(t *testing.T) {
	vars := []string{"zebra", "apple", "mango"}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeRemoved, Field: "zebra", OldType: "string"},
		{Kind: schemadiff.ChangeRemoved, Field: "apple", OldType: "string"},
		{Kind: schemadiff.ChangeTypeChanged, Field: "mango", OldType: "integer", NewType: "string"},
	}
	got := CorrelateVars(vars, changes)
	if len(got) != 3 {
		t.Fatalf("got %d risks, want 3: %+v", len(got), got)
	}
	want := []string{"apple", "mango", "zebra"}
	for i, r := range got {
		if r.Variable != want[i] {
			t.Errorf("got[%d].Variable = %q, want %q", i, r.Variable, want[i])
		}
	}
}

func TestCorrelateTemplate_FindsRemovedVariable(t *testing.T) {
	tpl := prompttemplate.Template{
		Kind: prompttemplate.KindMustache,
		Body: "Hello, {{user_id}}! Your balance is {{balance}}.",
	}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeRemoved, Field: "user_id", OldType: "string"},
	}
	got := CorrelateTemplate(tpl, changes)
	if len(got) != 1 {
		t.Fatalf("got %d risks, want 1: %+v", len(got), got)
	}
	if got[0].Variable != "user_id" {
		t.Errorf("Variable = %q, want %q", got[0].Variable, "user_id")
	}
}

func TestEndToEnd_SchemaRenameBreaksTemplate(t *testing.T) {
	// Before: schema has user_id.
	oldSchema := []byte(`{"properties": {
		"user_id":  {"type": "string"},
		"balance":  {"type": "number"}
	}}`)
	// After: user_id renamed to userId (manifests as remove+add).
	newSchema := []byte(`{"properties": {
		"userId":  {"type": "string"},
		"balance": {"type": "number"}
	}}`)

	changes, err := schemadiff.DiffJSONSchema(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("DiffJSONSchema error: %v", err)
	}

	// A template references the OLD name — and now points at nothing.
	tpl := prompttemplate.Template{
		Kind: prompttemplate.KindMustache,
		Body: "Hello {{user_id}}, your balance is {{balance}}.",
	}

	risks := CorrelateTemplate(tpl, changes)
	if len(risks) != 1 {
		t.Fatalf("got %d risks, want 1: %+v", len(risks), risks)
	}
	if risks[0].Variable != "user_id" {
		t.Errorf("Variable = %q, want %q", risks[0].Variable, "user_id")
	}
	if risks[0].Change.Kind != schemadiff.ChangeRemoved {
		t.Errorf("Change.Kind = %v, want ChangeRemoved", risks[0].Change.Kind)
	}

	// Sanity: the renderer fails on this template after the rename,
	// confirming the risk is real.
	_, renderErr := tpl.Render(map[string]string{
		"userId":  "alice",
		"balance": "100",
	})
	var mv *prompttemplate.MissingVarError
	if !errors.As(renderErr, &mv) {
		t.Fatalf("expected MissingVarError, got %v", renderErr)
	}
	if mv.Name != "user_id" {
		t.Errorf("MissingVarError.Name = %q, want %q", mv.Name, "user_id")
	}
}

func TestCorrelateVars_VarMatchesRemoved(t *testing.T) {
	vars := []string{"user_id"}
	changes := []schemadiff.Change{
		{Kind: schemadiff.ChangeRemoved, Field: "user_id", OldType: "string"},
	}
	got := CorrelateVars(vars, changes)
	if len(got) != 1 {
		t.Fatalf("got %d risks, want 1: %+v", len(got), got)
	}
	r := got[0]
	if r.Variable != "user_id" {
		t.Errorf("Variable = %q, want %q", r.Variable, "user_id")
	}
	if r.Change.Kind != schemadiff.ChangeRemoved {
		t.Errorf("Change.Kind = %v, want ChangeRemoved", r.Change.Kind)
	}
}
