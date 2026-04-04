package convert

import (
	"fmt"
	"strings"
)

// ConvertPytestToUnittestSource rewrites the common pytest surface into
// high-confidence unittest output and comments fixture shapes that do not map
// cleanly into a class-based lifecycle.
func ConvertPytestToUnittestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	blocks := parsePythonBlocks(strings.ReplaceAll(source, "\r\n", "\n"), 0)
	imports := []string{"import unittest"}
	seenImports := map[string]bool{"import unittest": true}
	prelude := make([]string, 0, 16)
	todoBlocks := make([]string, 0, 8)
	tests := make([]pythonBlock, 0, 8)
	fixtures := make([]pythonBlock, 0, 4)

	for _, block := range blocks {
		switch block.Kind {
		case "raw":
			collectPythonPrelude(block.Raw, &imports, seenImports, &prelude, func(trimmed string) bool {
				return trimmed == "import pytest" || strings.HasPrefix(trimmed, "from pytest ")
			})
		case "function":
			name, _, _ := extractPythonFuncParts(block.Signature)
			switch {
			case isPytestFixtureBlock(block):
				if isAutousePytestFixture(block) {
					fixtures = append(fixtures, block)
				} else {
					todoBlocks = append(todoBlocks, strings.Join(renderCommentedPythonBlock(block.Raw, "manual pytest fixture migration required"), "\n"))
				}
			case strings.HasPrefix(name, "test_"):
				tests = append(tests, block)
			default:
				prelude = append(prelude, block.Raw...)
			}
		default:
			prelude = append(prelude, block.Raw...)
		}
	}

	if len(fixtures) == 0 && len(tests) == 0 {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}

	classLines := buildUnittestClassFromPytest(fixtures, tests)
	todoLines := joinMultilineSection(todoBlocks)
	return joinPythonSections(imports, prelude, todoLines, classLines), nil
}

func buildUnittestClassFromPytest(fixtures, tests []pythonBlock) []string {
	className := "TestConverted"
	if len(tests) > 0 {
		name, _, _ := extractPythonFuncParts(tests[0].Signature)
		className = toPythonTestClassName(name)
	}

	setUpBody := make([]string, 0, 8)
	tearDownBody := make([]string, 0, 8)
	for _, fixture := range fixtures {
		before, after, hasYield := splitFixtureBodyAroundYield(fixture.Body)
		setUpBody = append(setUpBody, convertPytestBodyToUnittest(before)...)
		if hasYield {
			tearDownBody = append(tearDownBody, convertPytestBodyToUnittest(after)...)
		}
	}

	classLines := []string{fmt.Sprintf("class %s(unittest.TestCase):", className)}
	if len(setUpBody) > 0 {
		classLines = append(classLines, "    def setUp(self):")
		classLines = append(classLines, indentPythonLines(nonEmptyPythonBody(setUpBody), "        ")...)
		classLines = append(classLines, "")
	}
	if len(tearDownBody) > 0 {
		classLines = append(classLines, "    def tearDown(self):")
		classLines = append(classLines, indentPythonLines(nonEmptyPythonBody(tearDownBody), "        ")...)
		classLines = append(classLines, "")
	}

	for index, test := range tests {
		name, params, _ := extractPythonFuncParts(test.Signature)
		paramNames := []string(nil)
		paramExpr := ""
		body := make([]string, 0, len(test.Body)+8)

		for _, decorator := range test.Decorators {
			if names, expr, ok := parsePytestParametrizeDecorator(decorator); ok && len(paramNames) == 0 {
				paramNames = names
				paramExpr = expr
				continue
			}
			body = append(body, "# TERRAIN-TODO: manual pytest decorator migration required")
			body = append(body, "# "+decorator)
		}

		params = stripPytestParamNames(params, paramNames)
		classLines = append(classLines, fmt.Sprintf("    def %s(self):", name))
		if len(params) > 0 {
			body = append(body, "# TERRAIN-TODO: pytest fixture arguments require manual unittest setup")
		}
		convertedBody := convertPytestBodyToUnittest(test.Body)
		if len(paramNames) > 0 && paramExpr != "" {
			body = append(body, buildUnittestSubTestFromPytestParametrize(paramNames, paramExpr, convertedBody)...)
		} else {
			body = append(body, convertedBody...)
		}
		classLines = append(classLines, indentPythonLines(nonEmptyPythonBody(body), "        ")...)
		if index < len(tests)-1 {
			classLines = append(classLines, "")
		}
	}

	return trimPythonBlankEdges(classLines)
}

func collectPythonPrelude(raw []string, imports *[]string, seenImports map[string]bool, prelude *[]string, skipImport func(string) bool) {
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			*prelude = append(*prelude, "")
		case strings.HasPrefix(trimmed, "import "), strings.HasPrefix(trimmed, "from "):
			if skipImport != nil && skipImport(trimmed) {
				continue
			}
			appendUniqueLine(imports, seenImports, trimmed)
		default:
			*prelude = append(*prelude, line)
		}
	}
}

