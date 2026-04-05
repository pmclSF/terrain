package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type seleniumCypressASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertSeleniumToCypressSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeSeleniumToCypressAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Selenium conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeSeleniumToCypressAST(tree *jsSyntaxTree) seleniumCypressASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}
		if replacement, ok := convertSeleniumCallToCypress(node, tree.src); ok {
			edits = append(edits, replacementEditForCall(node, replacement))
			return false
		}
		if seleniumCallNeedsManualReview(node, tree.src) {
			unsupportedRows[int(node.StartPoint().Row)] = true
			return false
		}
		return true
	})

	return seleniumCypressASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertSeleniumCallToCypress(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertSeleniumExpectationToCypress(node, src); ok {
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
				return "cy.visit(" + args[0] + ")", true
			}
		case "sleep":
			if len(args) == 1 {
				return "cy.wait(" + args[0] + ")", true
			}
		case "executeScript":
			if len(args) == 1 && normalizeJSLiteral(args[0]) == "localStorage.clear()" {
				return "cy.clearLocalStorage()", true
			}
		}
	}

	if seleniumIsDriverZeroArgCall(object, src, "navigate") {
		switch property {
		case "refresh":
			return "cy.reload()", true
		case "back":
			return "cy.go('back')", true
		case "forward":
			return "cy.go('forward')", true
		}
	}

	if seleniumIsDriverZeroArgCall(object, src, "manage") && property == "deleteAllCookies" {
		return "cy.clearCookies()", true
	}

	if selector, ok := seleniumFindSelectorFromCall(object, src, "findElement"); ok {
		args := jsArgumentTexts(node, src)
		switch property {
		case "click":
			return "cy.get(" + selector + ").click()", true
		case "clear":
			return "cy.get(" + selector + ").clear()", true
		case "sendKeys":
			if len(args) == 1 {
				return "cy.get(" + selector + ").type(" + args[0] + ")", true
			}
		}
	}

	return "", false
}

func convertSeleniumExpectationToCypress(node *sitter.Node, src []byte) (string, bool) {
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

	if replacement, ok := seleniumLengthExpectationToCypress(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}
	if replacement, ok := seleniumDriverValueExpectationToCypress(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}
	if replacement, ok := seleniumElementExpectationToCypress(target, terminal, terminalArgs, src); ok {
		return replacement, true
	}

	return "", false
}

func seleniumLengthExpectationToCypress(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return "", false
	}
	if jsNodeText(jsMemberProperty(node), src) != "length" {
		return "", false
	}
	selector, ok := seleniumFindSelectorFromCall(jsMemberObject(node), src, "findElements")
	if !ok {
		return "", false
	}
	switch terminal {
	case "toBe":
		if len(terminalArgs) == 1 {
			if seleniumIsZeroLiteral(terminalArgs[0]) {
				return "cy.get(" + selector + ").should('not.exist')", true
			}
			return "cy.get(" + selector + ").should('have.length', " + terminalArgs[0] + ")", true
		}
	case "toBeGreaterThan":
		if len(terminalArgs) == 1 && seleniumIsZeroLiteral(terminalArgs[0]) {
			return "cy.get(" + selector + ").should('exist')", true
		}
	}
	return "", false
}

func seleniumDriverValueExpectationToCypress(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
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
				return "cy.url().should('include', " + terminalArgs[0] + ")", true
			}
		case "toBe":
			if len(terminalArgs) == 1 {
				return "cy.url().should('eq', " + terminalArgs[0] + ")", true
			}
		}
	case "getTitle":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			return "cy.title().should('eq', " + terminalArgs[0] + ")", true
		}
	}
	return "", false
}

func seleniumElementExpectationToCypress(node *sitter.Node, terminal string, terminalArgs []string, src []byte) (string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	selector, ok := seleniumFindSelectorFromCall(jsMemberObject(callee), src, "findElement")
	if !ok {
		return "", false
	}

	switch jsNodeText(jsMemberProperty(callee), src) {
	case "isDisplayed":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			if seleniumIsTrueLiteral(terminalArgs[0]) {
				return "cy.get(" + selector + ").should('be.visible')", true
			}
			if seleniumIsFalseLiteral(terminalArgs[0]) {
				return "cy.get(" + selector + ").should('not.be.visible')", true
			}
		}
	case "getText":
		switch terminal {
		case "toBe":
			if len(terminalArgs) == 1 {
				return "cy.get(" + selector + ").should('have.text', " + terminalArgs[0] + ")", true
			}
		case "toContain":
			if len(terminalArgs) == 1 {
				return "cy.get(" + selector + ").should('contain', " + terminalArgs[0] + ")", true
			}
		}
	case "getAttribute":
		callArgs := jsArgumentTexts(call, src)
		if terminal == "toBe" && len(callArgs) == 1 && len(terminalArgs) == 1 && normalizeJSLiteral(callArgs[0]) == "value" {
			return "cy.get(" + selector + ").should('have.value', " + terminalArgs[0] + ")", true
		}
	case "isSelected":
		if terminal == "toBe" && len(terminalArgs) == 1 && seleniumIsTrueLiteral(terminalArgs[0]) {
			return "cy.get(" + selector + ").should('be.checked')", true
		}
	case "isEnabled":
		if terminal == "toBe" && len(terminalArgs) == 1 {
			if seleniumIsTrueLiteral(terminalArgs[0]) {
				return "cy.get(" + selector + ").should('be.enabled')", true
			}
			if seleniumIsFalseLiteral(terminalArgs[0]) {
				return "cy.get(" + selector + ").should('be.disabled')", true
			}
		}
	}

	return "", false
}

