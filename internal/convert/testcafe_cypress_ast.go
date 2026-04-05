package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var testCafeCypressTrackedTMethods = map[string]bool{
	"expect":           true,
	"click":            true,
	"doubleClick":      true,
	"hover":            true,
	"typeText":         true,
	"navigateTo":       true,
	"wait":             true,
	"takeScreenshot":   true,
	"resizeWindow":     true,
	"setFilesToUpload": true,
}

func convertTestCafeToCypressSourceAST(source string) (string, string, string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", "", "", false
	}
	defer tree.Close()

	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	suiteName, pageURL := "", ""
	root := tree.tree.RootNode()
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child == nil || child.Type() != "expression_statement" {
			continue
		}
		if extractedSuiteName, extractedPageURL, ok := testCafeFixtureStatementInfo(child, tree.src); ok {
			suiteName = extractedSuiteName
			pageURL = extractedPageURL
			edits = append(edits, textEdit{
				start: int(child.StartByte()),
				end:   int(child.EndByte()),
			})
			break
		}
	}

	walkJSNodes(root, func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'testcafe'") || strings.Contains(module, "\"testcafe\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "variable_declarator":
			value := node.ChildByFieldName("value")
			if value == nil || !testCafeIsSelectorCallExpr(value, tree.src) {
				return true
			}
			if replacement, ok := testCafeSelectorExprToCypressValue(value, tree.src); ok {
				edits = append(edits, textEdit{
					start:       int(value.StartByte()),
					end:         int(value.EndByte()),
					replacement: replacement,
				})
				return false
			}
			unsupportedRows[int(value.StartPoint().Row)] = true
			return false
		case "call_expression":
			callee := jsCalleeNode(node)
			if callee != nil && callee.Type() == "identifier" && jsNodeText(callee, tree.src) == "test" {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: "it",
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if body := jsFunctionBodyNode(callback); body != nil {
						edits = append(edits, textEdit{
							start:       int(callback.StartByte()),
							end:         int(body.StartByte()),
							replacement: "() => ",
						})
					}
				}
				return true
			}
			if replacement, ok := convertTestCafeCallToCypress(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			if callee == nil {
				return true
			}
			if callee.Type() == "identifier" {
				name := jsNodeText(callee, tree.src)
				if testCafeUnsupportedCallNames[name] {
					unsupportedRows[int(node.StartPoint().Row)] = true
					return false
				}
				return true
			}
			if callee.Type() != "member_expression" {
				return true
			}
			root := jsBaseIdentifier(callee, tree.src)
			property := jsNodeText(jsMemberProperty(callee), tree.src)
			if root == "t" && (testCafeUnsupportedTMethods[property] || testCafeCypressTrackedTMethods[property]) {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	result := applyTextEdits(source, edits)
	if len(unsupportedRows) > 0 {
		result = commentSpecificLines(result, unsupportedRows, "manual TestCafe conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), suiteName, pageURL, true
}

func convertTestCafeCallToCypress(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertTestCafeExpectationToCypress(node, src); ok {
		return replacement, true
	}

	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	if jsBaseIdentifier(callee, src) != "t" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	argNodes := jsArgumentNodes(node)
	argTexts := jsArgumentTexts(node, src)
	switch property {
	case "click":
		if len(argNodes) == 1 {
			if target, ok := testCafeSelectorExprToCypressChain(argNodes[0], src); ok {
				return target + ".click()", true
			}
		}
	case "doubleClick":
		if len(argNodes) == 1 {
			if target, ok := testCafeSelectorExprToCypressChain(argNodes[0], src); ok {
				return target + ".dblclick()", true
			}
		}
	case "hover":
		if len(argNodes) == 1 {
			if target, ok := testCafeSelectorExprToCypressChain(argNodes[0], src); ok {
				return target + ".trigger('mouseover')", true
			}
		}
	case "typeText":
		if len(argNodes) == 2 {
			if target, ok := testCafeSelectorExprToCypressChain(argNodes[0], src); ok {
				return target + ".type(" + argTexts[1] + ")", true
			}
		}
	case "navigateTo":
		if len(argTexts) == 1 {
			return "cy.visit(" + argTexts[0] + ")", true
		}
	case "wait":
		if len(argTexts) == 1 {
			return "cy.wait(" + argTexts[0] + ")", true
		}
	case "takeScreenshot":
		return "cy.screenshot()", true
	}

	return "", false
}

func convertTestCafeExpectationToCypress(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	terminal := jsNodeText(jsMemberProperty(callee), src)
	expectCall := jsMemberObject(callee)
	if expectCall == nil || expectCall.Type() != "call_expression" {
		return "", false
	}
	expectCallee := jsCalleeNode(expectCall)
	if expectCallee == nil || expectCallee.Type() != "member_expression" {
		return "", false
	}
	if jsBaseIdentifier(expectCallee, src) != "t" || jsNodeText(jsMemberProperty(expectCallee), src) != "expect" {
		return "", false
	}

	args := jsArgumentNodes(expectCall)
	if len(args) != 1 {
		return "", false
	}
	callArgs := jsArgumentTexts(node, src)
	return testCafeExpectationExprToCypress(args[0], terminal, callArgs, src)
}

func testCafeExpectationExprToCypress(node *sitter.Node, terminal string, callArgs []string, src []byte) (string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return "", false
	}
	property := jsNodeText(jsMemberProperty(node), src)
	object := jsMemberObject(node)
	if object == nil {
		return "", false
	}
	target, ok := testCafeSelectorExprToCypressChain(object, src)
	if !ok {
		return "", false
	}

	switch property {
	case "exists":
		switch terminal {
		case "ok":
			return target + ".should('exist')", true
		case "notOk":
			return target + ".should('not.exist')", true
		}
	case "visible":
		switch terminal {
		case "ok":
			return target + ".should('be.visible')", true
		case "notOk":
			return target + ".should('not.be.visible')", true
		}
	case "count":
		if terminal == "eql" && len(callArgs) == 1 {
			return target + ".should('have.length', " + callArgs[0] + ")", true
		}
	case "innerText":
		switch terminal {
		case "eql":
			if len(callArgs) == 1 {
				return target + ".should('have.text', " + callArgs[0] + ")", true
			}
		case "contains":
			if len(callArgs) == 1 {
				return target + ".should('contain', " + callArgs[0] + ")", true
			}
		}
	case "value":
		if terminal == "eql" && len(callArgs) == 1 {
			return target + ".should('have.value', " + callArgs[0] + ")", true
		}
	}

	return "", false
}

func testCafeSelectorExprToCypressChain(node *sitter.Node, src []byte) (string, bool) {
	node = jsUnwrapAwait(node)
	if node == nil {
		return "", false
	}
	switch node.Type() {
	case "identifier":
		return "cy.get(" + jsNodeText(node, src) + ")", true
	case "string", "template_string":
		return "cy.get(" + jsNodeText(node, src) + ")", true
	}
	if node.Type() != "call_expression" {
		return "", false
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "Selector" || len(steps) == 0 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false
	}

	result := "cy.get(" + steps[0].args[0] + ")"
	baseOnly := true
	for _, step := range steps[1:] {
		switch step.method {
		case "nth":
			if len(step.args) != 1 {
				return "", false
			}
			result += ".eq(" + step.args[0] + ")"
		case "find":
			if len(step.args) != 1 {
				return "", false
			}
			result += ".find(" + step.args[0] + ")"
			baseOnly = false
		case "withText":
			if len(step.args) != 1 {
				return "", false
			}
			if baseOnly {
				result = "cy.contains(" + steps[0].args[0] + ", " + step.args[0] + ")"
			} else {
				result += ".contains(" + step.args[0] + ")"
			}
		default:
			return "", false
		}
	}
	return result, true
}

func testCafeSelectorExprToCypressValue(node *sitter.Node, src []byte) (string, bool) {
	node = jsUnwrapAwait(node)
	if node == nil || node.Type() != "call_expression" {
		return "", false
	}
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "Selector" || len(steps) != 1 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false
	}
	return steps[0].args[0], true
}

func unsupportedTestCafeCypressLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	rows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "variable_declarator":
			value := node.ChildByFieldName("value")
			if value != nil && value.Type() == "call_expression" {
				root, _, ok := extractJSCallChain(value, tree.src)
				if ok && root == "Selector" {
					if _, ok := testCafeSelectorExprToCypressValue(value, tree.src); !ok {
						rows[int(value.StartPoint().Row)] = true
						return false
					}
				}
			}
		case "call_expression":
			if _, ok := convertTestCafeCallToCypress(node, tree.src); ok {
				return false
			}
			callee := jsCalleeNode(node)
			if callee == nil {
				return true
			}
			if callee.Type() == "identifier" {
				name := jsNodeText(callee, tree.src)
				if testCafeUnsupportedCallNames[name] {
					rows[int(node.StartPoint().Row)] = true
					return false
				}
				return true
			}
			if callee.Type() != "member_expression" {
				return true
			}
			root := jsBaseIdentifier(callee, tree.src)
			property := jsNodeText(jsMemberProperty(callee), tree.src)
			switch {
			case root == "t" && (testCafeUnsupportedTMethods[property] || property == "expect" || property == "click" || property == "doubleClick" || property == "hover" || property == "typeText" || property == "navigateTo" || property == "wait" || property == "takeScreenshot"):
				rows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return rows, true
}
