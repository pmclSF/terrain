package signals

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRuleDocs_ExistOnDisk is the drift gate that pairs with the
// auto-stub generator in cmd/terrain-docs-gen. Every stable manifest
// entry must have its RuleURI resolved to a real file under
// docs/rules/. Experimental and planned entries are exempt — their
// detectors haven't shipped yet, so doc gaps are expected.
//
// Failures point at one of:
//   - a stable entry whose RuleURI was edited to a path the generator
//     wouldn't write (typo, wrong extension, off-tree)
//   - the generator hasn't been run since the entry was added; fix by
//     running `make docs-gen`
//   - someone hand-deleted a generated doc; running `make docs-gen`
//     restores it
func TestRuleDocs_ExistOnDisk(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromTest(t)

	for _, e := range allSignalManifest {
		if e.Status != StatusStable {
			continue
		}
		if !strings.HasPrefix(e.RuleURI, "docs/rules/") {
			t.Errorf("stable entry %q RuleURI %q does not point under docs/rules/", e.Type, e.RuleURI)
			continue
		}
		path := filepath.Join(repoRoot, filepath.FromSlash(e.RuleURI))
		if _, err := os.Stat(path); err != nil {
			t.Errorf(
				"stable entry %q points at %s which is not on disk; run `make docs-gen` to regenerate",
				e.Type, e.RuleURI,
			)
		}
	}
}

// TestRuleDocs_GeneratedHaveStubMarker confirms that every committed
// rule doc under docs/rules/ has the stub-end marker — i.e. it was
// produced by the generator and not hand-written without going
// through the canonical path. Catches a class of drift where someone
// hand-creates `docs/rules/foo/bar.md` without updating the manifest.
func TestRuleDocs_GeneratedHaveStubMarker(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromTest(t)
	rulesDir := filepath.Join(repoRoot, "docs", "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		t.Skipf("docs/rules/ does not exist: %v", err)
	}

	const stubEndMarker = "<!-- docs-gen: end stub."

	err := filepath.WalkDir(rulesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(data), stubEndMarker) {
			rel, _ := filepath.Rel(repoRoot, path)
			t.Errorf(
				"%s missing stub-end marker; generator did not produce this file. "+
					"Add the entry to internal/signals/manifest.go and run `make docs-gen`.",
				rel,
			)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", rulesDir, err)
	}
}

// repoRootFromTest finds the repo root by climbing from this test file
// until a go.mod is found. Same trick the docs-gen tool uses.
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod ancestor from %s", thisFile)
		}
		dir = parent
	}
}
