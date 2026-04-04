package convert

import (
	"fmt"
	"strings"
)

// ConvertUnittestToPytestSource rewrites the common unittest surface into
// high-confidence pytest output and comments class-only patterns that need
// manual follow-up.
func ConvertUnittestToPytestSource(source string) (string, error) {
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
				return trimmed == "import unittest" || strings.HasPrefix(trimmed, "from unittest ")
			})
			for _, line := range block.Raw {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "def setUpModule") || strings.HasPrefix(trimmed, "def tearDownModule") {
					prelude = append(prelude, renderCommentedPythonBlock([]string{trimmed}, "module-level unittest lifecycle requires manual pytest migration")...)
					needPytest = true
				}
			}
		case "class":
			_, bases := extractPythonClassParts(block.Signature)
			if !isUnittestClassBases(bases) {
				prelude = append(prelude, block.Raw...)
				continue
			}
			classLines, classNeedsPytest := convertUnittestClassToPytest(block)
			needPytest = needPytest || classNeedsPytest
			emitted = append(emitted, classLines...)
		default:
			prelude = append(prelude, block.Raw...)
		}
	}

	if needPytest {
		appendUniqueLine(&imports, seenImports, "import pytest")
	}
	if len(imports) == 0 && len(emitted) == 0 {
		return ensureTrailingNewline(strings.ReplaceAll(source, "\r\n", "\n")), nil
	}
	return joinPythonSections(imports, prelude, emitted), nil
}

func isUnittestClassBases(bases string) bool {
	bases = strings.TrimSpace(bases)
	return strings.Contains(bases, "unittest.TestCase") || strings.Contains(bases, "TestCase")
}

func convertUnittestClassToPytest(block pythonBlock) ([]string, bool) {
	methodBlocks := parsePythonBlocksLines(block.Body, 0)
	lines := make([]string, 0, 24)
	needPytest := false
	var setUpBlock, tearDownBlock *pythonBlock

	for _, method := range methodBlocks {
		switch method.Kind {
		case "raw":
			if len(trimPythonBlankEdges(method.Raw)) > 0 {
				lines = append(lines, renderCommentedPythonBlock(method.Raw, "class-level unittest statements require manual pytest migration")...)
				lines = append(lines, "")
			}
		case "function":
			name, _, _ := extractPythonFuncParts(method.Signature)
			switch name {
			case "setUp":
				copyBlock := method
				setUpBlock = &copyBlock
			case "tearDown":
				copyBlock := method
				tearDownBlock = &copyBlock
			default:
				if strings.HasPrefix(name, "test_") {
					testLines, testNeedsPytest := convertUnittestTestMethodToPytest(method)
					needPytest = needPytest || testNeedsPytest
					lines = append(lines, testLines...)
					lines = append(lines, "")
				} else {
					lines = append(lines, renderCommentedPythonBlock(method.Raw, "helper unittest methods require manual pytest migration")...)
					lines = append(lines, "")
				}
			}
		default:
			lines = append(lines, renderCommentedPythonBlock(block.Raw, "manual unittest class migration required")...)
			lines = append(lines, "")
		}
	}

	if setUpBlock != nil || tearDownBlock != nil {
		fixtureLines := buildPytestFixtureFromUnittest(setUpBlock, tearDownBlock)
		lines = append(fixtureLines, lines...)
		needPytest = true
	}

	return trimPythonBlankEdges(lines), needPytest
}

func buildPytestFixtureFromUnittest(setUpBlock, tearDownBlock *pythonBlock) []string {
	body := make([]string, 0, 12)
	name := "setup"
	if setUpBlock != nil && tearDownBlock != nil {
		name = "setup_teardown"
	}
	if setUpBlock == nil && tearDownBlock != nil {
		name = "teardown"
	}

	if setUpBlock != nil {
		body = append(body, convertUnittestBodyToPytest(setUpBlock.Body)...)
	}
	if tearDownBlock != nil {
		body = append(body, "yield")
		body = append(body, convertUnittestBodyToPytest(tearDownBlock.Body)...)
	}
	body = nonEmptyPythonBody(body)

	lines := []string{
		"@pytest.fixture(autouse=True)",
		fmt.Sprintf("def %s():", name),
	}
	lines = append(lines, indentPythonLines(body, "    ")...)
	lines = append(lines, "")
	return lines
}

