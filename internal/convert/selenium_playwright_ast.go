package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var seleniumPlaywrightStructuralCallees = map[string]string{
	"describe":      "test.describe",
	"describe.only": "test.describe.only",
	"describe.skip": "test.describe.skip",
	"context":       "test.describe",
	"it":            "test",
	"it.only":       "test.only",
	"it.skip":       "test.skip",
	"specify":       "test",
	"before":        "test.beforeAll",
	"after":         "test.afterAll",
	"beforeEach":    "test.beforeEach",
	"afterEach":     "test.afterEach",
}

type seleniumPlaywrightASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertSeleniumToPlaywrightSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeSeleniumToPlaywrightAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Selenium conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeSeleniumToPlaywrightAST(tree *jsSyntaxTree) seleniumPlaywrightASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'selenium-webdriver'") ||
				strings.Contains(module, "\"selenium-webdriver\"") ||
				strings.Contains(module, "'@jest/globals'") ||
				strings.Contains(module, "\"@jest/globals\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "variable_declarator":
			name := node.ChildByFieldName("name")
			value := node.ChildByFieldName("value")
			if name != nil && jsNodeText(name, tree.src) == "driver" && value == nil {
				edits = append(edits, seleniumStatementEdit(node, ""))
				return false
			}
			if value != nil && (seleniumIsRequireCall(value, tree.src, "selenium-webdriver") || seleniumIsRequireCall(value, tree.src, "@jest/globals")) {
				edits = append(edits, seleniumStatementEdit(node, ""))
				return false
			}
		case "call_expression":
			if seleniumIsDriverSetupHook(node, tree.src) || seleniumIsDriverTeardownHook(node, tree.src) {
				edits = append(edits, seleniumStatementEdit(node, ""))
				return false
			}

			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := seleniumPlaywrightStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if replacement, ok := seleniumPlaywrightCallbackPrefix(mapped); ok {
						if body := jsFunctionBodyNode(callback); body != nil {
							edits = append(edits, textEdit{
								start:       int(callback.StartByte()),
								end:         int(body.StartByte()),
								replacement: replacement,
							})
						}
					}
				}
				return true
			}

			if replacement, ok := convertSeleniumCallToPlaywrightAST(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			if seleniumCallNeedsManualReview(node, tree.src) {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return seleniumPlaywrightASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func seleniumPlaywrightCallbackPrefix(mapped string) (string, bool) {
	if strings.HasPrefix(mapped, "test.describe") {
		return "() => ", true
	}
	return "async ({ page }) => ", true
}

func convertSeleniumCallToPlaywrightAST(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertSeleniumExpectationToPlaywrightAST(node, src); ok {
		return replacement, true
	}

	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	property := jsNodeText(jsMemberProperty(callee), src)
	object := seleniumUnwrapExpression(jsMemberObject(callee))
	if object == nil {
		return "", false
	}

	if seleniumIsIdentifier(object, src, "driver") {
		args := jsArgumentTexts(node, src)
		switch property {
		case "get":
			if len(args) == 1 {
				return "await page.goto(" + args[0] + ")", true
			}
		case "sleep":
			if len(args) == 1 {
				return "await page.waitForTimeout(" + args[0] + ")", true
			}
		case "executeScript":
			if len(args) == 1 && normalizeJSLiteral(args[0]) == "localStorage.clear()" {
				return "await page.evaluate(() => localStorage.clear())", true
			}
		}
	}

	if seleniumIsDriverZeroArgCall(object, src, "navigate") {
		switch property {
		case "refresh":
			return "await page.reload()", true
		case "back":
			return "await page.goBack()", true
		case "forward":
			return "await page.goForward()", true
		}
	}

	if seleniumIsDriverZeroArgCall(object, src, "manage") && property == "deleteAllCookies" {
		return "await page.context().clearCookies()", true
	}

	if selector, ok := seleniumFindPlaywrightSelectorFromCall(object, src, "findElement"); ok {
		args := jsArgumentTexts(node, src)
		switch property {
		case "click":
			return "await " + selector + ".click()", true
		case "clear":
			return "await " + selector + ".clear()", true
		case "sendKeys":
			if len(args) == 1 {
				return "await " + selector + ".fill(" + args[0] + ")", true
			}
		}
	}

	return "", false
}

func convertSeleniumExpectationToPlaywrightAST(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	terminal := jsNodeText(jsMemberProperty(callee), src)
	expectCall := seleniumUnwrapExpression(jsMemberObject(callee))
	if expectCall == nil || expectCall.Type() != "call_expression" {
		return "", false
	}
	expectCallee := jsCalleeNode(expectCall)
	if expectCallee == nil || jsNodeText(expectCallee, src) != "expect" {
		return "", false
	}

	expectArgs := jsArgumentNodes(expectCall)
	terminalArgs := jsArgumentTexts(node, src)
	if len(expectArgs) != 1 {
		return "", false
	}
	target := seleniumUnwrapExpression(expectArgs[0])
	if target == nil {
		return "", false
	}

	if replacement, ok := seleniumLengthExpectationToPlaywright(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}
	if replacement, ok := seleniumDriverValueExpectationToPlaywright(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}
	if replacement, ok := seleniumElementExpectationToPlaywright(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}

	return "", false
}

func seleniumLengthExpectationToPlaywright(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return "", false
	}
	if jsNodeText(jsMemberProperty(node), src) != "length" {
		return "", false
	}
	selector, ok := seleniumFindPlaywrightSelectorFromCall(jsMemberObject(node), src, "findElements")
	if !ok {
		return "", false
	}
	switch terminal {
	case "toBe":
		if len(terminalArgs) == 1 {
			if seleniumIsZeroLiteral(terminalArgs[0]) {
				return "await expect(" + selector + ").not.toBeAttached()", true
			}
			return "await expect(" + selector + ").toHaveCount(" + terminalArgs[0] + ")", true
		}
	case "toBeGreaterThan":
		if len(terminalArgs) == 1 && seleniumIsZeroLiteral(terminalArgs[0]) {
			return "await expect(" + selector + ").toBeAttached()", true
		}
	}
	return "", false
}

func seleniumDriverValueExpectationToPlaywright(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	if !seleniumIsIdentifier(jsMemberObject(callee), src, "driver") {
		return "", false
	}

	switch jsNodeText(jsMemberProperty(callee), src) {
	case "getCurrentUrl":
		switch terminal {
		case "toContain":
			if len(terminalArgs) == 1 {
				return "await expect(page).toHaveURL(new RegExp(" + terminalArgs[0] + "))", true
			}
		case "toBe":
			if len(terminalArgs) == 1 {
				return "await expect(page).toHaveURL(" + terminalArgs[0] + ")", true
			}
		}
	case "getTitle":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			return "await expect(page).toHaveTitle(" + terminalArgs[0] + ")", true
		}
	}
	return "", false
}

