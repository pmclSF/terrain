// Package surfacelit implements the SurfaceLiteralPresenceGate.
//
// Before an AI-moat detector emits a finding referencing a surface name
// (e.g. "gpt-4o", "chatbot-prompt", "summarizer_template"), the gate
// verifies that the name literally appears as an identifier-like token
// in the flagged file. Names that the detector inferred but that don't
// actually appear in the file are the dominant FP class across 4 of
// 6 AI-moat detectors (surface-hallucination, 21-77% per detector).
//
// The check is intentionally narrow:
//
//   - Comments are stripped before the check, so a name that appears
//     only in a `// note: we used to call gpt-4` line doesn't count.
//   - The name must appear with non-token-character boundaries on both
//     sides — `gpt-4` matches inside `"model": "gpt-4"` (boundary chars
//     are `"` and `"`) but does not match inside `gpt-4-turbo` (`-` and
//     `-` are token chars on at least one side).
//   - File reads beyond MaxFileBytes return Skipped — the gate fails
//     open: when in doubt the finding stays.
//
// The gate is LLM-free, class-level, and reads files Terrain has
// already been authorized to scan. It does not parse syntax — it only
// strips comments and does a token-boundary substring search. That's
// the price of language-agnostic coverage; the AST-precise version is
// future work tracked in the mechanism's PromotionPlan.
package surfacelit

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MaxFileBytes is the upper bound for files the gate will read. Beyond
// this size we return Skipped — the gate fails open. Default mirrors
// the path-noise filter's "small file" intuition; tunable per call.
const MaxFileBytes = 4 * 1024 * 1024 // 4 MiB

// strippedCache caches comment-stripped file content keyed by absolute
// path + modtime. AI-moat detectors call Check per finding per file;
// the cache keeps repeated calls on the same file off the disk and
// off the regex pass.
//
// Bounded by file size: each entry holds a stripped copy of the file
// content. Eviction is a simple all-clear at ClearCache call sites
// (per-pipeline-run), keeping the implementation footprint small.
var (
	strippedCacheMu sync.RWMutex
	strippedCache   = map[string]strippedEntry{}
)

type strippedEntry struct {
	modTime time.Time
	size    int64
	body    []byte
}

// ClearCache drops the cached stripped-file content. The engine
// pipeline calls this between runs so a long-lived process doesn't
// accumulate stale entries.
func ClearCache() {
	strippedCacheMu.Lock()
	strippedCache = map[string]strippedEntry{}
	strippedCacheMu.Unlock()
}

// Result describes the gate's verdict for one (name, file) check.
type Result int

const (
	// Skipped means the gate could not produce a verdict (file unreadable,
	// too large, empty name). Callers fail open — keep the finding.
	Skipped Result = iota

	// Present means the name appeared as a token in the file's non-comment
	// content. The finding is allowed through.
	Present

	// Absent means the name was not found. The finding should be
	// suppressed (or demoted to shadow per the mechanism's state).
	Absent
)

func (r Result) String() string {
	switch r {
	case Present:
		return "present"
	case Absent:
		return "absent"
	default:
		return "skipped"
	}
}

// Check reads the file at path and returns whether `name` appears as a
// token in the file's non-comment content.
//
// Empty `name`, unreadable file, or oversized file all return Skipped.
// The gate fails open by design — callers treat Skipped the same as
// Present (the finding survives).
//
// The comment-stripped file content is cached per (path, mtime, size)
// so repeated Check calls against the same file by multiple detectors
// don't re-read or re-strip.
func Check(name, path string) (Result, error) {
	if strings.TrimSpace(name) == "" {
		return Skipped, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return Skipped, err
	}
	if info.Size() > MaxFileBytes {
		return Skipped, nil
	}
	stripped, err := cachedStripped(path, info)
	if err != nil {
		return Skipped, err
	}
	if containsAsToken(stripped, name) {
		return Present, nil
	}
	return Absent, nil
}

// cachedStripped returns the comment-stripped body for path, hitting
// the per-process cache when (path, mtime, size) match the last read.
func cachedStripped(path string, info os.FileInfo) ([]byte, error) {
	strippedCacheMu.RLock()
	entry, ok := strippedCache[path]
	strippedCacheMu.RUnlock()
	if ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.body, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body := stripComments(data)
	strippedCacheMu.Lock()
	strippedCache[path] = strippedEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		body:    body,
	}
	strippedCacheMu.Unlock()
	return body, nil
}

// CheckBytes performs the gate check against in-memory file content.
// Useful for tests and for callers that already have the file buffered.
func CheckBytes(name string, data []byte) Result {
	if strings.TrimSpace(name) == "" {
		return Skipped
	}
	stripped := stripComments(data)
	if containsAsToken(stripped, name) {
		return Present
	}
	return Absent
}

