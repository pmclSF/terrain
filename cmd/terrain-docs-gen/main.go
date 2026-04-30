// Command terrain-docs-gen regenerates deterministic documentation
// outputs from in-tree source-of-truth Go data. Today the outputs are:
//
//	docs/signals/manifest.json     from internal/signals.allSignalManifest
//	docs/severity-rubric.md        from internal/severity.clauses
//	docs/rules/<domain>/<slug>.md  one stub per manifest entry whose
//	                               RuleURI points under docs/rules/
//
// Stub rule docs are generated automatically; once a rule has hand-
// authored content, the generator preserves anything below the
// "<!-- docs-gen: end stub -->" marker so future regenerations don't
// stomp human-written prose. Authors edit anything *below* the marker.
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
	"strings"

	"github.com/pmclSF/terrain/internal/severity"
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

	if err := writeManifest(root); err != nil {
		return err
	}
	if err := writeSeverityRubric(root); err != nil {
		return err
	}
	if err := writeRuleDocs(root); err != nil {
		return err
	}
	return nil
}

func writeManifest(root string) error {
	path := filepath.Join(root, "docs", "signals", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	data, err := signals.MarshalManifestJSON()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println("wrote", path)
	return nil
}

func writeSeverityRubric(root string) error {
	path := filepath.Join(root, "docs", "severity-rubric.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	data := severity.RenderMarkdown()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println("wrote", path)
	return nil
}

// stubEndMarker is the sentinel below which authors write hand-curated
// content. The generator never overwrites anything below it.
const stubEndMarker = "<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->"

func writeRuleDocs(root string) error {
	for _, entry := range signals.Manifest() {
		// Only generate for entries whose RuleURI looks like an
		// in-repo doc path. External URLs (http(s)://...) and entries
		// that point outside docs/rules/ are skipped.
		if !strings.HasPrefix(entry.RuleURI, "docs/rules/") || !strings.HasSuffix(entry.RuleURI, ".md") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(entry.RuleURI))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}

		preserved := readPreservedTail(path)
		stub := renderRuleStub(entry)
		full := stub + "\n" + stubEndMarker + "\n"
		if preserved != "" {
			full += preserved
		}

		if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	fmt.Printf("wrote %d rule doc(s) under %s/docs/rules/\n", countDocsRulesEntries(), root)
	return nil
}

// readPreservedTail returns whatever was below stubEndMarker in the
// existing file, or "" if the file doesn't exist or has no marker yet.
func readPreservedTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	idx := strings.Index(string(data), stubEndMarker)
	if idx < 0 {
		return ""
	}
	tail := string(data[idx+len(stubEndMarker):])
	// Skip exactly one leading newline if present so the round-trip
	// concatenation `stub + "\n" + marker + "\n" + tail` doesn't
	// accumulate blanks.
	if strings.HasPrefix(tail, "\n") {
		tail = tail[1:]
	}
	return tail
}

func countDocsRulesEntries() int {
	n := 0
	for _, e := range signals.Manifest() {
		if strings.HasPrefix(e.RuleURI, "docs/rules/") && strings.HasSuffix(e.RuleURI, ".md") {
			n++
		}
	}
	return n
}

// renderRuleStub generates the deterministic stub block for a manifest
// entry. Output ends with a newline before stubEndMarker.
func renderRuleStub(e signals.ManifestEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", e.RuleID, e.Title)
	fmt.Fprintf(&b, "> Auto-generated stub. Edit anything below the marker; the generator preserves it.\n\n")

	fmt.Fprintf(&b, "**Type:** `%s`  \n", e.Type)
	fmt.Fprintf(&b, "**Domain:** %s  \n", e.Domain)
	fmt.Fprintf(&b, "**Default severity:** %s  \n", e.DefaultSeverity)
	fmt.Fprintf(&b, "**Status:** %s\n\n", e.Status)

	if e.Description != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", e.Description)
	}
	if e.Remediation != "" {
		fmt.Fprintf(&b, "## Remediation\n\n%s\n\n", e.Remediation)
	}
	if e.PromotionPlan != "" {
		fmt.Fprintf(&b, "## Promotion plan\n\n%s\n\n", e.PromotionPlan)
	}
	if len(e.EvidenceSources) > 0 {
		fmt.Fprintf(&b, "## Evidence sources\n\n")
		for _, src := range e.EvidenceSources {
			fmt.Fprintf(&b, "- `%s`\n", src)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Confidence range\n\n")
	fmt.Fprintf(&b, "Detector confidence is bracketed at [%.2f, %.2f] (heuristic in 0.2; calibration in 0.3).\n", e.ConfidenceMin, e.ConfidenceMax)

	return b.String()
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
