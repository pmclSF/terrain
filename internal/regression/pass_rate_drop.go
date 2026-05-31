package regression

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// PassRateDropConfig parameterizes the pass-rate-drop detector.
type PassRateDropConfig struct {
	// Threshold is the maximum acceptable pass-rate delta. Default 0.05.
	Threshold float64
}

// DefaultPassRateDropConfig returns the default config.
func DefaultPassRateDropConfig() PassRateDropConfig {
	return PassRateDropConfig{Threshold: 0.05}
}

// DetectPassRateDrop compares the success / total ratio between
// baseline and current and fires when the pass rate dropped past the
// configured threshold. Implements terrain/regression/pass-rate-drop.
//
// Distinct from eval-regression: eval-regression fires on score
// deltas (continuous metric drops); pass-rate-drop fires on
// success-count deltas (discrete success/failure outcome). A run
// can have unchanged scores but a different pass-rate if assertions
// changed; conversely, scores can drop without the pass rate moving.
// Adopters typically enable both.
func DetectPassRateDrop(baseline, current *evaladapter.EvalRun, cfg PassRateDropConfig) []models.Signal {
	if baseline == nil || current == nil {
		return nil
	}
	if baseline.Stats.Total == 0 || current.Stats.Total == 0 {
		return nil
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 0.05
	}

	basePass := float64(baseline.Stats.Successes) / float64(baseline.Stats.Total)
	curPass := float64(current.Stats.Successes) / float64(current.Stats.Total)
	delta := basePass - curPass
	if delta < cfg.Threshold {
		return nil
	}

	severity := models.SeverityHigh
	if delta >= 0.25 {
		severity = models.SeverityCritical
	}

	return []models.Signal{{
		Type:             signals.SignalPassRateDrop,
		Category:         models.CategoryAI,
		Severity:         severity,
		Confidence:       0.95,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceEvalExecution,
		Location: models.SignalLocation{
			File: current.Source,
		},
		Explanation: fmt.Sprintf(
			"Eval pass rate regressed: baseline %d/%d (%.1f%%) → current %d/%d (%.1f%%); delta %.1f%%.",
			baseline.Stats.Successes, baseline.Stats.Total, basePass*100,
			current.Stats.Successes, current.Stats.Total, curPass*100,
			delta*100,
		),
		SuggestedAction: "Inspect per-case eval-regression findings for the cases that flipped from pass to fail. If the new state is the intended one, update the baseline.",
		RuleID:          "terrain/regression/pass-rate-drop",
		RuleURI:         "docs/rules/regression/pass-rate-drop.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"baselinePassRate": basePass,
			"currentPassRate":  curPass,
			"delta":            delta,
			"framework":        string(current.Framework),
		},
	}}
}
