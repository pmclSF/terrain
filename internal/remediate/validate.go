// Package remediate is the closed-loop remediation validator: it applies a
// finding's structured Fix to the repo, re-runs detection, and reports
// whether the finding cleared WITHOUT introducing new findings. This is the
// operational definition of "valid remediation" — deterministic and key-free,
// so the gate never depends on an LLM.
//
// It generalizes the hand-written closed loop in
// internal/aipiperun/closed_loop_test.go (materialize a scaffold, re-run,
// finding resolves) into a reusable evaluator any detector family can use.
package remediate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/findings"
)

// ReRunFunc re-runs detection on a repo root and returns the findings. The
// caller wires the appropriate pipeline (AI composer + bridge, or the
// signal pipeline); the validator stays pipeline-agnostic.
type ReRunFunc func(root string) ([]findings.Finding, error)

// Verdict is the outcome of validating one finding's remediation.
type Verdict struct {
	// Applicable is false when the finding carries no mechanically-
	// applicable Fix (a judge-only suggestion). Such findings are out of
	// scope for the closed loop and must be routed to the judge fallback.
	Applicable bool

	// Cleared reports whether the target finding was absent after the fix
	// was applied and detection re-run.
	Cleared bool

	// NewFindings are findings present after the fix that were not present
	// before — regressions the remediation introduced. A valid remediation
	// introduces none.
	NewFindings []findings.Finding

	// Valid is the verdict: the finding cleared and nothing new appeared.
	Valid bool

	// Note is a short human-readable explanation of the verdict.
	Note string
}

// Validate applies the target finding's Fix to root, re-runs detection, and
// returns the Verdict. The fix is reverted before returning, so root is
// left as it was found (the validator is non-destructive for new_file
// fixes). `before` is the finding set that contained target.
func Validate(root string, target findings.Finding, before []findings.Finding, rerun ReRunFunc) (Verdict, error) {
	fix := firstFix(target)
	if fix == nil {
		return Verdict{Applicable: false, Note: "no applicable fix (judge-only)"}, nil
	}

	revert, applied, err := ApplyFix(root, *fix)
	if err != nil {
		return Verdict{Applicable: true, Note: "apply failed"}, fmt.Errorf("remediate: apply: %w", err)
	}
	defer func() { _ = revert() }()

	if !applied {
		// The fix changed nothing (e.g. a new_file whose target already
		// existed). A finding that "clears" after a no-op was not remediated
		// by Terrain, so the remediation is not proven — never report it valid.
		// This closes the closure-theater hole where a coincidentally-present
		// file makes an unperformed fix look validated.
		return Verdict{
			Applicable: true,
			Cleared:    false,
			Valid:      false,
			Note:       "fix target already present; remediation not attributable to Terrain",
		}, nil
	}

	after, err := rerun(root)
	if err != nil {
		return Verdict{Applicable: true, Note: "re-run failed"}, fmt.Errorf("remediate: rerun: %w", err)
	}

	beforeKeys := keySet(before)
	targetKey := Key(target)

	cleared := true
	var newFindings []findings.Finding
	for _, f := range after {
		k := Key(f)
		if k == targetKey {
			cleared = false
		}
		if _, seen := beforeKeys[k]; !seen {
			newFindings = append(newFindings, f)
		}
	}

	v := Verdict{
		Applicable:  true,
		Cleared:     cleared,
		NewFindings: newFindings,
		Valid:       cleared && len(newFindings) == 0,
	}
	v.Note = verdictNote(v)
	return v, nil
}

func verdictNote(v Verdict) string {
	switch {
	case v.Valid:
		return "finding cleared with no new findings"
	case !v.Cleared && len(v.NewFindings) > 0:
		return fmt.Sprintf("finding did not clear and %d new finding(s) appeared", len(v.NewFindings))
	case !v.Cleared:
		return "finding did not clear after applying the fix"
	default:
		return fmt.Sprintf("finding cleared but %d new finding(s) appeared", len(v.NewFindings))
	}
}

// FirstFixSuggestion returns the first suggestion carrying a mechanically-
// applicable Fix — the exact suggestion GateEligible and Validate act on.
// Callers that display "the validated fix" must use this rather than
// Suggestions[0], so the text shown is the one whose fix Terrain proved.
func FirstFixSuggestion(f findings.Finding) (findings.Suggestion, bool) {
	for i := range f.Suggestions {
		if f.Suggestions[i].Fix != nil {
			return f.Suggestions[i], true
		}
	}
	return findings.Suggestion{}, false
}

// firstFix returns the first suggestion's Fix, or nil when none is
// mechanically applicable.
func firstFix(f findings.Finding) *findings.Fix {
	if s, ok := FirstFixSuggestion(f); ok {
		return s.Fix
	}
	return nil
}

