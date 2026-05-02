package identity

import (
	"strings"
	"testing"
)

func TestBuildFindingID_Stable(t *testing.T) {
	t.Parallel()
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	if a != b {
		t.Errorf("same inputs produced different IDs:\n  a=%q\n  b=%q", a, b)
	}
}

func TestBuildFindingID_Shape(t *testing.T) {
	t.Parallel()
	id := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	// Format: detector@path:anchor#hash
	if !strings.HasPrefix(id, "weakAssertion@") {
		t.Errorf("ID missing detector prefix: %q", id)
	}
	if !strings.Contains(id, "internal/auth/login_test.go") {
		t.Errorf("ID missing path: %q", id)
	}
	if !strings.Contains(id, ":TestLogin#") {
		t.Errorf("ID missing :anchor#: %q", id)
	}
	// Hash is 8 hex chars at the end.
	hashAt := strings.LastIndexByte(id, '#')
	if hashAt < 0 || len(id)-hashAt-1 != 8 {
		t.Errorf("hash should be 8 hex chars at the end: %q", id)
	}
}

func TestBuildFindingID_DistinctOnRename(t *testing.T) {
	t.Parallel()
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestSignIn", 42)
	if a == b {
		t.Errorf("rename should produce distinct IDs:\n  a=%q\n  b=%q", a, b)
	}
}

func TestBuildFindingID_DistinctOnFileMove(t *testing.T) {
	t.Parallel()
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("weakAssertion", "internal/login/login_test.go", "TestLogin", 42)
	if a == b {
		t.Errorf("file move should produce distinct IDs:\n  a=%q\n  b=%q", a, b)
	}
}

func TestBuildFindingID_DistinctOnDetectorChange(t *testing.T) {
	t.Parallel()
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("mockHeavyTest", "internal/auth/login_test.go", "TestLogin", 42)
	if a == b {
		t.Errorf("different detectors should produce distinct IDs")
	}
}

func TestBuildFindingID_PathNormalization(t *testing.T) {
	t.Parallel()
	// Forward and back slashes should normalize to the same ID.
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("weakAssertion", "internal\\auth\\login_test.go", "TestLogin", 42)
	if a != b {
		t.Errorf("path with backslashes should normalize:\n  a=%q\n  b=%q", a, b)
	}
}

func TestBuildFindingID_LineAnchorWhenNoSymbol(t *testing.T) {
	t.Parallel()
	id := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "", 42)
	if !strings.Contains(id, ":L42#") {
		t.Errorf("expected line anchor :L42#, got %q", id)
	}
}

func TestBuildFindingID_PlaceholderWhenNothing(t *testing.T) {
	t.Parallel()
	// No symbol, no line — anchor falls back to "_".
	id := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "", 0)
	if !strings.Contains(id, ":_#") {
		t.Errorf("expected placeholder anchor :_#, got %q", id)
	}
}

func TestBuildFindingID_DistinctSymbolBeatsLine(t *testing.T) {
	t.Parallel()
	// When both symbol and line are present, symbol takes precedence
	// — so changing the line should NOT change the ID.
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	b := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 100)
	if a != b {
		t.Errorf("symbol should anchor; line should be ignored when symbol is present:\n  a=%q\n  b=%q", a, b)
	}
}

func TestBuildFindingID_LineMovesProduceDifferentIDsWithoutSymbol(t *testing.T) {
	t.Parallel()
	// Without a symbol, the line is the anchor, so line drift = new ID.
	// This is the known limitation that the AST-anchored 0.3 work fixes.
	a := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "", 42)
	b := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "", 100)
	if a == b {
		t.Errorf("line drift without symbol should change ID")
	}
}

func TestParseFindingID_RoundTrip(t *testing.T) {
	t.Parallel()
	orig := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)
	detector, path, anchor, hash, ok := ParseFindingID(orig)
	if !ok {
		t.Fatalf("failed to parse: %q", orig)
	}
	if detector != "weakAssertion" {
		t.Errorf("detector = %q, want weakAssertion", detector)
	}
	if path != "internal/auth/login_test.go" {
		t.Errorf("path = %q, want internal/auth/login_test.go", path)
	}
	if anchor != "TestLogin" {
		t.Errorf("anchor = %q, want TestLogin", anchor)
	}
	if len(hash) != 8 {
		t.Errorf("hash = %q, want 8 chars", hash)
	}
}

func TestParseFindingID_RejectsMalformed(t *testing.T) {
	t.Parallel()
	bad := []string{
		"",
		"detector",
		"detector@path",
		"detector@path:anchor",         // no #hash
		"@path:anchor#hash",            // empty detector
		"detector@:anchor#hash",        // empty path
		"detector@path:#hash",          // empty anchor
		"detector@path:anchor#",        // empty hash
		"detectorpath:anchor#hash",     // missing @
	}
	for _, b := range bad {
		_, _, _, _, ok := ParseFindingID(b)
		if ok {
			t.Errorf("ParseFindingID(%q) returned ok=true, want false", b)
		}
	}
}

func TestParseFindingID_AnchorWithColons(t *testing.T) {
	t.Parallel()
	// Anchors might contain ':' (e.g. nested test suites). The parse uses
	// the LAST ':' to split path from anchor.
	orig := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "Suite::TestLogin", 0)
	_, path, anchor, _, ok := ParseFindingID(orig)
	if !ok {
		t.Fatalf("parse failed: %q", orig)
	}
	if path != "internal/auth/login_test.go" {
		t.Errorf("path = %q, want internal/auth/login_test.go", path)
	}
	if anchor != "Suite::TestLogin" {
		t.Errorf("anchor = %q, want Suite::TestLogin", anchor)
	}
}

func TestMatchFindingID(t *testing.T) {
	t.Parallel()
	id := BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)

	// Same components: matches.
	if !MatchFindingID(id, "weakAssertion", "internal/auth/login_test.go", "TestLogin", 42) {
		t.Error("MatchFindingID should match its own components")
	}

	// Symbol takes precedence over line when both are present, so a
	// line drift with the same symbol is the *same* finding by ID.
	// This is the documented behavior.
	if !MatchFindingID(id, "weakAssertion", "internal/auth/login_test.go", "TestLogin", 99) {
		t.Error("MatchFindingID should ignore line when symbol matches (symbol is the anchor)")
	}

	// Different detector → different ID.
	if MatchFindingID(id, "mockHeavyTest", "internal/auth/login_test.go", "TestLogin", 42) {
		t.Error("MatchFindingID should not match a different detector")
	}

	// Different symbol → different ID.
	if MatchFindingID(id, "weakAssertion", "internal/auth/login_test.go", "TestSignIn", 42) {
		t.Error("MatchFindingID should not match a different symbol")
	}

	// Different file → different ID.
	if MatchFindingID(id, "weakAssertion", "internal/login/login_test.go", "TestLogin", 42) {
		t.Error("MatchFindingID should not match a different file")
	}
}

func TestNormalizeIDComponent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", "_"},
		{"  ", "_"},
		{"foo", "foo"},
		{"  foo  ", "foo"},
		{"foo bar", "foo_bar"},
		{"foo  \tbar", "foo_bar"},
		{"camelCase", "camelCase"},
	}
	for _, tc := range cases {
		got := normalizeIDComponent(tc.in)
		if got != tc.want {
			t.Errorf("normalizeIDComponent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
