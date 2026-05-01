package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func writeVitestProbeFile(t *testing.T, name, content string) (relPath, absPath string) {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return name, abs
}

func TestVitestInSource_RecognisesMarker(t *testing.T) {
	t.Parallel()

	rel, abs := writeVitestProbeFile(t, "src/add.ts", `
export function add(a: number, b: number) {
  return a + b;
}

if (import.meta.vitest) {
  const { describe, it, expect } = import.meta.vitest;
  describe('add', () => {
    it('handles two ints', () => {
      expect(add(1, 2)).toBe(3);
    });
  });
}
`)
	if !hasVitestInSourceMarker(rel, abs) {
		t.Errorf("expected vitest in-source marker to be recognised in %q", rel)
	}
}

func TestVitestInSource_IgnoresPlainSource(t *testing.T) {
	t.Parallel()

	rel, abs := writeVitestProbeFile(t, "src/util.ts", `
export function add(a: number, b: number) {
  return a + b;
}
`)
	if hasVitestInSourceMarker(rel, abs) {
		t.Errorf("plain source should not match vitest in-source marker")
	}
}

func TestVitestInSource_IgnoresNonJSExtensions(t *testing.T) {
	t.Parallel()

	// A .py file with the literal string should NOT be flagged — we only
	// scan JS/TS.
	rel, abs := writeVitestProbeFile(t, "src/decoy.py", `# comment: import.meta.vitest`)
	if hasVitestInSourceMarker(rel, abs) {
		t.Errorf("python file should not match vitest in-source marker")
	}
}

func TestVitestInSource_HandlesMissingFile(t *testing.T) {
	t.Parallel()

	if hasVitestInSourceMarker("src/missing.ts", "/nonexistent/path") {
		t.Errorf("missing file should not match")
	}
}
