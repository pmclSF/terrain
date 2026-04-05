package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var playwrightPuppeteerStructuralCallees = map[string]string{
	"test.describe":      "describe",
	"test.describe.only": "describe.only",
	"test.describe.skip": "describe.skip",
	"test.only":          "it.only",
	"test.skip":          "it.skip",
	"test.beforeAll":     "beforeAll",
	"test.afterAll":      "afterAll",
	"test.beforeEach":    "beforeEach",
	"test.afterEach":     "afterEach",
	"test":               "it",
}

type playwrightPuppeteerASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertPlaywrightToPuppeteerSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzePlaywrightToPuppeteerAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Playwright conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzePlaywrightToPuppeteerAST(tree *jsSyntaxTree) playwrightPuppeteerASTAnalysis {
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
			if mapped, ok := playwrightPuppeteerStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if replacement, ok := playwrightPuppeteerCallbackPrefix(mapped); ok {
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

			if replacement, ok := convertPlaywrightCallToPuppeteer(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}

			root, steps, ok := extractJSCallChain(node, tree.src)
			if !ok || len(steps) == 0 {
				return true
			}
			switch root {
			case "page":
				switch steps[0].method {
				case "route", "getByText", "getByRole", "getByTestId", "context":
					unsupportedRows[int(node.StartPoint().Row)] = true
					return false
				}
			case "request", "download", "context":
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return playwrightPuppeteerASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func playwrightPuppeteerCallbackPrefix(mapped string) (string, bool) {
	if strings.HasPrefix(mapped, "describe") {
		return "() => ", true
	}
	return "async () => ", true
}

func convertPlaywrightCallToPuppeteer(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertPlaywrightExpectationToPuppeteer(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 {
		return "", false
	}

	switch root {
	case "page":
		switch steps[0].method {
		case "goto":
			if len(steps) == 1 && len(steps[0].args) == 1 {
				return "await page.goto(" + steps[0].args[0] + ")", true
			}
		case "reload":
			if len(steps) == 1 {
				return "await page.reload()", true
			}
		case "goBack":
			if len(steps) == 1 {
				return "await page.goBack()", true
			}
		case "goForward":
			if len(steps) == 1 {
				return "await page.goForward()", true
			}
		case "setViewportSize":
			if len(steps) == 1 && len(steps[0].args) == 1 {
				return "await page.setViewport(" + steps[0].args[0] + ")", true
			}
		case "context":
			if len(steps) == 2 {
				switch steps[1].method {
				case "addCookies":
					if len(steps[1].args) == 1 {
						return "await page.setCookie(..." + steps[1].args[0] + ")", true
					}
				case "cookies":
					if len(steps[1].args) == 0 {
						return "await page.cookies()", true
					}
				case "clearCookies":
					if len(steps[1].args) == 0 {
						return "await page.deleteCookie(...(await page.cookies()))", true
					}
				}
			}
		case "locator":
			return convertPlaywrightLocatorChainToPuppeteer(steps)
		}
	case "context":
		return convertPlaywrightContextCallToPuppeteer(steps)
	}

	return "", false
}

func convertPlaywrightContextCallToPuppeteer(steps []jsCallStep) (string, bool) {
	if len(steps) != 1 {
		return "", false
	}

	switch steps[0].method {
	case "addCookies":
		if len(steps[0].args) == 1 {
			return "await page.setCookie(..." + steps[0].args[0] + ")", true
		}
	case "cookies":
		if len(steps[0].args) == 0 {
			return "await page.cookies()", true
		}
	case "clearCookies":
		if len(steps[0].args) == 0 {
			return "await page.deleteCookie(...(await page.cookies()))", true
		}
	}

	return "", false
}

func convertPlaywrightExpectationToPuppeteer(node *sitter.Node, src []byte) (string, bool) {
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

	argsNode := jsArgumentsNode(object)
	if argsNode == nil || argsNode.NamedChildCount() != 1 {
		return "", false
	}
	targetNode := argsNode.NamedChild(0)
	targetText := strings.TrimSpace(jsNodeText(targetNode, src))
	callArgs := jsArgumentTexts(node, src)

	switch property {
	case "toHaveURL":
		if targetText == "page" && len(callArgs) == 1 {
			return puppeteerExpectationAssertion("page.url()", callArgs[0]), true
		}
	case "toHaveTitle":
		if targetText == "page" && len(callArgs) == 1 {
			return puppeteerExpectationAssertion("await page.title()", callArgs[0]), true
		}
	}

	selector, ok := playwrightLocatorSelectorFromNode(targetNode, src)
	if !ok {
		return "", false
	}

	switch property {
	case "toBeVisible":
		return "expect(await page.$(" + selector + ")).toBeTruthy()", true
	case "toBeHidden":
		return "expect(await page.$(" + selector + ")).toBeFalsy()", true
	case "toBeAttached":
		if negated {
			return "expect(await page.$(" + selector + ")).toBeFalsy()", true
		}
		return "expect(await page.$(" + selector + ")).toBeTruthy()", true
	case "toHaveText":
		if len(callArgs) == 1 {
			return "expect(await page.$eval(" + selector + ", el => el.textContent)).toBe(" + callArgs[0] + ")", true
		}
	case "toContainText":
		if len(callArgs) == 1 {
			return "expect(await page.$eval(" + selector + ", el => el.textContent)).toContain(" + callArgs[0] + ")", true
		}
	case "toHaveValue":
		if len(callArgs) == 1 {
			return "expect(await page.$eval(" + selector + ", el => el.value)).toBe(" + callArgs[0] + ")", true
		}
	case "toHaveCount":
		if len(callArgs) == 1 {
			return "expect((await page.$$(" + selector + ")).length).toBe(" + callArgs[0] + ")", true
		}
	case "toBeChecked":
		return "expect(await page.$eval(" + selector + ", el => el.checked)).toBe(true)", true
	case "toHaveAttribute":
		if len(callArgs) == 2 {
			return "expect(await page.$eval(" + selector + ", (el, a) => el.getAttribute(a), " + callArgs[0] + ")).toBe(" + callArgs[1] + ")", true
		}
	}

	return "", false
}

func convertPlaywrightLocatorChainToPuppeteer(steps []jsCallStep) (string, bool) {
	if len(steps) == 0 || len(steps[0].args) != 1 {
		return "", false
	}
	selector := steps[0].args[0]
	if len(steps) == 1 {
		return "page.$(" + selector + ")", true
	}

	switch steps[1].method {
	case "fill":
		if len(steps) == 2 && len(steps[1].args) == 1 {
			return "await page.type(" + selector + ", " + steps[1].args[0] + ")", true
		}
	case "click":
		if len(steps) == 2 {
			return "await page.click(" + selector + ")", true
		}
	case "dblclick":
		if len(steps) == 2 {
			return "await page.click(" + selector + ", { clickCount: 2 })", true
		}
	case "hover":
		if len(steps) == 2 {
			return "await page.hover(" + selector + ")", true
		}
	case "textContent":
		if len(steps) == 2 {
			return "await page.$eval(" + selector + ", el => el.textContent)", true
		}
	case "isVisible":
		if len(steps) == 2 {
			return "!!(await page.$(" + selector + "))", true
		}
	case "waitFor":
		if len(steps) == 2 {
			return "await page.waitForSelector(" + selector + ")", true
		}
	case "evaluate":
		if len(steps) == 2 && len(steps[1].args) == 1 {
			return "await page.$eval(" + selector + ", " + steps[1].args[0] + ")", true
		}
	case "evaluateAll":
		if len(steps) == 2 && len(steps[1].args) == 1 {
			return "await page.$$eval(" + selector + ", " + steps[1].args[0] + ")", true
		}
	case "selectOption":
		if len(steps) == 2 && len(steps[1].args) == 1 {
			return "await page.select(" + selector + ", " + steps[1].args[0] + ")", true
		}
	case "clear":
		if len(steps) == 2 {
			return puppeteerClearSelector(selector), true
		}
	}

	return "", false
}

func playwrightLocatorSelectorFromNode(node *sitter.Node, src []byte) (string, bool) {
	node = jsUnwrapAwait(node)
	if node == nil || node.Type() != "call_expression" {
		return "", false
	}
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "page" || len(steps) == 0 || steps[0].method != "locator" || len(steps[0].args) != 1 || len(steps) != 1 {
		return "", false
	}
	return steps[0].args[0], true
}

func unsupportedPlaywrightPuppeteerLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzePlaywrightToPuppeteerAST(tree).unsupportedRows, true
}

func puppeteerClearSelector(selector string) string {
	return "await page.$eval(" + selector + ", el => { el.value = ''; el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); })"
}

func jsUnwrapAwait(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == "await_expression" && node.NamedChildCount() > 0 {
		return node.NamedChild(0)
	}
	return node
}