func isPytestFixtureBlock(block pythonBlock) bool {
	for _, decorator := range block.Decorators {
		if strings.HasPrefix(strings.TrimSpace(decorator), "@pytest.fixture") {
			return true
		}
	}
	return false
}

func isAutousePytestFixture(block pythonBlock) bool {
	for _, decorator := range block.Decorators {
		trimmed := strings.TrimSpace(decorator)
		if strings.HasPrefix(trimmed, "@pytest.fixture") && strings.Contains(trimmed, "autouse=True") {
			return true
		}
	}
	return false
}

func convertPytestBodyToUnittest(body []string) []string {
	out := make([]string, 0, len(body))
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "with pytest.raises("):
			out = append(out, convertPytestRaisesLineToUnittest(line))
		case strings.HasPrefix(trimmed, "assert "):
			out = append(out, convertPytestAssertLineToUnittest(line))
		default:
			out = append(out, strings.ReplaceAll(line, "pytest.fail(", "self.fail("))
		}
	}
	return trimPythonBlankEdges(out)
}

func convertPytestRaisesLineToUnittest(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return indent + "# TERRAIN-TODO: manual pytest.raises migration required"
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	if len(args) == 0 {
		return indent + "# TERRAIN-TODO: manual pytest.raises migration required"
	}
	if len(args) >= 2 {
		for _, extra := range args[1:] {
			extra = strings.TrimSpace(extra)
			if strings.HasPrefix(extra, "match=") {
				return indent + fmt.Sprintf("with self.assertRaisesRegex(%s, %s):", args[0], strings.TrimSpace(strings.TrimPrefix(extra, "match=")))
			}
		}
	}
	return indent + fmt.Sprintf("with self.assertRaises(%s):", args[0])
}

func convertPytestAssertLineToUnittest(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	expr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "assert "))

	switch {
	case expr == "True":
		return indent + "self.assertTrue(True)"
	case expr == "False":
		return indent + "self.assertFalse(False)"
	case strings.HasSuffix(expr, " is not None"):
		return indent + fmt.Sprintf("self.assertIsNotNone(%s)", strings.TrimSpace(strings.TrimSuffix(expr, " is not None")))
	case strings.HasSuffix(expr, " is None"):
		return indent + fmt.Sprintf("self.assertIsNone(%s)", strings.TrimSpace(strings.TrimSuffix(expr, " is None")))
	case strings.HasPrefix(expr, "isinstance(") && strings.HasSuffix(expr, ")"):
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(expr, "isinstance("), ")"))
		if len(args) >= 2 {
			return indent + fmt.Sprintf("self.assertIsInstance(%s, %s)", args[0], args[1])
		}
	}

	if left, right, ok := splitPythonBinaryExpr(expr, " == "); ok {
		return indent + fmt.Sprintf("self.assertEqual(%s, %s)", left, right)
	}
	if left, right, ok := splitPythonBinaryExpr(expr, " != "); ok {
		return indent + fmt.Sprintf("self.assertNotEqual(%s, %s)", left, right)
	}
	return indent + fmt.Sprintf("self.assertTrue(%s)", expr)
}

func nonEmptyPythonBody(body []string) []string {
	body = trimPythonBlankEdges(body)
	if len(body) == 0 {
		return []string{"pass"}
	}
	return body
}

func joinMultilineSection(chunks []string) []string {
	if len(chunks) == 0 {
		return nil
	}
	lines := make([]string, 0, len(chunks)*4)
	for i, chunk := range chunks {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, strings.Split(chunk, "\n")...)
	}
	return lines
}

func stripPytestParamNames(params, paramNames []string) []string {
	if len(paramNames) == 0 {
		return params
	}
	remove := map[string]bool{}
	for _, name := range paramNames {
		remove[strings.TrimSpace(name)] = true
	}
	filtered := make([]string, 0, len(params))
	for _, param := range params {
		key := strings.TrimSpace(strings.SplitN(param, "=", 2)[0])
		if remove[key] {
			continue
		}
		filtered = append(filtered, param)
	}
	return filtered
}

func buildUnittestSubTestFromPytestParametrize(paramNames []string, paramExpr string, body []string) []string {
	lines := make([]string, 0, len(body)+4)
	loopTarget := strings.Join(paramNames, ", ")
	if len(paramNames) == 1 {
		loopTarget = paramNames[0]
	}
	lines = append(lines, fmt.Sprintf("for %s in %s:", loopTarget, paramExpr))
	subParts := make([]string, 0, len(paramNames))
	for _, name := range paramNames {
		subParts = append(subParts, fmt.Sprintf("%s=%s", name, name))
	}
	lines = append(lines, "    with self.subTest("+strings.Join(subParts, ", ")+"):")
	lines = append(lines, indentPythonLines(nonEmptyPythonBody(body), "        ")...)
	return lines
}
