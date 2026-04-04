package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var jestJasmineExpectationMembers = map[string]string{
	"anything":         "anything",
	"objectContaining": "objectContaining",
	"arrayContaining":  "arrayContaining",
	"stringMatching":   "stringMatching",
	"any":              "any",
}

func convertJestToJasmineSourceAST(source string) (string, bool) {
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
			if strings.Contains(module, "'@jest/globals'") || strings.Contains(module, "\"@jest/globals\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "lexical_declaration", "variable_declaration":
			text := jsNodeText(node, tree.src)
			if strings.Contains(text, "require('@jest/globals')") || strings.Contains(text, "require(\"@jest/globals\")") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			replacement, ok := convertJestCallToJasmine(node, tree.src)
			if !ok {
				return true
			}
			edits = append(edits, replacementEditForCall(node, replacement))
			return false
		}
		return true
	})

	result := applyTextEdits(source, edits)
	if rows, ok := unsupportedJestMockLineRowsAST(source); ok && len(rows) > 0 {
		result = commentSpecificLines(result, rows, "manual Jest module mock conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func convertJestCallToJasmine(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil {
		return "", false
	}

	if replacement, ok := convertJestClockCall(node, src); ok {
		return replacement, true
	}

	if callee.Type() != "member_expression" {
		return "", false
	}

	if replacement, ok := convertJestMockChain(node, src); ok {
		return replacement, true
	}

	root := jsBaseIdentifier(callee, src)
	property := jsNodeText(jsMemberProperty(callee), src)
	args := jsArgumentTexts(node, src)
	switch root {
	case "jest":
		switch property {
		case "spyOn":
			if len(args) == 2 {
				return "spyOn(" + args[0] + ", " + args[1] + ")", true
			}
		case "fn":
			if len(args) == 1 {
				return "jasmine.createSpy().and.callFake(" + args[0] + ")", true
			}
			return "jasmine.createSpy()", true
		}
	case "expect":
		if mapped, ok := jestJasmineExpectationMembers[property]; ok {
			return "jasmine." + mapped + "(" + strings.Join(args, ", ") + ")", true
		}
	}

	return "", false
}

func convertJestClockCall(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	if jsBaseIdentifier(callee, src) != "jest" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	args := jsArgumentTexts(node, src)
	switch property {
	case "useFakeTimers":
		return "jasmine.clock().install()", true
	case "useRealTimers":
		return "jasmine.clock().uninstall()", true
	case "advanceTimersByTime":
		if len(args) == 1 {
			return "jasmine.clock().tick(" + args[0] + ")", true
		}
	case "setSystemTime":
		if len(args) == 1 {
			return "jasmine.clock().mockDate(" + args[0] + ")", true
		}
	}
	return "", false
}

func convertJestMockChain(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	baseCall := jsMemberObject(callee)
	if baseCall == nil || baseCall.Type() != "call_expression" {
		return "", false
	}

	baseCallee := jsCalleeNode(baseCall)
	baseArgs := jsArgumentTexts(baseCall, src)
	callArgs := jsArgumentTexts(node, src)
	if baseCallee == nil || baseCallee.Type() != "member_expression" {
		return "", false
	}

	if jsBaseIdentifier(baseCallee, src) == "jest" && jsNodeText(jsMemberProperty(baseCallee), src) == "spyOn" && len(baseArgs) == 2 {
		switch property {
		case "mockReturnValue":
			if len(callArgs) == 1 {
				return "spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").and.returnValue(" + callArgs[0] + ")", true
			}
		case "mockImplementation":
			if len(callArgs) == 1 {
				return "spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").and.callFake(" + callArgs[0] + ")", true
			}
		}
	}

	if jsBaseIdentifier(baseCallee, src) == "jest" && jsNodeText(jsMemberProperty(baseCallee), src) == "fn" {
		switch property {
		case "mockReturnValue":
			if len(callArgs) == 1 {
				return "jasmine.createSpy().and.returnValue(" + callArgs[0] + ")", true
			}
		case "mockResolvedValue":
			if len(callArgs) == 1 {
				return "jasmine.createSpy().and.returnValue(Promise.resolve(" + callArgs[0] + "))", true
			}
		case "mockRejectedValue":
			if len(callArgs) == 1 {
				return "jasmine.createSpy().and.returnValue(Promise.reject(" + callArgs[0] + "))", true
			}
		case "mockImplementation":
			if len(callArgs) == 1 {
				return "jasmine.createSpy().and.callFake(" + callArgs[0] + ")", true
			}
		}
	}

	return "", false
}

func unsupportedJestMockLineRowsAST(source string) (map[int]bool, bool) {
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
		if callee == nil || callee.Type() != "member_expression" {
			return true
		}
		if jsBaseIdentifier(callee, tree.src) != "jest" {
			return true
		}
		property := jsNodeText(jsMemberProperty(callee), tree.src)
		if property == "mock" || property == "doMock" {
			rows[int(node.StartPoint().Row)] = true
			return false
		}
		return true
	})

	return rows, true
}
