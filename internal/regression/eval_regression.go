// Package regression implements the regression/* stable rules.
// Each detector takes a baseline run + current run and emits a
// Signal when the current run regressed past the configured threshold.
package regression

import (
	"fmt"
	"math"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// EvalRegressionConfig parameterizes the eval-regression detector.
// Zero values are reasonable defaults; adopters override via
// terrain.yaml per-rule configuration.
type EvalRegressionConfig struct {
	// Threshold is the maximum acceptable delta on a case's primary
	// Score (or run PrimaryMetric for run-level comparison). Default
	// 0.05 (5% drop).
	Threshold float64

	// MinDelta sets the smallest delta to surface. Useful to suppress
	// noise from stochastic evals where small drifts are expected.
	// Default 0.0 (every drop above Threshold fires).
	MinDelta float64
}

// DefaultEvalRegressionConfig returns the default config.
func DefaultEvalRegressionConfig() EvalRegressionConfig {
	return EvalRegressionConfig{
		Threshold: 0.05,
		MinDelta:  0.0,
	}
}

// DetectEvalRegression compares a baseline EvalRun against a current
// EvalRun and emits a Signal for each case whose Score dropped past
// the configured threshold. Implements terrain/regression/eval-regression.
//
// Matching semantics:
//   - Cases match by ID. Cases present in baseline but not current
//     are not flagged here (they're handled by a separate rule
//     terrain/regression/baseline-not-set when no current case
//     exists for a baseline case).
//   - Cases present in current but not baseline are skipped (no
//     baseline to compare to).
//   - Run-level: if both runs have a PrimaryMetric and it dropped
//     past threshold, an additional "run-level" Signal is emitted.
func DetectEvalRegression(baseline, current *evaladapter.EvalRun, cfg EvalRegressionConfig) []models.Signal {
	if baseline == nil || current == nil {
		return nil
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 0.05
	}

	baselineByID := make(map[string]evaladapter.EvalCaseResult, len(baseline.Cases))
	for _, c := range baseline.Cases {
		baselineByID[c.ID] = c
	}

	var out []models.Signal

	for _, cur := range current.Cases {
		base, ok := baselineByID[cur.ID]
		if !ok {
			continue
		}
		delta := base.Score - cur.Score
		if delta < cfg.MinDelta {
			continue
		}
		if delta < cfg.Threshold {
			continue
		}
		out = append(out, buildEvalRegressionSignal(base, cur, delta, current, "case"))
	}

	// Run-level comparison.
	if baseline.Stats.PrimaryMetric > 0 && current.Stats.PrimaryMetric > 0 {
		delta := baseline.Stats.PrimaryMetric - current.Stats.PrimaryMetric
		if delta >= cfg.Threshold && delta >= cfg.MinDelta {
			out = append(out, buildRunLevelRegressionSignal(baseline, current, delta))
		}
	}

	return out
}

func buildEvalRegressionSignal(base, cur evaladapter.EvalCaseResult, delta float64, run *evaladapter.EvalRun, scope string) models.Signal {
	confidence := 0.95
	if math.Abs(delta) < 0.1 {
		confidence = 0.85
	}
	severity := models.SeverityHigh
	if delta >= 0.25 {
		severity = models.SeverityCritical
	}

	return models.Signal{
		Type:             signals.SignalEvalRegression,
		Category:         models.CategoryAI,
		Severity:         severity,
		Confidence:       confidence,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceEvalExecution,
		Location: models.SignalLocation{
			File: run.Source,
		},
		Explanation: fmt.Sprintf(
			"Eval case %q regressed: baseline score %.3f → current %.3f (delta %.3f). %s",
			cur.Name, base.Score, cur.Score, delta, formatRegressionReason(cur),
		),
		SuggestedAction: "Inspect the diff for prompt / model / retrieval changes that affect this case. If the regression is intentional, update the baseline with `terrain ai record`.",
		RuleID:          "terrain/regression/eval-regression",
		RuleURI:         "docs/rules/regression/eval-regression.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"caseId":        cur.ID,
			"caseName":      cur.Name,
			"baselineScore": base.Score,
			"currentScore":  cur.Score,
			"delta":         delta,
			"threshold":     cur.Threshold,
			"framework":     string(run.Framework),
			"scope":         scope,
		},
	}
}

func buildRunLevelRegressionSignal(baseline, current *evaladapter.EvalRun, delta float64) models.Signal {
	severity := models.SeverityHigh
	if delta >= 0.25 {
		severity = models.SeverityCritical
	}
	return models.Signal{
		Type:             signals.SignalEvalRegression,
		Category:         models.CategoryAI,
		Severity:         severity,
		Confidence:       0.9,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceEvalExecution,
		Location: models.SignalLocation{
			File: current.Source,
		},
		Explanation: fmt.Sprintf(
			"Run-level eval regressed: baseline primary metric %.3f → current %.3f (delta %.3f) across %d cases.",
			baseline.Stats.PrimaryMetric, current.Stats.PrimaryMetric, delta, current.Stats.Total,
		),
		SuggestedAction: "Inspect per-case findings (also under terrain/regression/eval-regression) to identify which cases drove the run-level drop. If intentional, update the baseline.",
		RuleID:          "terrain/regression/eval-regression",
		RuleURI:         "docs/rules/regression/eval-regression.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"baselineMetric": baseline.Stats.PrimaryMetric,
			"currentMetric":  current.Stats.PrimaryMetric,
			"delta":          delta,
			"framework":      string(current.Framework),
			"scope":          "run",
			"caseCount":      current.Stats.Total,
		},
	}
}

func formatRegressionReason(cur evaladapter.EvalCaseResult) string {
	if cur.Reason != "" {
		return "Framework reason: " + cur.Reason
	}
	return ""
}
