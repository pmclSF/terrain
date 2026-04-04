package convert

import (
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type jsCallStep struct {
	method string
	args   []string
}

var cypressPlaywrightStructuralCallees = map[string]string{
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

func convertCypressToPlaywrightSourceAST(source string) (string, bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false, false
	}
	defer tree.Close()

	edits := make([]textEdit, 0, 16)
	retryWarning := false

	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		if node.Type() != "call_expression" {
			return true
		}

		callee := jsCalleeNode(node)
		calleeText := jsNodeText(callee, tree.src)
		if mapped, ok := cypressPlaywrightStructuralCallees[calleeText]; ok {
			edits = append(edits, textEdit{
				start:       int(callee.StartByte()),
				end:         int(callee.EndByte()),
				replacement: mapped,
			})
			if callback := jsLastFunctionArg(node); callback != nil {
				if replacement, ok := cypressPlaywrightCallbackPrefix(mapped, callback, tree.src); ok {
					body := jsFunctionBodyNode(callback)
					if body != nil {
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

		replacement, warn, ok := convertCypressCallToPlaywright(node, tree.src)
		if !ok {
			return true
		}
		retryWarning = retryWarning || warn
		edits = append(edits, textEdit{
			start:       int(node.StartByte()),
			end:         int(node.EndByte()),
			replacement: replacement,
		})
		return false
	})

	return applyTextEdits(source, edits), retryWarning, true
}

func unsupportedCypressLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	rows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "call_expression":
			root, _, chainOK := extractJSCallChain(node, tree.src)
			if chainOK && root == "cy" {
				if _, _, handled := convertCypressCallToPlaywright(node, tree.src); !handled {
					rows[int(node.StartPoint().Row)] = true
					return false
				}
			}
		case "member_expression":
			if jsBaseIdentifier(node, tree.src) == "Cypress" {
				rows[int(node.StartPoint().Row)] = true
			}
		}
		return true
	})

	return rows, true
}

func convertCypressCallToPlaywright(node *sitter.Node, src []byte) (string, bool, bool) {
	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "cy" || len(steps) == 0 {
		return "", false, false
	}

	switch steps[0].method {
	case "visit":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await page.goto(" + steps[0].args[0] + ")", false, true
		}
	case "reload":
		if len(steps) == 1 {
			return "await page.reload()", false, true
		}
	case "go":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			switch normalizeJSLiteral(steps[0].args[0]) {
			case "back":
				return "await page.goBack()", false, true
			case "forward":
				return "await page.goForward()", false, true
			}
		}
	case "viewport":
		if len(steps) == 1 && len(steps[0].args) == 2 {
			return "await page.setViewportSize({ width: " + steps[0].args[0] + ", height: " + steps[0].args[1] + " })", false, true
		}
	case "screenshot":
		if len(steps) == 1 {
			if len(steps[0].args) == 1 {
				return "await page.screenshot({ path: " + steps[0].args[0] + " })", false, true
			}
			if len(steps[0].args) == 0 {
				return "await page.screenshot()", false, true
			}
		}
	case "wait":
		if len(steps) == 1 && len(steps[0].args) == 1 && isNumericLiteral(strings.TrimSpace(steps[0].args[0])) {
			return "await page.waitForTimeout(" + strings.TrimSpace(steps[0].args[0]) + ")", false, true
		}
	case "get":
		return convertCypressLocatorChain("page.locator", steps)
	case "contains":
		return convertCypressLocatorChain("page.getByText", steps)
	case "url":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) >= 1 {
			switch normalizeJSLiteral(steps[1].args[0]) {
			case "include":
				if len(steps[1].args) == 2 {
					return "expect(page.url()).toContain(" + steps[1].args[1] + ")", true, true
				}
			case "eq":
				if len(steps[1].args) == 2 {
					return "expect(page.url()).toBe(" + steps[1].args[1] + ")", true, true
				}
			}
		}
	case "title":
		if len(steps) == 2 && steps[1].method == "should" && len(steps[1].args) == 2 && normalizeJSLiteral(steps[1].args[0]) == "eq" {
			return "await expect(page).toHaveTitle(" + steps[1].args[1] + ")", true, true
		}
	}

	return "", false, false
}

