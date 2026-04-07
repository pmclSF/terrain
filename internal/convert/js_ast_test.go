package convert

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

func TestParseJSSyntaxTree_ParsesJavaScriptAndTypeScript(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`const total = 1 + 2`,
		`const total: number = 1 + 2`,
	} {
		tree, ok := parseJSSyntaxTree(source)
		if !ok {
			t.Fatalf("parseJSSyntaxTree(%q) returned ok=false", source)
		}
		if tree == nil || tree.tree == nil || tree.tree.RootNode() == nil {
			t.Fatalf("parseJSSyntaxTree(%q) returned incomplete tree", source)
		}
		tree.Close()
	}
}

func TestApplyTextEdits_SortsDescendingAndSkipsOverlaps(t *testing.T) {
	t.Parallel()

	got := applyTextEdits("0123456789", []textEdit{
		{start: 2, end: 4, replacement: "AB"},
		{start: 0, end: 1, replacement: "X"},
		{start: 2, end: 5, replacement: "LONG"},
		{start: -1, end: 2, replacement: "bad"},
		{start: 7, end: 99, replacement: "bad"},
	})

	want := "X1LONG56789"
	if got != want {
		t.Fatalf("applyTextEdits() = %q, want %q", got, want)
	}
}

func TestReplacementEditForCall_UsesAwaitParentWhenPresent(t *testing.T) {
	t.Parallel()

	source := "async function run() { await page.goto('/login'); }"
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		t.Fatal("parseJSSyntaxTree returned ok=false")
	}
	defer tree.Close()

	var callNodeFound bool
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		callNodeFound = true
		edit := replacementEditForCall(node, "browser.url('/login')")
		got := applyTextEdits(source, []textEdit{edit})
		want := "async function run() { browser.url('/login'); }"
		if got != want {
			t.Fatalf("replacementEditForCall() produced %q, want %q", got, want)
		}
		return false
	})

	if !callNodeFound {
		t.Fatal("expected to find call_expression")
	}
}
