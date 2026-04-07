package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

var puppeteerPlaywrightStructuralCallees = map[string]string{
	"describe":      "test.describe",
	"describe.only": "test.describe.only",
	"describe.skip": "test.describe.skip",
	"it.only":       "test.only",
	"it.skip":       "test.skip",
	"beforeAll":     "test.beforeAll",
	"afterAll":      "test.afterAll",
	"beforeEach":    "test.beforeEach",
	"afterEach":     "test.afterEach",
	"it":            "test",
}

type puppeteerPlaywrightASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertPuppeteerToPlaywrightSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzePuppeteerToPlaywrightAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Puppeteer conversion required")
	}
	result = rePptrRequireImport.ReplaceAllString(result, "")
	result = rePptrESMImport.ReplaceAllString(result, "")
	result = rePptrBrowserPageDecl.ReplaceAllString(result, "")
	result = rePptrBeforeAllBlock.ReplaceAllString(result, "\n")
	result = rePptrAfterAllBlock.ReplaceAllString(result, "\n")
	result = rePptrLaunchLine.ReplaceAllString(result, "")
	result = rePptrNewPageLine.ReplaceAllString(result, "")
	result = rePptrCloseLine.ReplaceAllString(result, "")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzePuppeteerToPlaywrightAST(tree *jsSyntaxTree) puppeteerPlaywrightASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'puppeteer'") || strings.Contains(module, "\"puppeteer\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "lexical_declaration", "variable_declaration":
			text := strings.TrimSpace(jsNodeText(node, tree.src))
			if text == "const puppeteer = require('puppeteer');" || text == "const puppeteer = require(\"puppeteer\");" || text == "let browser, page;" {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			calleeText := jsNodeText(callee, tree.src)
			if mapped, ok := puppeteerPlaywrightStructuralCallees[calleeText]; ok {
				edits = append(edits, textEdit{
					start:       int(callee.StartByte()),
					end:         int(callee.EndByte()),
					replacement: mapped,
				})
				if callback := jsLastFunctionArg(node); callback != nil {
					if replacement, ok := puppeteerPlaywrightCallbackPrefix(mapped); ok {
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

			if replacement, ok := convertPuppeteerCallToPlaywright(node, tree.src); ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}

			root, _, ok := extractJSCallChain(node, tree.src)
			if ok && strings.HasPrefix(root, "page") {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return puppeteerPlaywrightASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func puppeteerPlaywrightCallbackPrefix(mapped string) (string, bool) {
	if strings.HasPrefix(mapped, "test.describe") {
		return "() => ", true
	}
	return "async ({ page }) => ", true
}

func convertPuppeteerCallToPlaywright(node *sitter.Node, src []byte) (string, bool) {
	if replacement, ok := convertPuppeteerExpectationToPlaywright(node, src); ok {
		return replacement, true
	}

	root, steps, ok := extractJSCallChain(node, src)
	if !ok || root != "page" || len(steps) == 0 {
		return "", false
	}

	switch steps[0].method {
	case "goto":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await page.goto(" + steps[0].args[0] + ")", true
		}
	case "type":
		if len(steps) == 1 {
			if replacement, ok := puppeteerTypeArgsToPlaywright(steps[0].args); ok {
				return replacement, true
			}
		}
	case "click":
		if len(steps) == 1 {
			if replacement, ok := puppeteerClickArgsToPlaywright(steps[0].args); ok {
				return replacement, true
			}
		}
	case "hover":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await page.locator(" + steps[0].args[0] + ").hover()", true
		}
	case "select":
		if len(steps) == 1 {
			if replacement, ok := puppeteerSelectArgsToPlaywright(steps[0].args); ok {
				return replacement, true
			}
		}
	case "focus":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			return "await page.locator(" + steps[0].args[0] + ").focus()", true
		}
	case "waitForSelector":
		if len(steps) == 1 {
			if replacement, ok := puppeteerWaitForSelectorArgsToPlaywright(steps[0].args); ok {
				return replacement, true
			}
		}
	case "$eval":
		if len(steps) == 1 {
			if replacement, ok := puppeteerEvalArgsToPlaywright(steps[0].args, false); ok {
				return replacement, true
			}
		}
	case "$$eval":
		if len(steps) == 1 {
			if replacement, ok := puppeteerEvalArgsToPlaywright(steps[0].args, true); ok {
				return replacement, true
			}
		}
	case "setViewport":
		if len(steps) == 1 && len(steps[0].args) == 1 {
			if width, height, ok := parseViewportSizeArg(steps[0].args[0]); ok {
				return "await page.setViewportSize({ width: " + width + ", height: " + height + " })", true
			}
		}
	case "setCookie":
		if len(steps) == 1 {
			if cookies, ok := puppeteerCookieArgsToPlaywright(steps[0].args); ok {
				return "await page.context().addCookies(" + cookies + ")", true
			}
		}
	case "cookies":
		if len(steps) == 1 && len(steps[0].args) == 0 {
			return "await page.context().cookies()", true
		}
	case "deleteCookie":
		if len(steps) == 1 && len(steps[0].args) == 0 {
			return "await page.context().clearCookies()", true
		}
	}

	return "", false
}

func puppeteerCookieArgsToPlaywright(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	if len(args) == 1 {
		arg := strings.TrimSpace(args[0])
		switch {
		case strings.HasPrefix(arg, "[") && strings.HasSuffix(arg, "]"):
			return arg, true
		case strings.HasPrefix(arg, "{") && strings.HasSuffix(arg, "}"):
			return "[" + arg + "]", true
		default:
			return "", false
		}
	}
	return "[" + strings.Join(args, ", ") + "]", true
}

func puppeteerTypeArgsToPlaywright(args []string) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	return "await page.locator(" + args[0] + ").fill(" + args[1] + ")", true
}

