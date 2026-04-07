package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var cypressWdioStructuralCallees = map[string]string{
	"describe":      "describe",
	"describe.only": "describe.only",
	"describe.skip": "describe.skip",
	"context":       "describe",
	"it":            "it",
	"it.only":       "it.only",
	"it.skip":       "it.skip",
	"specify":       "it",
	"before":        "before",
	"after":         "after",
	"beforeEach":    "beforeEach",
	"afterEach":     "afterEach",
}

type cypressWdioASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertCypressToWdioSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeCypressToWdioAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Cypress conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeCypressToWdioAST(tree *jsSyntaxTree) cypressWdioASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "call_expression":
			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := cypressWdioStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if replacement, ok := cypressSeleniumCallbackPrefix(mapped, callback); ok {
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
			if replacement, ok := convertCypressCallToWdio(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			root, _, ok := extractJSCallChain(node, tree.src)
			if ok && root == "cy" {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		case "member_expression":
			if jsBaseIdentifier(node, tree.src) == "Cypress" {
				unsupportedRows[int(node.StartPoint().Row)] = true
			}
		}
		return true
	})

	return cypressWdioASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertCypressCallToWdio(node *sitter.Node, src []byte) (string, bool) {
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "cy" || len(steps) == 0 {
		return "", false
	}

	switch steps[0].method {
	case "visit":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await browser.url(" + steps[0].args[0] + ")", true
		}
	case "reload":
		if len(steps) == 1 {
			return "await browser.refresh()", true
		}
	case "go":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			switch normalizeJSLiteral(steps[0].args[0]) {
			case "back":
				return "await browser.back()", true
			case "forward":
				return "await browser.forward()", true
			}
		}
	case "wait":
		if len(steps) == 1 && len(steps[0].args) == 1 && isNumericLiteral(steps[0].args[0]) {
			return "await browser.pause(" + steps[0].args[0] + ")", true
		}
	case "clearCookies":
		if len(steps) == 1 {
			return "await browser.deleteCookies()", true
		}
	case "getCookies":
		if len(steps) == 1 {
			return "await browser.getCookies()", true
		}
	case "clearLocalStorage":
		if len(steps) == 1 {
			return "await browser.execute(() => localStorage.clear())", true
		}
	case "log":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "console.log(" + steps[0].args[0] + ")", true
		}
	case "window":
		if len(steps) == 2 && steps[1].method == "then" && len(steps[1].args) == 1 {
			return "await browser.execute(" + steps[1].args[0] + ")", true
		}
	case "get":
		return convertCypressGetChainToWdio(steps)
	case "contains":
		return convertCypressContainsChainToWdio(steps)
	case "url":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) == 2 {
			switch normalizeJSLiteral(steps[1].args[0]) {
			case "include":
				return "await expect(browser).toHaveUrlContaining(" + steps[1].args[1] + ")", true
			case "eq":
				return "await expect(browser).toHaveUrl(" + steps[1].args[1] + ")", true
			}
		}
	case "title":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) == 2 && normalizeJSLiteral(steps[1].args[0]) == "eq" {
			return "await expect(browser).toHaveTitle(" + steps[1].args[1] + ")", true
		}
	}

	return "", false
}

