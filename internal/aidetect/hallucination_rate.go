package aidetect

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// HallucinationRateDetector flags eval runs whose hallucination-shaped
// failure rate exceeds the configured threshold. This is the first
// detector that consumes snap.EvalRuns (populated by the Promptfoo
// adapter today; DeepEval / Ragas adapters will populate the same
// shape).
//
// A case is considered hallucination-shaped when any of the following
// is true:
//
//   - NamedScores["faithfulness"]   < 0.5
//   - NamedScores["factuality"]     < 0.5
//   - NamedScores["grounding"]      < 0.5
//   - NamedScores["hallucination"]  > 0.5  (inverse polarity)
//   - FailureReason contains "fabricat", "hallucinat", "grounding",
//     "made up", "ungrounded"
//
// The rate is hallucinationCases / totalCases. The default threshold
// is 0.05 (5%). One signal per EvalRun where the rate exceeds the
// threshold; the metadata includes the rate, the threshold, and a
// per-named-score breakdown so reviewers can see what drove it.
type HallucinationRateDetector struct {
	// Threshold is the maximum acceptable hallucination rate. 0 uses
	// the default of 0.05 (5%).
	Threshold float64
}

// hallucinationKeywords are FailureReason substrings that mark a case
// as hallucination-shaped, used when NamedScores aren't populated.
var hallucinationKeywords = []string{
	"fabricat", "hallucinat", "grounding", "made up", "ungrounded",
}

// Detect emits SignalAIHallucinationRate per offending EvalRun.
func (d *HallucinationRateDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	threshold := d.Threshold
	if threshold <= 0 {
		threshold = 0.05
	}

	var out []models.Signal
	for _, env := range snap.EvalRuns {
		result, err := airun.ParseEvalRunPayload(env)
		if err != nil || result == nil {
			continue
		}
		// Denominator: only count cases that produced a meaningful
		// score. Errored cases (provider crashed, network timeout, no
		// scoring at all) dilute the hallucination rate — pre-0.2.x a
		// 50-case suite with 40 errors and 5 hallucinated valid cases
		// reported 10% (5/50) instead of the actual 50% (5/10) among
		// scoreable cases. Catastrophic eval suite degradation hid the
		// hallucination signal in infra noise.
		scoreable := 0
		hallucinated := 0
		for _, c := range result.Cases {
			if !caseIsScoreable(c) {
				continue
			}
			scoreable++
			if caseLooksHallucinated(c) {
				hallucinated++
			}
		}
		if scoreable == 0 {
			continue
		}
		total := scoreable
		rate := float64(hallucinated) / float64(total)
		if rate <= threshold {
			continue
		}
		out = append(out, models.Signal{
			Type:        signals.SignalAIHallucinationRate,
			Category:    models.CategoryAI,
			Severity:    models.SeverityHigh,
			Confidence:  0.9,
			Location:    models.SignalLocation{File: env.SourcePath, ScenarioID: env.RunID},
			Explanation: fmt.Sprintf("Eval run reports a hallucination-shaped failure rate of %.1f%% (%d of %d cases), above the threshold of %.1f%%.",
				rate*100, hallucinated, total, threshold*100),
			SuggestedAction: "Investigate the failing cases; tighten retrieval or grounding before merging. Bump the threshold only with documented justification.",

			SeverityClauses: []string{"sev-high-006"},
			Actionability:   models.ActionabilityImmediate,
			LifecycleStages: []models.LifecycleStage{models.StageCIRun},
			AIRelevance:     models.AIRelevanceHigh,
			RuleID:          "TER-AI-108",
			RuleURI:         "docs/rules/ai/hallucination-rate.md",
			DetectorVersion: "0.2.0",
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.9,
				IntervalLow:  0.82,
				IntervalHigh: 0.95,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceRuntime},
			},
			EvidenceSource:   models.SourceRuntime,
			EvidenceStrength: models.EvidenceStrong,
			Metadata: map[string]any{
				"framework":        env.Framework,
				"runId":            env.RunID,
				"hallucinated":     hallucinated,
				"totalCases":       total,
				"hallucinationRate": rate,
				"threshold":        threshold,
			},
		})
	}
	return out
}

// caseIsScoreable returns true when an eval case produced a real
// score we can interpret. Excludes cases that errored before
// evaluation (no NamedScores, no Score, no Success bool that would
// itself be meaningless without a grader) — including these in the
// denominator dilutes the hallucination rate.
func caseIsScoreable(c airun.EvalCase) bool {
	if len(c.NamedScores) > 0 {
		return true
	}
	if c.Success {
		return true
	}
	if c.Score > 0 {
		return true
	}
	// A failure with a FailureReason is grading output we should count.
	if c.FailureReason != "" {
		return true
	}
	return false
}

// hallucinationGroundingKeys lists named-score keys whose semantics are
// "low value means ungrounded / hallucinated." Pre-0.2.x the detector
// matched any key containing the substring "ground", which collided
// with non-AI metric names like `background_score` or
// `playground_metric`. Whitelist instead.
var hallucinationGroundingKeys = map[string]bool{
	"groundedness":           true,
	"groundtruth":            true,
	"answer_grounding":       true,
	"answer_grounding_score": true,
	"retrieval_grounding":    true,
}

// caseLooksHallucinated returns true when the case's named scores or
// failure reason indicate a hallucination-shaped problem.
func caseLooksHallucinated(c airun.EvalCase) bool {
	for k, v := range c.NamedScores {
		key := strings.ToLower(k)
		switch {
		case key == "faithfulness" && v < 0.5:
			return true
		case key == "factuality" && v < 0.5:
			return true
		case key == "grounding" && v < 0.5:
			return true
		case key == "hallucination" && v > 0.5:
			return true
		case hallucinationGroundingKeys[key] && v < 0.5:
			return true
		}
	}
	low := strings.ToLower(c.FailureReason)
	for _, kw := range hallucinationKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}