func seleniumElementExpectationToPlaywright(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	selector, ok := seleniumFindPlaywrightSelectorFromCall(jsMemberObject(callee), src, "findElement")
	if !ok {
		return "", false
	}

	switch jsNodeText(jsMemberProperty(callee), src) {
	case "isDisplayed":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			if seleniumIsTrueLiteral(terminalArgs[0]) {
				return "await expect(" + selector + ").toBeVisible()", true
			}
			if seleniumIsFalseLiteral(terminalArgs[0]) {
				return "await expect(" + selector + ").toBeHidden()", true
			}
		}
	case "getText":
		switch terminal {
		case "toBe":
			if len(terminalArgs) == 1 {
				return "await expect(" + selector + ").toHaveText(" + terminalArgs[0] + ")", true
			}
		case "toContain":
			if len(terminalArgs) == 1 {
				return "await expect(" + selector + ").toContainText(" + terminalArgs[0] + ")", true
			}
		}
	case "getAttribute":
		callArgs := jsArgumentTexts(call, src)
		if terminal == "toBe" && len(callArgs) == 1 && len(terminalArgs) == 1 && normalizeJSLiteral(callArgs[0]) == "value" {
			return "await expect(" + selector + ").toHaveValue(" + terminalArgs[0] + ")", true
		}
	case "isSelected":
		if terminal == "toBe" && len(terminalArgs) == 1 && seleniumIsTrueLiteral(terminalArgs[0]) {
			return "await expect(" + selector + ").toBeChecked()", true
		}
	case "isEnabled":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			if seleniumIsTrueLiteral(terminalArgs[0]) {
				return "await expect(" + selector + ").toBeEnabled()", true
			}
			if seleniumIsFalseLiteral(terminalArgs[0]) {
				return "await expect(" + selector + ").toBeDisabled()", true
			}
		}
	}

	return "", false
}

