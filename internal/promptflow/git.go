package promptflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiscoverFromGit walks the working tree at root for after-state
// templates + schemas, then for each discovered schema attempts to
// retrieve its content at baseRef via `git show <baseRef>:<path>`.
//
// baseRef is validated via `git rev-parse` before any schema lookup;
// an invalid ref returns an error rather than silently producing
// zero before-content (which would look like "no findings — all
// clean" rather than "your base-ref is wrong").
//
// Schemas absent in baseRef (new files) are omitted from the before
// map — Analyze then treats them as "no diff" (a brand-new schema
// can't break a pre-existing template reference).
//
// Returns the after-state Discoveries and the before-state schema
// content map ready to pass to Analyze.
//
// Cancellation: the context is honoured for the git subprocesses, so
// a SIGINT or analyze --timeout can interrupt the per-schema fetch
// loop. A nil context falls back to context.Background.
func DiscoverFromGit(ctx context.Context, root, baseRef string) (Discoveries, map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateGitRef(ctx, root, baseRef); err != nil {
		return Discoveries{}, nil, err
	}
	after, err := Discover(root)
	if err != nil {
		return Discoveries{}, nil, err
	}
	before := map[string][]byte{}
	for _, schema := range after.Schemas {
		if err := ctx.Err(); err != nil {
			return Discoveries{}, nil, err
		}
		// Normalize the path to forward slashes for both the git
		// argument (git always uses /) and the map key. Otherwise
		// callers on Windows looking up by "schemas/user.json"
		// would miss the OS-separator key "schemas\user.json"
		// produced by filepath.Walk.
		key := filepath.ToSlash(schema.Path)
		body, ok := gitShow(ctx, root, baseRef, key)
		if ok {
			before[key] = body
		}
	}
	return after, before, nil
}

// validateGitRef checks that baseRef is resolvable in the repo at
// root. Returns a clear error pointing at the offending ref when not.
func validateGitRef(ctx context.Context, root, baseRef string) error {
	if baseRef == "" {
		return fmt.Errorf("promptflow: empty git base-ref")
	}
	if strings.HasPrefix(baseRef, "-") {
		return fmt.Errorf("promptflow: git base-ref %q must not start with %q", baseRef, "-")
	}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", baseRef+"^{commit}")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("promptflow: %q is not a valid git ref in %s (git rev-parse: %s)",
			baseRef, root, bytes.TrimSpace(stderr.Bytes()))
	}
	return nil
}

// gitShow returns the file content at path as of baseRef. Returns
// (nil, false) when git exits non-zero (file didn't exist in baseRef,
// or baseRef isn't valid). All other paths return (body, true).
//
// gitShow caps reads at MaxFileBytes to match Discover — a 5 GB
// historical file cannot OOM the analyze run.
func gitShow(ctx context.Context, repoRoot, baseRef, path string) ([]byte, bool) {
	if baseRef == "" || path == "" {
		return nil, false
	}
	if strings.HasPrefix(baseRef, "-") || strings.HasPrefix(path, "-") {
		return nil, false
	}
	cmd := exec.CommandContext(ctx, "git", "show", fmt.Sprintf("%s:%s", baseRef, path))
	cmd.Dir = repoRoot
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false
	}
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	body, _ := io.ReadAll(io.LimitReader(stdout, MaxFileBytes))
	// Drain any remainder so the subprocess can exit cleanly.
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return nil, false
	}
	return body, true
}
