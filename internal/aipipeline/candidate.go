package aipipeline

import "context"

// Candidate is the unit of work that flows through pipeline stages. A
// stage may add atoms, set Continue=false to drop the candidate, or
// record a fallback marker. Stages must not broaden the candidate set
// (one candidate in, at most one candidate out).
type Candidate struct {
	// Path is the file path being evaluated (repo-relative).
	Path string

	// Lang is the source language tag (`python`, `typescript`, `go`,
	// etc.). Empty when unknown.
	Lang string

	// RuleID is the detector rule under evaluation. A single file may
	// flow through the pipeline once per rule.
	RuleID string

	// Cohort is the repo-shape cohort classification when available.
	// Empty string means "unknown" — composers fall back to global base
	// rate.
	Cohort string

	// Src holds the file contents. Populated lazily by the first stage
	// that needs them. Subsequent stages reuse the cached bytes.
	Src []byte

	// Atoms accumulate across stages. Order matches stage order; each
	// atom's Source field identifies the producer.
	Atoms []EvidenceAtom

	// Fallbacks records non-fatal degradations (e.g. "ast=unavailable").
	// The composer uses these to apply confidence penalties or, in Gate
	// mode, to suppress the finding entirely.
	Fallbacks []string

	// Diff carries per-PR diff context when running in PR mode.
	// Nil in full-codebase mode. ChangeScope stage reads this.
	Diff *DiffContext
}

// DiffContext describes which lines/files the current PR changes. When
// populated, the ChangeScope stage emits atoms for diff-touched
// candidates.
type DiffContext struct {
	// TouchedFiles maps repo-relative paths to the set of changed line
	// numbers (1-based). Empty set means file changed but specific
	// lines unknown (treat whole file as touched).
	TouchedFiles map[string]map[int]struct{}

	// BaseSHA / HeadSHA identify the PR base and head. Optional —
	// included for evidence rendering.
	BaseSHA string
	HeadSHA string
}

// IsLineTouched reports whether the given (file, line) was changed in
// the diff. Returns false when Diff is nil.
func (d *DiffContext) IsLineTouched(file string, line int) bool {
	if d == nil {
		return false
	}
	lines, ok := d.TouchedFiles[file]
	if !ok {
		return false
	}
	if len(lines) == 0 {
		return true // whole-file flag
	}
	_, ok = lines[line]
	return ok
}

// IsFileTouched reports whether the diff touched any line in the file.
func (d *DiffContext) IsFileTouched(file string) bool {
	if d == nil {
		return false
	}
	_, ok := d.TouchedFiles[file]
	return ok
}

// AddAtom appends an atom and records its source for explainability.
func (c *Candidate) AddAtom(atom EvidenceAtom) {
	c.Atoms = append(c.Atoms, atom)
}

// AddFallback records a degraded path marker (e.g. "ast=unavailable").
func (c *Candidate) AddFallback(marker string) {
	c.Fallbacks = append(c.Fallbacks, marker)
}

// HasFallback reports whether a specific fallback marker was recorded.
func (c *Candidate) HasFallback(marker string) bool {
	for _, m := range c.Fallbacks {
		if m == marker {
			return true
		}
	}
	return false
}

// Stage is the interface every pipeline stage implements. Each Run call
// receives the candidate and a context, mutates the candidate (appends
// atoms / sets fallbacks), and returns whether the candidate should
// continue through subsequent stages.
type Stage interface {
	// Name identifies the stage in evidence chains and logs.
	Name() string

	// Run executes the stage. The stage MUST NOT broaden the candidate
	// set; either the candidate continues or it is dropped (Continue=false).
	Run(ctx context.Context, c *Candidate) StageResult
}

// StageResult is the outcome of a stage's evaluation of one candidate.
type StageResult struct {
	// Continue determines whether the candidate proceeds to the next
	// stage. False drops the candidate from further evaluation; the
	// composer does not see dropped candidates.
	Continue bool
}
