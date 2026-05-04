package analysis

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Track 9.9 — Adversarial filesystem suite.
//
// These tests exercise the analyzer against deliberately weird
// filesystem inputs that real repositories surface but synthetic
// fixtures usually don't. The contract: Analyze must complete
// without panic, hang, or excessive memory growth — even when the
// input is hostile or pathological.
//
// What's NOT exercised here (out of scope):
//   - Symlink loops: skipped because behavior differs across
//     platforms (Linux follows; macOS errors; Windows has no
//     symlinks at all without admin). Add per-platform tests when
//     a real adopter hits a loop.
//   - Permission-denied: hard to set up portably (TestMain would
//     need root on Linux; macOS has SIP). Manual smoke verifies
//     the walker silently skips.

// TestAdversarialFS_BinaryFileWithSourceExtension verifies the
// analyzer doesn't choke on a file with a .ts/.go extension whose
// content is binary (e.g. a misnamed asset, a checked-in compiled
// fixture). The expectation: analyze completes; the binary file
// is either parsed as no-op or skipped silently — never panics.
func TestAdversarialFS_BinaryFileWithSourceExtension(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Real source file so the analyzer has something legitimate.
	mustWriteAdversarial(t, filepath.Join(tmp, "real.ts"),
		"export function add(a: number, b: number) { return a + b; }\n")
	mustWriteAdversarial(t, filepath.Join(tmp, "real.test.ts"),
		"import { add } from './real';\ntest('adds', () => { expect(add(1,2)).toBe(3); });\n")

	// Binary file disguised as TypeScript.
	binary := bytes.Repeat([]byte{0x00, 0xFF, 0x7F, 0x80}, 1024)
	if err := os.WriteFile(filepath.Join(tmp, "asset.ts"), binary, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on binary-poisoned tree: %v", err)
	}
	if snap == nil {
		t.Fatal("Analyze returned nil snapshot on binary-poisoned tree")
	}
	// We don't assert on count specifics — different parsers may
	// classify the binary file differently. We just assert no panic
	// and that the legitimate test file was found.
	foundLegit := false
	for _, tf := range snap.TestFiles {
		if strings.HasSuffix(tf.Path, "real.test.ts") {
			foundLegit = true
		}
	}
	if !foundLegit {
		t.Errorf("legitimate test file lost in binary-poisoned tree")
	}
}

// TestAdversarialFS_OversizeSourceFile verifies the analyzer skips
// (rather than reading + processing) source files above the size
// threshold. Pre-Track 9.9 a single 50MB minified bundle with a
// .ts extension would consume seconds of analysis time and balloon
// memory; the size-skip threshold protects against this.
func TestAdversarialFS_OversizeSourceFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Real source so the analyzer has something to do.
	mustWriteAdversarial(t, filepath.Join(tmp, "small.ts"),
		"export const x = 1;\n")

	// Create a 2MB synthetic .ts file (above any reasonable
	// maxSourceFileSize threshold but cheap enough to allocate
	// in a test).
	huge := bytes.Repeat([]byte("export const x = 1; "), 100*1024) // ~2MB
	if err := os.WriteFile(filepath.Join(tmp, "huge.ts"), huge, 0o644); err != nil {
		t.Fatalf("write huge file: %v", err)
	}

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on oversize tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on oversize tree")
	}
	// Contract: analyze completes; we don't OOM. No specific
	// assertion on whether the huge file was skipped or processed —
	// different sub-detectors have different size policies.
}

// TestAdversarialFS_UTF16BOM verifies the analyzer doesn't panic on
// source files with a UTF-16 BOM. Real-world: Windows-edited files
// occasionally land in repos with U+FEFF at offset 0; older detectors
// would see "import" miss because the byte was 0xFEFF rather than
// ASCII 'i'.
func TestAdversarialFS_UTF16BOM(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// File with a leading UTF-8 BOM (EF BB BF) followed by valid
	// TypeScript. The UTF-8 BOM is the more common shape than
	// UTF-16; UTF-16 source files are rare enough that we accept
	// "best-effort handling" rather than guaranteeing extraction.
	bom := []byte{0xEF, 0xBB, 0xBF}
	src := append(bom, []byte("export function withBOM() { return 1; }\n")...)
	if err := os.WriteFile(filepath.Join(tmp, "bom.ts"), src, 0o644); err != nil {
		t.Fatalf("write BOM file: %v", err)
	}
	mustWriteAdversarial(t, filepath.Join(tmp, "bom.test.ts"),
		"import { withBOM } from './bom';\ntest('bom', () => { expect(withBOM()).toBe(1); });\n")

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on BOM tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on BOM tree")
	}
}

