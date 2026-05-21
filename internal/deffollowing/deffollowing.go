// Package deffollowing is the intra-repo def-following layer for the
// assertion-counter family (assertionFreeImport, assertionFreeTest,
// weakAssertion).
//
// The legacy detectors count assertions by scanning the immediate
// test body for `assert*` / `expect*` / `t.is*` / `g.Expect*` tokens.
// That misses helper functions: a test that calls
// `verifyResponse(actual, expected)` and defines `verifyResponse` in
// the same repo gets zero assertions, even though the helper makes
// `expect(actual).toEqual(expected)` inside its body.
//
// The fix is structural: when a call expression appears in a test
// body, look up the callee's definition in-repo; if found, count
// assertion tokens inside the definition's body as transitive
// assertions. The mechanism is gated by `a1_def_following` and
// recurses at most MaxDepth levels so a fixture that pulls through
// many helpers terminates cleanly.
//
// The package is deliberately language-light. For JS/TS, Python, and
// Go it locates function/class-method definitions via regex anchored
// at the file scope. Full per-language AST resolution is planned as a
// follow-on; the structural improvement here clears the dominant FP
// class for the three consumer detectors.
package deffollowing

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "a1_def_following"

// MaxDepth is the recursion ceiling for def-following. The first level
// is the test body itself; subsequent levels follow named calls into
// in-repo function definitions. Three levels is enough to cover
// "test → helper → utility" chains without runaway work on a fixture
// that bottoms out in a deep call graph.
const MaxDepth = 3

// AssertionTokens is the set of identifier patterns that count as an
// assertion when found inside a function body. The patterns intentionally
// allow trailing characters (`assertTrue`, `expect(...)`, `t.isTrue`,
// `g.Expect`) because the actual assertion is the call, not the bare name.
var AssertionTokens = []*regexp.Regexp{
	regexp.MustCompile(`\bassert[A-Z]`),                // assertEqual, assertTrue, assertCalled, ...
	regexp.MustCompile(`\bexpect\s*\(`),                 // expect(...)
	regexp.MustCompile(`\bt\.is[A-Z]`),                  // Tap / Node t.is*
	regexp.MustCompile(`\bt\.assert[A-Z]`),              // testing.T t.assert*
	regexp.MustCompile(`\bg\.Expect`),                   // Gomega
	regexp.MustCompile(`\bnp\.testing\.[a-z]`),          // numpy
	regexp.MustCompile(`\bself\.assert[A-Z]`),           // Python unittest
	regexp.MustCompile(`\bmock\.assert_`),                // mock.assert_called_with
	regexp.MustCompile(`\.assert_called`),                // *.assert_called*
	regexp.MustCompile(`\bassert\s+\w`),                  // Python bare `assert x`
}

// Counter tracks transitive assertions across an in-repo function call
// graph. Construct once per scan; reuse across multiple test bodies in
// the same repo.
//
// Concurrency: Counter is NOT safe for concurrent use across
// goroutines. Each goroutine that needs def-following should construct
// its own Counter via NewCounter. The expensive per-repo defCache is
// the only shared state — share it across detectors that run serially
// in one pipeline pass, not across parallel goroutines.
//
// The ensure-index path is guarded by sync.Once so even an accidental
// concurrent first-use of a single Counter doesn't race on the index
// build; the visited map is mutated per-call though, so concurrent
// Count() invocations on the same Counter still race.
type Counter struct {
	root string
	// defCache maps identifier → list of bodies. Same-named helpers
	// across multiple files all get indexed; the follower sums
	// assertions across every body to avoid silently miscounting when
	// utility helpers shadow the test helper a caller actually invokes.
	defCache  map[string][]string
	indexOnce sync.Once
	visited   map[string]bool
	// countMu serializes Count / CountTransitive so a single Counter
	// can be shared across parallel scan goroutines without racing on
	// the visited map. The expensive index build is sync.Once-guarded
	// independently.
	countMu sync.Mutex
}

// NewCounter constructs a Counter for the given repo root.
func NewCounter(root string) *Counter {
	return &Counter{
		root:     root,
		defCache: map[string][]string{},
		visited:  map[string]bool{},
	}
}