func convertCypressLocatorChain(locatorFactory string, steps []jsCallStep) (string, bool, bool) {
	if len(steps) == 0 || len(steps[0].args) != 1 {
		return "", false, false
	}

	locator := locatorFactory + "(" + steps[0].args[0] + ")"
	remaining := steps[1:]
	if len(remaining) > 0 {
		switch remaining[0].method {
		case "first":
			locator += ".first()"
			remaining = remaining[1:]
		case "last":
			locator += ".last()"
			remaining = remaining[1:]
		case "eq":
			if len(remaining[0].args) != 1 {
				return "", false, false
			}
			locator += ".nth(" + remaining[0].args[0] + ")"
			remaining = remaining[1:]
		}
	}

	if len(remaining) == 0 {
		return locator, false, true
	}
	if len(remaining) > 2 {
		return "", false, false
	}

	switch remaining[0].method {
	case "click":
		return "await " + locator + ".click()", false, true
	case "dblclick":
		return "await " + locator + ".dblclick()", false, true
	case "type":
		if len(remaining[0].args) == 1 {
			return "await " + locator + ".fill(" + remaining[0].args[0] + ")", false, true
		}
	case "clear":
		if len(remaining) == 2 && remaining[1].method == "type" && len(remaining[1].args) == 1 {
			return "await " + locator + ".fill(" + remaining[1].args[0] + ")", false, true
		}
		if len(remaining[0].args) == 0 {
			return "await " + locator + ".clear()", false, true
		}
	case "check":
		return "await " + locator + ".check()", false, true
	case "uncheck":
		return "await " + locator + ".uncheck()", false, true
	case "select":
		if len(remaining[0].args) == 1 {
			return "await " + locator + ".selectOption(" + remaining[0].args[0] + ")", false, true
		}
	case "focus":
		return "await " + locator + ".focus()", false, true
	case "blur":
		return "await " + locator + ".blur()", false, true
	case "scrollIntoView":
		return "await " + locator + ".scrollIntoViewIfNeeded()", false, true
	case "should":
		return convertCypressShouldChain(locator, remaining[0])
	}

	return "", false, false
}

func convertCypressShouldChain(locator string, step jsCallStep) (string, bool, bool) {
	if len(step.args) == 0 {
		return "", false, false
	}

	retryWarning := true
	matcher := normalizeJSLiteral(step.args[0])
	switch matcher {
	case "be.visible":
		return "await expect(" + locator + ").toBeVisible()", retryWarning, true
	case "not.be.visible":
		return "await expect(" + locator + ").toBeHidden()", retryWarning, true
	case "exist":
		return "await expect(" + locator + ").toBeAttached()", retryWarning, true
	case "not.exist":
		return "await expect(" + locator + ").not.toBeAttached()", retryWarning, true
	case "have.text":
		if len(step.args) == 2 {
			return "await expect(" + locator + ").toHaveText(" + step.args[1] + ")", retryWarning, true
		}
	case "contain", "contain.text":
		if len(step.args) == 2 {
			return "await expect(" + locator + ").toContainText(" + step.args[1] + ")", retryWarning, true
		}
	case "have.value":
		if len(step.args) == 2 {
			return "await expect(" + locator + ").toHaveValue(" + step.args[1] + ")", retryWarning, true
		}
	case "be.checked":
		return "await expect(" + locator + ").toBeChecked()", retryWarning, true
	case "be.disabled":
		return "await expect(" + locator + ").toBeDisabled()", retryWarning, true
	case "be.enabled":
		return "await expect(" + locator + ").toBeEnabled()", retryWarning, true
	case "have.class":
		if len(step.args) == 2 {
			return "await expect(" + locator + ").toHaveClass(" + step.args[1] + ")", retryWarning, true
		}
	case "have.length":
		if len(step.args) == 2 {
			return "await expect(" + locator + ").toHaveCount(" + step.args[1] + ")", retryWarning, true
		}
	case "have.length.greaterThan":
		if len(step.args) == 2 {
			return "expect(await " + locator + ".count()).toBeGreaterThan(" + step.args[1] + ")", retryWarning, true
		}
	case "not.be.empty":
		return "await expect(" + locator + ").not.toBeEmpty()", retryWarning, true
	}

	return "", false, false
}