func seleniumFindSelectorFromCall(node *sitter.Node, src []byte, method string) (string, bool) {
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
	if jsNodeText(jsMemberProperty(callee), src) != method {
		return "", false
	}
	args := jsArgumentNodes(call)
	if len(args) != 1 {
		return "", false
	}
	return seleniumByCSSArg(args[0], src)
}

func seleniumByCSSArg(node *sitter.Node, src []byte) (string, bool) {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return "", false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	if !seleniumIsIdentifier(jsMemberObject(callee), src, "By") {
		return "", false
	}
	if jsNodeText(jsMemberProperty(callee), src) != "css" {
		return "", false
	}
	args := jsArgumentTexts(call, src)
	if len(args) != 1 {
		return "", false
	}
	return args[0], true
}

func seleniumIsDriverZeroArgCall(node *sitter.Node, src []byte, method string) bool {
	call := seleniumUnwrapExpression(node)
	if call == nil || call.Type() != "call_expression" {
		return false
	}
	callee := jsCalleeNode(call)
	if callee == nil || callee.Type() != "member_expression" {
		return false
	}
	return seleniumIsIdentifier(jsMemberObject(callee), src, "driver") &&
		jsNodeText(jsMemberProperty(callee), src) == method &&
		len(jsArgumentNodes(call)) == 0
}

func seleniumIsIdentifier(node *sitter.Node, src []byte, name string) bool {
	node = seleniumUnwrapExpression(node)
	if node == nil {
		return false
	}
	return jsNodeText(node, src) == name
}

func seleniumUnwrapExpression(node *sitter.Node) *sitter.Node {
	for node != nil {
		switch node.Type() {
		case "await_expression", "parenthesized_expression":
			if node.NamedChildCount() == 0 {
				return node
			}
			node = node.NamedChild(0)
		default:
			return node
		}
	}
	return nil
}

func seleniumIsTrueLiteral(text string) bool {
	return strings.TrimSpace(text) == "true"
}

func seleniumIsFalseLiteral(text string) bool {
	return strings.TrimSpace(text) == "false"
}

func seleniumIsZeroLiteral(text string) bool {
	return strings.TrimSpace(text) == "0"
}

func unsupportedSeleniumCypressLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeSeleniumToCypressAST(tree).unsupportedRows, true
}

func seleniumCallNeedsManualReview(node *sitter.Node, src []byte) bool {
	callee := jsCalleeNode(node)
	if callee == nil {
		return false
	}
	if callee.Type() == "identifier" {
		if jsNodeText(callee, src) != "expect" {
			return false
		}
		args := jsArgumentNodes(node)
		if len(args) != 1 {
			return false
		}
		text := jsNodeText(args[0], src)
		return strings.Contains(text, "driver.") ||
			strings.Contains(text, "By.") ||
			strings.Contains(text, "until.") ||
			strings.Contains(text, "Actions") ||
			strings.Contains(text, "Key.")
	}
	if callee.Type() != "member_expression" {
		return false
	}

	base := jsBaseIdentifier(callee, src)
	switch base {
	case "driver", "By", "until", "Actions", "Key":
		return true
	}

	expectCall := seleniumUnwrapExpression(jsMemberObject(callee))
	if expectCall != nil && expectCall.Type() == "call_expression" {
		expectCallee := jsCalleeNode(expectCall)
		if expectCallee != nil && jsNodeText(expectCallee, src) == "expect" {
			args := jsArgumentNodes(expectCall)
			if len(args) == 1 {
				text := jsNodeText(args[0], src)
				return strings.Contains(text, "driver.") ||
					strings.Contains(text, "By.") ||
					strings.Contains(text, "until.") ||
					strings.Contains(text, "Actions") ||
					strings.Contains(text, "Key.")
			}
		}
	}
	return false
}
