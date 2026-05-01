package aidetect

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// skipDirs are directories the AI-config walker never descends into.
// These MUST match the canonical set in
// internal/analysis/repository_scan.go — drift here causes detectors to
// re-scan trees other walkers correctly avoid. Worst case (the bug we
// just fixed): descending into .terrain/ and re-detecting the engine's
// own previously-saved snapshots, which inflated signal counts on every
// successive `terrain analyze --write-snapshot` run (18 → 22 → 38 on
// three identical runs). The .terrain entry was missing from this list
// entirely.
var skipDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	"dist":          true,
	"build":         true,
	"benchmarks":    true,
	"coverage":      true,
	".next":         true,
	".turbo":        true,
	".nuxt":         true,
	"vendor":        true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".tox":          true,
	".venv":         true,
	"venv":          true,
	".idea":         true,
	".vscode":       true,
	".terrain":      true,
	"target":        true,
}

// scanOpts tunes the walker. Detectors compose their narrow allowlist
// (extensions + filename markers) and pass it in.
type scanOpts struct {
	// extensions is the set of lowercase file extensions to consider
	// (e.g. ".yaml", ".json"). Empty = match everything.
	extensions map[string]bool
	// markers is a list of substring markers; at least one must appear
	// in the file's lowercase relative path for the file to be returned.
	// Empty = no filename marker filter.
	markers []string
}

// walkRepoForConfigs walks root and returns repo-relative paths whose
// extension+filename match the given options. Skips known noisy
// directories. Returns paths in deterministic (filepath.Walk) order,
// which is sorted by directory entry name on most OSes.
func walkRepoForConfigs(root string, opts scanOpts) []string {
	var out []string
	if root == "" {
		return out
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if len(opts.extensions) > 0 && !opts.extensions[ext] {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if len(opts.markers) > 0 {
			lower := strings.ToLower(rel)
			matched := false
			for _, m := range opts.markers {
				if strings.Contains(lower, m) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}
		out = append(out, rel)
		return nil
	})
	return out
}

// uniquePaths merges N path lists into one with stable ordering and
// duplicate suppression. Used by detectors that combine the snapshot's
// TestFiles / Scenarios with a fresh repo walk.
func uniquePaths(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, p := range list {
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// snapshotPaths pulls TestFile and Scenario paths from a snapshot.
// Helper used alongside walkRepoForConfigs by every AI detector.
func snapshotPaths(snap *models.TestSuiteSnapshot) []string {
	if snap == nil {
		return nil
	}
	out := make([]string, 0, len(snap.TestFiles)+len(snap.Scenarios))
	for _, tf := range snap.TestFiles {
		out = append(out, tf.Path)
	}
	for _, sc := range snap.Scenarios {
		if sc.Path != "" {
			out = append(out, sc.Path)
		}
	}
	return out
}
