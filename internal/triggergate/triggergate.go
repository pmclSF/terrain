// Package triggergate is the deprecatedTestPattern trigger-gate. It
// implements two AST-shaped predicates that filter the dominant FP
// classes of the `deprecatedTestPattern` detector:
//
//  1. ImportsFrom(file, patterns) — only fire framework-specific
//     sub-rules when the file actually imports the framework. Suppresses
//     framework-mismatch FPs where (e.g.) the enzyme sub-rule fired on
//     files that never imported enzyme or any enzyme-adapter-*.
//
//  2. IsSetTimeoutAtConfigScope(file, line) — flag jest/mocha-style
//     test-timeout configuration calls (jest.setTimeout,
//     test.setTimeout) only when they appear at config scope: module
//     top-level, inside beforeAll/beforeEach blocks, or in setup
//     files. In-test scope calls are intentional per-test overrides,
//     not deprecated config.
//
// The gate is mechanism-gated by `deprecated_test_pattern_trigger_gate`.
// Off → predicates return permissive defaults (true) so legacy behavior
// runs. Shadow → predicates compute real verdicts and the caller emits
// would-suppress events on disagreement. On → the predicates' verdicts
// are authoritative.
package triggergate

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "deprecated_test_pattern_trigger_gate"

// ImportsFrom reports whether the file at `path` imports any module
// matching one of `patterns`. Patterns may use a single trailing `*`
// as a wildcard (e.g. "enzyme-adapter-*" matches "enzyme-adapter-react-16").
//
// Recognizes:
//   - ES import: `import X from 'pkg'`, `import {X} from 'pkg'`
//   - CommonJS:  `require('pkg')`, `const X = require('pkg')`
//   - TS import-type: `import type {X} from 'pkg'`
//   - Dynamic import: `import('pkg')`
//
// Comments are stripped before matching, so `// import 'enzyme'` does
// NOT count as an import.
func ImportsFrom(path string, patterns []string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return importsFromBytes(data, patterns), nil
}

// ImportsFromBytes is the in-memory variant of ImportsFrom.
func ImportsFromBytes(data []byte, patterns []string) bool {
	return importsFromBytes(data, patterns)
}

func importsFromBytes(data []byte, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	s := stripBlockComments(string(data))
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		// Drop trailing line comment for cleaner matching.
		if i := findLineComment(line); i >= 0 {
			line = line[:i]
		}
		for _, m := range importStmtRe.FindAllStringSubmatch(line, -1) {
			module := firstNonEmptySubmatch(m, 1, 2, 3, 4, 5)
			if module == "" {
				continue
			}
			if matchesAny(module, patterns) {
				return true
			}
		}
	}
	return false
}

// importStmtRe captures the imported module name from ES, CJS, and
// dynamic import shapes. Each alternation has one capture group; the
// outer code picks the first non-empty submatch.
var importStmtRe = regexp.MustCompile(
	// ES: import ... from 'mod' or "mod"
	`import\s+(?:[^'"]+\s+from\s+)?['"]([^'"]+)['"]|` +
		// import type {X} from 'mod'
		`import\s+type\s+[^'"]+\s+from\s+['"]([^'"]+)['"]|` +
		// const x = require('mod') / require('mod')
		`require\(\s*['"]([^'"]+)['"]\s*\)|` +
		// dynamic import('mod')
		`import\(\s*['"]([^'"]+)['"]\s*\)|` +
		// Bare `import 'mod'`
		`import\s+['"]([^'"]+)['"]`,
)

func matchesAny(module string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(module, prefix) {
				return true
			}
		} else if module == p {
			return true
		}
	}
	return false
}

func firstNonEmptySubmatch(m []string, indices ...int) string {
	for _, i := range indices {
		if i < len(m) && m[i] != "" {
			return m[i]
		}
	}
	return ""
}

// IsSetTimeoutAtConfigScope reports whether the line in `path` where
// a setTimeout-style call appears is at config scope. Returns true for
// module top-level, beforeAll/beforeEach/setup-file scopes; false for
// in-test scope (inside it/test/describe blocks).
//
// Determination is a brace-depth walk from the file start to the
// target line, treating `it(`/`test(`/`describe(` as test scopes.
// Inside a test scope, brace depth >= the entry depth means we're in
// the test body. The walk handles single/double/backtick strings and
// `//`/`/* */` comments.
func IsSetTimeoutAtConfigScope(path string, line int) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return isSetTimeoutAtConfigScopeBytes(data, line), nil
}

// IsSetTimeoutAtConfigScopeBytes is the in-memory variant.
func IsSetTimeoutAtConfigScopeBytes(data []byte, line int) bool {
	return isSetTimeoutAtConfigScopeBytes(data, line)
}

// scopeKind is the kind of nesting frame the walker is in.
type scopeKind int

const (
	scopeFile scopeKind = iota
	scopeTest             // inside an it/test/describe body
)

type scopeFrame struct {
	kind  scopeKind
	depth int // brace depth at which this frame opened
}

