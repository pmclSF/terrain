package convert

import (
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var playwrightCypressStructuralCallees = map[string]string{
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

var (
	reJSObjectWidthField  = regexp.MustCompile(`\bwidth\s*:\s*([^,}]+)`)
	reJSObjectHeightField = regexp.MustCompile(`\bheight\s*:\s*([^,}]+)`)
	reJSObjectPathField   = regexp.MustCompile(`\bpath\s*:\s*([^,}]+)`)
	reJSObjectNameField   = regexp.MustCompile(`\bname\s*:\s*([^,}]+)`)
)

type playwrightCypressASTResult struct {
	source          string
	unsupportedRows map[int]bool
}

func convertPlaywrightToCypressSourceAST(source string) (playwrightCypressASTResult, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return playwrightCypressASTResult{}, false
	}
	defer tree.Close()

	analysis := analyzePlaywrightToCypressAST(tree)
	return playwrightCypressASTResult{
		source:          applyTextEdits(source, analysis.edits),
		unsupportedRows: analysis.unsupportedRows,
	}, true
}

type playwrightCypressASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func analyzePlaywrightToCypressAST(tree *jsSyntaxTree) playwrightCypressASTAnalysis {
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
			if mapped, ok := playwrightCypressStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					body := jsFunctionBodyNode(callback)
					if body != nil {
						edits = append(edits, textEdit{
							start:       int(callback.StartByte()),
							end:         int(body.StartByte()),
							replacement: "() => ",
						})
					}
				}
				return true
			}

			if replacement, ok := convertPlaywrightCallToCypress(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}
			root, _, ok := extractJSCallChain(node, tree.src)
			if ok && (root == "page" || root == "request" || root == "context") {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		case "member_expression":
			base := jsBaseIdentifier(node, tree.src)
			if base == "page" || base == "request" || base == "context" {
				unsupportedRows[int(node.StartPoint().Row)] = true
			}
		}
		return true
	})

	return playwrightCypressASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertPlaywrightCallToCypress(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertPlaywrightExpectationCall(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "page" || len(steps) == 0 {
		return "", false
	}

	switch steps[0].method {
	case "goto":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "cy.visit(" + steps[0].args[0] + ")", true
		}
	case "reload":
		if len(steps) == 1 {
			return "cy.reload()", true
		}
	case "goBack":
		if len(steps) == 1 {
			return "cy.go('back')", true
		}
	case "goForward":
		if len(steps) == 1 {
			return "cy.go('forward')", true
		}
	case "waitForTimeout":
		if len(steps) == 1 && len(steps[0].args) == 1 && isNumericLiteral(strings.TrimSpace(steps[0].args[0])) {
			return "cy.wait(" + strings.TrimSpace(steps[0].args[0]) + ")", true
		}
	case "setViewportSize":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			if width, height, ok := parseViewportSizeArg(steps[0].args[0]); ok {
				return "cy.viewport(" + width + ", " + height + ")", true
			}
		}
	case "screenshot":
		if len(steps) == 1 {
			if len(steps[0].args) == 0 {
				return "cy.screenshot()", true
			}
			if len(steps[0].args) == 1 {
				if path, ok := parseScreenshotPathArg(steps[0].args[0]); ok {
					return "cy.screenshot(" + path + ")", true
				}
			}
		}
	}

	if query, ok := playwrightQueryStepsToCypress(steps); ok {
		return query, true
	}

	return "", false
}

func convertPlaywrightExpectationCall(node *sitter.Node, src []byte) (string, bool) {
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
	if len(args) != 1 {
		return "", false
	}
	target := strings.TrimSpace(args[0])

	switch property {
	case "toBeVisible":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			if negated {
				return chain + ".should('not.be.visible')", true
			}
			return chain + ".should('be.visible')", true
		}
	case "toBeAttached":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			if negated {
				return chain + ".should('not.exist')", true
			}
			return chain + ".should('exist')", true
		}
	case "toHaveText":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return chain + ".should('have.text', " + callArgs[0] + ")", true
			}
		}
	case "toContainText":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return chain + ".should('contain', " + callArgs[0] + ")", true
			}
		}
	case "toHaveValue":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return chain + ".should('have.value', " + callArgs[0] + ")", true
			}
		}
	case "toHaveClass":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return chain + ".should('have.class', " + callArgs[0] + ")", true
			}
		}
	case "toBeChecked":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			return chain + ".should('be.checked')", true
		}
	case "toBeDisabled":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			return chain + ".should('be.disabled')", true
		}
	case "toBeEnabled":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			return chain + ".should('be.enabled')", true
		}
	case "toHaveCount":
		if chain, ok := playwrightExprToCypressChain(target); ok {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return chain + ".should('have.length', " + callArgs[0] + ")", true
			}
		}
	case "toHaveURL":
		if target == "page" {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return "cy.url().should('include', " + callArgs[0] + ")", true
			}
		}
	case "toHaveTitle":
		if target == "page" {
			callArgs := jsArgumentTexts(node, src)
			if len(callArgs) == 1 {
				return "cy.title().should('eq', " + callArgs[0] + ")", true
			}
		}
	}

	return "", false
}

