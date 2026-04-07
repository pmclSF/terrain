package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var playwrightWdioStructuralCallees = map[string]string{
	"test.describe":      "describe",
	"test.describe.only": "describe.only",
	"test.describe.skip": "describe.skip",
	"test.only":          "it.only",
	"test.skip":          "it.skip",
	"test.beforeAll":     "before",
	"test.afterAll":      "after",
	"test.beforeEach":    "beforeEach",
	"test.afterEach":     "afterEach",
	"test":               "it",
}

type playwrightWdioASTResult struct {
	source          string
	unsupportedRows map[int]bool
}

func convertPlaywrightToWdioSourceAST(source string) (playwrightWdioASTResult, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return playwrightWdioASTResult{}, false
	}
	defer tree.Close()

	analysis := analyzePlaywrightToWdioAST(tree)
	return playwrightWdioASTResult{
		source:          applyTextEdits(source, analysis.edits),
		unsupportedRows: analysis.unsupportedRows,
	}, true
}

type playwrightWdioASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func analyzePlaywrightToWdioAST(tree *jsSyntaxTree) playwrightWdioASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'@playwright/test'") || strings.Contains(module, "\"@playwright/test\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := playwrightWdioStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					body := jsFunctionBodyNode(callback)
					if body != nil {
						replacement := "async () => "
						if strings.HasPrefix(mapped, "describe") {
							replacement = "() => "
						}
						edits = append(edits, textEdit{
							start:       int(callback.StartByte()),
							end:         int(body.StartByte()),
							replacement: replacement,
						})
					}
				}
				return true
			}

			if replacement, ok := convertPlaywrightCallToWdio(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			root, _, chainOK := extractJSCallChain(node, tree.src)
			if chainOK && (strings.HasPrefix(root, "page") || root == "request" || root == "context") {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return playwrightWdioASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertPlaywrightCallToWdio(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertPlaywrightExpectToWdio(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 {
		return "", false
	}

	switch root {
	case "page":
		if replacement, ok := convertPlaywrightPageCallToWdio(steps); ok {
			return replacement, true
		}
		if replacement, ok := convertPlaywrightQueryStepsToWdio(steps); ok {
			return replacement, true
		}
	case "context":
		if replacement, ok := convertPlaywrightContextCallToWdio(steps); ok {
			return replacement, true
		}
	case "page.keyboard":
		if len(steps) == 1 && steps[0].method == "press" && len(steps[0].args) == 1 {
			return "await browser.keys([" + steps[0].args[0] + "])", true
		}
	}

	return "", false
}

func convertPlaywrightPageCallToWdio(steps []jsCallStep) (string, bool) {
	step := steps[0]
	switch step.method {
	case "goto":
		if len(steps) == 1 && len(step.args) == 1 {
			return "await browser.url(" + step.args[0] + ")", true
		}
	case "waitForTimeout":
		if len(steps) == 1 && len(step.args) == 1 {
			return "await browser.pause(" + step.args[0] + ")", true
		}
	case "evaluate":
		return "await browser.execute(" + strings.Join(step.args, ", ") + ")", true
	case "title":
		if len(steps) == 1 {
			return "await browser.getTitle()", true
		}
	case "url":
		if len(steps) == 1 {
			return "await browser.getUrl()", true
		}
	case "reload":
		if len(steps) == 1 {
			return "await browser.refresh()", true
		}
	case "goBack":
		if len(steps) == 1 {
			return "await browser.back()", true
		}
	case "goForward":
		if len(steps) == 1 {
			return "await browser.forward()", true
		}
	case "context":
		if len(steps) == 2 {
			switch steps[1].method {
			case "addCookies":
				if len(steps[1].args) == 1 {
					return "await browser.setCookies(" + steps[1].args[0] + ")", true
				}
			case "cookies":
				if len(steps[1].args) == 0 {
					return "await browser.getCookies()", true
				}
			case "clearCookies":
				if len(steps[1].args) == 0 {
					return "await browser.deleteCookies()", true
				}
			}
		}
	}
	return "", false
}

func convertPlaywrightContextCallToWdio(steps []jsCallStep) (string, bool) {
	if len(steps) != 1 {
		return "", false
	}

	switch steps[0].method {
	case "addCookies":
		if len(steps[0].args) == 1 {
			return "await browser.setCookies(" + steps[0].args[0] + ")", true
		}
	case "cookies":
		if len(steps[0].args) == 0 {
			return "await browser.getCookies()", true
		}
	case "clearCookies":
		if len(steps[0].args) == 0 {
			return "await browser.deleteCookies()", true
		}
	}

	return "", false
}

func convertPlaywrightExpectToWdio(node *sitter.Node, src []byte) (string, bool) {
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

	args := jsArgumentTexts(object, src)
	callArgs := jsArgumentTexts(node, src)
	if len(args) != 1 {
		return "", false
	}
	target := strings.TrimSpace(args[0])

	switch target {
	case "page":
		switch property {
		case "toHaveURL":
			if len(callArgs) == 1 {
				return "await expect(browser).toHaveUrl(" + callArgs[0] + ")", true
			}
		case "toHaveTitle":
			if len(callArgs) == 1 {
				return "await expect(browser).toHaveTitle(" + callArgs[0] + ")", true
			}
		}
		return "", false
	}

	switch property {
	case "toBeVisible":
		if query, ok := playwrightExprToWdio(target, false); ok {
			if negated {
				return "await expect(" + query + ").not.toBeDisplayed()", true
			}
			return "await expect(" + query + ").toBeDisplayed()", true
		}
	case "toBeHidden":
		if query, ok := playwrightExprToWdio(target, false); ok {
			return "await expect(" + query + ").not.toBeDisplayed()", true
		}
	case "toBeAttached":
		if query, ok := playwrightExprToWdio(target, false); ok {
			if negated {
				return "await expect(" + query + ").not.toExist()", true
			}
			return "await expect(" + query + ").toExist()", true
		}
	case "toHaveText":
		if len(callArgs) == 1 {
			if query, ok := playwrightExprToWdio(target, false); ok {
				return "await expect(" + query + ").toHaveText(" + callArgs[0] + ")", true
			}
		}
	case "toContainText":
		if len(callArgs) == 1 {
			if query, ok := playwrightExprToWdio(target, false); ok {
				return "await expect(" + query + ").toHaveTextContaining(" + callArgs[0] + ")", true
			}
		}
	case "toHaveValue":
		if len(callArgs) == 1 {
			if query, ok := playwrightExprToWdio(target, false); ok {
				return "await expect(" + query + ").toHaveValue(" + callArgs[0] + ")", true
			}
		}
	case "toHaveCount":
		if len(callArgs) == 1 {
			if query, ok := playwrightExprToWdio(target, true); ok {
				return "await expect(" + query + ").toBeElementsArrayOfSize(" + callArgs[0] + ")", true
			}
		}
	case "toBeChecked":
		if query, ok := playwrightExprToWdio(target, false); ok {
			return "await expect(" + query + ").toBeSelected()", true
		}
	case "toBeEnabled":
		if query, ok := playwrightExprToWdio(target, false); ok {
			return "await expect(" + query + ").toBeEnabled()", true
		}
	case "toBeDisabled":
		if query, ok := playwrightExprToWdio(target, false); ok {
			return "await expect(" + query + ").toBeDisabled()", true
		}
	case "toHaveAttribute":
		if len(callArgs) == 2 {
			if query, ok := playwrightExprToWdio(target, false); ok {
				return "await expect(" + query + ").toHaveAttribute(" + callArgs[0] + ", " + callArgs[1] + ")", true
			}
		}
	}

	return "", false
}

func convertPlaywrightQueryStepsToWdio(steps []jsCallStep) (string, bool) {
	query, remaining, ok := basePlaywrightQueryToWdio(steps, false)
	if !ok {
		return "", false
	}
	if len(remaining) == 0 {
		return query, true
	}
	if len(remaining) > 1 {
		return "", false
	}

	step := remaining[0]
	switch step.method {
	case "fill":
		if len(step.args) == 1 {
			return "await " + query + ".setValue(" + step.args[0] + ")", true
		}
	case "click":
		return "await " + query + ".click()", true
	case "dblclick":
		return "await " + query + ".doubleClick()", true
	case "hover":
		return "await " + query + ".moveTo()", true
	case "textContent":
		return "await " + query + ".getText()", true
	case "isVisible":
		return "await " + query + ".isDisplayed()", true
	case "waitFor":
		if len(step.args) == 0 {
			return "await " + query + ".waitForDisplayed()", true
		}
		if len(step.args) == 1 && strings.Contains(step.args[0], "visible") {
			return "await " + query + ".waitForDisplayed()", true
		}
	case "clear":
		return "await " + query + ".clearValue()", true
	case "selectOption":
		if len(step.args) == 1 {
			if strings.Contains(step.args[0], "label:") {
				if label, ok := parseNamedObjectField(step.args[0], "label"); ok {
					return "await " + query + ".selectByVisibleText(" + label + ")", true
				}
			}
			return "await " + query + ".selectByAttribute('value', " + step.args[0] + ")", true
		}
	case "check", "uncheck":
		return "await " + query + ".click()", true
	}

	return "", false
}

func basePlaywrightQueryToWdio(steps []jsCallStep, plural bool) (string, []jsCallStep, bool) {
	if len(steps) == 0 {
		return "", nil, false
	}
	step := steps[0]
	switch step.method {
	case "locator":
		if len(step.args) != 1 {
			return "", nil, false
		}
		if plural {
			return "$$(" + step.args[0] + ")", steps[1:], true
		}
		return "$(" + step.args[0] + ")", steps[1:], true
	case "getByText":
		if len(step.args) != 1 {
			return "", nil, false
		}
		text := normalizeJSLiteral(step.args[0])
		return "$(`*=" + text + "`)", steps[1:], true
	}
	return "", nil, false
}

func playwrightExprToWdio(expr string, plural bool) (string, bool) {
	expr = strings.TrimSpace(expr)
	switch {
	case strings.HasPrefix(expr, "page.locator(") && strings.HasSuffix(expr, ")"):
		inner := strings.TrimSpace(expr[len("page.locator(") : len(expr)-1])
		if plural {
			return "$$(" + inner + ")", true
		}
		return "$(" + inner + ")", true
	case strings.HasPrefix(expr, "page.getByText(") && strings.HasSuffix(expr, ")"):
		inner := strings.TrimSpace(expr[len("page.getByText(") : len(expr)-1])
		return "$(`*=" + normalizeJSLiteral(inner) + "`)", true
	default:
		return "", false
	}
}

func unsupportedPlaywrightWdioLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzePlaywrightToWdioAST(tree).unsupportedRows, true
}

func parseNamedObjectField(arg, field string) (string, bool) {
	pattern := field + ":"
	idx := strings.Index(arg, pattern)
	if idx < 0 {
		return "", false
	}
	value := strings.TrimSpace(arg[idx+len(pattern):])
	value = strings.TrimRight(strings.TrimSpace(strings.TrimSuffix(value, "}")), ",")
	if value == "" {
		return "", false
	}
	return value, true
}
