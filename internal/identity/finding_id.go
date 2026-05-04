package identity

import (
	"fmt"
	"strconv"
	"strings"
)

// FindingID is the stable identifier for a single signal/finding emitted by
// a Terrain detector. It enables suppressions (`.terrain/suppressions.yaml`),
// the `terrain explain finding <id>` round-trip, baseline-aware gating
// (`--new-findings-only`), and cross-run deduplication.
//
// Shape:
//
//	{detector}@{normalized_path}:{anchor}#{hash}
//
// Where:
//   - detector       = the signal type (e.g. "weakAssertion")
//   - normalized_path = forward-slash, repo-relative path
//   - anchor         = symbol name when present, "L<line>" otherwise,
//                      "_" when neither is available
//   - hash           = 8 hex chars derived from the canonical form for
//                      collision resistance
//
// Example: `weakAssertion@internal/auth/login_test.go:TestLogin#a1b2c3d4`
//
// Stability guarantees:
//   - Same (detector, path, symbol, line) → same ID across runs.
//   - Whitespace changes inside the file do NOT change the ID, *as long as*
//     the symbol name and line number are preserved by the detector. (Line
//     drift is a known limitation; AST-anchored 0.3 work removes it.)
//   - File rename or symbol rename produces a new ID. That's the right
//     thing — the underlying finding has moved.
//
// Trade-offs:
//   - The ID is human-readable enough to mention in a PR comment, but
//     also unique enough that two findings of the same type on the same
//     line (different symbols / different sub-locations) get distinct IDs.
//   - The hash is short (8 chars = 32 bits) — collision risk in any single
//     repo is effectively zero, but the ID is not a global identifier.

// BuildFindingID constructs a stable finding ID from its components.
//
// Empty signalType is treated as "_" rather than producing an invalid
// id; this keeps callers that don't yet emit a type from breaking the
// suppression file format. Empty file is also tolerated (yields an
// "_" path component) but in practice every detector emits a file.
func BuildFindingID(signalType, file, symbol string, line int) string {
	detector := normalizeIDComponent(signalType)
	path := normalizePathOrPlaceholder(file)
	anchor := buildAnchor(symbol, line)

	canonical := detector + "::" + path + "::" + anchor
	hash := GenerateID(canonical) // returns 16 hex chars
	short := hash[:8]

	return fmt.Sprintf("%s@%s:%s#%s", detector, path, anchor, short)
}

// ParseFindingID extracts the components from a finding ID. Returns
// (detector, path, anchor, hash) and ok=false if the ID doesn't match
// the expected shape. Useful for `terrain explain finding <id>` where
// we want to validate the input before searching the snapshot.
func ParseFindingID(id string) (detector, path, anchor, hash string, ok bool) {
	// Split on '#' to peel off the hash.
	hashAt := strings.LastIndexByte(id, '#')
	if hashAt < 0 {
		return "", "", "", "", false
	}
	hash = id[hashAt+1:]
	rest := id[:hashAt]

	// Split on '@' to peel off the detector.
	atAt := strings.IndexByte(rest, '@')
	if atAt < 0 {
		return "", "", "", "", false
	}
	detector = rest[:atAt]
	rest = rest[atAt+1:]

	// Split path from anchor on the FIRST ':' after the detector. File
	// paths in Terrain are repo-relative POSIX paths (no ':'); anchors
	// may legitimately contain ':' (e.g. "TestSuite::TestCase"), so the
	// first ':' is the unambiguous separator.
	colonAt := strings.IndexByte(rest, ':')
	if colonAt < 0 {
		return "", "", "", "", false
	}
	path = rest[:colonAt]
	anchor = rest[colonAt+1:]

	if detector == "" || path == "" || anchor == "" || hash == "" {
		return "", "", "", "", false
	}
	return detector, path, anchor, hash, true
}

// MatchFindingID returns true when `id` could correspond to the given
// signal coordinates. The hash is recomputed from the components and
// compared; the human-readable prefix is also checked. This is what
// `terrain explain finding <id>` uses to round-trip an ID back to a
// signal in the snapshot.
func MatchFindingID(id, signalType, file, symbol string, line int) bool {
	expected := BuildFindingID(signalType, file, symbol, line)
	return id == expected
}

// ── helpers ─────────────────────────────────────────────────────────

// normalizeIDComponent tames a string for use in an ID component:
// trims whitespace, replaces internal whitespace with "_". Does not
// alter case (signal types are camelCase by convention; preserved).
func normalizeIDComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "_"
	}
	// Replace any whitespace runs with single underscore.
	var out strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				out.WriteByte('_')
				prevSpace = true
			}
			continue
		}
		out.WriteRune(r)
		prevSpace = false
	}
	return out.String()
}

func normalizePathOrPlaceholder(file string) string {
	p := NormalizePath(file)
	if p == "" {
		return "_"
	}
	return p
}

func buildAnchor(symbol string, line int) string {
	sym := normalizeIDComponent(symbol)
	if sym != "" && sym != "_" {
		return sym
	}
	if line > 0 {
		return "L" + strconv.Itoa(line)
	}
	return "_"
}