func extractJSCallChain(node *sitter.Node, src []byte) (string, []jsCallStep, bool) {
	if node == nil || node.Type() != "call_expression" {
		return "", nil, false
	}

	callee := jsCalleeNode(node)
	if callee == nil {
		return "", nil, false
	}

	if callee.Type() == "member_expression" {
		property := jsNodeText(jsMemberProperty(callee), src)
		object := jsMemberObject(callee)
		args := jsArgumentTexts(node, src)
		if object != nil && object.Type() == "call_expression" {
			root, steps, ok := extractJSCallChain(object, src)
			if !ok {
				return "", nil, false
			}
			return root, append(steps, jsCallStep{method: property, args: args}), true
		}
		root := jsNodeText(object, src)
		if root == "" {
			return "", nil, false
		}
		return root, []jsCallStep{{method: property, args: args}}, true
	}

	root := jsNodeText(callee, src)
	if root == "" {
		return "", nil, false
	}
	return root, []jsCallStep{{method: "", args: jsArgumentTexts(node, src)}}, true
}

func jsArgumentTexts(node *sitter.Node, src []byte) []string {
	args := jsArgumentsNode(node)
	if args == nil || args.NamedChildCount() == 0 {
		return nil
	}
	values := make([]string, 0, int(args.NamedChildCount()))
	for i := 0; i < int(args.NamedChildCount()); i++ {
		values = append(values, jsNodeText(args.NamedChild(i), src))
	}
	return values
}

func jsLastFunctionArg(node *sitter.Node) *sitter.Node {
	args := jsArgumentsNode(node)
	if args == nil {
		return nil
	}
	for i := int(args.NamedChildCount()) - 1; i >= 0; i-- {
		child := args.NamedChild(i)
		switch child.Type() {
		case "arrow_function", "function_expression", "function":
			return child
		}
	}
	return nil
}

func jsFunctionBodyNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if body := node.ChildByFieldName("body"); body != nil {
		return body
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "statement_block" || strings.HasSuffix(child.Type(), "_statement") || strings.HasSuffix(child.Type(), "_expression") {
			return child
		}
	}
	return nil
}

func cypressPlaywrightCallbackPrefix(mappedCallee string, callback *sitter.Node, src []byte) (string, bool) {
	body := jsFunctionBodyNode(callback)
	if body == nil {
		return "", false
	}
	if strings.HasPrefix(mappedCallee, "test.describe") {
		return "() => ", true
	}
	return "async ({ page }) => ", true
}

func normalizeJSLiteral(text string) string {
	text = strings.TrimSpace(text)
	if len(text) >= 2 {
		quote := text[0]
		if (quote == '\'' && text[len(text)-1] == '\'') || (quote == '"' && text[len(text)-1] == '"') || (quote == '`' && text[len(text)-1] == '`') {
			body := text[1 : len(text)-1]
			var out strings.Builder
			out.Grow(len(body))
			escaped := false
			for i := 0; i < len(body); i++ {
				ch := body[i]
				if escaped {
					switch ch {
					case quote, '\\', '`':
						out.WriteByte(ch)
					default:
						out.WriteByte('\\')
						out.WriteByte(ch)
					}
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				out.WriteByte(ch)
			}
			if escaped {
				out.WriteByte('\\')
			}
			return out.String()
		}
	}
	return text
}

func isNumericLiteral(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	cleaned := strings.ReplaceAll(text, "_", "")
	if cleaned == "" {
		return false
	}
	_, err := strconv.ParseFloat(cleaned, 64)
	return err == nil
}
