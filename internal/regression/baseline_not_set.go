package regression

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectBaselineNotSet emits a Signal when an EvalRun exists but no
// baseline EvalRun is available to compare against. Implements
// terrain/regression/baseline-not-set.
//
// Without a baseline, the eval-regression rule has nothing to
// compare to — the run can't fail regression detection regardless of
// its score. The rule fires once per current run when baseline is
// nil or empty, prompting the adopter to record a baseline before
// the next change.
//
// Severity defaults to medium: not having a baseline doesn't break
// anything immediately, but it means regression detection is off.
func DetectBaselineNotSet(baseline, current *evaladapter.EvalRun) []models.Signal {
	if current == nil || len(current.Cases) == 0 {
		return nil
	}
	if baseline != nil && len(baseline.Cases) > 0 {
		return nil
	}

	return []models.Signal{{
		Type:             signals.SignalBaselineNotSet,
		Category:         models.CategoryAI,
		Severity:         models.SeverityMedium,
		Confidence:       0.99,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceEvalExecution,
		Location: models.SignalLocation{
			File: current.Source,
		},
		Explanation: fmt.Sprintf(
			"Eval run at %s has %d cases but no baseline is recorded. Eval-regression detection is disabled until a baseline exists; current scores can't be compared.",
			current.Source, len(current.Cases),
		),
		SuggestedAction: "Run `terrain ai record` on the current main-branch state to lock the baseline. Subsequent PRs will be compared against it.",
		RuleID:          "terrain/regression/baseline-not-set",
		RuleURI:         "docs/rules/regression/baseline-not-set.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"caseCount": len(current.Cases),
			"framework": string(current.Framework),
		},
	}}
}
