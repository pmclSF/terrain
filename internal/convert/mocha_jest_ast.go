package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func convertMochaToJestSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	edits := make([]textEdit, 0, 16)
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'chai'") || strings.Contains(module, "\"chai\"") ||
				strings.Contains(module, "'sinon'") || strings.Contains(module, "\"sinon\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "lexical_declaration", "variable_declaration":
			text := jsNodeText(node, tree.src)
			if strings.Contains(text, "require('chai')") || strings.Contains(text, "require(\"chai\")") ||
				strings.Contains(text, "require('sinon')") || strings.Contains(text, "require(\"sinon\")") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			if callee != nil && callee.Type() == "identifier" {
				switch jsNodeText(callee, tree.src) {
				case "before":
					edits = append(edits, textEdit{
						start:       int(callee.StartByte()),
						end:         int(callee.EndByte()),
						replacement: "beforeAll",
					})
					return true
				case "after":
					edits = append(edits, textEdit{
						start:       int(callee.StartByte()),
						end:         int(callee.EndByte()),
						replacement: "afterAll",
					})
					return true
				}
			}

			replacement, ok := convertMochaCallToJest(node, tree.src)
			if !ok {
				return true
			}
			edits = append(edits, replacementEditForCall(node, replacement))
			return false
		case "member_expression":
			replacement, ok := convertMochaMemberExprToJest(node, tree.src)
			if !ok {
				return true
			}
			edits = append(edits, textEdit{
				start:       int(node.StartByte()),
				end:         int(node.EndByte()),
				replacement: replacement,
			})
			return false
		}
		return true
	})

	result := applyTextEdits(source, edits)
	if rows, ok := unsupportedMochaLineRowsAST(source); ok && len(rows) > 0 {
		result = commentSpecificLines(result, rows, "manual Mocha assertion conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func convertMochaCallToJest(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	base, parts, ok := jsMemberChainParts(callee, src)
	if !ok || len(parts) == 0 {
		return "", false
	}
	args := jsArgumentTexts(node, src)

	if base.Type() == "identifier" {
		root := jsNodeText(base, src)
		switch {
		case root == "sinon" && equalStrings(parts, "useFakeTimers"):
			return "jest.useFakeTimers()", true
		case root == "sinon" && equalStrings(parts, "reset"):
			return "jest.clearAllMocks()", true
		case root == "sinon" && equalStrings(parts, "restore"):
			return "jest.restoreAllMocks()", true
		case root == "sinon" && equalStrings(parts, "assert", "calledOnce") && len(args) == 1:
			return "expect(" + args[0] + ").toHaveBeenCalledTimes(1)", true
		case root == "sinon" && equalStrings(parts, "assert", "called") && len(args) == 1:
			return "expect(" + args[0] + ").toHaveBeenCalled()", true
		case root == "sinon" && equalStrings(parts, "assert", "calledWith") && len(args) >= 2:
			return "expect(" + args[0] + ").toHaveBeenCalledWith(" + strings.Join(args[1:], ", ") + ")", true
		case root == "sinon" && equalStrings(parts, "spy") && len(args) == 2:
			return "jest.spyOn(" + args[0] + ", " + args[1] + ")", true
		case root == "sinon" && equalStrings(parts, "stub"):
			if len(args) == 0 {
				return "jest.fn()", true
			}
			if len(args) == 2 {
				return "jest.spyOn(" + args[0] + ", " + args[1] + ")", true
			}
		case root == "clock" && equalStrings(parts, "tick") && len(args) == 1:
			return "jest.advanceTimersByTime(" + args[0] + ")", true
		case root == "clock" && equalStrings(parts, "restore"):
			return "jest.useRealTimers()", true
		}
	}

	if base.Type() == "call_expression" {
		baseCallee := jsCalleeNode(base)
		baseArgs := jsArgumentTexts(base, src)
		if baseCallee == nil {
			return "", false
		}

		if baseCallee.Type() == "identifier" && jsNodeText(baseCallee, src) == "expect" && len(baseArgs) == 1 {
			target := baseArgs[0]
			switch {
			case equalStrings(parts, "to", "deep", "equal") && len(args) == 1:
				return "expect(" + target + ").toEqual(" + args[0] + ")", true
			case equalStrings(parts, "to", "equal") && len(args) == 1:
				return "expect(" + target + ").toBe(" + args[0] + ")", true
			case equalStrings(parts, "to", "have", "lengthOf") && len(args) == 1:
				return "expect(" + target + ").toHaveLength(" + args[0] + ")", true
			case equalStrings(parts, "to", "contain") && len(args) == 1:
				return "expect(" + target + ").toContain(" + args[0] + ")", true
			}
		}

		if baseCallee.Type() == "member_expression" {
			innerBase, innerParts, ok := jsMemberChainParts(baseCallee, src)
			if !ok || innerBase == nil || innerBase.Type() != "identifier" || jsNodeText(innerBase, src) != "sinon" {
				return "", false
			}
			switch {
			case equalStrings(innerParts, "stub"):
				if len(baseArgs) == 0 {
					switch {
					case equalStrings(parts, "callsFake") && len(args) == 1:
						return "jest.fn().mockImplementation(" + args[0] + ")", true
					case equalStrings(parts, "returns") && len(args) == 1:
						return "jest.fn().mockReturnValue(" + args[0] + ")", true
					case equalStrings(parts, "resolves") && len(args) == 1:
						return "jest.fn().mockResolvedValue(" + args[0] + ")", true
					case equalStrings(parts, "rejects") && len(args) == 1:
						return "jest.fn().mockRejectedValue(" + args[0] + ")", true
					}
				}
				if len(baseArgs) == 2 {
					switch {
					case equalStrings(parts, "callsFake") && len(args) == 1:
						return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockImplementation(" + args[0] + ")", true
					case equalStrings(parts, "returns") && len(args) == 1:
						return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockReturnValue(" + args[0] + ")", true
					case equalStrings(parts, "resolves") && len(args) == 1:
						return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockResolvedValue(" + args[0] + ")", true
					case equalStrings(parts, "rejects") && len(args) == 1:
						return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockRejectedValue(" + args[0] + ")", true
					}
				}
			}
		}
	}

	return "", false
}

func convertMochaMemberExprToJest(node *sitter.Node, src []byte) (string, bool) {
	base, parts, ok := jsMemberChainParts(node, src)
	if !ok || base == nil || len(parts) == 0 {
		return "", false
	}
	if base.Type() != "call_expression" {
		return "", false
	}
	callee := jsCalleeNode(base)
	args := jsArgumentTexts(base, src)
	if callee == nil || callee.Type() != "identifier" || jsNodeText(callee, src) != "expect" || len(args) != 1 {
		return "", false
	}

	switch {
	case equalStrings(parts, "to", "be", "true"):
		return "expect(" + args[0] + ").toBe(true)", true
	case equalStrings(parts, "to", "be", "false"):
		return "expect(" + args[0] + ").toBe(false)", true
	}
	return "", false
}

func unsupportedMochaLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	rows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		callee := jsCalleeNode(node)
		if callee == nil {
			return true
		}
		if callee.Type() != "member_expression" {
			return true
		}
		base, parts, ok := jsMemberChainParts(callee, tree.src)
		if !ok || base == nil || len(parts) == 0 {
			return true
		}
		switch {
		case base.Type() == "identifier" && jsNodeText(base, tree.src) == "chai" && equalStrings(parts, "expect"):
			rows[int(node.StartPoint().Row)] = true
			return false
		case base.Type() == "identifier" && jsNodeText(base, tree.src) == "assert":
			rows[int(node.StartPoint().Row)] = true
			return false
		}
		return true
	})

	return rows, true
}

func jsMemberChainParts(node *sitter.Node, src []byte) (*sitter.Node, []string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return nil, nil, false
	}
	parts := make([]string, 0, 4)
	current := node
	for current != nil && current.Type() == "member_expression" {
		property := jsMemberProperty(current)
		if property == nil {
			return nil, nil, false
		}
		parts = append([]string{jsNodeText(property, src)}, parts...)
		object := jsMemberObject(current)
		if object == nil {
			return nil, nil, false
		}
		if object.Type() == "member_expression" {
			current = object
			continue
		}
		return object, parts, true
	}
	return nil, nil, false
}

func equalStrings(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