// Count returns the number of assertion tokens found in `body` plus
// any transitively reachable in-repo function bodies (up to MaxDepth).
// The mechanism's state is consulted: off → only count tokens in the
// supplied body (legacy behavior); shadow/on → follow definitions.
func (c *Counter) Count(reg *mechanisms.Registry, body string) int {
	count := countAssertions(body)
	if reg.State(MechanismName) == mechanisms.StateOff {
		return count
	}
	c.ensureDefIndex()
	c.countMu.Lock()
	defer c.countMu.Unlock()
	// Reset per-test visited set so two unrelated tests don't share
	// "already counted" state.
	c.visited = map[string]bool{}
	return count + c.followCalls(body, 1)
}

// CountTransitive runs the def-following without the legacy-body
// addition, useful for tests that want to inspect only the
// transitively-discovered count.
func (c *Counter) CountTransitive(body string, maxDepth int) int {
	c.ensureDefIndex()
	c.countMu.Lock()
	defer c.countMu.Unlock()
	c.visited = map[string]bool{}
	if maxDepth > MaxDepth {
		maxDepth = MaxDepth
	}
	return c.followCallsWithCap(body, 1, maxDepth)
}

func (c *Counter) followCalls(body string, depth int) int {
	return c.followCallsWithCap(body, depth, MaxDepth)
}

func (c *Counter) followCallsWithCap(body string, depth, cap int) int {
	if depth > cap {
		return 0
	}
	total := 0
	for _, name := range extractCallNames(body) {
		if c.visited[name] {
			continue
		}
		defs, ok := c.defCache[name]
		if !ok || len(defs) == 0 {
			continue
		}
		c.visited[name] = true
		// Sum across every body indexed under the same name. When two
		// files define the same helper, we don't know which one the
		// caller actually resolves to; counting both is the safest
		// stance — under-counting assertions over-flags
		// assertionFreeTest, the worse failure mode.
		for _, def := range defs {
			total += countAssertions(def)
			total += c.followCallsWithCap(def, depth+1, cap)
		}
	}
	return total
}

// extractCallNames returns the set of identifiers that appear as the
// callee in a `call(` expression in `body`. The returned slice is
// deduplicated.
var callRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

func extractCallNames(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range callRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if name == "" || isLanguageKeyword(name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func isLanguageKeyword(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "case", "return", "function", "var", "let",
		"const", "class", "new", "throw", "try", "catch", "finally", "import", "export",
		"async", "await", "yield", "def", "True", "False", "None",
		"func", "package", "type", "struct", "interface", "map", "chan",
		"go", "select", "defer", "range":
		return true
	}
	return false
}

// assertionSpan is one (start, end) byte range that a pattern matched.
type assertionSpan struct{ start, end int }

// countAssertions counts how many distinct assertion sites `body`
// contains. Multiple regex patterns may match overlapping byte ranges
// (e.g. `self.assertEqual` matches both `\bassert[A-Z]` and
// `\bself\.assert[A-Z]`); the dedup pass collapses overlapping matches
// so each site is counted once.
func countAssertions(body string) int {
	var spans []assertionSpan
	for _, re := range AssertionTokens {
		for _, m := range re.FindAllStringIndex(body, -1) {
			spans = append(spans, assertionSpan{m[0], m[1]})
		}
	}
	if len(spans) == 0 {
		return 0
	}
	// Insertion-sort by start (n stays small in practice).
	for i := 1; i < len(spans); i++ {
		j := i
		for j > 0 && spans[j-1].start > spans[j].start {
			spans[j-1], spans[j] = spans[j], spans[j-1]
			j--
		}
	}
	count := 1
	curEnd := spans[0].end
	for i := 1; i < len(spans); i++ {
		if spans[i].start >= curEnd {
			count++
			curEnd = spans[i].end
		} else if spans[i].end > curEnd {
			curEnd = spans[i].end
		}
	}
	return count
}

// ensureDefIndex builds an identifier → function-body index for the
// repo on first call. Subsequent calls return immediately. sync.Once
// guarantees the walk runs exactly once even under concurrent first-
// use; once built the index is read-only and shared safely.
func (c *Counter) ensureDefIndex() {
	c.indexOnce.Do(c.buildDefIndex)
}

func (c *Counter) buildDefIndex() {
	_ = filepath.Walk(c.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == ".venv" || name == "venv" ||
				name == "dist" || name == "build" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".py", ".go":
			// continue
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		c.indexDefinitions(string(data), ext)
		return nil
	})
}

func (c *Counter) indexDefinitions(content, ext string) {
	switch ext {
	case ".py":
		c.indexPython(content)
	case ".go":
		c.indexGo(content)
	default:
		c.indexJS(content)
	}
}

