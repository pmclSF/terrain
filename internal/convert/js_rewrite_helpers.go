package convert

import (
	"regexp"
	"strings"
)

func replaceCodeRegexMatches(source string, re *regexp.Regexp, replace func(match string, groups []string) string) string {
	if !re.MatchString(source) {
		return source
	}

	mask := jsCodeMask(source)
	matches := re.FindAllStringSubmatchIndex(source, -1)
	if len(matches) == 0 {
		return source
	}

	var out strings.Builder
	last := 0
	replaced := false

	for _, idxs := range matches {
		start, end := idxs[0], idxs[1]
		if start < 0 || end <= start || start >= len(mask) || !mask[start] {
			continue
		}

		groups := make([]string, 0, (len(idxs)-2)/2)
		for i := 2; i < len(idxs); i += 2 {
			if idxs[i] < 0 || idxs[i+1] < 0 {
				groups = append(groups, "")
				continue
			}
			groups = append(groups, source[idxs[i]:idxs[i+1]])
		}

		replacement := replace(source[start:end], groups)
		if replacement == "" {
			continue
		}

		out.WriteString(source[last:start])
		out.WriteString(replacement)
		last = end
		replaced = true
	}

	if !replaced {
		return source
	}

	out.WriteString(source[last:])
	return out.String()
}

func replaceCodeRegexString(source string, re *regexp.Regexp, repl string) string {
	return replaceCodeRegexMatches(source, re, func(match string, _ []string) string {
		return re.ReplaceAllString(match, repl)
	})
}

func jsCodeMask(source string) []bool {
	mask := make([]bool, len(source))
	var quote byte
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(source); i++ {
		ch := source[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				mask[i] = true
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(source) && source[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
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

		if ch == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
			continue
		}
		mask[i] = true
	}

	return mask
}

func rewriteSourceCalls(source, prefix string, rewrite func(args []string) (string, bool)) string {
	var out strings.Builder
	last := 0
	var quote byte
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(source); i++ {
		ch := source[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(source) && source[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
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

		if ch == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
			continue
		}

		if !strings.HasPrefix(source[i:], prefix) {
			continue
		}

		open := i + len(prefix) - 1
		close := findMatchingParenInSource(source, open)
		if close < 0 {
			continue
		}

		args := splitTopLevelArgs(source[open+1 : close])
		replacement, ok := rewrite(args)
		if !ok {
			continue
		}

		out.WriteString(source[last:i])
		out.WriteString(replacement)
		last = close + 1
		i = close
	}

	if last == 0 {
		return source
	}
	out.WriteString(source[last:])
	return out.String()
}

func findMatchingParenInSource(source string, open int) int {
	if open < 0 || open >= len(source) || source[open] != '(' {
		return -1
	}

	depth := 0
	var quote byte
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := open; i < len(source); i++ {
		ch := source[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(source) && source[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
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

		if ch == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		switch ch {
		case '\'', '"', '`':
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

func jsSourceParses(source string) bool {
	tree, ok := parseJSSyntaxTree(source)
	if tree != nil {
		tree.Close()
	}
	return ok
}
