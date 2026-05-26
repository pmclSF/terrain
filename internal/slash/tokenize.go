package slash

import (
	"fmt"
	"strings"
)

// tokenize splits a slash-command body into tokens. Whitespace
// separates tokens; quoted strings preserve internal whitespace.
//
// Supported quotes: `"..."` and `'...'`. Backslash inside double
// quotes escapes the next character literally.
//
// Examples:
//
//	"dismiss reason:false-positive"     → ["dismiss", "reason:false-positive"]
//	"dismiss reason:\"line N\""          → ["dismiss", "reason:line N"]
//	"explain terrain/ai/x"               → ["explain", "terrain/ai/x"]
func tokenize(body string) ([]string, error) {
	var tokens []string
	var buf strings.Builder
	inQuote := byte(0) // 0 = not in quote; '"' or '\'' otherwise
	i := 0
	for i < len(body) {
		c := body[i]
		if inQuote != 0 {
			if c == '\\' && inQuote == '"' && i+1 < len(body) {
				// Escape sequence inside double quotes.
				buf.WriteByte(body[i+1])
				i += 2
				continue
			}
			if c == inQuote {
				// Close quote.
				inQuote = 0
				i++
				continue
			}
			buf.WriteByte(c)
			i++
			continue
		}
		// Not in a quote.
		if c == ' ' || c == '\t' {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
			i++
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			i++
			continue
		}
		// `key:` followed by an open quote — handle by treating the
		// quote as the start of the value.
		if c == ':' && i+1 < len(body) && (body[i+1] == '"' || body[i+1] == '\'') {
			buf.WriteByte(':')
			inQuote = body[i+1]
			i += 2
			continue
		}
		buf.WriteByte(c)
		i++
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated quote (%c)", inQuote)
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens, nil
}
