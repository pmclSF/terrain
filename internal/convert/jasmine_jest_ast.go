package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var jasmineExpectationMembers = map[string]string{
	"anything":         "anything",
	"objectContaining": "objectContaining",
	"arrayContaining":  "arrayContaining",
	"stringMatching":   "stringMatching",
	"any":              "any",
}

type jasmineJestASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertJasmineToJestSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeJasmineToJestAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Jasmine matcher migration required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeJasmineToJestAST(tree *jsSyntaxTree) jasmineJestASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		replacement, ok := convertJasmineCallToJest(node, tree.src)
		if ok {
			edits = append(edits, replacementEditForCall(node, replacement))
			return false
		}

		callee := jsCalleeNode(node)
		if callee == nil || callee.Type() != "member_expression" {
			return true
		}
		if jsBaseIdentifier(callee, tree.src) == "jasmine" && jsNodeText(jsMemberProperty(callee), tree.src) == "addMatchers" {
			unsupportedRows[int(node.StartPoint().Row)] = true
			return false
		}
		return true
	})

	return jasmineJestASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertJasmineCallToJest(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil {
		return "", false
	}

	if replacement, ok := convertJasmineClockCall(node, src); ok {
		return replacement, true
	}

	if callee.Type() == "identifier" && jsNodeText(callee, src) == "spyOn" {
		args := jsArgumentTexts(node, src)
		if len(args) == 2 {
			return "jest.spyOn(" + args[0] + ", " + args[1] + ")", true
		}
	}

	if callee.Type() == "member_expression" {
		if replacement, ok := convertJasmineAndChain(node, src); ok {
			return replacement, true
		}

		root := jsBaseIdentifier(callee, src)
		property := jsNodeText(jsMemberProperty(callee), src)
		args := jsArgumentTexts(node, src)
		switch root {
		case "jasmine":
			switch property {
			case "createSpy":
				return "jest.fn()", true
			case "createSpyObj":
				return convertJasmineCreateSpyObjCall(args), true
			default:
				if mapped, ok := jasmineExpectationMembers[property]; ok {
					return "expect." + mapped + "(" + strings.Join(args, ", ") + ")", true
				}
			}
		}
	}

	return "", false
}

func convertJasmineClockCall(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	object := jsMemberObject(callee)
	if object == nil || object.Type() != "call_expression" {
		return "", false
	}
	clockCallee := jsCalleeNode(object)
	if clockCallee == nil || clockCallee.Type() != "member_expression" {
		return "", false
	}
	if jsBaseIdentifier(clockCallee, src) != "jasmine" || jsNodeText(jsMemberProperty(clockCallee), src) != "clock" {
		return "", false
	}

	args := jsArgumentTexts(node, src)
	switch property {
	case "install":
		return "jest.useFakeTimers()", true
	case "uninstall":
		return "jest.useRealTimers()", true
	case "tick":
		if len(args) == 1 {
			return "jest.advanceTimersByTime(" + args[0] + ")", true
		}
	case "mockDate":
		if len(args) == 1 {
			return "jest.setSystemTime(" + args[0] + ")", true
		}
	}
	return "", false
}

func convertJasmineAndChain(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	andNode := jsMemberObject(callee)
	if andNode == nil || andNode.Type() != "member_expression" || jsNodeText(jsMemberProperty(andNode), src) != "and" {
		return "", false
	}
	baseCall := jsMemberObject(andNode)
	if baseCall == nil || baseCall.Type() != "call_expression" {
		return "", false
	}

	baseCallee := jsCalleeNode(baseCall)
	baseArgs := jsArgumentTexts(baseCall, src)
	callArgs := jsArgumentTexts(node, src)
	if baseCallee == nil {
		return "", false
	}

	if baseCallee.Type() == "identifier" && jsNodeText(baseCallee, src) == "spyOn" && len(baseArgs) == 2 {
		switch property {
		case "returnValue":
			if len(callArgs) == 1 {
				return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockReturnValue(" + callArgs[0] + ")", true
			}
		case "callFake":
			if len(callArgs) == 1 {
				return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ").mockImplementation(" + callArgs[0] + ")", true
			}
		case "callThrough":
			return "jest.spyOn(" + baseArgs[0] + ", " + baseArgs[1] + ")", true
		}
	}

	if baseCallee.Type() == "member_expression" &&
		jsBaseIdentifier(baseCallee, src) == "jasmine" &&
		jsNodeText(jsMemberProperty(baseCallee), src) == "createSpy" {
		switch property {
		case "returnValue":
			if len(callArgs) == 1 {
				return "jest.fn().mockReturnValue(" + callArgs[0] + ")", true
			}
		case "callFake":
			if len(callArgs) == 1 {
				return "jest.fn().mockImplementation(" + callArgs[0] + ")", true
			}
		}
	}

	return "", false
}

func convertJasmineCreateSpyObjCall(args []string) string {
	if len(args) < 2 {
		return "{}"
	}

	methodArg := strings.TrimSpace(args[1])
	if strings.HasPrefix(methodArg, "[") && strings.HasSuffix(methodArg, "]") {
		methodArg = strings.TrimSpace(methodArg[1 : len(methodArg)-1])
	}
	methods := splitTopLevelArgs(methodArg)
	items := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.Trim(strings.TrimSpace(method), `"'`)
		if method == "" {
			continue
		}
		items = append(items, method+": jest.fn()")
	}
	if len(items) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(items, ", ") + " }"
}

func unsupportedJasmineLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeJasmineToJestAST(tree).unsupportedRows, true
}
