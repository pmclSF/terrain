package aidetect

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// CostRegressionDetector flags a per-case token-cost regression
// between the current eval run and its baseline. Pairs with the
// --baseline mechanism in the analyse pipeline (commit on this
// branch); when no baseline is attached the detector stays quiet.
//
// Detection model:
//
//   For each EvalRun in snap.EvalRuns:
//     1. Find a same-framework EvalRun in snap.Baseline.EvalRuns.
//        Match by (framework, runId) when both have RunIDs; fall back
//        to the first run of the matching framework.
//     2. Compute avg-token-cost-per-case for both.
//     3. If current / baseline - 1 > threshold (default 0.25),
//        emit a signal with the percentage increase.
//
// The detector only looks at cases that ran in BOTH runs (matched on
// CaseID). This avoids spurious increases when the eval suite grows.
type CostRegressionDetector struct {
	// Threshold is the maximum acceptable proportional cost increase.
	// 0 uses the default of 0.25 (25%).
	Threshold float64

	// MinAbsDelta is the minimum absolute change in avg cost-per-case
	// (in USD) required before the relative-percentage check fires.
	// Pre-0.2.x this floor didn't exist, so a tiny absolute regression
	// (e.g. $0.0001 → $0.0002 = +100%) paged at High severity. Default
	// 0.0005 USD per case — large enough to ignore single-token
	// fluctuations on cheap models, small enough to catch real shifts.
	MinAbsDelta float64
}

// Detect emits SignalAICostRegression per regressed eval run.
func (d *CostRegressionDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil || snap.Baseline == nil {
		return nil
	}
	threshold := d.Threshold
	if threshold <= 0 {
		threshold = 0.25
	}
	minAbs := d.MinAbsDelta
	if minAbs <= 0 {
		minAbs = 0.0005
	}

	var out []models.Signal
	for _, env := range snap.EvalRuns {
		baseEnv, ok := matchBaselineEnvelope(env, snap.Baseline.EvalRuns)
		if !ok {
			continue
		}
		current, err := airun.ParseEvalRunPayload(env)
		if err != nil || current == nil {
			continue
		}
		baseline, err := airun.ParseEvalRunPayload(baseEnv)
		if err != nil || baseline == nil {
			continue
		}

		curAvg, baseAvg, paired := pairedAverageCost(current, baseline)
		if paired == 0 || baseAvg <= 0 {
			continue
		}
		delta := curAvg/baseAvg - 1.0
		if delta <= threshold {
			continue
		}
		// Both relative AND absolute have to clear. Fixes the "cried
		// wolf on tiny costs" regression: 0.0001→0.0002 = +100% but
		// the absolute delta is $0.0001/case — operationally noise.
		if curAvg-baseAvg < minAbs {
			continue
		}
		out = append(out, models.Signal{
			Type:        signals.SignalAICostRegression,
			Category:    models.CategoryAI,
			Severity:    models.SeverityMedium,
			Confidence:  0.9,
			Location:    models.SignalLocation{File: env.SourcePath, ScenarioID: env.RunID},
			Explanation: fmt.Sprintf("Average cost-per-case rose %.1f%% versus the baseline run (%.4f → %.4f over %d paired cases). Threshold: %.0f%%.",
				delta*100, baseAvg, curAvg, paired, threshold*100),
			SuggestedAction: "Investigate the prompt or model change for unintended bloat. Bump the baseline if the increase is intentional.",

			SeverityClauses: []string{"sev-medium-006"},
			Actionability:   models.ActionabilityScheduled,
			LifecycleStages: []models.LifecycleStage{models.StageMaintenance, models.StageCIRun},
			AIRelevance:     models.AIRelevanceHigh,
			RuleID:          "TER-AI-107",
			RuleURI:         "docs/rules/ai/cost-regression.md",
			DetectorVersion: "0.2.0",
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.9,
				IntervalLow:  0.85,
				IntervalHigh: 0.95,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceRuntime},
			},
			EvidenceSource:   models.SourceRuntime,
			EvidenceStrength: models.EvidenceStrong,
			Metadata: map[string]any{
				"framework":       env.Framework,
				"runId":           env.RunID,
				"baselineRunId":   baseEnv.RunID,
				"currentAvgCost":  curAvg,
				"baselineAvgCost": baseAvg,
				"deltaPct":        delta,
				"threshold":       threshold,
				"pairedCases":     paired,
			},
		})
	}
	return out
}

// matchBaselineEnvelope picks the baseline envelope to compare against.
// Prefers (framework, runId) match when both have RunIDs; otherwise
// returns the first envelope whose framework matches.
func matchBaselineEnvelope(env models.EvalRunEnvelope, baselines []models.EvalRunEnvelope) (models.EvalRunEnvelope, bool) {
	if env.RunID != "" {
		for _, b := range baselines {
			if b.Framework == env.Framework && b.RunID == env.RunID {
				return b, true
			}
		}
	}
	for _, b := range baselines {
		if b.Framework == env.Framework {
			return b, true
		}
	}
	return models.EvalRunEnvelope{}, false
}

// pairedAverageCost returns the avg cost-per-case across cases that
// appear in BOTH runs (matched by CaseID), the baseline avg over the
// same paired set, and the count of pairs. Cases without a CaseID, or
// only present in one side, are skipped — without that filter, eval-
// suite growth would produce spurious increases.
func pairedAverageCost(current, baseline *airun.EvalRunResult) (curAvg, baseAvg float64, paired int) {
	baseByID := make(map[string]airun.EvalCase, len(baseline.Cases))
	for _, c := range baseline.Cases {
		if c.CaseID != "" {
			baseByID[c.CaseID] = c
		}
	}
	var sumCur, sumBase float64
	for _, c := range current.Cases {
		if c.CaseID == "" {
			continue
		}
		base, ok := baseByID[c.CaseID]
		if !ok {
			continue
		}
		sumCur += c.TokenUsage.Cost
		sumBase += base.TokenUsage.Cost
		paired++
	}
	if paired == 0 {
		return 0, 0, 0
	}
	return sumCur / float64(paired), sumBase / float64(paired), paired
}
