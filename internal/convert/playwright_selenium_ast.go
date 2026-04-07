package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var playwrightSeleniumStructuralCallees = map[string]string{
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

type playwrightSeleniumASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertPlaywrightToSeleniumSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzePlaywrightToSeleniumAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Playwright conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzePlaywrightToSeleniumAST(tree *jsSyntaxTree) playwrightSeleniumASTAnalysis {
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
			if mapped, ok := playwrightSeleniumStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if body := jsFunctionBodyNode(callback); body != nil {
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

			if replacement, ok := convertPlaywrightCallToSeleniumAST(node, tree.src); ok {
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

	return playwrightSeleniumASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertPlaywrightCallToSeleniumAST(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertPlaywrightExpectationToSeleniumAST(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || len(steps) == 0 {
		return "", false
	}

	switch root {
	case "page":
		if replacement, ok := convertPlaywrightPageCallToSelenium(steps); ok {
			return replacement, true
		}
		if replacement, ok := convertPlaywrightQueryStepsToSelenium(steps); ok {
			return replacement, true
		}
	case "context":
		if replacement, ok := convertPlaywrightContextCallToSelenium(steps); ok {
			return replacement, true
		}
	}
	return "", false
}

func convertPlaywrightPageCallToSelenium(steps []jsCallStep) (string, bool) {
	step := steps[0]
	switch step.method {
	case "goto":
		if len(steps) == 1 && len(step.args) == 1 {
			return "await driver.get(" + step.args[0] + ")", true
		}
	case "reload":
		if len(steps) == 1 {
			return "await driver.navigate().refresh()", true
		}
	case "goBack":
		if len(steps) == 1 {
			return "await driver.navigate().back()", true
		}
	case "goForward":
		if len(steps) == 1 {
			return "await driver.navigate().forward()", true
		}
	case "setViewportSize":
		if len(steps) == 1 && len(step.args) == 1 {
			if width, height, ok := parseViewportSizeArg(step.args[0]); ok {
				return "await driver.manage().window().setRect({ width: " + width + ", height: " + height + " })", true
			}
		}
	case "waitForTimeout":
		if len(steps) == 1 && len(step.args) == 1 {
			return "await driver.sleep(" + step.args[0] + ")", true
		}
	case "context":
		if len(steps) == 2 && steps[1].method == "clearCookies" && len(steps[1].args) == 0 {
			return "await driver.manage().deleteAllCookies()", true
		}
	case "evaluate":
		if len(steps) == 1 && len(step.args) == 1 && strings.Contains(step.args[0], "localStorage.clear()") {
			return "await driver.executeScript(\"localStorage.clear()\")", true
		}
	}
	return "", false
}

func convertPlaywrightContextCallToSelenium(steps []jsCallStep) (string, bool) {
	if len(steps) == 1 && steps[0].method == "clearCookies" && len(steps[0].args) == 0 {
		return "await driver.manage().deleteAllCookies()", true
	}
	return "", false
}

func convertPlaywrightExpectationToSeleniumAST(node *sitter.Node, src []byte) (string, bool) {
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
				return seleniumExpectationAssertion("await driver.getCurrentUrl()", callArgs[0]), true
			}
		case "toHaveTitle":
			if len(callArgs) == 1 {
				return seleniumExpectationAssertion("await driver.getTitle()", callArgs[0]), true
			}
		}
		return "", false
	}

	elementQuery, pluralQuery, ok := playwrightExprToSeleniumQueries(target)
	if !ok {
		return "", false
	}

	switch property {
	case "toBeVisible":
		if negated {
			return "expect(await (await " + elementQuery + ").isDisplayed()).toBe(false)", true
		}
		return "expect(await (await " + elementQuery + ").isDisplayed()).toBe(true)", true
	case "toBeHidden":
		return "expect(await (await " + elementQuery + ").isDisplayed()).toBe(false)", true
	case "toBeAttached":
		if negated {
			return "expect((await " + pluralQuery + ").length).toBe(0)", true
		}
		return "expect((await " + pluralQuery + ").length).toBeGreaterThan(0)", true
	case "toHaveText":
		if len(callArgs) == 1 {
			return "expect(await (await " + elementQuery + ").getText()).toBe(" + callArgs[0] + ")", true
		}
	case "toContainText":
		if len(callArgs) == 1 {
			return "expect(await (await " + elementQuery + ").getText()).toContain(" + callArgs[0] + ")", true
		}
	case "toHaveValue":
		if len(callArgs) == 1 {
			return "expect(await (await " + elementQuery + ").getAttribute(\"value\")).toBe(" + callArgs[0] + ")", true
		}
	case "toBeChecked":
		return "expect(await (await " + elementQuery + ").isSelected()).toBe(true)", true
	case "toBeDisabled":
		return "expect(await (await " + elementQuery + ").isEnabled()).toBe(false)", true
	case "toBeEnabled":
		return "expect(await (await " + elementQuery + ").isEnabled()).toBe(true)", true
	case "toHaveCount":
		if len(callArgs) == 1 {
			return "expect((await " + pluralQuery + ").length).toBe(" + callArgs[0] + ")", true
		}
	}

	return "", false
}

func convertPlaywrightQueryStepsToSelenium(steps []jsCallStep) (string, bool) {
	elementQuery, remaining, ok := basePlaywrightQueryToSelenium(steps, false)
	if !ok {
		return "", false
	}
	if len(remaining) == 0 {
		return elementQuery, true
	}
	if len(remaining) > 1 {
		return "", false
	}

	step := remaining[0]
	switch step.method {
	case "fill":
		if len(step.args) == 1 {
			return "await " + elementQuery + ".sendKeys(" + step.args[0] + ")", true
		}
	case "click":
		return "await " + elementQuery + ".click()", true
	case "clear":
		return "await " + elementQuery + ".clear()", true
	case "check":
		return "const checkbox = await " + elementQuery + ";\n    if (!(await checkbox.isSelected())) await checkbox.click()", true
	case "uncheck":
		return "const checkbox = await " + elementQuery + ";\n    if (await checkbox.isSelected()) await checkbox.click()", true
	case "selectOption":
		if len(step.args) == 1 {
			value := step.args[0]
			if strings.Contains(value, "label:") {
				if label, ok := parseNamedObjectField(value, "label"); ok {
					value = label
				}
			} else if strings.Contains(value, "value:") {
				if parsed, ok := parseNamedObjectField(value, "value"); ok {
					value = parsed
				}
			}
			return "await " + elementQuery + ".sendKeys(" + value + ")", true
		}
	}

	return "", false
}

func basePlaywrightQueryToSelenium(steps []jsCallStep, plural bool) (string, []jsCallStep, bool) {
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
			return "driver.findElements(By.css(" + step.args[0] + "))", steps[1:], true
		}
		return "driver.findElement(By.css(" + step.args[0] + "))", steps[1:], true
	case "getByText":
		if len(step.args) != 1 {
			return "", nil, false
		}
		query := "driver.findElement(By.xpath(`//*[contains(text()," + step.args[0] + ")]`))"
		if plural {
			query = "driver.findElements(By.xpath(`//*[contains(text()," + step.args[0] + ")]`))"
		}
		return query, steps[1:], true
	}
	return "", nil, false
}

func playwrightExprToSeleniumQueries(expr string) (string, string, bool) {
	expr = strings.TrimSpace(expr)
	switch {
	case strings.HasPrefix(expr, "page.locator(") && strings.HasSuffix(expr, ")"):
		inner := strings.TrimSpace(expr[len("page.locator(") : len(expr)-1])
		return "driver.findElement(By.css(" + inner + "))", "driver.findElements(By.css(" + inner + "))", true
	case strings.HasPrefix(expr, "page.getByText(") && strings.HasSuffix(expr, ")"):
		inner := strings.TrimSpace(expr[len("page.getByText(") : len(expr)-1])
		query := "driver.findElement(By.xpath(`//*[contains(text()," + inner + ")]`))"
		plural := "driver.findElements(By.xpath(`//*[contains(text()," + inner + ")]`))"
		return query, plural, true
	default:
		return "", "", false
	}
}

func unsupportedPlaywrightSeleniumLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzePlaywrightToSeleniumAST(tree).unsupportedRows, true
}
