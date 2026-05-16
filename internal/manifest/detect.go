package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Detect walks the directory tree rooted at root and returns one
// Manifest per recognized manifest file. Errors parsing an individual
// manifest are returned but do not stop the walk: each entry's nil-vs-
// non-nil Manifest indicates whether parsing succeeded.
//
// Common vendored / generated paths are skipped (vendor/, node_modules/,
// .venv/, __pycache__/, dist/, build/, .git/). Adopters can override
// via filter when this becomes part of the public detection pipeline.
func Detect(root string) ([]*Manifest, []error) {
	var manifests []*Manifest
	var errs []error

	skipDirs := map[string]bool{
		"vendor": true, "node_modules": true, ".venv": true, "venv": true,
		"__pycache__": true, "dist": true, "build": true, ".git": true,
		".terrain": true, "target": true, "_build": true,
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission errors and the like — record and continue.
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Also skip hidden directories at depth > 0 (e.g., .pytest_cache).
			if path != root && strings.HasPrefix(d.Name(), ".") && !isWhitelistedDotDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		m, err := parseByFilename(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if m != nil {
			manifests = append(manifests, m)
		}
		return nil
	})
	if walkErr != nil {
		errs = append(errs, fmt.Errorf("walk %s: %w", root, walkErr))
	}
	return manifests, errs
}

// parseByFilename dispatches to the right parser based on the file basename.
// Returns nil if the file isn't a recognized manifest.
func parseByFilename(path string) (*Manifest, error) {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch {
	case lower == "pyproject.toml":
		return ParsePyProject(path)
	case lower == "package.json":
		return ParsePackageJSON(path)
	case lower == "requirements.txt" ||
		strings.HasPrefix(lower, "requirements-") && strings.HasSuffix(lower, ".txt") ||
		strings.HasPrefix(lower, "requirements_") && strings.HasSuffix(lower, ".txt") ||
		lower == "constraints.txt":
		return ParseRequirementsTxt(path)
	}
	return nil, nil
}

func isWhitelistedDotDir(name string) bool {
	// .config and similar are user data, not vendored deps.
	return false
}

// fileExists is a small helper for callers that want to check before
// parsing without surfacing fs.ErrNotExist.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
