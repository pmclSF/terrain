package remediate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestApplyFix_RejectsSymlinkEscape: a repo that commits a directory symlink
// pointing outside the root cannot be used to make a Fix write outside the
// sandbox during validation. This guards the closed loop against hostile
// content when Terrain validates a fix against a third-party repository it
// did not author.
func TestApplyFix_RejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()

	// Commit `evals` as a symlink to a directory outside the repo root.
	if err := os.Symlink(outside, filepath.Join(root, "evals")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fix := findings.Fix{Kind: findings.FixNewFile, Path: "evals/pwned.yaml", Content: "x"}
	if _, _, err := ApplyFix(root, fix); err == nil {
		t.Fatal("ApplyFix must reject a path that escapes the root via a symlinked ancestor")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "pwned.yaml")); statErr == nil {
		t.Fatal("ApplyFix wrote outside the repo root despite the symlinked ancestor")
	}
}

// TestApplyFix_RejectsSymlinkedFileEscape: an edit_in_place fix whose target is
// itself a committed symlink to a file outside the root must be rejected, not
// followed (which would clobber the outside file).
func TestApplyFix_RejectsSymlinkedFileEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(root, "config.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fix := findings.Fix{Kind: findings.FixEditInPlace, Path: "config.txt", Content: "pwned"}
	if _, _, err := ApplyFix(root, fix); err == nil {
		t.Fatal("ApplyFix must reject an edit whose target symlinks outside the root")
	}
	if b, _ := os.ReadFile(outsideFile); string(b) != "original" {
		t.Fatal("ApplyFix followed a symlink and clobbered a file outside the repo root")
	}
}

// TestApplyFix_AllowsRealNestedPath: the sandbox still allows a legitimate
// nested path (no symlink) and its revert removes the file — the guard is not
// over-broad.
func TestApplyFix_AllowsRealNestedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fix := findings.Fix{Kind: findings.FixNewFile, Path: "evals/new.yaml", Content: "x"}
	revert, _, err := ApplyFix(root, fix)
	if err != nil {
		t.Fatalf("ApplyFix rejected a legitimate nested path: %v", err)
	}
	created := filepath.Join(root, "evals", "new.yaml")
	if _, statErr := os.Stat(created); statErr != nil {
		t.Fatalf("ApplyFix did not create the file: %v", statErr)
	}
	if err := revert(); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if _, statErr := os.Stat(created); statErr == nil {
		t.Fatal("revert did not remove the created file")
	}
}
