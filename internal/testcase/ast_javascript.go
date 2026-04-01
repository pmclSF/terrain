package testcase

import (
	"context"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	tsTypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// extractJSWithAST uses tree-sitter to parse JS/TS source and extract test cases.
// This replaces the regex+brace-counting approach with a real AST, eliminating
// false positives from patterns inside string literals and comments.
func extractJSWithAST(src, relPath, framework string) []TestCase {
	srcBytes := []byte(src)

	parser := sitter.NewParser()
	defer parser.Close()

	// Choose language based on file extension.
	if strings.HasSuffix(relPath, ".ts") || strings.HasSuffix(relPath, ".tsx") {
		parser.SetLanguage(tsTypescript.GetLanguage())
	} else {
		parser.SetLanguage(javascript.GetLanguage())
	}

	tree, err := parser.ParseCtx(context.Background(), nil, srcBytes)
	if err != nil || tree == nil {
		// Fallback to regex-based extraction on parse failure.
		return extractJS(src, relPath, framework)
	}
	defer tree.Close()

	root := tree.RootNode()
	var cases []TestCase
	walkJSNode(root, srcBytes, nil, &cases)
	return cases
}

// walkJSNode recursively walks the AST looking for describe/it/test calls.
func walkJSNode(node *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase) {
	if node == nil {
		return
	}

	// Look for call_expression nodes that are describe/it/test.
	if node.Type() == "call_expression" {
		callee := getCalleeNameJS(node, src)

		switch {
		case isDescribeLike(callee):
			name := extractFirstStringArg(node, src)
			if name != "" {
				newStack := append(append([]string{}, suiteStack...), name)
				// Walk the callback body with the new suite scope.
				callbackBody := findCallbackBody(node)
				if callbackBody != nil {
					walkJSNode(callbackBody, src, newStack, cases)
					return // Don't walk children again — we already walked the body.
				}
			}

		case isTestLike(callee):
			name := extractFirstStringArg(node, src)
			if name != "" {
				tc := TestCase{
					TestName:       name,
					SuiteHierarchy: copySuiteStack(suiteStack),
					Line:           int(node.StartPoint().Row) + 1,
					ExtractionKind: ExtractionStatic,
					Confidence:     ConfidenceSyntaxMatch,
				}
				*cases = append(*cases, tc)
			}
			return // Don't descend into test body.

		case isEachLike(callee):
			baseName := eachBaseName(callee)
			// .each()('name', fn) — the name is in the NEXT call expression up.
			// tree-sitter parses test.each([...])('name', fn) as a nested call.
			name := extractEachName(node, src)
			if name == "" {
				// Try parent (test.each([...])('name', fn) may be parent call).
				if node.Parent() != nil && node.Parent().Type() == "call_expression" {
					name = extractFirstStringArg(node.Parent(), src)
				}
			}
			if name != "" {
				if baseName == "describe" {
					newStack := append(append([]string{}, suiteStack...), name)
					callbackBody := findCallbackBody(node.Parent())
					if callbackBody != nil {
						walkJSNode(callbackBody, src, newStack, cases)
						return
					}
				} else {
					instances := estimateEachInstancesFromAST(node, src)
					if instances > 0 {
						const maxEnumerated = 100
						if instances > maxEnumerated {
							instances = maxEnumerated
						}
						for i := 1; i <= instances; i++ {
							tc := TestCase{
								TestName:       name,
								SuiteHierarchy: copySuiteStack(suiteStack),
								Line:           int(node.StartPoint().Row) + 1,
								ExtractionKind: ExtractionStatic,
								Confidence:     ConfidenceHeuristic,
								Parameterized: &ParameterizationInfo{
									IsTemplate:         false,
									ParamSignature:     "case_" + strconv.Itoa(i),
									EstimatedInstances: instances,
								},
							}
							*cases = append(*cases, tc)
						}
					} else {
						tc := TestCase{
							TestName:       name,
							SuiteHierarchy: copySuiteStack(suiteStack),
							Line:           int(node.StartPoint().Row) + 1,
							ExtractionKind: ExtractionParameterizedTemplate,
							Confidence:     ConfidenceInferred,
							Parameterized: &ParameterizationInfo{
								IsTemplate: true,
							},
						}
						*cases = append(*cases, tc)
					}
					return
				}
			}
		}
	}

	// Recurse into children.
	for i := 0; i < int(node.ChildCount()); i++ {
		walkJSNode(node.Child(i), src, suiteStack, cases)
	}
}

// --- Callee identification ---

func getCalleeNameJS(node *sitter.Node, src []byte) string {
	if node.Type() != "call_expression" || node.ChildCount() == 0 {
		return ""
	}
	callee := node.Child(0)
	return nodeText(callee, src)
}

func isDescribeLike(callee string) bool {
	switch callee {
	case "describe", "context", "suite",
		"describe.only", "describe.skip",
		"context.only", "context.skip",
		"fdescribe", "xdescribe":
		return true
	}
	return false
}

func isTestLike(callee string) bool {
	switch callee {
	case "it", "test", "specify",
		"it.only", "it.skip", "it.todo",
		"test.only", "test.skip", "test.todo",
		"fit", "xit", "ftest", "xtest":
		return true
	}
	return false
}

func isEachLike(callee string) bool {
	return strings.HasSuffix(callee, ".each") ||
		strings.Contains(callee, ".each(") ||
		strings.Contains(callee, ".each[")
}

func eachBaseName(callee string) string {
	parts := strings.SplitN(callee, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// --- Argument extraction ---

func extractFirstStringArg(node *sitter.Node, src []byte) string {
	args := findArguments(node)
	if args == nil || args.ChildCount() < 2 { // ( and ) are children
		return ""
	}
	// First named child of arguments is the first argument.
	for i := 0; i < int(args.NamedChildCount()); i++ {
		arg := args.NamedChild(i)
		if arg.Type() == "string" || arg.Type() == "template_string" {
			return extractStringContent(arg, src)
		}
	}
	return ""
}

func extractStringContent(node *sitter.Node, src []byte) string {
	text := nodeText(node, src)
	if len(text) < 2 {
		return text
	}
	// Strip quotes and decode escape sequences.
	first := text[0]
	switch first {
	case '"':
		return decodeQuotedString(text[1:len(text)-1], '"')
	case '\'':
		return decodeQuotedString(text[1:len(text)-1], '\'')
	case '`':
		return decodeTemplateLiteral(text[1 : len(text)-1])
	}
	return text
}

func findArguments(node *sitter.Node) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "arguments" {
			return child
		}
	}
	return nil
}

func findCallbackBody(node *sitter.Node) *sitter.Node {
	args := findArguments(node)
	if args == nil {
		return nil
	}
	// Look for arrow_function or function_expression in arguments.
	for i := 0; i < int(args.NamedChildCount()); i++ {
		arg := args.NamedChild(i)
		switch arg.Type() {
		case "arrow_function", "function_expression", "function":
			// Return the statement_block body.
			for j := 0; j < int(arg.ChildCount()); j++ {
				child := arg.Child(j)
				if child.Type() == "statement_block" {
					return child
				}
			}
		}
	}
	return nil
}

// --- Each/parameterized ---

func extractEachName(callNode *sitter.Node, src []byte) string {
	// test.each([...])('name', fn) — the parent call has the name as first string arg.
	parent := callNode.Parent()
	if parent != nil && parent.Type() == "call_expression" {
		return extractFirstStringArg(parent, src)
	}
	return ""
}

func estimateEachInstancesFromAST(node *sitter.Node, src []byte) int {
	// Find the array argument in .each([a, b, c]).
	args := findArguments(node)
	if args == nil {
		return 0
	}
	for i := 0; i < int(args.NamedChildCount()); i++ {
		arg := args.NamedChild(i)
		if arg.Type() == "array" {
			// Count top-level elements.
			return int(arg.NamedChildCount())
		}
	}
	return 0
}

// --- Utility ---

func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func copySuiteStack(stack []string) []string {
	if len(stack) == 0 {
		return nil
	}
	cp := make([]string, len(stack))
	copy(cp, stack)
	return cp
}