// indexJS matches function declarations, const-arrow, and class method
// bodies in JS/TS source. Bodies are captured by brace-balanced scan.
//
// jsMethodRe accepts an optional `async` prefix and skips over it via
// a non-capturing group, so `async fn() {}` shorthand methods get
// indexed under `fn` (not under the stripped `async` keyword).
var (
	jsFuncRe   = regexp.MustCompile(`(?m)(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(`)
	jsConstRe  = regexp.MustCompile(`(?m)(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>\s*\{`)
	jsMethodRe = regexp.MustCompile(`(?m)^\s*(?:async\s+)?([A-Za-z_$][\w$]*)\s*\([^)]*\)\s*\{`)
)

func (c *Counter) indexJS(content string) {
	for _, m := range jsFuncRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		c.indexBraceBody(name, content, m[1])
	}
	for _, m := range jsConstRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		// Find the opening `{` after the match.
		open := strings.Index(content[m[1]-1:], "{")
		if open < 0 {
			continue
		}
		c.indexBraceBody(name, content, m[1]-1+open)
	}
	for _, m := range jsMethodRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		// Skip language keywords / common false positives.
		if isLanguageKeyword(name) {
			continue
		}
		c.indexBraceBody(name, content, m[1]-1)
	}
}

// indexBraceBody captures the function body starting at the `{`
// closest to the match-end and appends it to defCache[name]. Same-
// named helpers across multiple files each contribute their body;
// followCallsWithCap sums assertions across all of them.
func (c *Counter) indexBraceBody(name, content string, fromIdx int) {
	// Find the opening brace at or after fromIdx.
	open := -1
	for i := fromIdx; i < len(content); i++ {
		if content[i] == '{' {
			open = i
			break
		}
	}
	if open < 0 {
		return
	}
	depth := 0
	for i := open; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				c.defCache[name] = append(c.defCache[name], content[open+1:i])
				return
			}
		}
	}
}

// indexPython matches `def name(` and captures the indented body.
var pyDefRe = regexp.MustCompile(`(?m)^(\s*)def\s+([A-Za-z_][\w]*)\s*\(`)

func (c *Counter) indexPython(content string) {
	matches := pyDefRe.FindAllStringSubmatchIndex(content, -1)
	for i, m := range matches {
		indent := content[m[2]:m[3]]
		name := content[m[4]:m[5]]
		// Body starts after the closing ): on the def line.
		bodyStart := strings.Index(content[m[1]:], ":")
		if bodyStart < 0 {
			continue
		}
		bodyStart = m[1] + bodyStart + 1
		// Body ends at the next line at indent <= `indent`. Use the
		// next match's start as an upper bound for efficiency.
		bodyEnd := len(content)
		for j := i + 1; j < len(matches); j++ {
			nextIndent := content[matches[j][2]:matches[j][3]]
			if len(nextIndent) <= len(indent) {
				bodyEnd = matches[j][0]
				break
			}
		}
		c.defCache[name] = append(c.defCache[name], content[bodyStart:bodyEnd])
	}
}

// indexGo matches `func Name(` and `func (r *T) Name(` declarations.
var goFuncRe = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s+)?([A-Za-z_][\w]*)\s*\(`)

func (c *Counter) indexGo(content string) {
	for _, m := range goFuncRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		c.indexBraceBody(name, content, m[1]-1)
	}
}

// ── Shadow-mode helper ────────────────────────────────────────────

// GateLift is the canonical wire-up for assertion-counter detectors.
// Given the immediate-body count and a Counter, it computes the
// transitive count. When state=off, returns the immediate count.
// When state=shadow and the transitive count exceeds the immediate
// count, emits a would-suppress event (the finding would have been
// dropped because the transitive count crosses the assertion-presence
// threshold). When state=on, returns the transitive count and lets
// the caller's threshold logic make the final call.
func GateLift(reg *mechanisms.Registry, c *Counter, body, ruleID, file string, immediateCount int) int {
	state := reg.State(MechanismName)
	if state == mechanisms.StateOff {
		return immediateCount
	}
	transitive := c.CountTransitive(body, MaxDepth)
	total := immediateCount + transitive
	if state == mechanisms.StateShadow && transitive > 0 && immediateCount == 0 {
		shadow.Emit(shadow.Event{
			Mechanism: MechanismName,
			RuleID:    ruleID,
			Action:    shadow.ActionSuppress,
			File:      file,
			Reasons: []string{
				"transitive assertion(s) reachable via in-repo def-following",
			},
		})
	}
	return total
}
