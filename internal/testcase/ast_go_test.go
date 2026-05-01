package testcase

import "testing"

// TestExtractGo_HierarchicalSubtests confirms that nested t.Run calls
// build the full SuiteHierarchy path. Closes the 0.2 test-discovery
// gap where deeply nested subtests were attributed only to their
// outermost test function.
func TestExtractGo_HierarchicalSubtests(t *testing.T) {
	t.Parallel()

	src := `package foo

import "testing"

func TestThing(t *testing.T) {
	t.Run("group", func(t *testing.T) {
		t.Run("inner-a", func(t *testing.T) {})
		t.Run("inner-b", func(t *testing.T) {
			t.Run("deepest", func(t *testing.T) {})
		})
	})
	t.Run("sibling", func(t *testing.T) {})
}
`
	cases := extractGoWithAST(src, "thing_test.go", "go-test")

	byName := map[string]TestCase{}
	for _, c := range cases {
		byName[c.TestName] = c
	}

	want := map[string][]string{
		"TestThing": nil,                                       // top-level: empty hierarchy
		"group":     {"TestThing"},                             // child of TestThing
		"sibling":   {"TestThing"},                             // sibling of "group"
		"inner-a":   {"TestThing", "group"},                    // inside "group"
		"inner-b":   {"TestThing", "group"},                    // sibling of inner-a
		"deepest":   {"TestThing", "group", "inner-b"},         // deepest case
	}

	for name, wantStack := range want {
		got, ok := byName[name]
		if !ok {
			t.Errorf("missing test case %q (extracted: %v)", name, mapKeys(byName))
			continue
		}
		if !slicesEqual(got.SuiteHierarchy, wantStack) {
			t.Errorf("%q hierarchy = %v, want %v", name, got.SuiteHierarchy, wantStack)
		}
	}
}

// TestExtractGo_SiblingsDoNotShareStack guards against a subtle bug:
// if the hierarchy slice is shared between iterations, two sibling
// t.Runs end up with each other's names in their own stack.
func TestExtractGo_SiblingsDoNotShareStack(t *testing.T) {
	t.Parallel()

	src := `package foo

import "testing"

func TestParent(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		t.Run("a-inner", func(t *testing.T) {})
	})
	t.Run("b", func(t *testing.T) {
		t.Run("b-inner", func(t *testing.T) {})
	})
}
`
	cases := extractGoWithAST(src, "p_test.go", "go-test")

	for _, c := range cases {
		if c.TestName == "b-inner" {
			for _, hop := range c.SuiteHierarchy {
				if hop == "a" {
					t.Errorf("b-inner hierarchy %v leaked sibling 'a'", c.SuiteHierarchy)
				}
			}
		}
	}
}

func mapKeys(m map[string]TestCase) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
