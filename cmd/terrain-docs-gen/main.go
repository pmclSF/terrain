// Command terrain-docs-gen regenerates the deterministic JSON exports of
// in-tree manifests under docs/. Today the only output is
// docs/signals/manifest.json, derived from internal/signals.allSignalManifest.
//
// The generator is the source of truth — `make docs-gen` writes; `make
// docs-verify` writes to a tempdir and diffs against the committed copy.
// CI runs verify on every PR; a non-zero diff fails the gate.
//
// Usage:
//
//	terrain-docs-gen [-out <dir>]
//
// Default -out is the repo root, resolved by climbing parents from cwd
// until a go.mod is found, so the binary works whether you run it from
// the repo root or from a subdirectory (or from a temp checkout in CI).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/signals"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "terrain-docs-gen:", err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "output root (defaults to repo root containing go.mod)")
	flag.Parse()

	root, err := resolveRoot(*out)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(root, "docs", "signals", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(manifestPath), err)
	}
	data, err := signals.MarshalManifestJSON()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", manifestPath, err)
	}
	fmt.Println("wrote", manifestPath)
	return nil
}

// resolveRoot returns the explicit -out value if set, otherwise climbs from
// cwd until a directory containing go.mod is found. Errors if neither
// path resolves.
func resolveRoot(explicit string) (string, error) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "", errors.New("could not find go.mod ancestor; pass -out explicitly")
}
