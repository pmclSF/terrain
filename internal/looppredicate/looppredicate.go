// Package looppredicate is the loop-predicate gate for the
// dynamicTestGeneration detector. The legacy regex over-matches:
// it fires on any it()/test() that happens to follow a forEach /
// map / for in the surrounding context, even when the test builder
// isn't actually wrapped by that loop.
//
// IsTestBuilderInLoop reports whether the line in the source file
// IS wrapped by a loop construct: `for (...) { ... it(...) }`,
// `arr.forEach(...)`, `arr.map(...)`, `it.each([...])`, or
// `test.each(...)`. The Gate keeps the finding when this is true
// (genuine dynamic test generation) and suppresses it when the AST
// shows no surrounding loop (the FP class of the regex match).
//
// The walker handles single/double/backtick strings, line + block
// comments, and properly nests both loop scopes and function scopes.
// Each loop is recorded as a frame; the frame closes when its opening
// brace's matching `}` is seen.
package looppredicate

import (
	"os"
	"regexp"

	"github.com/pmclSF/terrain/internal/mechanisms"
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
// dynamicTestGeneration detector. Routes through mechanisms.GateSuppress
// for uniform state-machine semantics.
//
// Polarity: the dynamicTestGeneration regex is over-broad and matches
// surrounding-context shapes where the it/test/describe call isn't
// actually wrapped by a loop. The gate suppresses ONLY when the AST
// confirms the line is NOT in a loop — that's the structural FP class
// the gate exists to remove.
//
//	mechanism off → Keep=true (legacy regex behavior)
//	mechanism on  + line IS in loop  → Keep=true (true positive)
//	mechanism on  + line NOT in loop → Keep=false (suppress FP)
//	mechanism shadow → Keep=true, emit would-suppress event on FP class
//	unreadable file or error → Keep=true (fail open)
func Gate(reg *mechanisms.Registry, path string, line int, ruleID string) bool {
	return mechanisms.GateSuppress(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: path, Line: line},
		true, func() mechanisms.PredicateResult {
			inLoop, err := IsTestBuilderInLoop(path, line)
			if err != nil || inLoop {
				// Fail open on read error; in-loop is a true positive.
				return mechanisms.PredicateResult{Fired: false}
			}
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"regex matched but AST confirms test builder is NOT wrapped by a loop"},
			}
		})
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
