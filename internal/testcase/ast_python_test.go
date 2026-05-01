package testcase

import (
	"strings"
	"testing"
)

// TestExtractPython_ParametrizeValues confirms the 0.2 work that
// extracts literal parametrize values into ParameterizationInfo.Values.
// Closes the round-4 finding "pytest parametrize value extraction
// (currently estimates count only)".
func TestExtractPython_ParametrizeValues(t *testing.T) {
	t.Parallel()

	src := `import pytest

@pytest.mark.parametrize("name", ["alice", "bob", "carol"])
def test_login(name):
    assert login(name)
`
	cases := extractPythonWithAST(src, "tests/login_test.py", "pytest")

	if len(cases) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(cases))
	}
	want := []string{`"alice"`, `"bob"`, `"carol"`}
	for i, c := range cases {
		if c.Parameterized == nil {
			t.Errorf("case %d has nil Parameterized", i)
			continue
		}
		if len(c.Parameterized.Values) != 1 {
			t.Errorf("case %d Values=%v, want 1 entry", i, c.Parameterized.Values)
			continue
		}
		if c.Parameterized.Values[0] != want[i] {
			t.Errorf("case %d Values[0]=%q, want %q", i, c.Parameterized.Values[0], want[i])
		}
	}
}

// TestExtractPython_ParametrizeTuples confirms multi-parameter tuples
// are captured whole so consumers can render the original formatting.
func TestExtractPython_ParametrizeTuples(t *testing.T) {
	t.Parallel()

	src := `import pytest

@pytest.mark.parametrize("a,b", [(1, 2), (3, 4)])
def test_add(a, b):
    assert add(a, b) == a + b
`
	cases := extractPythonWithAST(src, "tests/add_test.py", "pytest")
	if len(cases) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(cases))
	}
	if cases[0].Parameterized.Values[0] != "(1, 2)" {
		t.Errorf("got %q, want (1, 2)", cases[0].Parameterized.Values[0])
	}
	if cases[1].Parameterized.Values[0] != "(3, 4)" {
		t.Errorf("got %q, want (3, 4)", cases[1].Parameterized.Values[0])
	}
}

// TestExtractPython_ParametrizeDynamic falls back to the
// count/template path when the value list is not a static literal.
func TestExtractPython_ParametrizeDynamic(t *testing.T) {
	t.Parallel()

	src := `import pytest
from .fixtures import all_users

@pytest.mark.parametrize("name", all_users())
def test_greet(name):
    assert greet(name)
`
	cases := extractPythonWithAST(src, "tests/greet_test.py", "pytest")
	if len(cases) != 1 {
		t.Fatalf("dynamic value list should produce 1 template case, got %d", len(cases))
	}
	if cases[0].Parameterized == nil || !cases[0].Parameterized.IsTemplate {
		t.Errorf("expected IsTemplate=true on dynamic-values case, got %+v", cases[0].Parameterized)
	}
	if !strings.Contains(string(cases[0].ExtractionKind), "parameterized_template") {
		t.Errorf("expected parameterized_template kind, got %q", cases[0].ExtractionKind)
	}
}