func playwrightQueryStepsToCypress(steps []jsCallStep) (string, bool) {
	if len(steps) == 0 {
		return "", false
	}

	var chain string
	switch steps[0].method {
	case "locator":
		if len(steps[0].args) != 1 {
			return "", false
		}
		chain = "cy.get(" + steps[0].args[0] + ")"
	case "getByText":
		if len(steps[0].args) != 1 {
			return "", false
		}
		chain = "cy.contains(" + steps[0].args[0] + ")"
	case "getByRole":
		if len(steps[0].args) != 2 {
			return "", false
		}
		name, ok := parseGetByRoleNameArg(steps[0].args[1])
		if !ok {
			return "", false
		}
		chain = "cy.contains('[role=' + " + steps[0].args[0] + " + ']', " + name + ")"
	default:
		return "", false
	}

	remaining := steps[1:]
	for len(remaining) > 0 {
		step := remaining[0]
		switch step.method {
		case "locator":
			if len(step.args) != 1 {
				return "", false
			}
			chain += ".find(" + step.args[0] + ")"
		case "nth":
			if len(step.args) != 1 {
				return "", false
			}
			chain += ".eq(" + step.args[0] + ")"
		case "click":
			return chain + ".click()", len(remaining) == 1
		case "dblclick":
			return chain + ".dblclick()", len(remaining) == 1
		case "fill":
			if len(step.args) == 1 {
				return chain + ".type(" + step.args[0] + ")", len(remaining) == 1
			}
			return "", false
		case "clear":
			return chain + ".clear()", len(remaining) == 1
		case "check":
			return chain + ".check()", len(remaining) == 1
		case "uncheck":
			return chain + ".uncheck()", len(remaining) == 1
		case "selectOption":
			if len(step.args) == 1 {
				return chain + ".select(" + step.args[0] + ")", len(remaining) == 1
			}
			return "", false
		case "focus":
			return chain + ".focus()", len(remaining) == 1
		case "blur":
			return chain + ".blur()", len(remaining) == 1
		case "scrollIntoViewIfNeeded":
			return chain + ".scrollIntoView()", len(remaining) == 1
		case "hover":
			return chain + ".trigger('mouseover')", len(remaining) == 1
		default:
			return "", false
		}
		remaining = remaining[1:]
	}

	return chain, true
}

func playwrightExprToCypressChain(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	replacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{rePWGetByRoleNamed, `cy.contains('[role=' + $1 + ']', $2)`},
		{rePWGetByTextStandalone, `cy.contains($1)`},
		{rePWLocatorStandalone, `cy.get($1)`},
		{rePWNestedLocator, `.find($1)`},
		{rePWNth, `.eq($1)`},
	}
	for _, replacement := range replacements {
		expr = replacement.re.ReplaceAllString(expr, replacement.repl)
	}
	if strings.Contains(expr, "page.") {
		return "", false
	}
	return expr, true
}

func parseViewportSizeArg(arg string) (string, string, bool) {
	width := reJSObjectWidthField.FindStringSubmatch(arg)
	height := reJSObjectHeightField.FindStringSubmatch(arg)
	if len(width) < 2 || len(height) < 2 {
		return "", "", false
	}
	return strings.TrimSpace(width[1]), strings.TrimSpace(height[1]), true
}

func parseScreenshotPathArg(arg string) (string, bool) {
	match := reJSObjectPathField.FindStringSubmatch(arg)
	if len(match) < 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func parseGetByRoleNameArg(arg string) (string, bool) {
	match := reJSObjectNameField.FindStringSubmatch(arg)
	if len(match) < 2 {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func unsupportedPlaywrightLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzePlaywrightToCypressAST(tree).unsupportedRows, true
}