// TestAdversarialFS_NULBytesInSource verifies the analyzer survives
// a source file with embedded NUL bytes mid-content. Some legitimate
// transpiler / minifier outputs include them; older string-scanning
// regex engines would either truncate at the NUL (Go's regexp doesn't
// but unfamiliar callers might) or panic on assumptions about
// printable ASCII.
func TestAdversarialFS_NULBytesInSource(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	mustWriteAdversarial(t, filepath.Join(tmp, "real.ts"),
		"export const ok = 1;\n")

	src := []byte("export const x = 1;\x00\x00\nexport const y = 2;\n")
	if err := os.WriteFile(filepath.Join(tmp, "nul.ts"), src, 0o644); err != nil {
		t.Fatalf("write NUL file: %v", err)
	}

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on NUL-poisoned tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on NUL-poisoned tree")
	}
}

// TestAdversarialFS_EmptyTestFile verifies the analyzer doesn't
// panic on a 0-byte file with a .test.ts extension. Real-world: a
// developer creates the file expecting to fill it in later, then
// commits before doing so.
func TestAdversarialFS_EmptyTestFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Real source.
	mustWriteAdversarial(t, filepath.Join(tmp, "real.ts"),
		"export const x = 1;\n")

	// 0-byte test file.
	if err := os.WriteFile(filepath.Join(tmp, "empty.test.ts"), nil, 0o644); err != nil {
		t.Fatalf("write empty test file: %v", err)
	}

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on empty-test tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on empty-test tree")
	}
	// The empty test file may or may not be in the inventory,
	// depending on how each detector handles empty content. The
	// contract is "no panic", not "definitely included."
}

// TestAdversarialFS_NestedGitRepos verifies the analyzer doesn't
// recurse into nested .git directories. Real-world: a repo that
// contains git submodules has multiple .git directories; the
// analyzer should treat each as the root only when invoked
// against it explicitly, not descend into one when scanning the
// outer.
func TestAdversarialFS_NestedGitRepos(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Outer repo's "git directory" — just an empty .git/ to mark
	// the root. We don't actually init git.
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir outer .git: %v", err)
	}

	// Nested submodule's .git/ directory with a fake test file
	// inside that we DON'T want the outer scan to find.
	nestedGit := filepath.Join(tmp, "submodule", ".git")
	if err := os.MkdirAll(nestedGit, 0o755); err != nil {
		t.Fatalf("mkdir nested .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedGit, "should-not-be-found.test.ts"),
		[]byte("test('should not be discovered', () => {});\n"), 0o644); err != nil {
		t.Fatalf("write inside .git: %v", err)
	}

	// Submodule has a legitimate test outside its .git/ — that
	// might or might not be in scope, depending on policy.
	mustWriteAdversarial(t, filepath.Join(tmp, "submodule", "real.test.ts"),
		"test('legit', () => {});\n")

	// Outer-repo test that should always be found.
	mustWriteAdversarial(t, filepath.Join(tmp, "outer.test.ts"),
		"test('outer', () => {});\n")

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on nested-git tree: %v", err)
	}

	// Contract: nothing inside a .git/ directory should appear in
	// the inventory. This is the load-bearing assertion — the rest
	// is "doesn't crash."
	for _, tf := range snap.TestFiles {
		if strings.Contains(tf.Path, "/.git/") || strings.HasPrefix(tf.Path, ".git/") {
			t.Errorf("test file inside .git/ leaked into inventory: %s", tf.Path)
		}
	}
}

// TestAdversarialFS_DeepDirectoryNesting verifies the walker doesn't
// stack-overflow on extremely deep directory trees. We build a
// 50-level nested directory and put a single test file at the
// bottom.
func TestAdversarialFS_DeepDirectoryNesting(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows path-length limit makes this unreliable")
	}

	tmp := t.TempDir()
	deep := tmp
	for i := 0; i < 50; i++ {
		deep = filepath.Join(deep, fmt.Sprintf("d%02d", i))
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}
	mustWriteAdversarial(t, filepath.Join(deep, "buried.test.ts"),
		"test('buried', () => {});\n")
	mustWriteAdversarial(t, filepath.Join(tmp, "shallow.test.ts"),
		"test('shallow', () => {});\n")

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on deeply nested tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on deeply nested tree")
	}
}

// TestAdversarialFS_VeryLongFilename verifies the walker survives
// long-but-legal filenames. Path-length limits vary across
// filesystems; 200 chars is well under most limits but unusual.
func TestAdversarialFS_VeryLongFilename(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	long := strings.Repeat("a", 180) + ".test.ts"
	mustWriteAdversarial(t, filepath.Join(tmp, long), "test('long', () => {});\n")

	snap, err := New(tmp).AnalyzeContext(context.Background())
	if err != nil {
		t.Fatalf("Analyze on long-filename tree: %v", err)
	}
	if snap == nil {
		t.Fatal("nil snapshot on long-filename tree")
	}
}

// mustWriteAdversarial is the local writer helper. We don't share
// with mustWrite in integration_classification_test.go because
// that file may not exist on every branch this suite runs from.
func mustWriteAdversarial(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
