package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitignoreMatcher_BasicPatterns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := []byte(`# comment line
node_modules/
*.log
/build
!build/keep.txt
dist
src/generated/
`)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), body, 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	m := loadGitignoreMatcher(root)
	if m == nil || len(m.rules) == 0 {
		t.Fatalf("expected gitignore rules to load")
	}

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		// Floating directory pattern
		{"node_modules", true, true},
		{"frontend/node_modules", true, true},
		// Files inside the floating dir match via the segment walk.
		{"node_modules/lodash/index.js", false, true},
		// Wildcard match anywhere
		{"foo.log", false, true},
		{"deep/nested/bar.log", false, true},
		// Anchored path
		{"build", true, true},
		{"build/output.txt", false, true},
		// Anchored path negation
		{"build/keep.txt", false, false},
		// Floating path matches at any depth
		{"dist", true, true},
		{"packages/foo/dist", true, true},
		// Nested anchored
		{"src/generated", true, true},
		{"src/generated/proto.pb.go", false, true},
		// Unrelated
		{"src/auth.ts", false, false},
		{"README.md", false, false},
	}

	for _, tc := range cases {
		got := m.match(tc.path, tc.isDir)
		if got != tc.want {
			t.Errorf("match(%q, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
		}
	}
}

func TestGitignoreMatcher_MissingFile(t *testing.T) {
	t.Parallel()

	m := loadGitignoreMatcher(t.TempDir())
	if m == nil {
		t.Fatalf("expected non-nil matcher when .gitignore missing")
	}
	if m.match("anything", false) {
		t.Errorf("expected no matches when .gitignore missing")
	}
}

func TestDiscoverTestFiles_HonoursGitignore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "vendored"), 0o755); err != nil {
		t.Fatalf("mkdir vendored: %v", err)
	}

	// Real test file we should discover.
	if err := os.WriteFile(
		filepath.Join(root, "src", "auth.test.ts"),
		[]byte("describe('auth', () => { test('x', () => {}) })\n"),
		0o644,
	); err != nil {
		t.Fatalf("write keep test: %v", err)
	}
	// Vendored test file we should skip.
	if err := os.WriteFile(
		filepath.Join(root, "vendored", "something.test.ts"),
		[]byte("describe('vendor', () => { test('y', () => {}) })\n"),
		0o644,
	); err != nil {
		t.Fatalf("write skip test: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, ".gitignore"),
		[]byte("vendored/\n"),
		0o644,
	); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	files, err := discoverTestFiles(root)
	if err != nil {
		t.Fatalf("discoverTestFiles: %v", err)
	}
	for _, f := range files {
		if filepath.ToSlash(f.Path) == "vendored/something.test.ts" {
			t.Fatalf("expected gitignored test file to be skipped; found %s", f.Path)
		}
	}

	// Sanity: keep file is present.
	found := false
	for _, f := range files {
		if filepath.ToSlash(f.Path) == "src/auth.test.ts" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected src/auth.test.ts in results, got %+v", files)
	}
}
