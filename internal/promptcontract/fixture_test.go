package promptcontract

import (
	"path/filepath"
	"testing"
)

// TestFixtures runs the drift-fixture acceptance matrix:
// positives MUST fire on the expected variable; negatives MUST stay silent.
func TestFixtures(t *testing.T) {
	base := "testdata/drift-fixtures"
	cases := []struct {
		dir      string
		wantFire bool
		wantVar  string
	}{
		{"positive/py_pydantic_removed", true, "user_id"},
		{"positive/py_langchain_inputvars", true, "account_id"},
		{"positive/py_format_unpack", true, "account_id"},     // .format(**model) render binding
		{"positive/py_optional_transparent", true, "user_id"}, // Optional[X] unwraps to X (transparent)
		{"negative/py_local_var", false, ""},
		{"negative/py_consistent", false, ""},
		{"negative/non_ai_braces", false, ""},
		{"negative/py_inherited", false, ""},         // inherited field is valid (no FP)
		{"negative/py_property", false, ""},          // @property attribute is valid (no FP)
		{"negative/py_library_type", false, ""},      // type imported from a library, not the local schema (no FP)
		{"negative/py_format_consistent", false, ""}, // every .format placeholder is a field (no FP)
		{"negative/py_format_untyped", false, ""},    // **payload is not a typed schema param (no FP)
		{"negative/py_generic_wrapper", false, ""},   // RunContext[X].attr binds to the wrapper, not X (no FP)
		{"negative/py_self_attr", false, ""},         // self.x set in __post_init__ is a valid attribute (no FP)
		{"negative/py_open_contract", false, ""},     // setattr/__getattr__ -> open contract, unknowable (no FP)
		{"negative/py_base_subclass", false, ""},     // attr on a subclass; base-typed var is polymorphic (no FP)
		{"negative/py_local_assign", false, ""},      // attr assigned (obj.attr=...) in scope -> dynamic (no FP)
	}
	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			drift, err := AnalyzeInRepo(filepath.Join(base, c.dir))
			if err != nil {
				t.Fatalf("AnalyzeInRepo: %v", err)
			}
			if !c.wantFire {
				if len(drift) != 0 {
					t.Errorf("expected SILENT, got %d drift: %+v", len(drift), drift)
				}
				return
			}
			if len(drift) == 0 {
				t.Fatalf("expected drift on %q, got none", c.wantVar)
			}
			for _, d := range drift {
				if d.Variable == c.wantVar {
					return
				}
			}
			t.Errorf("expected drift on %q, got %+v", c.wantVar, drift)
		})
	}
}
