package convert

import (
	"fmt"
	"strings"
)

// ConvertNose2ToPytestSource rewrites the common nose/nose2 assertion and
// decorator surface into high-confidence pytest output.
func ConvertNose2ToPytestSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	blocks := parsePythonBlocks(strings.ReplaceAll(source, "\r\n", "\n"), 0)
	imports := make([]string, 0, 4)
	seenImports := make(map[string]bool)
	prelude := make([]string, 0, 16)
	emitted := make([]string, 0, 32)
	needPytest := false

	for _, block := range blocks {
		switch block.Kind {
		case "raw":
			collectPythonPrelude(block.Raw, &imports, seenImports, &prelude, func(trimmed string) bool {
				return strings.HasPrefix(trimmed, "from nose.tools import") ||
					strings.HasPrefix(trimmed, "from nose2.tools import") ||
					trimmed == "import nose2" ||
					trimmed == "import nose"
			})
		case "function":
			name, _, _ := extractPythonFuncParts(block.Signature)
			if strings.HasPrefix(name, "test_") {
				testLines, usesPytest := convertNoseTestToPytest(block)
				emitted = append(emitted, testLines...)
				emitted = append(emitted, "")
				needPytest = needPytest || usesPytest
			} else {
				prelude = append(prelude, block.Raw...)
			}
		default:
			prelude = append(prelude, block.Raw...)
		}
	}

	if needPytest {
		appendUniqueLine(&imports, seenImports, "import pytest")
	}
	return joinPythonSections(imports, prelude, emitted), nil
}

func convertNoseTestToPytest(block pythonBlock) ([]string, bool) {
	name, _, _ := extractPythonFuncParts(block.Signature)
	lines := make([]string, 0, len(block.Decorators)+len(block.Body)+2)
	needPytest := false

	for _, decorator := range block.Decorators {
		converted, usesPytest, ok := convertNoseDecorator(decorator, block.Signature)
		if ok {
			lines = append(lines, converted)
			needPytest = needPytest || usesPytest
			continue
		}
		lines = append(lines, renderCommentedPythonBlock([]string{decorator}, "manual nose decorator migration required")...)
	}

	lines = append(lines, fmt.Sprintf("def %s%s:", name, noseSignatureSuffix(block.Signature)))
	lines = append(lines, indentPythonLines(convertNoseBodyToPytest(block.Body), "    ")...)
	return trimPythonBlankEdges(lines), needPytest
}

func noseSignatureSuffix(signature string) string {
	_, params, _ := extractPythonFuncParts(signature)
	if len(params) == 0 {
		return "()"
	}
	return "(" + strings.Join(params, ", ") + ")"
}

func convertNoseDecorator(decorator, signature string) (string, bool, bool) {
	trimmed := strings.TrimSpace(decorator)
	switch {
	case strings.HasPrefix(trimmed, "@params("):
		args, ok := extractDecoratorArgs(trimmed)
		if !ok {
			return "", false, false
		}
		_, params, _ := extractPythonFuncParts(signature)
		paramName := "params"
		if len(params) > 0 {
			paramName = strings.TrimSpace(params[0])
		}
		return fmt.Sprintf("@pytest.mark.parametrize(%q, [%s])", paramName, strings.Join(args, ", ")), true, true
	case strings.HasPrefix(trimmed, "@attr("):
		args, ok := extractDecoratorArgs(trimmed)
		if !ok || len(args) == 0 {
			return "", false, false
		}
		mark := strings.Trim(args[0], `"'`)
		if mark == "" {
			return "", false, false
		}
		return "@pytest.mark." + mark, true, true
	default:
		return "", false, false
	}
}

func convertNoseBodyToPytest(body []string) []string {
	out := make([]string, 0, len(body))
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "assert_equal("):
			out = append(out, convertNoseAssertionLine(line, "assert_equal", "=="))
		case strings.HasPrefix(trimmed, "assert_true("):
			out = append(out, convertNoseUnaryAssertionLine(line, "assert_true", false))
		case strings.HasPrefix(trimmed, "assert_false("):
			out = append(out, convertNoseUnaryAssertionLine(line, "assert_false", true))
		case strings.HasPrefix(trimmed, "assert_in("):
			out = append(out, convertNoseAssertionLine(line, "assert_in", "in"))
		default:
			out = append(out, line)
		}
	}
	return nonEmptyPythonBody(trimPythonBlankEdges(out))
}

func convertNoseAssertionLine(line, method, operator string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return indent + "# TERRAIN-TODO: manual nose assertion migration required"
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	if len(args) < 2 {
		return indent + "# TERRAIN-TODO: manual nose assertion migration required"
	}
	switch operator {
	case "==", "in":
		return indent + fmt.Sprintf("assert %s %s %s", args[0], operator, args[1])
	default:
		return indent + "# TERRAIN-TODO: manual nose assertion migration required"
	}
}

func convertNoseUnaryAssertionLine(line, method string, negate bool) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return indent + "# TERRAIN-TODO: manual nose assertion migration required"
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	if len(args) < 1 {
		return indent + "# TERRAIN-TODO: manual nose assertion migration required"
	}
	if negate {
		return indent + fmt.Sprintf("assert not %s", args[0])
	}
	return indent + fmt.Sprintf("assert %s", args[0])
}
