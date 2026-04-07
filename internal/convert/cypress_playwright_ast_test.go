package convert

import (
	"reflect"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestExtractJSCallChain_ParsesNestedCypressChain(t *testing.T) {
	t.Parallel()

	source := `cy.get('#submit').eq(1).click()`
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		t.Fatal("parseJSSyntaxTree returned ok=false")
	}
	defer tree.Close()

	var gotRoot string
	var gotSteps []jsCallStep
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		root, steps, ok := extractJSCallChain(node, tree.src)
		if !ok || root != "cy" || len(steps) != 3 {
			return true
		}
		gotRoot = root
		gotSteps = steps
		return false
	})

	wantSteps := []jsCallStep{
		{method: "get", args: []string{"'#submit'"}},
		{method: "eq", args: []string{"1"}},
		{method: "click", args: nil},
	}
	if gotRoot != "cy" {
		t.Fatalf("root = %q, want cy", gotRoot)
	}
	if !reflect.DeepEqual(gotSteps, wantSteps) {
		t.Fatalf("steps = %#v, want %#v", gotSteps, wantSteps)
	}
}

func TestConvertCypressCallToPlaywright_HandlesClearTypeChain(t *testing.T) {
	t.Parallel()

	source := `cy.get('#name').clear().type('Ada')`
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		t.Fatal("parseJSSyntaxTree returned ok=false")
	}
	defer tree.Close()

	var got string
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		replacement, _, ok := convertCypressCallToPlaywright(node, tree.src)
		if !ok {
			return true
		}
		got = replacement
		return false
	})

	want := "await page.locator('#name').fill('Ada')"
	if got != want {
		t.Fatalf("convertCypressCallToPlaywright() = %q, want %q", got, want)
	}
}

func TestUnsupportedCypressLineRowsAST_OnlyMarksRealUnsupportedCalls(t *testing.T) {
	t.Parallel()

	source := `// cy.wrap(user)
const note = "cy.session('keep-me-literal')";
it('uses unsupported helpers', () => {
  cy.session('real');
  cy.visit('/ok');
});`

	rows, ok := unsupportedCypressLineRowsAST(source)
	if !ok {
		t.Fatal("unsupportedCypressLineRowsAST returned ok=false")
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1 (%v)", len(rows), rows)
	}
	if !rows[3] {
		t.Fatalf("expected unsupported row 3 to be marked, got %v", rows)
	}
}

func TestNormalizeJSLiteral_HandlesEscapedQuotes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`'it\'s fine'`:       "it's fine",
		`"say \"hello\""`:    `say "hello"`,
		"`keep \\` literal`": "keep ` literal",
		`plainIdentifier`:    "plainIdentifier",
	}

	for input, want := range cases {
		if got := normalizeJSLiteral(input); got != want {
			t.Fatalf("normalizeJSLiteral(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsNumericLiteral_AcceptsFloatsAndSeparators(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"1000", "1_000", "1.5", "2.0e3"} {
		if !isNumericLiteral(input) {
			t.Fatalf("expected %q to be recognized as numeric", input)
		}
	}
	for _, input := range []string{"abc", "1..5", "1ms", ""} {
		if isNumericLiteral(input) {
			t.Fatalf("expected %q to be rejected as numeric", input)
		}
	}
}
