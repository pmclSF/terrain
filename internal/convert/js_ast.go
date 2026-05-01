package convert

import (
	"context"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	tsTypescript "github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/pmclSF/terrain/internal/parserpool"
)

type jsSyntaxTree struct {
	parser *sitter.Parser
	lang   *sitter.Language // pool key for Release on Close
	tree   *sitter.Tree
	src    []byte
}

type textEdit struct {
	start       int
	end         int
	replacement string
}

func parseJSSyntaxTree(source string) (*jsSyntaxTree, bool) {
	src := []byte(source)
	languages := []*sitter.Language{
		tsTypescript.GetLanguage(),
		javascript.GetLanguage(),
	}

	for _, language := range languages {
		parser := parserpool.Acquire(language)

		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err == nil && tree != nil && !tree.RootNode().HasError() {
			return &jsSyntaxTree{
				parser: parser,
				lang:   language,
				tree:   tree,
				src:    src,
			}, true
		}
		if tree != nil {
			tree.Close()
		}
		parserpool.Release(language, parser)
	}

	return nil, false
}

func (t *jsSyntaxTree) Close() {
	if t == nil {
		return
	}
	if t.tree != nil {
		t.tree.Close()
	}
	if t.parser != nil {
		// Pooled parser: return it for reuse instead of Close().
		parserpool.Release(t.lang, t.parser)
	}
}

func walkJSNodes(node *sitter.Node, visit func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !visit(node) {
		return
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkJSNodes(node.NamedChild(i), visit)
	}
}

func applyTextEdits(source string, edits []textEdit) string {
	if len(edits) == 0 {
		return source
	}

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start == edits[j].start {
			return edits[i].end > edits[j].end
		}
		return edits[i].start > edits[j].start
	})

	result := source
	lastStart := len(source) + 1
	for _, edit := range edits {
		if edit.start < 0 || edit.end < edit.start || edit.end > len(result) {
			continue
		}
		if edit.start >= lastStart {
			continue
		}
		result = result[:edit.start] + edit.replacement + result[edit.end:]
		lastStart = edit.start
	}
	return result
}

func jsNodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func jsArgumentsNode(node *sitter.Node) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "arguments" {
			return child
		}
	}
	return nil
}

func jsArgumentNodes(node *sitter.Node) []*sitter.Node {
	args := jsArgumentsNode(node)
	if args == nil || args.NamedChildCount() == 0 {
		return nil
	}
	values := make([]*sitter.Node, 0, int(args.NamedChildCount()))
	for i := 0; i < int(args.NamedChildCount()); i++ {
		values = append(values, args.NamedChild(i))
	}
	return values
}

func jsCalleeNode(node *sitter.Node) *sitter.Node {
	if node == nil || node.Type() != "call_expression" {
		return nil
	}
	if callee := node.ChildByFieldName("function"); callee != nil {
		return callee
	}
	if node.NamedChildCount() > 0 {
		return node.NamedChild(0)
	}
	return nil
}

func jsMemberObject(node *sitter.Node) *sitter.Node {
	if node == nil || node.Type() != "member_expression" {
		return nil
	}
	if object := node.ChildByFieldName("object"); object != nil {
		return object
	}
	if node.NamedChildCount() > 0 {
		return node.NamedChild(0)
	}
	return nil
}

func jsMemberProperty(node *sitter.Node) *sitter.Node {
	if node == nil || node.Type() != "member_expression" {
		return nil
	}
	if property := node.ChildByFieldName("property"); property != nil {
		return property
	}
	if node.NamedChildCount() > 1 {
		return node.NamedChild(1)
	}
	return nil
}

func jsBaseIdentifier(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "identifier", "property_identifier":
		return jsNodeText(node, src)
	case "member_expression":
		return jsBaseIdentifier(jsMemberObject(node), src)
	}
	return ""
}

func replacementEditForCall(node *sitter.Node, replacement string) textEdit {
	target := node
	if parent := node.Parent(); parent != nil && parent.Type() == "await_expression" {
		target = parent
	}
	return textEdit{
		start:       int(target.StartByte()),
		end:         int(target.EndByte()),
		replacement: replacement,
	}
}
