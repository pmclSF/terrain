package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/deps"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/promptcontract"
	"github.com/pmclSF/terrain/internal/remediate"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// fixItem is one validated, mechanically-applicable fix, with every line it
// changes (for the dry-run preview).
type fixItem struct {
	fix     *findings.Fix
	changes [][2]string // {oldLine, newLine} for each differing line
}

// runFix implements `terrain fix`: it applies the closed-loop-validated
// remediations the analyzer can prove — today, the correct-side prompt→schema
// drift corrections. It is dry-run by DEFAULT (prints the diff, writes nothing);
// `--apply` writes the changes. This is the one command that edits source, so it
// never does so without the explicit flag, and it leaves undo one command away
// (git). Only fixes whose remediation is validated appear here — a finding with
// no proven fix stays advisory (see `terrain report`).
func runFix(root string, apply bool) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	// A bad/unreadable root must be an error, not a false "all clear" — this
	// command edits source, so silently reporting "nothing to fix" on a typo'd
	// path is the wrong failure mode.
	if fi, statErr := os.Stat(abs); statErr != nil {
		return fmt.Errorf("cannot read repository root %q: %v", abs, statErr)
	} else if !fi.IsDir() {
		return fmt.Errorf("repository root %q is not a directory", abs)
	}

	if !apply {
		items, err := collectValidatedFixes(abs)
		if err != nil {
			return err
		}
		renderFixDryRun(items)
		return nil
	}

	// Apply to a fixpoint: each round recomputes fixes from the CURRENT source,
	// so two fixes in the same file can never clobber one another (each fix's
	// Content is the whole file computed from the prior state). The cap is a
	// runaway guard — a validated fix always clears its finding, so the loop
	// converges well within it.
	applied := 0
	var appliedPaths []string
	seenPath := map[string]bool{}
	const maxRounds = 100
	for round := 0; round < maxRounds; round++ {
		items, err := collectValidatedFixes(abs)
		if err != nil {
			// Partial state: report what was already written before failing.
			if applied > 0 {
				renderFixApplied(applied, appliedPaths)
			}
			return err
		}
		// collectValidatedFixes only returns byte-changing fixes, so every item
		// here makes real progress and clears its finding next round — the loop
		// converges and can never spin on a no-op.
		if len(items) == 0 {
			break
		}
		it := items[0]
		_, ok, err := remediate.ApplyFix(abs, *it.fix)
		if err != nil {
			if applied > 0 {
				renderFixApplied(applied, appliedPaths)
			}
			return fmt.Errorf("apply fix to %s: %w", it.fix.Path, err)
		}
		if !ok {
			break // no-op (target vanished) — stop rather than spin
		}
		applied++
		if !seenPath[it.fix.Path] {
			seenPath[it.fix.Path] = true
			appliedPaths = append(appliedPaths, it.fix.Path)
		}
	}
	renderFixApplied(applied, appliedPaths)
	return nil
}

// collectValidatedFixes returns every finding that carries a
// mechanically-applicable, closed-loop-validated fix — exactly the set
// `terrain fix` may safely write. It runs the SAME detectors whose remediations
// the gate can validate (defaultFixRegistry) and keeps only findings that are
// GateEligible, so `terrain fix` clears precisely what the trust floor blocks
// on. Earlier revisions hardcoded only the prompt→schema drift detector, so a
// repo blocked on the validated deps/drift-risk fix was told "nothing to fix".
// It returns a detector error so callers don't mistake a failed scan for a
// clean repo.
func collectValidatedFixes(abs string) ([]fixItem, error) {
	drift, err := promptcontract.AnalyzeInRepo(abs)
	if err != nil {
		return nil, err
	}
	sigs := promptcontract.ToSignals(drift)
	// Dependency drift-risk (caret un-pinning) — the other gate-validated fix.
	// Detect walks the repo from Root; the empty snapshot is just call context.
	depsDetector := &deps.DriftRiskDetector{Root: abs}
	sigs = append(sigs, depsDetector.Detect(&models.TestSuiteSnapshot{})...)

	lookup := ruleIDForSignalType()
	fixReg := defaultFixRegistry()
	vReg := remediate.DefaultValidityRegistry()

	var out []fixItem
	for _, s := range sigs {
		f := findings.FromSignal(s, lookup(s.Type))
		fs := []findings.Finding{f}
		fixReg.Attach(abs, fs)
		// Only findings whose remediation is closed-loop validated are safe to
		// write automatically — the same GateEligible test the gate uses.
		if !remediate.GateEligible(fs[0], vReg) {
			continue
		}
		sug, ok := remediate.FirstFixSuggestion(fs[0])
		if !ok || sug.Fix == nil {
			continue
		}
		fix := sug.Fix
		// Skip a fix that would not change any bytes — a byte-exact no-op can't
		// clear its finding, so applying it would spin the fixpoint loop and
		// misreport. Robust regardless of any producer's shape.
		if cur, readErr := os.ReadFile(filepath.Join(abs, fix.Path)); readErr == nil && string(cur) == fix.Content {
			continue
		}
		out = append(out, fixItem{fix: fix, changes: changedLines(abs, fix)})
	}
	return out, nil
}

