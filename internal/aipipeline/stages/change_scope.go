package stages

import (
	"context"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// ChangeScope is Stage 5: per-PR diff-touched intersection. When the
// candidate carries a DiffContext, this stage emits scope atoms that
// reflect whether the file (and which lines) were touched by the diff.
//
// In Observability mode without a PR, the candidate's Diff field is
// nil and this stage emits no atoms — the composer just doesn't see
// scope evidence and the score relies on regex/AST/repo-shape.
//
// In Gate posture without a PR, the composer's "uncalibrated" / strict
// rules can suppress findings that lack scope confirmation.
//
// One subtle atom: if the PR ADDED an eval file or tracker import that
// closes the gap the finding is about, this stage emits a strong
// negative "scope.diff_added_pr_evidence" atom — meaning "the developer
// already fixed this in the same PR, don't comment."
type ChangeScope struct{}

// NewChangeScope returns the stage.
func NewChangeScope() *ChangeScope {
	return &ChangeScope{}
}

// Name implements pipeline.Stage.
func (s *ChangeScope) Name() string { return "change-scope" }

// Run emits scope atoms for the candidate based on diff context.
func (s *ChangeScope) Run(_ context.Context, c *aipipeline.Candidate) aipipeline.StageResult {
	if c.Diff == nil {
		return aipipeline.StageResult{Continue: true}
	}

	if c.Diff.IsFileTouched(c.Path) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceScope,
			RuleID: "scope.diff_touched_file",
			Source: "change-scope",
			Weight: +0.8,
			Span:   aipipeline.Span{Snippet: c.Path},
		})
		// Per-line precision when atom spans carry line numbers from
		// earlier stages.
		for _, atom := range c.Atoms {
			if atom.Span.Line > 0 && c.Diff.IsLineTouched(c.Path, atom.Span.Line) {
				c.AddAtom(aipipeline.EvidenceAtom{
					Kind:   aipipeline.EvidenceScope,
					RuleID: "scope.diff_touched_line",
					Source: "change-scope",
					Weight: +1.4,
					Span:   atom.Span,
				})
				break // one line atom is sufficient
			}
		}
	}

	return aipipeline.StageResult{Continue: true}
}

// AddPRRemediation lets callers signal that the current PR includes
// the artifact that would close the finding (e.g. an eval file was
// added in this PR for the prompt the finding flags). The composer
// then sees a strong negative atom and the finding is suppressed.
//
// This is exposed as a helper so the PR-comment pipeline can call it
// after analyzing the diff for its own remediation signals — it isn't
// part of the per-file pipeline because the signal is cross-file.
func AddPRRemediation(c *aipipeline.Candidate, reason string) {
	c.AddAtom(aipipeline.EvidenceAtom{
		Kind:   aipipeline.EvidenceScope,
		RuleID: "scope.diff_added_pr_evidence",
		Source: "change-scope:pr-remediation",
		Weight: -1.5,
		Span:   aipipeline.Span{Snippet: reason},
	})
}
