package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var testCafeUnsupportedCallNames = map[string]bool{
	"Role":           true,
	"RequestMock":    true,
	"ClientFunction": true,
	"RequestLogger":  true,
	"RequestHook":    true,
}

var testCafeUnsupportedTMethods = map[string]bool{
	"useRole":            true,
	"switchToIframe":     true,
	"switchToMainWindow": true,
	"pressKey":           true,
	"rightClick":         true,
	"eval":               true,
}

func convertTestCafeToPlaywrightSourceAST(source string) (string, bool) {
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
			if strings.Contains(module, "'testcafe'") || strings.Contains(module, "\"testcafe\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "variable_declarator":
			value := node.ChildByFieldName("value")
			if value == nil {
				return true
			}
			if testCafeIsSelectorCallExpr(value, tree.src) {
				if replacement, ok := testCafeSelectorExprToPlaywright(value, tree.src); ok {
					edits = append(edits, textEdit{
						start:       int(value.StartByte()),
						end:         int(value.EndByte()),
						replacement: replacement,
					})
					return false
				}
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			if callee != nil && callee.Type() == "identifier" && jsNodeText(callee, tree.src) == "test" {
				if callback := jsLastFunctionArg(node); callback != nil {
					if body := jsFunctionBodyNode(callback); body != nil {
						edits = append(edits, textEdit{
							start:       int(callback.StartByte()),
							end:         int(body.StartByte()),
							replacement: "async ({ page }) => ",
						})
					}
				}
				return true
			}
			if replacement, ok := convertTestCafeCallToPlaywright(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
		}
		return true
	})

	result := applyTextEdits(source, edits)
	if rows, ok := unsupportedTestCafePlaywrightLineRowsAST(source); ok && len(rows) > 0 {
		result = commentSpecificLines(result, rows, "manual TestCafe conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func convertTestCafeCallToPlaywright(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertTestCafeExpectationToPlaywright(node, src); ok {
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
			if target, ok := testCafeSelectorExprToPlaywright(argNodes[0], src); ok {
				return "await " + target + ".click()", true
			}
		}
	case "doubleClick":
		if len(argNodes) == 1 {
			if target, ok := testCafeSelectorExprToPlaywright(argNodes[0], src); ok {
				return "await " + target + ".dblclick()", true
			}
		}
	case "hover":
		if len(argNodes) == 1 {
			if target, ok := testCafeSelectorExprToPlaywright(argNodes[0], src); ok {
				return "await " + target + ".hover()", true
			}
		}
	case "typeText":
		if len(argNodes) == 2 {
			if target, ok := testCafeSelectorExprToPlaywright(argNodes[0], src); ok {
				return "await " + target + ".fill(" + argTexts[1] + ")", true
			}
		}
	case "navigateTo":
		if len(argTexts) == 1 {
			return "await page.goto(" + argTexts[0] + ")", true
		}
	case "wait":
		if len(argTexts) == 1 {
			return "await page.waitForTimeout(" + argTexts[0] + ")", true
		}
	case "takeScreenshot":
		return "await page.screenshot()", true
	case "resizeWindow":
		if len(argTexts) == 2 {
			return "await page.setViewportSize({ width: " + argTexts[0] + ", height: " + argTexts[1] + " })", true
		}
	case "setFilesToUpload":
		if len(argNodes) == 2 {
			if target, ok := testCafeSelectorExprToPlaywright(argNodes[0], src); ok {
				return "await " + target + ".setInputFiles(" + argTexts[1] + ")", true
			}
		}
	}

	return "", false
}

func convertTestCafeExpectationToPlaywright(node *sitter.Node, src []byte) (string, bool) {
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
	return testCafeExpectationExprToPlaywright(args[0], terminal, callArgs, src)
}

func testCafeExpectationExprToPlaywright(node *sitter.Node, terminal string, callArgs []string, src []byte) (string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return "", false
	}
	property := jsNodeText(jsMemberProperty(node), src)
	object := jsMemberObject(node)
	if object == nil {
		return "", false
	}
	target, ok := testCafeSelectorExprToPlaywright(object, src)
	if !ok {
		return "", false
	}

	switch property {
	case "exists":
		switch terminal {
		case "ok":
			return "await expect(" + target + ").toBeAttached()", true
		case "notOk":
			return "await expect(" + target + ").not.toBeAttached()", true
		}
	case "visible":
		switch terminal {
		case "ok":
			return "await expect(" + target + ").toBeVisible()", true
		case "notOk":
			return "await expect(" + target + ").toBeHidden()", true
		}
	case "count":
		if terminal == "eql" && len(callArgs) == 1 {
			return "await expect(" + target + ").toHaveCount(" + callArgs[0] + ")", true
		}
	case "innerText":
		switch terminal {
		case "eql":
			if len(callArgs) == 1 {
				return "await expect(" + target + ").toHaveText(" + callArgs[0] + ")", true
			}
		case "contains":
			if len(callArgs) == 1 {
				return "await expect(" + target + ").toContainText(" + callArgs[0] + ")", true
			}
		}
	case "value":
		if terminal == "eql" && len(callArgs) == 1 {
			return "await expect(" + target + ").toHaveValue(" + callArgs[0] + ")", true
		}
	}

	return "", false
}

func testCafeSelectorExprToPlaywright(node *sitter.Node, src []byte) (string, bool) {
	node = jsUnwrapAwait(node)
	if node == nil {
		return "", false
	}
	switch node.Type() {
	case "identifier":
		return jsNodeText(node, src), true
	case "string", "template_string":
		return "page.locator(" + jsNodeText(node, src) + ")", true
	}
	if node.Type() != "call_expression" {
		return "", false
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "Selector" || len(steps) == 0 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false
	}

	result := "page.locator(" + steps[0].args[0] + ")"
	for _, step := range steps[1:] {
		switch step.method {
		case "nth":
			if len(step.args) != 1 {
				return "", false
			}
			result += ".nth(" + step.args[0] + ")"
		case "find":
			if len(step.args) != 1 {
				return "", false
			}
			result += ".locator(" + step.args[0] + ")"
		case "withText":
			if len(step.args) != 1 {
				return "", false
			}
			result += ".filter({ hasText: " + step.args[0] + " })"
		default:
			return "", false
		}
	}
	return result, true
}

func testCafeIsSelectorCallExpr(node *sitter.Node, src []byte) bool {
	node = jsUnwrapAwait(node)
	if node == nil || node.Type() != "call_expression" {
		return false
	}
	root, _, ok := extractJSCallChain(node, src)
	return ok && root == "Selector"
}

func unsupportedTestCafePlaywrightLineRowsAST(source string) (map[int]bool, bool) {
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
					if _, ok := testCafeSelectorExprToPlaywright(value, tree.src); !ok {
						rows[int(value.StartPoint().Row)] = true
						return false
					}
				}
			}
		case "call_expression":
			if _, ok := convertTestCafeCallToPlaywright(node, tree.src); ok {
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
			case root == "t" && (testCafeUnsupportedTMethods[property] || property == "expect" || property == "click" || property == "doubleClick" || property == "hover" || property == "typeText" || property == "navigateTo" || property == "wait" || property == "takeScreenshot" || property == "resizeWindow" || property == "setFilesToUpload"):
				rows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return rows, true
}
