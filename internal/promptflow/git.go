package promptflow

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
)

// DiscoverFromGit walks the working tree at root for after-state
// templates + schemas, then for each discovered schema attempts to
// retrieve its content at baseRef via `git show <baseRef>:<path>`.
//
// Schemas absent in baseRef (new files) are omitted from the before
// map — Analyze then treats them as "no diff" (a brand-new schema
// can't break a pre-existing template reference).
//
// Returns the after-state Discoveries and the before-state schema
// content map ready to pass to Analyze.
func DiscoverFromGit(root, baseRef string) (Discoveries, map[string][]byte, error) {
	after, err := Discover(root)
	if err != nil {
		return Discoveries{}, nil, err
	}
	before := map[string][]byte{}
	for _, schema := range after.Schemas {
		body, ok := gitShow(root, baseRef, filepath.ToSlash(schema.Path))
		if ok {
			before[schema.Path] = body
		}
	}
	return after, before, nil
}

// gitShow returns the file content at path as of baseRef. Returns
// (nil, false) when git exits non-zero (file didn't exist in baseRef,
// or baseRef isn't valid). All other paths return (body, true).
func gitShow(repoRoot, baseRef, path string) ([]byte, bool) {
	if baseRef == "" || path == "" {
		return nil, false
	}
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", baseRef, path))
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	return stdout.Bytes(), true
}
