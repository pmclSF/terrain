package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type wdioPlaywrightASTResult struct {
	source          string
	retryWarning    bool
	unsupportedRows map[int]bool
}

func convertWdioToPlaywrightSourceAST(source string) (wdioPlaywrightASTResult, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return wdioPlaywrightASTResult{}, false
	}
	defer tree.Close()

	analysis := analyzeWdioToPlaywrightAST(tree)
	return wdioPlaywrightASTResult{
		source:          applyTextEdits(source, analysis.edits),
		retryWarning:    analysis.retryWarning,
		unsupportedRows: analysis.unsupportedRows,
	}, true
}

type wdioPlaywrightASTAnalysis struct {
	edits           []textEdit
	retryWarning    bool
	unsupportedRows map[int]bool
}

func analyzeWdioToPlaywrightAST(tree *jsSyntaxTree) wdioPlaywrightASTAnalysis {
	edits := make([]textEdit, 0, 16)
	warn := false
	unsupportedRows := map[int]bool{}

	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'@wdio/globals'") ||
				strings.Contains(module, "\"@wdio/globals\"") ||
				strings.Contains(module, "'webdriverio'") ||
				strings.Contains(module, "\"webdriverio\"") ||
				strings.Contains(module, "'@playwright/test'") ||
				strings.Contains(module, "\"@playwright/test\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := cypressPlaywrightStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					body := jsFunctionBodyNode(callback)
					if body != nil {
						replacement := "async ({ page }) => "
						if strings.HasPrefix(mapped, "test.describe") {
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

			if replacement, replacementWarn, ok := convertWdioCallToPlaywright(node, tree.src); ok {
				warn = warn || replacementWarn
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			root, _, ok := extractJSCallChain(node, tree.src)
			if ok && (root == "browser" || root == "$" || root == "$$") {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return wdioPlaywrightASTAnalysis{
		edits:           edits,
		retryWarning:    warn,
		unsupportedRows: unsupportedRows,
	}
}

func convertWdioCallToPlaywright(node *sitter.Node, src []byte) (string, bool, bool) {
	if replacement, warn, ok := convertWdioExpectCall(node, src); ok {
		return replacement, warn, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 {
		return "", false, false
	}

	switch root {
	case "browser":
		return convertWdioBrowserCall(steps)
	case "$", "$$":
		return convertWdioSelectorCall(root, steps)
	}

	return "", false, false
}

func convertWdioBrowserCall(steps []jsCallStep) (string, bool, bool) {
	step := steps[0]
	switch step.method {
	case "url":
		if len(steps) == 1 && len(step.args) == 1 {
			return "await page.goto(" + step.args[0] + ")", false, true
		}
	case "pause":
		if len(steps) == 1 && len(step.args) == 1 && isNumericLiteral(strings.TrimSpace(step.args[0])) {
			return "await page.waitForTimeout(" + strings.TrimSpace(step.args[0]) + ")", false, true
		}
	case "refresh":
		if len(steps) == 1 {
			return "await page.reload()", false, true
		}
	case "back":
		if len(steps) == 1 {
			return "await page.goBack()", false, true
		}
	case "forward":
		if len(steps) == 1 {
			return "await page.goForward()", false, true
		}
	case "getTitle":
		if len(steps) == 1 {
			return "await page.title()", false, true
		}
	case "getUrl":
		if len(steps) == 1 {
			return "page.url()", false, true
		}
	case "keys":
		if len(steps) == 1 && len(step.args) == 1 {
			if replacement, ok := wdioBrowserKeysArgToPlaywright(step.args[0]); ok {
				return replacement, false, true
			}
		}
	case "setCookies":
		if len(steps) == 1 && len(step.args) == 1 {
			if cookies, ok := wdioCookieArgToPlaywright(step.args[0]); ok {
				return "await page.context().addCookies(" + cookies + ")", false, true
			}
		}
	case "getCookies":
		if len(steps) == 1 && len(step.args) == 0 {
			return "await page.context().cookies()", false, true
		}
	case "deleteCookies":
		if len(steps) == 1 && len(step.args) == 0 {
			return "await page.context().clearCookies()", false, true
		}
	case "execute":
		args := strings.Join(step.args, ", ")
		return "await page.evaluate(" + args + ")", false, true
	}
	return "", false, false
}

func convertWdioSelectorCall(root string, steps []jsCallStep) (string, bool, bool) {
	if len(steps) == 0 || steps[0].method != "" || len(steps[0].args) != 1 {
		return "", false, false
	}

	locator := wdioSelectorToPlaywright(root, steps[0].args[0])
	remaining := steps[1:]
	if len(remaining) == 0 {
		return locator, false, true
	}

	step := remaining[0]
	switch step.method {
	case "setValue":
		if len(remaining) == 1 && len(step.args) == 1 {
			return "await " + locator + ".fill(" + step.args[0] + ")", false, true
		}
	case "click":
		if len(remaining) == 1 {
			return "await " + locator + ".click()", false, true
		}
	case "doubleClick":
		if len(remaining) == 1 {
			return "await " + locator + ".dblclick()", false, true
		}
	case "clearValue":
		if len(remaining) == 1 {
			return "await " + locator + ".clear()", false, true
		}
	case "moveTo":
		if len(remaining) == 1 {
			return "await " + locator + ".hover()", false, true
		}
	case "getText":
		if len(remaining) == 1 {
			return "await " + locator + ".textContent()", false, true
		}
	case "isDisplayed":
		if len(remaining) == 1 {
			return "await " + locator + ".isVisible()", false, true
		}
	case "isExisting":
		if len(remaining) == 1 {
			return "await " + locator + ".isVisible()", false, true
		}
	case "waitForDisplayed":
		if len(remaining) == 1 {
			return "await " + locator + ".waitFor({ state: 'visible' })", false, true
		}
	case "waitForExist":
		if len(remaining) == 1 {
			return "await " + locator + ".waitFor()", false, true
		}
	case "selectByVisibleText":
		if len(remaining) == 1 && len(step.args) == 1 {
			return "await " + locator + ".selectOption({ label: " + step.args[0] + " })", false, true
		}
	case "selectByAttribute":
		if len(remaining) == 1 && len(step.args) == 2 && normalizeJSLiteral(step.args[0]) == "value" {
			return "await " + locator + ".selectOption(" + step.args[1] + ")", false, true
		}
	case "getAttribute":
		if len(remaining) == 1 && len(step.args) == 1 {
			return "await " + locator + ".getAttribute(" + step.args[0] + ")", false, true
		}
	}

	return "", false, false
}

func convertWdioExpectCall(node *sitter.Node, src []byte) (string, bool, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false, false
	}
	property := jsNodeText(jsMemberProperty(callee), src)
	object := jsMemberObject(callee)
	negated := false
	if object != nil && object.Type() == "member_expression" && jsNodeText(jsMemberProperty(object), src) == "not" {
		negated = true
		object = jsMemberObject(object)
	}
	if object == nil || object.Type() != "call_expression" || jsNodeText(jsCalleeNode(object), src) != "expect" {
		return "", false, false
	}

	args := jsArgumentTexts(object, src)
	if len(args) != 1 {
		return "", false, false
	}
	target := strings.TrimSpace(args[0])
	callArgs := jsArgumentTexts(node, src)

	switch target {
	case "browser":
		switch property {
		case "toHaveUrl":
			if len(callArgs) == 1 {
				return "await expect(page).toHaveURL(" + callArgs[0] + ")", false, true
			}
		case "toHaveUrlContaining":
			if len(callArgs) == 1 {
				return "await expect(page).toHaveURL(new RegExp(" + callArgs[0] + "))", false, true
			}
		case "toHaveTitle":
			if len(callArgs) == 1 {
				return "await expect(page).toHaveTitle(" + callArgs[0] + ")", false, true
			}
		}
		return "", false, false
	}

	locator, ok := wdioExprToPlaywright(target)
	if !ok {
		return "", false, false
	}

	switch property {
	case "toBeDisplayed":
		if negated {
			return "await expect(" + locator + ").toBeHidden()", true, true
		}
		return "await expect(" + locator + ").toBeVisible()", true, true
	case "toExist":
		if negated {
			return "await expect(" + locator + ").not.toBeAttached()", true, true
		}
		return "await expect(" + locator + ").toBeAttached()", true, true
	case "toHaveTextContaining":
		if len(callArgs) == 1 {
			return "await expect(" + locator + ").toContainText(" + callArgs[0] + ")", true, true
		}
	case "toHaveText":
		if len(callArgs) == 1 {
			return "await expect(" + locator + ").toHaveText(" + callArgs[0] + ")", true, true
		}
	case "toHaveValue":
		if len(callArgs) == 1 {
			return "await expect(" + locator + ").toHaveValue(" + callArgs[0] + ")", true, true
		}
	case "toBeElementsArrayOfSize":
		if len(callArgs) == 1 {
			return "await expect(" + locator + ").toHaveCount(" + callArgs[0] + ")", true, true
		}
	case "toBeSelected":
		return "await expect(" + locator + ").toBeChecked()", true, true
	case "toBeEnabled":
		return "await expect(" + locator + ").toBeEnabled()", true, true
	case "toBeDisabled":
		return "await expect(" + locator + ").toBeDisabled()", true, true
	case "toHaveAttribute":
		if len(callArgs) == 2 {
			return "await expect(" + locator + ").toHaveAttribute(" + callArgs[0] + ", " + callArgs[1] + ")", true, true
		}
	}

	return "", false, false
}

func wdioExprToPlaywright(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	switch {
	case strings.HasPrefix(expr, "$$("):
		return wdioSelectorToPlaywright("$$", expr[3:len(expr)-1]), true
	case strings.HasPrefix(expr, "$("):
		return wdioSelectorToPlaywright("$", expr[2:len(expr)-1]), true
	default:
		return "", false
	}
}

func wdioSelectorToPlaywright(root, selector string) string {
	selector = strings.TrimSpace(selector)
	switch {
	case strings.HasPrefix(selector, "'=") && strings.HasSuffix(selector, "'"):
		return "page.getByText('" + selector[2:len(selector)-1] + "')"
	case strings.HasPrefix(selector, "\"=") && strings.HasSuffix(selector, "\""):
		return "page.getByText(\"" + selector[2:len(selector)-1] + "\")"
	case strings.HasPrefix(selector, "'*=") && strings.HasSuffix(selector, "'"):
		return "page.getByText('" + selector[3:len(selector)-1] + "')"
	case strings.HasPrefix(selector, "\"*=") && strings.HasSuffix(selector, "\""):
		return "page.getByText(\"" + selector[3:len(selector)-1] + "\")"
	default:
		return "page.locator(" + selector + ")"
	}
}

func unsupportedWdioLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeWdioToPlaywrightAST(tree).unsupportedRows, true
}