func convertCypressGetChainToWdio(steps []jsCallStep) (string, bool) {
	if len(steps) == 0 || len(steps[0].args) != 1 {
		return "", false
	}
	query := "$(" + steps[0].args[0] + ")"
	remaining := steps[1:]
	if len(remaining) == 0 {
		return query, true
	}
	if len(remaining) > 2 {
		return "", false
	}

	step := remaining[0]
	switch step.method {
	case "click":
		return "await " + query + ".click()", true
	case "dblclick":
		return "await " + query + ".doubleClick()", true
	case "type":
		if len(step.args) == 1 {
			return "await " + query + ".setValue(" + step.args[0] + ")", true
		}
	case "clear":
		if len(remaining) == 2 && remaining[1].method == "type" && len(remaining[1].args) == 1 {
			return "await " + query + ".setValue(" + remaining[1].args[0] + ")", true
		}
		if len(step.args) == 0 {
			return "await " + query + ".clearValue()", true
		}
	case "select":
		if len(step.args) == 1 {
			return "await " + query + ".selectByVisibleText(" + step.args[0] + ")", true
		}
	case "check", "uncheck":
		return "await " + query + ".click()", true
	case "trigger":
		if len(step.args) == 1 && normalizeJSLiteral(step.args[0]) == "mouseover" {
			return "await " + query + ".moveTo()", true
		}
	case "invoke":
		if len(step.args) == 1 && normalizeJSLiteral(step.args[0]) == "text" {
			return "await " + query + ".getText()", true
		}
		if len(step.args) == 2 && normalizeJSLiteral(step.args[0]) == "attr" {
			return "await " + query + ".getAttribute(" + step.args[1] + ")", true
		}
	case "should":
		return convertCypressShouldToWdio(steps[0].args[0], step)
	}

	return "", false
}

func convertCypressContainsChainToWdio(steps []jsCallStep) (string, bool) {
	if len(steps) == 0 || len(steps[0].args) != 1 {
		return "", false
	}
	query, ok := cypressContainsToWdioSelector(steps[0].args[0])
	if !ok {
		return "", false
	}
	if len(steps) == 1 {
		return query, true
	}
	if len(steps) == 2 && steps[1].method == "click" {
		return "await " + query + ".click()", true
	}
	return "", false
}

func convertCypressShouldToWdio(selector string, step jsCallStep) (string, bool) {
	if len(step.args) == 0 {
		return "", false
	}
	query := "$(" + selector + ")"
	matcher := normalizeJSLiteral(step.args[0])
	switch matcher {
	case "be.visible":
		return "await expect(" + query + ").toBeDisplayed()", true
	case "not.be.visible":
		return "await expect(" + query + ").not.toBeDisplayed()", true
	case "exist":
		return "await expect(" + query + ").toExist()", true
	case "not.exist":
		return "await expect(" + query + ").not.toExist()", true
	case "have.text":
		if len(step.args) == 2 {
			return "await expect(" + query + ").toHaveText(" + step.args[1] + ")", true
		}
	case "contain", "contain.text":
		if len(step.args) == 2 {
			return "await expect(" + query + ").toHaveTextContaining(" + step.args[1] + ")", true
		}
	case "have.value":
		if len(step.args) == 2 {
			return "await expect(" + query + ").toHaveValue(" + step.args[1] + ")", true
		}
	case "have.length":
		if len(step.args) == 2 {
			return "await expect($$(" + selector + ")).toBeElementsArrayOfSize(" + step.args[1] + ")", true
		}
	case "be.checked":
		return "await expect(" + query + ").toBeSelected()", true
	case "be.enabled":
		return "await expect(" + query + ").toBeEnabled()", true
	case "be.disabled":
		return "await expect(" + query + ").toBeDisabled()", true
	case "have.attr":
		if len(step.args) == 3 {
			return "await expect(" + query + ").toHaveAttribute(" + step.args[1] + ", " + step.args[2] + ")", true
		}
	}
	return "", false
}

func cypressContainsToWdioSelector(arg string) (string, bool) {
	trimmed := strings.TrimSpace(arg)
	if len(trimmed) < 2 {
		return "", false
	}
	first := trimmed[0]
	last := trimmed[len(trimmed)-1]
	if (first == '\'' && last == '\'') || (first == '"' && last == '"') || (first == '`' && last == '`') {
		value := strings.ReplaceAll(normalizeJSLiteral(trimmed), "`", "\\`")
		return "$(`*=" + value + "`)", true
	}
	return "", false
}

func unsupportedCypressWdioLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeCypressToWdioAST(tree).unsupportedRows, true
}