// stripComments removes line comments (//... or #...) and block comments
// (/* ... */) from the input. Triple-quoted Python strings are NOT
// stripped — they're frequently used as inline string literals that
// legitimately contain surface names. The gate accepts that trade-off
// because docstring placement of a surface name is itself a meaningful
// signal that the surface exists in the file.
func stripComments(data []byte) []byte {
	s := string(data)
	s = blockCommentRe.ReplaceAllString(s, " ")
	// Process line by line for line-comment stripping. Avoid touching
	// `//` that lives inside a string literal — best effort: don't strip
	// when the prefix of the line contains an odd number of quotes.
	var out strings.Builder
	out.Grow(len(s))
	for _, line := range splitLines(s) {
		trimmed := strings.TrimLeft(line, " \t")
		// Whole-line shell/Python comment.
		if strings.HasPrefix(trimmed, "#") {
			out.WriteByte('\n')
			continue
		}
		// Whole-line // comment.
		if strings.HasPrefix(trimmed, "//") {
			out.WriteByte('\n')
			continue
		}
		// Mid-line // comment, only when not inside a likely string.
		if idx := findLineCommentStart(line); idx >= 0 {
			out.WriteString(line[:idx])
			out.WriteByte('\n')
			continue
		}
		// Mid-line # comment — only strip when we're past any opening
		// quote on the line.
		if idx := findHashCommentStart(line); idx >= 0 {
			out.WriteString(line[:idx])
			out.WriteByte('\n')
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

var blockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

func splitLines(s string) []string {
	// strings.Split with "\n" handles \n; CRLF endings still produce a
	// trailing \r which we trim per-line elsewhere.
	return strings.Split(s, "\n")
}

// findLineCommentStart returns the byte index of `//` if it's not
// inside a string literal on the line, otherwise -1.
func findLineCommentStart(line string) int {
	return findCommentStart(line, "//")
}

func findHashCommentStart(line string) int {
	return findCommentStart(line, "#")
}

// findCommentStart returns the first occurrence of `marker` on `line`
// that is not preceded by an unbalanced single or double quote. Best
// effort — strings with escaped quotes are handled via a simple
// backslash check.
func findCommentStart(line, marker string) int {
	inSingle, inDouble, inBacktick := false, false, false
	escape := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && (inSingle || inDouble || inBacktick) {
			escape = true
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
			if i+len(marker) <= len(line) && line[i:i+len(marker)] == marker {
				return i
			}
		}
	}
	return -1
}

// containsAsToken reports whether `name` appears in `data` with non-
// token characters on both sides (or at file boundaries). A "token
// character" is [A-Za-z0-9_.-] — extended beyond standard identifier
// chars to include `-` and `.` so model names form a single unit:
// searching for "gpt-4" inside `gpt-4-turbo` does NOT match (the
// trailing `-` is a token char, so `gpt-4` is a prefix, not a full
// match). Searching for "gpt-4o-mini" inside `["gpt-4o-mini", ...]`
// does match because the surrounding `"` is not in the token class.
func containsAsToken(data []byte, name string) bool {
	if len(name) == 0 || len(name) > len(data) {
		return false
	}
	s := string(data)
	idx := 0
	for {
		hit := strings.Index(s[idx:], name)
		if hit < 0 {
			return false
		}
		pos := idx + hit
		// Boundary on the left.
		if pos == 0 || !isTokenChar(s[pos-1]) {
			// Boundary on the right.
			end := pos + len(name)
			if end == len(s) || !isTokenChar(s[end]) {
				return true
			}
		}
		idx = pos + 1
	}
}

func isTokenChar(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_' || b == '-' || b == '.'
}

// Reason returns a one-line human-readable explanation for a Result.
// Useful for the shadow-event Reasons field.
func Reason(r Result, name, path string) string {
	switch r {
	case Present:
		return fmt.Sprintf("surface literal %q present in %s", name, path)
	case Absent:
		return fmt.Sprintf("surface literal %q absent from %s", name, path)
	default:
		return fmt.Sprintf("surface literal %q check skipped for %s", name, path)
	}
}

// MechanismName is the canonical entry name in mechanisms.yaml.
const MechanismName = "surface_literal_presence_gate"

// Decision encapsulates the gate verdict + how the caller should act on
// it given the mechanism's current state.
type Decision struct {
	// Result is the raw presence check outcome.
	Result Result

	// Keep is true when the caller should keep emitting the finding.
	// False means the gate is live and the finding should be suppressed.
	Keep bool

	// ShadowAction names what the gate would have done if it were live.
	// Used by callers that emit shadow events themselves.
	ShadowAction string
}