func puppeteerClickArgsToPlaywright(args []string) (string, bool) {
	if len(args) == 1 {
		return "await page.locator(" + args[0] + ").click()", true
	}
	if len(args) == 2 && puppeteerClickIsDouble(args[1]) {
		return "await page.locator(" + args[0] + ").dblclick()", true
	}
	return "", false
}

func puppeteerSelectArgsToPlaywright(args []string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	selector := args[0]
	values := args[1:]
	if len(values) == 1 {
		return "await page.locator(" + selector + ").selectOption(" + values[0] + ")", true
	}
	return "await page.locator(" + selector + ").selectOption([" + strings.Join(values, ", ") + "])", true
}

func puppeteerEvalArgsToPlaywright(args []string, evaluateAll bool) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	method := "evaluate"
	if evaluateAll {
		method = "evaluateAll"
	}
	return "await page.locator(" + args[0] + ")." + method + "(" + args[1] + ")", true
}

func puppeteerClickIsDouble(arg string) bool {
	trimmed := strings.TrimSpace(arg)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return false
	}
	fields := splitTopLevelArgs(trimmed[1 : len(trimmed)-1])
	if len(fields) != 1 {
		return false
	}
	field := strings.TrimSpace(fields[0])
	sep := strings.Index(field, ":")
	if sep < 0 {
		return false
	}
	key := strings.TrimSpace(field[:sep])
	value := strings.TrimSpace(field[sep+1:])
	return key == "clickCount" && value == "2"
}

func puppeteerWaitForSelectorArgsToPlaywright(args []string) (string, bool) {
	if len(args) == 0 || len(args) > 2 {
		return "", false
	}

	selector := strings.TrimSpace(args[0])
	if selector == "" {
		return "", false
	}
	if len(args) == 1 {
		return "await page.locator(" + selector + ").waitFor()", true
	}

	options, ok := puppeteerWaitForSelectorOptionsToPlaywright(args[1])
	if !ok {
		return "", false
	}
	if options == "" {
		return "await page.locator(" + selector + ").waitFor()", true
	}
	return "await page.locator(" + selector + ").waitFor(" + options + ")", true
}

func puppeteerWaitForSelectorOptionsToPlaywright(arg string) (string, bool) {
	trimmed := strings.TrimSpace(arg)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return "", false
	}

	fields := splitTopLevelArgs(trimmed[1 : len(trimmed)-1])
	parts := make([]string, 0, 2)
	state := ""

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		sep := strings.Index(field, ":")
		if sep < 0 {
			return "", false
		}
		key := strings.TrimSpace(field[:sep])
		value := strings.TrimSpace(field[sep+1:])
		switch key {
		case "visible":
			switch value {
			case "true":
				if state != "" && state != "'visible'" {
					return "", false
				}
				state = "'visible'"
			case "false":
				continue
			default:
				return "", false
			}
		case "hidden":
			switch value {
			case "true":
				if state != "" && state != "'hidden'" {
					return "", false
				}
				state = "'hidden'"
			case "false":
				continue
			default:
				return "", false
			}
		case "timeout":
			if value == "" {
				return "", false
			}
			parts = append(parts, "timeout: "+value)
		default:
			return "", false
		}
	}

	if state != "" {
		parts = append([]string{"state: " + state}, parts...)
	}
	if len(parts) == 0 {
		return "", true
	}
	return "{ " + strings.Join(parts, ", ") + " }", true
}

