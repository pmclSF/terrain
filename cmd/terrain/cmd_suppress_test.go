package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/suppression"
)

// TestRunSuppress_CreatesNewFile verifies the happy path: no existing
// suppressions file, runSuppress writes a fresh one with the schema
// header + one entry.
func TestRunSuppress_CreatesNewFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := identity.BuildFindingID("weakAssertion", "internal/auth/login_test.go", "TestLogin", 42)

	if err := runSuppress(id, "false positive — sanitized upstream", "2026-08-01", "@platform", root); err != nil {
		t.Fatalf("runSuppress: %v", err)
	}

	// Verify the file shape via the loader: it should round-trip
	// cleanly into one valid Entry.
	res, err := suppression.Load(filepath.Join(root, suppression.DefaultPath))
	if err != nil {
		t.Fatalf("load written file: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(res.Entries))
	}
	e := res.Entries[0]
	if e.FindingID != id || !strings.Contains(e.Reason, "sanitized") || e.Owner != "@platform" {
		t.Errorf("entry mismatch: %+v", e)
	}
}

// TestRunSuppress_AppendsToExisting verifies that runSuppress appends
// when the file already has entries (preserves prior ones).
func TestRunSuppress_AppendsToExisting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	idA := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	idB := identity.BuildFindingID("mockHeavyTest", "b.go", "Y", 2)

	if err := runSuppress(idA, "first", "", "", root); err != nil {
		t.Fatal(err)
	}
	if err := runSuppress(idB, "second", "", "", root); err != nil {
		t.Fatal(err)
	}

	res, err := suppression.Load(filepath.Join(root, suppression.DefaultPath))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(res.Entries))
	}
}

// TestRunSuppress_RejectsDuplicate verifies the second call with the
// same finding ID returns a usage error (not silently appending a
// duplicate).
func TestRunSuppress_RejectsDuplicate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)

	if err := runSuppress(id, "first", "", "", root); err != nil {
		t.Fatal(err)
	}
	err := runSuppress(id, "second", "", "", root)
	if err == nil || !strings.Contains(err.Error(), "already suppressed") {
		t.Errorf("expected 'already suppressed' error, got %v", err)
	}
}

// TestRunSuppress_RejectsBadID verifies that a malformed finding ID
// is rejected before any file is touched.
func TestRunSuppress_RejectsBadID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	err := runSuppress("not-a-finding-id", "ok", "", "", root)
	if err == nil || !strings.Contains(err.Error(), "invalid finding ID") {
		t.Errorf("expected invalid-ID error, got %v", err)
	}
	// Verify no file was created.
	if _, err := os.Stat(filepath.Join(root, suppression.DefaultPath)); err == nil {
		t.Error("file should not exist after a rejected call")
	}
}

// TestRunSuppress_RequiresReason verifies the reason flag is enforced.
func TestRunSuppress_RequiresReason(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	err := runSuppress(id, "", "", "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--reason is required") {
		t.Errorf("expected reason-required error, got %v", err)
	}
}

// TestRunSuppress_RejectsBadExpiryShape verifies that a non-ISO-shaped
// expires fails fast.
func TestRunSuppress_RejectsBadExpiryShape(t *testing.T) {
	t.Parallel()
	id := identity.BuildFindingID("weakAssertion", "a.go", "X", 1)
	err := runSuppress(id, "ok", "next-month", "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("expected YYYY-MM-DD error, got %v", err)
	}
}

// TestLooksLikeISODate covers the small validator.
func TestLooksLikeISODate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"2026-08-01", true},
		{"2099-12-31", true},
		{"2026/08/01", false},
		{"08-01-2026", false},
		{"2026-8-1", false}, // not zero-padded
		{"", false},
		{"abc", false},
	}
	for _, tc := range cases {
		got := looksLikeISODate(tc.in)
		if got != tc.want {
			t.Errorf("looksLikeISODate(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
