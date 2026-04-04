package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func convertWdioToCypressSourceAST(source string) (string, bool) {
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
			if strings.Contains(module, "'@wdio/globals'") ||
				strings.Contains(module, "\"@wdio/globals\"") ||
				strings.Contains(module, "'webdriverio'") ||
				strings.Contains(module, "\"webdriverio\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			if replacement, ok := convertWdioCallToCypress(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
		}
		return true
	})

	result := applyTextEdits(source, edits)
	if rows, ok := unsupportedWdioCypressLineRowsAST(source); ok && len(rows) > 0 {
		result = commentSpecificLines(result, rows, "manual WebdriverIO conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func convertWdioCallToCypress(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertWdioExpectCallToCypress(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 {
		return "", false
	}

	switch root {
	case "browser":
		return convertWdioBrowserCallToCypress(steps)
	case "$", "$$":
		return convertWdioSelectorCallToCypress(root, steps)
	}

	return "", false
}

func convertWdioBrowserCallToCypress(steps []jsCallStep) (string, bool) {
	step := steps[0]
	switch step.method {
	case "url":
		if len(steps) == 1 && len(step.args) == 1 {
			return "cy.visit(" + step.args[0] + ")", true
		}
	case "pause":
		if len(steps) == 1 && len(step.args) == 1 {
			return "cy.wait(" + step.args[0] + ")", true
		}
	case "refresh":
		if len(steps) == 1 {
			return "cy.reload()", true
		}
	case "back":
		if len(steps) == 1 {
			return "cy.go('back')", true
		}
	case "forward":
		if len(steps) == 1 {
			return "cy.go('forward')", true
		}
	case "getTitle":
		if len(steps) == 1 {
			return "cy.title()", true
		}
	case "getUrl":
		if len(steps) == 1 {
			return "cy.url()", true
		}
	case "keys":
		if len(steps) == 1 && len(step.args) == 1 {
			return "cy.get('body').type(" + step.args[0] + ")", true
		}
	case "deleteCookies":
		if len(steps) == 1 {
			return "cy.clearCookies()", true
		}
	case "getCookies":
		if len(steps) == 1 {
			return "cy.getCookies()", true
		}
	case "execute":
		if len(steps) == 1 {
			if len(step.args) == 1 && strings.Contains(step.args[0], "localStorage.clear()") {
				return "cy.clearLocalStorage()", true
			}
			return "cy.window().then(" + strings.Join(step.args, ", ") + ")", true
		}
	}
	return "", false
}

func convertWdioSelectorCallToCypress(root string, steps []jsCallStep) (string, bool) {
	if len(steps) == 0 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false
	}

	chain := wdioSelectorToCypress(root, steps[0].args[0])
	remaining := steps[1:]
	if len(remaining) == 0 {
		return chain, true
	}

	step := remaining[0]
	switch step.method {
	case "setValue":
		if len(remaining) == 1 && len(step.args) == 1 {
			return chain + ".clear().type(" + step.args[0] + ")", true
		}
	case "click":
		if len(remaining) == 1 {
			return chain + ".click()", true
		}
	case "doubleClick":
		if len(remaining) == 1 {
			return chain + ".dblclick()", true
		}
	case "clearValue":
		if len(remaining) == 1 {
			return chain + ".clear()", true
		}
	case "moveTo":
		if len(remaining) == 1 {
			return chain + ".trigger('mouseover')", true
		}
	case "getText":
		if len(remaining) == 1 {
			return chain + ".invoke('text')", true
		}
	case "isDisplayed":
		if len(remaining) == 1 {
			return chain + ".should('be.visible')", true
		}
	case "isExisting":
		if len(remaining) == 1 {
			return chain + ".should('exist')", true
		}
	case "waitForDisplayed":
		if len(remaining) == 1 {
			return chain + ".should('be.visible')", true
		}
	case "waitForExist":
		if len(remaining) == 1 {
			return chain + ".should('exist')", true
		}
	case "selectByVisibleText":
		if len(remaining) == 1 && len(step.args) == 1 {
			return chain + ".select(" + step.args[0] + ")", true
		}
	case "selectByAttribute":
		if len(remaining) == 1 && len(step.args) == 2 && normalizeJSLiteral(step.args[0]) == "value" {
			return chain + ".select(" + step.args[1] + ")", true
		}
	case "getAttribute":
		if len(remaining) == 1 && len(step.args) == 1 {
			return chain + ".invoke('attr', " + step.args[0] + ")", true
		}
	}

	return "", false
}

