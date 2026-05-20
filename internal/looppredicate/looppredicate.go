// Package looppredicate is the loop-predicate gate for the
// dynamicTestGeneration detector. The legacy detector flags any
// it() / test() call inside a `describe` body — including
// `it.each([...])` table-driven tests, `cases.forEach(c => it(c.name,
// ...))`, and straightforward `for` loops generating per-iteration
// tests. Those are the standard table-test idiom, not a bug.
//
// IsTestBuilderInLoop reports whether the line in the source file is
// wrapped by a loop construct: `for (...) { ... it(...) }`,
// `arr.forEach(...)`, `arr.map(...)`, `it.each([...])`, or
// `test.each(...)`. When the predicate fires (returns true), the
// dynamicTestGeneration finding for that line should be suppressed —
// the test generation is intentional.
//
// The walker handles single/double/backtick strings, line + block
// comments, and properly nests both loop scopes and function scopes.
// Each loop is recorded as a frame; the frame closes when its opening
// brace's matching `}` is seen.
package looppredicate

import (
	"os"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "a3_loop_predicate"

// IsTestBuilderInLoop reports whether the source line is wrapped by a
// loop construct. Returns false on unreadable or empty files.
func IsTestBuilderInLoop(path string, line int) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return IsTestBuilderInLoopBytes(data, line), nil
}

// IsTestBuilderInLoopBytes is the in-memory variant.
func IsTestBuilderInLoopBytes(data []byte, targetLine int) bool {
	s := string(data)
	currentLine := 1
	depth := 0
	type frame struct {
		isLoop bool
		depth  int
	}
	stack := []frame{{isLoop: false, depth: 0}}
	inSingle, inDouble := false, false
	// Template literals nest: each open `` ` `` pushes; each `${...}`
	// suspends string state until the matching `}`.
	backtickDepth := 0
	templateStack := []int{}
	inLineComment, inBlockComment := false, false

	inBacktickActive := func() bool {
		return backtickDepth > 0 && (len(templateStack) == 0 || templateStack[len(templateStack)-1] < depth)
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			currentLine++
			inLineComment = false
			if currentLine > targetLine {
				break
			}
			continue
		}
		if inLineComment {
			continue
		}
		if inBlockComment {
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingle || inDouble {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '\'' && inSingle {
				inSingle = false
			} else if c == '"' && inDouble {
				inDouble = false
			}
			continue
		}
		if inBacktickActive() {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '`' {
				backtickDepth--
				continue
			}
			if c == '$' && i+1 < len(s) && s[i+1] == '{' {
				depth++
				templateStack = append(templateStack, depth)
				i++
				continue
			}
			continue
		}
		switch c {
		case '/':
			if i+1 < len(s) && s[i+1] == '/' {
				inLineComment = true
				i++
				continue
			}
			if i+1 < len(s) && s[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		case '\'':
			inSingle = true
			continue
		case '"':
			inDouble = true
			continue
		case '`':
			backtickDepth++
			continue
		case '{':
			depth++
			loop := loopScopeOpensAt(s, i)
			stack = append(stack, frame{isLoop: loop, depth: depth})
		case '}':
			if len(templateStack) > 0 && templateStack[len(templateStack)-1] == depth {
				templateStack = templateStack[:len(templateStack)-1]
				depth--
				continue
			}
			top := stack[len(stack)-1]
			if top.depth == depth {
				stack = stack[:len(stack)-1]
			}
			depth--
		}
		if currentLine == targetLine {
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j].isLoop {
					return true
				}
			}
			return false
		}
	}
	return false
}

// loopScopeOpensAt reports whether the `{` at index i opens a loop
// construct. Scans back up to ~200 chars for known loop tokens.
var loopOpenerRe = regexp.MustCompile(
	`(?:^|[^A-Za-z0-9_$])(for|while|do)\s*\(|` +
		`\.\s*(forEach|map|filter|reduce|reduceRight|flatMap|some|every)\s*\(|` +
		`\b(it|test|describe)\.\s*(each|concurrent\.each)\s*\(|` +
		`\b(it|test)\s*\.\s*each\b`,
)

func loopScopeOpensAt(s string, openBraceIdx int) bool {
	start := openBraceIdx - 200
	if start < 0 {
		start = 0
	}
	prefix := s[start:openBraceIdx]
	return loopOpenerRe.MatchString(prefix)
}

// ── Gate helper ────────────────────────────────────────────────────

// Gate is the canonical mechanism-state wire-up for the
// dynamicTestGeneration detector. Returns Keep=true when the
// mechanism is off, when the line is not in a loop, or on
// unreadable file. Returns Keep=false when the mechanism is on AND
// the line is wrapped by a loop. Shadow → keeps but emits a
// would-suppress event.
func Gate(reg *mechanisms.Registry, path string, line int, ruleID string) (keep bool) {
	state := reg.State(MechanismName)
	if state == mechanisms.StateOff {
		return true
	}
	inLoop, err := IsTestBuilderInLoop(path, line)
	if err != nil {
		return true // fail open
	}
	if !inLoop {
		return true
	}
	if state == mechanisms.StateOn {
		return false
	}
	// Shadow.
	shadow.Emit(shadow.Event{
		Mechanism: MechanismName,
		RuleID:    ruleID,
		Action:    shadow.ActionSuppress,
		File:      path,
		Line:      line,
		Reasons:   []string{"test builder wrapped by a loop construct"},
	})
	return true
}

// SourceShapes returns a short human-readable list of the loop shapes
// the predicate recognizes. Used by `terrain doctor` to explain why a
// finding was demoted.
func SourceShapes() []string {
	return []string{
		"for (...)",
		"while (...)",
		"do { ... } while",
		".forEach(...)",
		".map(...)",
		"it.each([...])",
		"test.each([...])",
		"describe.each([...])",
	}
}

// reservedWordOnly is unused; kept as an extension point for a future
// "skip when the loop is bounded by a known small literal" optimisation.
var _ = strings.Builder{}
