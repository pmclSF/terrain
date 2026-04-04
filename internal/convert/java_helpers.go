package convert

import "strings"

func splitTopLevelArgs(input string) []string {
	args := make([]string, 0, 4)
	current := strings.Builder{}
	depth := 0
	var quote byte
	escaped := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if quote != 0 {
			current.WriteByte(ch)
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
			current.WriteByte(ch)
		case '(', '[', '{':
			depth++
			current.WriteByte(ch)
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
				continue
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	tail := strings.TrimSpace(current.String())
	if tail != "" || len(args) > 0 {
		args = append(args, tail)
	}
	return args
}

func findMatchingParenSameLine(line string, open int) int {
	if open < 0 || open >= len(line) || line[open] != '(' {
		return -1
	}

	depth := 0
	var quote byte
	escaped := false

	for i := open; i < len(line); i++ {
		ch := line[i]
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
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func swapFirstTwoArgsOnCallLines(source string, methods ...string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, method := range methods {
			lines[i] = swapFirstTwoArgsOnLine(lines[i], method)
		}
	}
	return strings.Join(lines, "\n")
}

func swapFirstTwoArgsOnLine(line, method string) string {
	search := method + "("
	for start := 0; start < len(line); {
		rel := strings.Index(line[start:], search)
		if rel < 0 {
			return line
		}
		idx := start + rel
		if idx > 0 && (isIdentifierChar(line[idx-1]) || line[idx-1] == '.') {
			start = idx + len(search)
			continue
		}
		open := idx + len(method)
		close := findMatchingParenSameLine(line, open)
		if close < 0 {
			return line
		}
		args := splitTopLevelArgs(line[open+1 : close])
		if len(args) < 2 {
			start = close + 1
			continue
		}
		args[0], args[1] = args[1], args[0]
		line = line[:open+1] + strings.Join(args, ", ") + line[close:]
		start = close + 1
	}
	return line
}

func isIdentifierChar(ch byte) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func countJavaBraces(line string) (open, close int) {
	var quote byte
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
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
		case '{':
			open++
		case '}':
			close++
		}
	}

	return open, close
}