func seleniumFindPlaywrightSelectorFromCall(node *sitter.Node, src []byte, method string) (string, bool) {
	kind, selector, ok := seleniumFindSelectorArgFromCall(node, src, method)
	if !ok {
		return "", false
	}
	switch kind {
	case "css":
		return "page.locator(" + selector + ")", true
	case "xpath":
		return "page.locator('xpath=' + " + selector + ")", true
	default:
		return "", false
	}
}

func seleniumFindSelectorArgFromCall(node *sitter.Node, src []byte, method string) (string, string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", "", false
	}
	if !seleniumIsIdentifier(jsMemberObject(callee), src, "driver") {
		return "", "", false
	}
	if jsNodeText(jsMemberProperty(callee), src) != method {
		return "", "", false
	}
	args := jsArgumentNodes(call)
	if len(args) != 1 {
		return "", "", false
	}
	return seleniumByArg(args[0], src)
}

func seleniumByArg(node *sitter.Node, src []byte) (string, string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", "", false
	}
	if !seleniumIsIdentifier(jsMemberObject(callee), src, "By") {
		return "", "", false
	}
	method := jsNodeText(jsMemberProperty(callee), src)
	args := jsArgumentTexts(call, src)
	if len(args) != 1 {
		return "", "", false
	}
	switch method {
	case "css", "xpath":
		return method, args[0], true
	default:
		return "", "", false
	}
}

func seleniumIsRequireCall(node *sitter.Node, src []byte, module string) bool {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return false
	}
	callee := jsCalleeNode(call)
	if callee == nil || jsNodeText(callee, src) != "require" {
		return false
	}
	args := jsArgumentTexts(call, src)
	return len(args) == 1 && normalizeJSLiteral(args[0]) == module
}

func seleniumIsDriverSetupHook(node *sitter.Node, src []byte) bool {
	callee := jsCalleeNode(node)
	if callee == nil || jsNodeText(callee, src) != "beforeAll" {
		return false
	}
	callback := jsLastFunctionArg(node)
	if callback == nil {
		return false
	}
	body := jsFunctionBodyNode(callback)
	if body == nil {
		return false
	}
	bodyText := jsNodeText(body, src)
	return strings.Contains(bodyText, "new Builder") && strings.Contains(bodyText, ".build(") && strings.Contains(bodyText, "driver")
}

func seleniumIsDriverTeardownHook(node *sitter.Node, src []byte) bool {
	callee := jsCalleeNode(node)
	if callee == nil || jsNodeText(callee, src) != "afterAll" {
		return false
	}
	callback := jsLastFunctionArg(node)
	if callback == nil {
		return false
	}
	body := jsFunctionBodyNode(callback)
	if body == nil {
		return false
	}
	bodyText := jsNodeText(body, src)
	return strings.Contains(bodyText, "driver.quit")
}

func seleniumStatementEdit(node *sitter.Node, replacement string) textEdit {
	target := node
	for current := node; current != nil; current = current.Parent() {
		switch current.Type() {
		case "import_statement", "expression_statement", "lexical_declaration":
			target = current
			return textEdit{
				start:       int(target.StartByte()),
				end:         int(target.EndByte()),
				replacement: replacement,
			}
		case "variable_declaration":
			target = current
		}
	}
	return textEdit{
		start:       int(target.StartByte()),
		end:         int(target.EndByte()),
		replacement: replacement,
	}
}

func unsupportedSeleniumPlaywrightLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeSeleniumToPlaywrightAST(tree).unsupportedRows, true
}