// Key identifies a finding for before/after set comparison. Positionally-
// distinct findings are keyed by rule + path + line + column (stable, no
// message-sensitivity). Only when a finding has neither line nor column — as
// with the schema→prompt drift detector, which emits one finding per
// (template, schema field) at line 0 — is the short message folded in to keep
// distinct findings distinct. Without the discriminator, two such findings
// collide: a valid remediation would read as "did not clear", or a genuinely
// new finding would be dropped from regression selection. Restricting
// the message to the position-less case avoids making a count-bearing message
// change look like a new finding for positionally-anchored rules.
func Key(f findings.Finding) string {
	disc := ""
	if f.PrimaryLoc.Line == 0 && f.PrimaryLoc.Column == 0 {
		disc = f.ShortMessage
	}
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%s",
		f.RuleID, f.PrimaryLoc.Path, f.PrimaryLoc.Line, f.PrimaryLoc.Column, disc)
}

func keySet(fs []findings.Finding) map[string]struct{} {
	m := make(map[string]struct{}, len(fs))
	for _, f := range fs {
		m[Key(f)] = struct{}{}
	}
	return m
}

// ApplyFix applies a structured Fix to root and returns a revert closure plus
// whether it ACTUALLY changed the filesystem. applied is false when the fix was
// a no-op — e.g. a new_file whose target already existed — so the closed-loop
// validator never credits Terrain with a remediation it did not perform.
func ApplyFix(root string, fix findings.Fix) (revert func() error, applied bool, err error) {
	switch fix.Kind {
	case findings.FixNewFile:
		return applyNewFile(root, fix)
	case findings.FixEditInPlace:
		return applyEditInPlace(root, fix)
	default:
		return nil, false, fmt.Errorf("remediate: unsupported fix kind %q", fix.Kind)
	}
}

// applyEditInPlace replaces the full contents of an existing file, returning
// a revert that restores the original bytes (and original mode).
func applyEditInPlace(root string, fix findings.Fix) (func() error, bool, error) {
	abs, err := safeAbs(root, fix.Path)
	if err != nil {
		return nil, false, err
	}

	orig, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, fmt.Errorf("remediate: edit_in_place target %q: %w", fix.Path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(abs, []byte(fix.Content), info.Mode().Perm()); err != nil {
		return nil, false, err
	}
	return func() error { return os.WriteFile(abs, orig, info.Mode().Perm()) }, true, nil
}

func applyNewFile(root string, fix findings.Fix) (func() error, bool, error) {
	abs, err := safeAbs(root, fix.Path)
	if err != nil {
		return nil, false, err
	}

	if _, statErr := os.Lstat(abs); statErr == nil {
		// Target already exists — Terrain did not create it, so a clearance is
		// not attributable to this fix. applied=false; nothing to revert.
		return func() error { return nil }, false, nil
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(abs, []byte(fix.Content), 0o644); err != nil {
		return nil, false, err
	}
	return func() error { return os.Remove(abs) }, true, nil
}

// safeRel rejects absolute paths and lexical ".." escapes, returning a cleaned
// repo-relative path. It is a lexical check only — it does NOT resolve
// symlinks; callers that touch the filesystem must go through safeAbs.
func safeRel(p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("remediate: fix path %q must be repo-relative", p)
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("remediate: fix path %q escapes the repo root", p)
	}
	return clean, nil
}

// safeAbs resolves a repo-relative fix path to an absolute path guaranteed to
// stay within root, even when the repo under validation commits a symlink. It
// first applies the lexical safeRel check, then resolves the deepest ancestor
// of the target that actually exists on disk through any symlinks and verifies
// the result is still inside the resolved root. Only an existing path can carry
// a symlink; every component we are about to create cannot smuggle one in. So a
// hostile Fix — including one whose parent directory is a committed symlink
// pointing outside the repo — can never write outside root. This holds the same
// bar promptflow.Discover holds for reads.
func safeAbs(root, p string) (string, error) {
	rel, err := safeRel(p)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("remediate: resolve root %q: %w", root, err)
	}
	abs := filepath.Join(realRoot, rel)

	// Walk up to the deepest ancestor that already exists; that is the only
	// place a symlink could hide. Components below it will be created by us.
	anc := abs
	for {
		if _, statErr := os.Lstat(anc); statErr == nil {
			break
		}
		parent := filepath.Dir(anc)
		if parent == anc {
			break
		}
		anc = parent
	}
	realAnc, err := filepath.EvalSymlinks(anc)
	if err != nil {
		return "", fmt.Errorf("remediate: resolve %q: %w", anc, err)
	}
	if realAnc != realRoot && !strings.HasPrefix(realAnc, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("remediate: fix path %q escapes the repo root via a symlink", p)
	}
	return abs, nil
}