// changedLines returns every line that differs between the file on disk and the
// fix's replacement content, as {old,new} trimmed pairs — a proper preview of
// what `--apply` will write (a single fix may rewrite many lines, e.g. pinning
// every caret dependency in a manifest).
func changedLines(abs string, fix *findings.Fix) [][2]string {
	cur, err := os.ReadFile(filepath.Join(abs, fix.Path))
	if err != nil {
		return nil
	}
	before := strings.Split(string(cur), "\n")
	after := strings.Split(fix.Content, "\n")
	n := len(before)
	if len(after) < n {
		n = len(after)
	}
	var changes [][2]string
	for i := 0; i < n; i++ {
		if before[i] == after[i] {
			continue
		}
		o, w := strings.TrimSpace(before[i]), strings.TrimSpace(after[i])
		if o == "" && w == "" {
			continue
		}
		changes = append(changes, [2]string{o, w})
	}
	return changes
}

// pluralFix returns "fix" for n == 1 and "fixes" otherwise. "fix" takes an "es"
// plural, which the generic plural() helper (bare "s" append) gets wrong.
func pluralFix(n int) string {
	if n == 1 {
		return "fix"
	}
	return "fixes"
}

func renderFixDryRun(items []fixItem) {
	fmt.Println()
	if len(items) == 0 {
		fmt.Printf("  %s  %s\n\n",
			uitokens.Ok(uitokens.GlyphOK()),
			uitokens.Muted("nothing to fix — no validated remediations found"))
		return
	}
	fmt.Printf("  %s\n\n", uitokens.Muted(fmt.Sprintf("%d VALIDATED %s READY", len(items), strings.ToUpper(pluralFix(len(items))))))
	for _, it := range items {
		fmt.Printf("  %s\n", uitokens.Muted(it.fix.Path))
		for _, ch := range it.changes {
			fmt.Printf("    %s %s\n", uitokens.Alert("-"), uitokens.Muted(ch[0]))
			fmt.Printf("    %s %s\n", uitokens.Ok("+"), ch[1])
		}
		fmt.Println()
	}
	fmt.Printf("  %s %s  %s\n\n",
		uitokens.Muted("nothing written"), uitokens.Muted(uitokens.GlyphChevron()),
		"re-run with "+uitokens.Link("terrain fix --apply")+uitokens.Muted(" to write these"))
}

func renderFixApplied(n int, paths []string) {
	fmt.Println()
	if n == 0 {
		fmt.Printf("  %s  %s\n\n",
			uitokens.Ok(uitokens.GlyphOK()),
			uitokens.Muted("nothing to fix — no validated remediations found"))
		return
	}
	fmt.Printf("  %s  %s\n", uitokens.Ok(uitokens.GlyphOK()),
		uitokens.Bold(fmt.Sprintf("applied %d %s", n, pluralFix(n))))
	// Scope the undo to exactly the files Terrain changed. A bare `git checkout .`
	// would discard every other unstaged change in the working tree.
	undo := "git restore --"
	if len(paths) > 0 {
		undo = "git restore -- " + strings.Join(paths, " ")
	}
	fmt.Printf("  %s %s %s  %s %s\n\n",
		uitokens.Muted("review"), uitokens.Muted(uitokens.GlyphChevron()), uitokens.Link("git diff"),
		uitokens.Muted("undo "+uitokens.GlyphChevron()), uitokens.Link(undo))
}