func isSetTimeoutAtConfigScopeBytes(data []byte, targetLine int) bool {
	s := string(data)
	currentLine := 1
	depth := 0
	stack := []scopeFrame{{kind: scopeFile, depth: 0}}
	inSingle, inDouble := false, false
	// Template literals nest: each open `` ` `` pushes; each `${...}`
	// inside a template suspends the string state until the matching
	// `}`. templateStack holds the brace depth at which each pending
	// interpolation opened.
	backtickDepth := 0
	templateStack := []int{}
	inLineComment, inBlockComment := false, false

	inBacktickActive := func() bool {
		// In a backtick string when the depth is positive AND no
		// interpolation is currently open (because while we're inside
		// `${...}` we're in code, not string).
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
				i++ // skip escaped char
				continue
			}
			if c == '\'' && inSingle {
				inSingle = false
			} else if c == '"' && inDouble {
				inDouble = false
			}
			continue
		}
		// Backtick template-literal string mode (active only when no
		// `${...}` interpolation is currently open).
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
				// Enter interpolation: increment brace depth, push the
				// depth marker. Subsequent `{` and `}` are code, not
				// string content.
				depth++
				templateStack = append(templateStack, depth)
				i++ // consume the '{'
				continue
			}
			continue
		}
		// Not in string/comment.
		switch c {
		case '/':
			if i+1 < len(s) {
				if s[i+1] == '/' {
					inLineComment = true
					i++
					continue
				}
				if s[i+1] == '*' {
					inBlockComment = true
					i++
					continue
				}
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
			// If the recent identifier was a test scope, push frame.
			if testScopeOpensAt(s, i) {
				stack = append(stack, scopeFrame{kind: scopeTest, depth: depth})
			}
		case '}':
			// First check whether we're closing a template interpolation.
			if len(templateStack) > 0 && templateStack[len(templateStack)-1] == depth {
				templateStack = templateStack[:len(templateStack)-1]
				depth--
				continue
			}
			top := stack[len(stack)-1]
			if top.depth == depth && top.kind == scopeTest {
				stack = stack[:len(stack)-1]
			}
			depth--
		}
		if currentLine == targetLine {
			// We've reached the line. Decide based on the current frame.
			top := stack[len(stack)-1]
			return top.kind == scopeFile
		}
	}
	// If the target line was never reached (e.g. line > file length),
	// default to true — the gate fails open.
	return true
}

// testScopeOpensAt reports whether the brace at index i opens an
// it/test/describe block. Looks backward for the opening callsite.
var testCallRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_$])(it|test|describe)\s*(\.\w+)?\s*\(`)

func testScopeOpensAt(s string, openBraceIdx int) bool {
	// Scan back up to ~120 chars looking for the call site that opened
	// this block.
	start := openBraceIdx - 120
	if start < 0 {
		start = 0
	}
	prefix := s[start:openBraceIdx]
	return testCallRe.MatchString(prefix)
}

// stripBlockComments removes /* ... */ blocks from the input.
func stripBlockComments(s string) string {
	return blockCommentRe.ReplaceAllString(s, " ")
}

var blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// findLineComment returns the position of `//` that starts a comment
// (not inside a string literal). Returns -1 if none.
func findLineComment(line string) int {
	inSingle, inDouble, inBacktick := false, false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && (inSingle || inDouble || inBacktick) && i+1 < len(line) {
			i++
			continue
		}
		switch c {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}
		if !inSingle && !inDouble && !inBacktick {
			if c == '/' && i+1 < len(line) && line[i+1] == '/' {
				return i
			}
		}
	}
	return -1
}

// ── shadow-mode helpers ────────────────────────────────────────────

// GateImports is the canonical wire-up for sub-rules that should only
// fire when a framework is imported. Returns Keep=true when the
// imports check passes (gate is satisfied), or when the mechanism is
// off / the file is unreadable (fail open). Emits shadow events on
// would-suppress.
//
// Routes through mechanisms.GateSuppress so the off/shadow/on state
// machine is shared with every other gate helper in the codebase.
func GateImports(reg *mechanisms.Registry, path, ruleID string, patterns []string) bool {
	return mechanisms.GateSuppress(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: path},
		true, func() mechanisms.PredicateResult {
			hit, err := ImportsFrom(path, patterns)
			if err != nil {
				// Unreadable file → fail open (predicate does not fire).
				return mechanisms.PredicateResult{Fired: false}
			}
			if hit {
				return mechanisms.PredicateResult{Fired: false}
			}
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"file does not import any of: " + strings.Join(patterns, ", ")},
			}
		})
}

// GateSetTimeoutScope is the canonical wire-up for setTimeout config-
// scope filtering. Returns Keep=true when the call is at config scope
// (the deprecation should fire) or the mechanism is off. Returns
// Keep=false when the call is in-test scope and the mechanism is on.
func GateSetTimeoutScope(reg *mechanisms.Registry, path string, line int, ruleID string) bool {
	return mechanisms.GateSuppress(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: path, Line: line},
		true, func() mechanisms.PredicateResult {
			atConfig, err := IsSetTimeoutAtConfigScope(path, line)
			if err != nil || atConfig {
				return mechanisms.PredicateResult{Fired: false}
			}
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"setTimeout call is in-test scope, not config scope"},
			}
		})
}
