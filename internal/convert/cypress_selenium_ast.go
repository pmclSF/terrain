package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var cypressSeleniumStructuralCallees = map[string]string{
	"describe":      "describe",
	"describe.only": "describe.only",
	"describe.skip": "describe.skip",
	"context":       "describe",
	"it":            "it",
	"it.only":       "it.only",
	"it.skip":       "it.skip",
	"specify":       "it",
	"before":        "beforeAll",
	"after":         "afterAll",
	"beforeEach":    "beforeEach",
	"afterEach":     "afterEach",
}

type seleniumLocator struct {
	elementExpr string
	listExpr    string
}

type cypressSeleniumASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertCypressToSeleniumSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeCypressToSeleniumAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Cypress conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeCypressToSeleniumAST(tree *jsSyntaxTree) cypressSeleniumASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "call_expression":
			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := cypressSeleniumStructuralCallees[calleeText]; ok {
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

			replacement, ok := convertCypressCallToSelenium(node, tree.src)
			if ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}

			root, _, chainOK := extractJSCallChain(node, tree.src)
			if chainOK && root == "cy" {
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

	return cypressSeleniumASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func cypressSeleniumCallbackPrefix(mappedCallee string, callback *sitter.Node) (string, bool) {
	if jsFunctionBodyNode(callback) == nil {
		return "", false
	}
	if strings.HasPrefix(mappedCallee, "describe") {
		if callback.Type() == "function_expression" || callback.Type() == "function" {
			return "function() ", true
		}
		return "() => ", true
	}
	if callback.Type() == "function_expression" || callback.Type() == "function" {
		return "async function() ", true
	}
	return "async () => ", true
}

func convertCypressCallToSelenium(node *sitter.Node, src []byte) (string, bool) {
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "cy" || len(steps) == 0 {
		return "", false
	}

	switch steps[0].method {
	case "visit":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await driver.get(" + steps[0].args[0] + ")", true
		}
	case "reload":
		if len(steps) == 1 {
			return "await driver.navigate().refresh()", true
		}
	case "go":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			switch normalizeJSLiteral(steps[0].args[0]) {
			case "back":
				return "await driver.navigate().back()", true
			case "forward":
				return "await driver.navigate().forward()", true
			}
			if isNumericLiteral(steps[0].args[0]) {
				return "await driver.navigate().back() /* cy.go(" + steps[0].args[0] + ") */", true
			}
		}
	case "wait":
		if len(steps) == 1 && len(steps[0].args) == 1 && isNumericLiteral(steps[0].args[0]) {
			return "await driver.sleep(" + steps[0].args[0] + ")", true
		}
	case "clearCookies":
		if len(steps) == 1 {
			return "await driver.manage().deleteAllCookies()", true
		}
	case "clearLocalStorage":
		if len(steps) == 1 {
			return `await driver.executeScript("localStorage.clear()")`, true
		}
	case "get":
		return convertCypressLocatorChainToSelenium(steps)
	case "contains":
		return convertCypressContainsChainToSelenium(steps)
	case "url":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) >= 1 {
			switch normalizeJSLiteral(steps[1].args[0]) {
			case "include":
				if len(steps[1].args) == 2 {
					return "expect(await driver.getCurrentUrl()).toContain(" + steps[1].args[1] + ")", true
				}
			case "eq":
				if len(steps[1].args) == 2 {
					return "expect(await driver.getCurrentUrl()).toBe(" + steps[1].args[1] + ")", true
				}
			}
		}
	case "title":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) == 2 && normalizeJSLiteral(steps[1].args[0]) == "eq" {
			return "expect(await driver.getTitle()).toBe(" + steps[1].args[1] + ")", true
		}
	}

	return "", false
}

func convertCypressLocatorChainToSelenium(steps []jsCallStep) (string, bool) {
	locator, remaining, ok := cypressGetLocatorToSelenium(steps)
	if !ok {
		return "", false
	}
	return seleniumActionOrAssertion(locator, remaining)
}

func convertCypressContainsChainToSelenium(steps []jsCallStep) (string, bool) {
	if len(steps) == 0 || len(steps[0].args) != 1 {
		return "", false
	}
	locator := seleniumLocator{
		elementExpr: "(await driver.findElement(By.xpath(`//*[contains(text()," + steps[0].args[0] + ")]`)))",
	}
	return seleniumActionOrAssertion(locator, steps[1:])
}