func convertWdioExpectCallToCypress(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	property := jsNodeText(jsMemberProperty(callee), src)
	object := jsMemberObject(callee)
	negated := false
	if object != nil && object.Type() == "member_expression" && jsNodeText(jsMemberProperty(object), src) == "not" {
		negated = true
		object = jsMemberObject(object)
	}
	if object == nil || object.Type() != "call_expression" || jsNodeText(jsCalleeNode(object), src) != "expect" {
		return "", false
	}

	argNodes := jsArgumentNodes(object)
	callArgs := jsArgumentTexts(node, src)
	if len(argNodes) != 1 {
		return "", false
	}
	target := strings.TrimSpace(jsNodeText(argNodes[0], src))

	if target == "browser" {
		switch property {
		case "toHaveUrl":
			if len(callArgs) == 1 {
				return "cy.url().should('eq', " + callArgs[0] + ")", true
			}
		case "toHaveUrlContaining":
			if len(callArgs) == 1 {
				return "cy.url().should('include', " + callArgs[0] + ")", true
			}
		case "toHaveTitle":
			if len(callArgs) == 1 {
				return "cy.title().should('eq', " + callArgs[0] + ")", true
			}
		}
		return "", false
	}

	chain, ok := wdioNodeToCypress(argNodes[0], src)
	if !ok {
		return "", false
	}

	switch property {
	case "toBeDisplayed":
		if negated {
			return chain + ".should('not.be.visible')", true
		}
		return chain + ".should('be.visible')", true
	case "toExist":
		if negated {
			return chain + ".should('not.exist')", true
		}
		return chain + ".should('exist')", true
	case "toHaveText":
		if len(callArgs) == 1 {
			return chain + ".should('have.text', " + callArgs[0] + ")", true
		}
	case "toHaveTextContaining":
		if len(callArgs) == 1 {
			return chain + ".should('contain', " + callArgs[0] + ")", true
		}
	case "toHaveValue":
		if len(callArgs) == 1 {
			return chain + ".should('have.value', " + callArgs[0] + ")", true
		}
	case "toBeElementsArrayOfSize":
		if len(callArgs) == 1 {
			return chain + ".should('have.length', " + callArgs[0] + ")", true
		}
	case "toBeSelected":
		return chain + ".should('be.checked')", true
	case "toBeEnabled":
		return chain + ".should('be.enabled')", true
	case "toBeDisabled":
		return chain + ".should('be.disabled')", true
	case "toHaveAttribute":
		if len(callArgs) == 2 {
			return chain + ".should('have.attr', " + callArgs[0] + ", " + callArgs[1] + ")", true
		}
	}

	return "", false
}

func wdioNodeToCypress(node *sitter.Node, src []byte) (string, bool) {
	node = jsUnwrapAwait(node)
	if node == nil || node.Type() != "call_expression" {
		return "", false
	}
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false
	}
	if root != "$" && root != "$$" {
		return "", false
	}
	return wdioSelectorToCypress(root, steps[0].args[0]), true
}

func wdioSelectorToCypress(root, selector string) string {
	selector = strings.TrimSpace(selector)
	switch {
	case strings.HasPrefix(selector, "'=") && strings.HasSuffix(selector, "'"):
		return "cy.contains('" + selector[2:len(selector)-1] + "')"
	case strings.HasPrefix(selector, "\"=") && strings.HasSuffix(selector, "\""):
		return "cy.contains(\"" + selector[2:len(selector)-1] + "\")"
	case strings.HasPrefix(selector, "'*=") && strings.HasSuffix(selector, "'"):
		return "cy.contains('" + selector[3:len(selector)-1] + "')"
	case strings.HasPrefix(selector, "\"*=") && strings.HasSuffix(selector, "\""):
		return "cy.contains(\"" + selector[3:len(selector)-1] + "\")"
	default:
		return "cy.get(" + selector + ")"
	}
}

func unsupportedWdioCypressLineRowsAST(source string) (map[int]bool, bool) {
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
		if _, handled := convertWdioCallToCypress(node, tree.src); handled {
			return false
		}
		root, _, ok := extractJSCallChain(node, tree.src)
		if ok && (root == "browser" || root == "$" || root == "$$") {
			rows[int(node.StartPoint().Row)] = true
			return false
		}
		return true
	})

	return rows, true
}