func convertUnittestTestMethodToPytest(block pythonBlock) ([]string, bool) {
	name, params, _ := extractPythonFuncParts(block.Signature)
	lines := make([]string, 0, len(block.Decorators)+len(block.Body)+4)
	needPytest := false

	for _, decorator := range block.Decorators {
		if converted, ok := buildPytestDecoratorFromUnittest(decorator); ok {
			lines = append(lines, converted)
			needPytest = true
			continue
		}
		lines = append(lines, renderCommentedPythonBlock([]string{decorator}, "manual unittest decorator migration required")...)
	}

	params = stripSelfParam(params)
	if len(params) > 0 {
		lines = append(lines, "# TERRAIN-TODO: unittest method parameters require manual pytest migration")
	}
	lines = append(lines, fmt.Sprintf("def %s():", name))
	body, bodyNeedsPytest := convertUnittestBodyLinesToPytest(block.Body)
	needPytest = needPytest || bodyNeedsPytest
	lines = append(lines, indentPythonLines(nonEmptyPythonBody(body), "    ")...)
	return trimPythonBlankEdges(lines), needPytest
}

func convertUnittestBodyToPytest(body []string) []string {
	lines, _ := convertUnittestBodyLinesToPytest(body)
	return lines
}

func convertUnittestBodyLinesToPytest(body []string) ([]string, bool) {
	out := make([]string, 0, len(body))
	needPytest := false
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "with self.assertRaises("):
			out = append(out, strings.Replace(line, "with self.assertRaises(", "with pytest.raises(", 1))
			needPytest = true
		case strings.HasPrefix(trimmed, "with self.assertRaisesRegex("):
			out = append(out, convertAssertRaisesRegexLine(line))
			needPytest = true
		case strings.HasPrefix(trimmed, "with self.assertWarns("):
			out = append(out, strings.Replace(line, "with self.assertWarns(", "with pytest.warns(", 1))
			needPytest = true
		case strings.Contains(trimmed, "self.assert"):
			converted, usesPytest := convertUnittestAssertionLineToPytest(line)
			out = append(out, converted)
			needPytest = needPytest || usesPytest
		default:
			out = append(out, line)
		}
	}
	return trimPythonBlankEdges(out), needPytest
}

func convertAssertRaisesRegexLine(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return indent + "# TERRAIN-TODO: manual assertRaisesRegex migration required"
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	if len(args) < 2 {
		return indent + "# TERRAIN-TODO: manual assertRaisesRegex migration required"
	}
	return indent + fmt.Sprintf("with pytest.raises(%s, match=%s):", args[0], args[1])
}

func convertUnittestAssertionLineToPytest(line string) (string, bool) {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	index := strings.Index(trimmed, "self.")
	if index < 0 {
		return line, false
	}
	call := trimmed[index:]
	open := strings.IndexByte(call, '(')
	close := strings.LastIndexByte(call, ')')
	if open < 0 || close <= open {
		return indent + "# TERRAIN-TODO: manual unittest assertion migration required", false
	}
	method := call[len("self."):open]
	args := splitTopLevelArgs(call[open+1 : close])

	switch method {
	case "assertEqual":
		if len(args) >= 2 {
			return indent + fmt.Sprintf("assert %s == %s", args[0], args[1]), false
		}
	case "assertNotEqual":
		if len(args) >= 2 {
			return indent + fmt.Sprintf("assert %s != %s", args[0], args[1]), false
		}
	case "assertTrue":
		if len(args) >= 1 {
			return indent + fmt.Sprintf("assert %s", args[0]), false
		}
	case "assertFalse":
		if len(args) >= 1 {
			return indent + fmt.Sprintf("assert not %s", args[0]), false
		}
	case "assertIsNone":
		if len(args) >= 1 {
			return indent + fmt.Sprintf("assert %s is None", args[0]), false
		}
	case "assertIsNotNone":
		if len(args) >= 1 {
			return indent + fmt.Sprintf("assert %s is not None", args[0]), false
		}
	case "assertIn":
		if len(args) >= 2 {
			return indent + fmt.Sprintf("assert %s in %s", args[0], args[1]), false
		}
	case "assertNotIn":
		if len(args) >= 2 {
			return indent + fmt.Sprintf("assert %s not in %s", args[0], args[1]), false
		}
	case "assertIsInstance":
		if len(args) >= 2 {
			return indent + fmt.Sprintf("assert isinstance(%s, %s)", args[0], args[1]), false
		}
	}

	return indent + "# TERRAIN-TODO: manual unittest assertion migration required", false
}