func cypressGetLocatorToSelenium(steps []jsCallStep) (seleniumLocator, []jsCallStep, bool) {
	if len(steps) == 0 || steps[0].method != "get" || len(steps[0].args) != 1 {
		return seleniumLocator{}, nil, false
	}

	selector := steps[0].args[0]
	listExpr := "(await driver.findElements(By.css(" + selector + ")))"
	elementExpr := "(await driver.findElement(By.css(" + selector + ")))"
	remaining := steps[1:]

	if len(remaining) > 0 {
		switch remaining[0].method {
		case "first":
			elementExpr = "(" + listExpr + "[0])"
			remaining = remaining[1:]
		case "last":
			elementExpr = "(" + listExpr + ".slice(-1)[0])"
			remaining = remaining[1:]
		case "eq":
			if len(remaining[0].args) != 1 {
				return seleniumLocator{}, nil, false
			}
			elementExpr = "(" + listExpr + "[" + remaining[0].args[0] + "])"
			remaining = remaining[1:]
		}
	}

	if len(remaining) > 0 && remaining[0].method == "find" {
		if len(remaining[0].args) != 1 {
			return seleniumLocator{}, nil, false
		}
		elementExpr = "(await " + elementExpr + ".findElement(By.css(" + remaining[0].args[0] + ")))"
		listExpr = ""
		remaining = remaining[1:]
	}

	return seleniumLocator{
		elementExpr: elementExpr,
		listExpr:    listExpr,
	}, remaining, true
}

func seleniumActionOrAssertion(locator seleniumLocator, remaining []jsCallStep) (string, bool) {
	if len(remaining) == 0 {
		return locator.elementExpr, true
	}
	if len(remaining) > 2 {
		return "", false
	}

	switch remaining[0].method {
	case "click":
		return "await " + locator.elementExpr + ".click()", true
	case "dblclick":
		return "await " + locator.elementExpr + ".click();\n    await " + locator.elementExpr + ".click()", true
	case "type":
		if len(remaining[0].args) == 1 {
			return "await " + locator.elementExpr + ".sendKeys(" + remaining[0].args[0] + ")", true
		}
	case "clear":
		if len(remaining) == 2 && remaining[1].method == "type" && len(remaining[1].args) == 1 {
			return "await " + locator.elementExpr + ".clear();\n    await " + locator.elementExpr + ".sendKeys(" + remaining[1].args[0] + ")", true
		}
		if len(remaining[0].args) == 0 {
			return "await " + locator.elementExpr + ".clear()", true
		}
	case "check":
		return "const checkbox = " + locator.elementExpr + ";\n    if (!(await checkbox.isSelected())) await checkbox.click()", true
	case "uncheck":
		return "const checkbox = " + locator.elementExpr + ";\n    if (await checkbox.isSelected()) await checkbox.click()", true
	case "select":
		if len(remaining[0].args) == 1 {
			return "await " + locator.elementExpr + ".sendKeys(" + remaining[0].args[0] + ")", true
		}
	case "should":
		return convertCypressShouldToSelenium(locator, remaining[0])
	}

	return "", false
}

func convertCypressShouldToSelenium(locator seleniumLocator, step jsCallStep) (string, bool) {
	if len(step.args) == 0 {
		return "", false
	}

	matcher := normalizeJSLiteral(step.args[0])
	switch matcher {
	case "be.visible":
		return "expect(await " + locator.elementExpr + ".isDisplayed()).toBe(true)", true
	case "not.be.visible":
		return "expect(await " + locator.elementExpr + ".isDisplayed()).toBe(false)", true
	case "exist":
		if locator.listExpr != "" {
			return "expect(" + locator.listExpr + ".length).toBeGreaterThan(0)", true
		}
	case "not.exist":
		if locator.listExpr != "" {
			return "expect(" + locator.listExpr + ".length).toBe(0)", true
		}
	case "have.text":
		if len(step.args) == 2 {
			return "expect(await " + locator.elementExpr + ".getText()).toBe(" + step.args[1] + ")", true
		}
	case "contain", "contain.text":
		if len(step.args) == 2 {
			return "expect(await " + locator.elementExpr + ".getText()).toContain(" + step.args[1] + ")", true
		}
	case "have.value":
		if len(step.args) == 2 {
			return `expect(await ` + locator.elementExpr + `.getAttribute("value")).toBe(` + step.args[1] + `)`, true
		}
	case "be.checked":
		return "expect(await " + locator.elementExpr + ".isSelected()).toBe(true)", true
	case "be.disabled":
		return "expect(await " + locator.elementExpr + ".isEnabled()).toBe(false)", true
	case "be.enabled":
		return "expect(await " + locator.elementExpr + ".isEnabled()).toBe(true)", true
	case "have.class":
		if len(step.args) == 2 {
			return `expect(await ` + locator.elementExpr + `.getAttribute("class")).toContain(` + step.args[1] + `)`, true
		}
	case "have.length":
		if locator.listExpr != "" && len(step.args) == 2 {
			return "expect(" + locator.listExpr + ".length).toBe(" + step.args[1] + ")", true
		}
	case "have.length.greaterThan":
		if locator.listExpr != "" && len(step.args) == 2 {
			return "expect(" + locator.listExpr + ".length).toBeGreaterThan(" + step.args[1] + ")", true
		}
	}

	return "", false
}

func unsupportedCypressSeleniumLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzeCypressToSeleniumAST(tree).unsupportedRows, true
}