func convertPuppeteerExpectationToPlaywright(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}
	property := jsNodeText(jsMemberProperty(callee), src)
	object := jsMemberObject(callee)
	if object == nil || object.Type() != "call_expression" || jsNodeText(jsCalleeNode(object), src) != "expect" {
		return "", false
	}

	argsNode := jsArgumentsNode(object)
	if argsNode == nil || argsNode.NamedChildCount() != 1 {
		return "", false
	}
	targetNode := argsNode.NamedChild(0)
	targetNode = jsUnwrapAwait(targetNode)
	callArgs := jsArgumentTexts(node, src)

	if targetNode.Type() == "call_expression" {
		root, steps, ok := extractJSCallChain(targetNode, src)
		if ok && root == "page" && len(steps) == 1 {
			switch steps[0].method {
			case "url":
				if len(callArgs) == 1 {
					switch property {
					case "toBe":
						return "await expect(page).toHaveURL(" + callArgs[0] + ")", true
					case "toContain":
						return "await expect(page).toHaveURL(" + playwrightPatternArg(callArgs[0]) + ")", true
					case "toMatch":
						return "await expect(page).toHaveURL(" + playwrightPatternArg(callArgs[0]) + ")", true
					}
				}
			case "title":
				if len(callArgs) == 1 {
					switch property {
					case "toBe":
						return "await expect(page).toHaveTitle(" + callArgs[0] + ")", true
					case "toContain":
						return "await expect(page).toHaveTitle(" + playwrightPatternArg(callArgs[0]) + ")", true
					case "toMatch":
						return "await expect(page).toHaveTitle(" + playwrightPatternArg(callArgs[0]) + ")", true
					}
				}
			case "$":
				if len(steps[0].args) == 1 {
					switch property {
					case "toBeTruthy":
						return "await expect(page.locator(" + steps[0].args[0] + ")).toBeVisible()", true
					case "toBeFalsy":
						return "await expect(page.locator(" + steps[0].args[0] + ")).toBeHidden()", true
					}
				}
			case "$eval":
				if len(steps[0].args) == 2 && len(callArgs) == 1 {
					expr := strings.TrimSpace(steps[0].args[1])
					switch {
					case expr == "el => el.textContent" && property == "toBe":
						return "await expect(page.locator(" + steps[0].args[0] + ")).toHaveText(" + callArgs[0] + ")", true
					case expr == "el => el.textContent" && property == "toContain":
						return "await expect(page.locator(" + steps[0].args[0] + ")).toContainText(" + callArgs[0] + ")", true
					case expr == "el => el.value" && property == "toBe":
						return "await expect(page.locator(" + steps[0].args[0] + ")).toHaveValue(" + callArgs[0] + ")", true
					}
				}
			}
		}
	}

	if targetNode.Type() == "member_expression" {
		base, parts, ok := jsMemberChainParts(targetNode, src)
		if ok && base != nil && base.Type() == "await_expression" {
			baseCall := jsUnwrapAwait(base)
			if baseCall != nil && baseCall.Type() == "call_expression" {
				root, steps, ok := extractJSCallChain(baseCall, src)
				if ok && root == "page" && len(steps) == 1 && steps[0].method == "$$" && len(steps[0].args) == 1 &&
					equalStrings(parts, "length") && property == "toBe" && len(callArgs) == 1 {
					return "await expect(page.locator(" + steps[0].args[0] + ")).toHaveCount(" + callArgs[0] + ")", true
				}
			}
		}
	}

	return "", false
}

func unsupportedPuppeteerPlaywrightLineRowsAST(source string) (map[int]bool, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return nil, false
	}
	defer tree.Close()

	return analyzePuppeteerToPlaywrightAST(tree).unsupportedRows, true
}
