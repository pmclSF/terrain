// Command terrain-docs-linkcheck scans docs/ for broken intra-repo
// markdown links. It is a Track 9.8 deliverable for the parity-gated
// 0.2.0 release plan: docs that promise the user a path to a related
// page should not break that promise silently.
//
// What it checks:
//
//	[text](relative/path.md)         — target file must exist
//	[text](../other/path.md)         — same; resolved relative to source
//	[text](relative/path.md#anchor)  — file must exist; anchor not validated
//
// What it skips:
//
//	[text](https://...)              — external; out of scope
//	[text](http://...)               — external; out of scope
//	[text](mailto:...)               — non-document
//	[text](#anchor-only)             — same-page anchor; out of scope today
//	<img src="..." />                — HTML; out of scope today
//
// Exit codes:
//
//	0 — all links resolve
//	1 — one or more broken links (output names every offender + source)
//	2 — invocation error (bad flags, can't read filesystem)
//
// Wired into the release-readiness pipeline via `make docs-linkcheck`.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// markdownLinkPattern matches `[label](target)` outside of code spans /
// fences. Code-span / fence stripping happens before this pattern runs
// so backtick-wrapped link literals don't false-positive.
//
// Tolerates nested parens inside the label (rare but valid) by using
// a lazy match on the label and a non-greedy match on the target. The
// target is captured between the first paren after `]` and the
// matching close-paren — markdown disallows whitespace inside the
// target so the "no whitespace, no inner paren" assumption holds for
// the link shapes we actually emit in the docs tree.
var markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// codeFencePattern matches a triple-backtick fence (open or close).
// Used to skip everything between fences before link extraction.
var codeFencePattern = regexp.MustCompile("^```")

// inlineCodeSpanPattern matches `code` spans within a line. Stripped
// before link extraction so that something like `[a](b)` inside a
// code span is not flagged.
var inlineCodeSpanPattern = regexp.MustCompile("`[^`]+`")

type brokenLink struct {
	source string
	line   int
	target string
	reason string
}

// defaultSkipPrefixes is the set of doc subtrees the linkchecker
// ignores by default. These contain planning notes, internal-eng
// scratch, and legacy material whose link discipline is not part of
// the user-facing 0.2.0 contract. Override with -include-internal
// to scan them too — useful before doing the cleanup pass that
// retires the inherited debt in those directories.
var defaultSkipPrefixes = []string{
	"docs/internal/",
	"docs/legacy/",
}

func main() {
	root := flag.String("root", "docs", "directory to scan")
	includeInternal := flag.Bool("include-internal", false,
		"also check docs/internal/ and docs/legacy/ (otherwise skipped — they hold planning notes whose links are inherited debt)")
	flag.Parse()

	if _, err := os.Stat(*root); err != nil {
		fmt.Fprintf(os.Stderr, "linkcheck: cannot read root %q: %v\n", *root, err)
		os.Exit(2)
	}

	skipPrefixes := defaultSkipPrefixes
	if *includeInternal {
		skipPrefixes = nil
	}

	broken, err := scan(*root, skipPrefixes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "linkcheck: %v\n", err)
		os.Exit(2)
	}

	if len(broken) == 0 {
		fmt.Printf("linkcheck: scanned %s, all intra-repo links resolve.\n", *root)
		return
	}

	sort.SliceStable(broken, func(i, j int) bool {
		if broken[i].source != broken[j].source {
			return broken[i].source < broken[j].source
		}
		return broken[i].line < broken[j].line
	})

	fmt.Fprintf(os.Stderr, "::error::%d broken intra-repo link(s) under %s:\n", len(broken), *root)
	for _, b := range broken {
		fmt.Fprintf(os.Stderr, "  %s:%d  →  %s    (%s)\n",
			b.source, b.line, b.target, b.reason)
	}
	os.Exit(1)
}

func scan(root string, skipPrefixes []string) ([]brokenLink, error) {
	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		for _, p := range skipPrefixes {
			if strings.HasPrefix(path, p) {
				return nil
			}
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}

	var broken []brokenLink
	for _, f := range files {
		hits, err := checkFile(f)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", f, err)
		}
		broken = append(broken, hits...)
	}
	return broken, nil
}

func checkFile(path string) ([]brokenLink, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var broken []brokenLink

	// Walk lines so we can produce per-line diagnostics, and so the
	// fence-tracker can toggle in/out of code blocks. Splitting on
	// "\n" rather than using bufio.Scanner because we want to keep
	// trailing-newline behavior simple for a small docs corpus.
	lines := strings.Split(string(data), "\n")
	inFence := false
	for i, line := range lines {
		if codeFencePattern.MatchString(strings.TrimSpace(line)) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		// Strip inline code spans before link extraction.
		stripped := inlineCodeSpanPattern.ReplaceAllString(line, "")

		matches := markdownLinkPattern.FindAllStringSubmatch(stripped, -1)
		for _, m := range matches {
			target := m[2]
			if shouldSkip(target) {
				continue
			}
			if reason := resolveTarget(path, target); reason != "" {
				broken = append(broken, brokenLink{
					source: path,
					line:   i + 1,
					target: target,
					reason: reason,
				})
			}
		}
	}
	return broken, nil
}

func shouldSkip(target string) bool {
	switch {
	case strings.HasPrefix(target, "http://"),
		strings.HasPrefix(target, "https://"),
		strings.HasPrefix(target, "mailto:"),
		strings.HasPrefix(target, "tel:"):
		return true
	case strings.HasPrefix(target, "#"):
		// Same-page anchors. Verifying these would require parsing
		// every heading + slugifying — out of scope today.
		return true
	}
	return false
}

func resolveTarget(source, target string) string {
	// Strip anchor and query if present — we only verify the file.
	clean := target
	if i := strings.IndexAny(clean, "#?"); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" {
		// Pure anchor link — already handled by shouldSkip, but
		// guard against `?` only.
		return ""
	}

	// Resolve relative to the source file's directory.
	resolved := filepath.Join(filepath.Dir(source), clean)
	resolved = filepath.Clean(resolved)

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "no such file"
		}
		return fmt.Sprintf("stat error: %v", err)
	}
	if info.IsDir() {
		// Some docs link to a directory expecting an implicit
		// README. Accept if README.md exists.
		if _, err := os.Stat(filepath.Join(resolved, "README.md")); err == nil {
			return ""
		}
		return "directory link with no README.md"
	}
	return ""
}
