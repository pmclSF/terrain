package convert

import (
	"fmt"
	"regexp"
	"strings"
)

type pythonBlock struct {
	Kind       string
	Decorators []string
	Signature  string
	Body       []string
	Raw        []string
}

var (
	rePythonFuncSignature  = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)
	rePythonClassSignature = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(([^)]*)\))?\s*:`)
)

func parsePythonBlocks(source string, indent int) []pythonBlock {
	return parsePythonBlocksLines(strings.Split(source, "\n"), indent)
}

func parsePythonBlocksLines(lines []string, indent int) []pythonBlock {
	blocks := make([]pythonBlock, 0, 8)
	for i := 0; i < len(lines); {
		if block, next, ok := tryParsePythonStructuredBlock(lines, i, indent); ok {
			blocks = append(blocks, block)
			i = next
			continue
		}

		start := i
		i++
		for i < len(lines) {
			if _, _, ok := tryParsePythonStructuredBlock(lines, i, indent); ok {
				break
			}
			if strings.TrimSpace(lines[i]) != "" && countIndent(lines[i]) < indent {
				break
			}
			i++
		}
		blocks = append(blocks, pythonBlock{
			Kind: "raw",
			Raw:  cloneLines(lines[start:i]),
		})
	}
	return mergeAdjacentRawPythonBlocks(blocks)
}

func tryParsePythonStructuredBlock(lines []string, start, indent int) (pythonBlock, int, bool) {
	if start >= len(lines) {
		return pythonBlock{}, start, false
	}

	i := start
	decorators := make([]string, 0, 2)
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			return pythonBlock{}, start, false
		}
		if countIndent(lines[i]) != indent || !strings.HasPrefix(trimmed, "@") {
			break
		}
		decorators = append(decorators, trimPythonIndent(lines[i], indent))
		i++
	}

	if i >= len(lines) || countIndent(lines[i]) != indent {
		return pythonBlock{}, start, false
	}

	trimmed := strings.TrimSpace(lines[i])
	kind := ""
	switch {
	case strings.HasPrefix(trimmed, "def "), strings.HasPrefix(trimmed, "async def "):
		kind = "function"
	case strings.HasPrefix(trimmed, "class "):
		kind = "class"
	default:
		return pythonBlock{}, start, false
	}

	signature := trimPythonIndent(lines[i], indent)
	i++
	bodyStart := i
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) != "" && countIndent(lines[i]) <= indent {
			break
		}
		i++
	}

	return pythonBlock{
		Kind:       kind,
		Decorators: decorators,
		Signature:  signature,
		Body:       dedentPythonLines(lines[bodyStart:i], indent+4),
		Raw:        dedentPythonLines(lines[start:i], indent),
	}, i, true
}

func mergeAdjacentRawPythonBlocks(blocks []pythonBlock) []pythonBlock {
	merged := make([]pythonBlock, 0, len(blocks))
	for _, block := range blocks {
		if len(merged) > 0 && block.Kind == "raw" && merged[len(merged)-1].Kind == "raw" {
			merged[len(merged)-1].Raw = append(merged[len(merged)-1].Raw, block.Raw...)
			continue
		}
		merged = append(merged, block)
	}
	return merged
}

func countIndent(line string) int {
	count := 0
	for count < len(line) && line[count] == ' ' {
		count++
	}
	return count
}

func trimPythonIndent(line string, indent int) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if len(line) < indent {
		return strings.TrimLeft(line, " ")
	}
	return line[indent:]
}

func dedentPythonLines(lines []string, indent int) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = trimPythonIndent(line, indent)
	}
	return result
}

func cloneLines(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

func extractPythonFuncParts(signature string) (name string, params []string, async bool) {
	matches := rePythonFuncSignature.FindStringSubmatch(strings.TrimSpace(signature))
	if len(matches) != 3 {
		return "", nil, false
	}
	name = matches[1]
	async = strings.HasPrefix(strings.TrimSpace(signature), "async ")
	rawParams := strings.TrimSpace(matches[2])
	if rawParams == "" {
		return name, nil, async
	}
	for _, part := range splitTopLevelArgs(rawParams) {
		param := strings.TrimSpace(part)
		if param == "" {
			continue
		}
		params = append(params, param)
	}
	return name, params, async
}

func extractPythonClassParts(signature string) (name, bases string) {
	matches := rePythonClassSignature.FindStringSubmatch(strings.TrimSpace(signature))
	if len(matches) < 2 {
		return "", ""
	}
	name = matches[1]
	if len(matches) > 2 {
		bases = strings.TrimSpace(matches[2])
	}
	return name, bases
}

func renderCommentedPythonBlock(raw []string, todo string) []string {
	lines := make([]string, 0, len(raw)+2)
	lines = append(lines, "# TERRAIN-TODO: "+todo)
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			lines = append(lines, "#")
			continue
		}
		lines = append(lines, "# "+line)
	}
	return lines
}

func appendUniqueLine(lines *[]string, seen map[string]bool, line string) {
	if seen[line] {
		return
	}
	seen[line] = true
	*lines = append(*lines, line)
}

func joinPythonSections(sections ...[]string) string {
	all := make([]string, 0, 64)
	for _, section := range sections {
		if len(section) == 0 {
			continue
		}
		if len(all) > 0 && strings.TrimSpace(all[len(all)-1]) != "" {
			all = append(all, "")
		}
		all = append(all, section...)
	}
	return ensureTrailingNewline(strings.Join(trimPythonBlankEdges(all), "\n"))
}

func trimPythonBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return nil
	}
	return collapseSequentialBlankPythonLines(lines[start:end])
}

func collapseSequentialBlankPythonLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && lastBlank {
			continue
		}
		out = append(out, line)
		lastBlank = blank
	}
	return out
}

func indentPythonLines(lines []string, indent string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, indent+line)
	}
	return out
}

func splitFixtureBodyAroundYield(body []string) (before, after []string, hasYield bool) {
	for i, line := range body {
		if strings.TrimSpace(line) == "yield" {
			return cloneLines(body[:i]), cloneLines(body[i+1:]), true
		}
	}
	return cloneLines(body), nil, false
}

func toPythonTestClassName(testName string) string {
	testName = strings.TrimSpace(testName)
	testName = strings.TrimPrefix(testName, "test_")
	if testName == "" {
		return "TestConverted"
	}
	parts := strings.FieldsFunc(testName, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	b.WriteString("Test")
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	if b.Len() == len("Test") {
		return "TestConverted"
	}
	return b.String()
}

func stripSelfParam(params []string) []string {
	filtered := make([]string, 0, len(params))
	for _, param := range params {
		if strings.TrimSpace(param) == "self" {
			continue
		}
		filtered = append(filtered, param)
	}
	return filtered
}

func splitPythonBinaryExpr(expr, op string) (string, string, bool) {
	depth := 0
	var quote byte
	escaped := false

	for i := 0; i <= len(expr)-len(op); i++ {
		ch := expr[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
			continue
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			continue
		}

		if depth == 0 && strings.HasPrefix(expr[i:], op) {
			left := strings.TrimSpace(expr[:i])
			right := strings.TrimSpace(expr[i+len(op):])
			if left != "" && right != "" {
				return left, right, true
			}
		}
	}

	return "", "", false
}

func buildPytestDecoratorFromUnittest(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "@unittest.expectedFailure"):
		return "@pytest.mark.xfail", true
	case strings.HasPrefix(trimmed, "@unittest.skipIf("):
		args, ok := extractDecoratorArgs(trimmed)
		if !ok || len(args) < 2 {
			return "", false
		}
		return fmt.Sprintf("@pytest.mark.skipif(%s, reason=%s)", args[0], args[1]), true
	case strings.HasPrefix(trimmed, "@unittest.skipUnless("):
		args, ok := extractDecoratorArgs(trimmed)
		if !ok || len(args) < 2 {
			return "", false
		}
		return fmt.Sprintf("@pytest.mark.skipif(not %s, reason=%s)", args[0], args[1]), true
	case strings.HasPrefix(trimmed, "@unittest.skip("):
		args, ok := extractDecoratorArgs(trimmed)
		if !ok || len(args) < 1 {
			return "", false
		}
		return fmt.Sprintf("@pytest.mark.skip(reason=%s)", args[0]), true
	default:
		return "", false
	}
}

func extractDecoratorArgs(line string) ([]string, bool) {
	open := strings.IndexByte(line, '(')
	close := strings.LastIndexByte(line, ')')
	if open < 0 || close <= open {
		return nil, false
	}
	return splitTopLevelArgs(line[open+1 : close]), true
}

func parsePytestParametrizeDecorator(line string) ([]string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "@pytest.mark.parametrize(") {
		return nil, "", false
	}
	args, ok := extractDecoratorArgs(trimmed)
	if !ok || len(args) < 2 {
		return nil, "", false
	}
	names, ok := parsePytestParamNames(args[0])
	if !ok || len(names) == 0 {
		return nil, "", false
	}
	return names, strings.TrimSpace(args[1]), true
}

func parsePytestParamNames(arg string) ([]string, bool) {
	trimmed := strings.TrimSpace(arg)
	switch {
	case len(trimmed) >= 2 && ((trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'')):
		raw := trimmed[1 : len(trimmed)-1]
		parts := strings.Split(raw, ",")
		names := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			names = append(names, name)
		}
		return names, len(names) > 0
	case len(trimmed) >= 2 && ((trimmed[0] == '(' && trimmed[len(trimmed)-1] == ')') || (trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']')):
		items := splitTopLevelArgs(trimmed[1 : len(trimmed)-1])
		names := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if len(item) >= 2 && ((item[0] == '"' && item[len(item)-1] == '"') || (item[0] == '\'' && item[len(item)-1] == '\'')) {
				item = item[1 : len(item)-1]
			}
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			names = append(names, item)
		}
		return names, len(names) > 0
	default:
		return nil, false
	}
}
