package aidetect

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// RetrievalRegressionDetector flags drops in retrieval-quality scores
// (context relevance, nDCG, coverage, faithfulness) between an eval
// run and its baseline. Lives alongside aiCostRegression and shares
// the same baseline-snapshot mechanism; consumers see one signal per
// retrieval-quality axis that regressed beyond the threshold.
//
// Detection model:
//
//   For each EvalRun in snap.EvalRuns:
//     1. Match a same-framework EvalRun in snap.Baseline.EvalRuns.
//     2. For each retrievalScoreKeys entry, compute paired-case
//        average across cases that ran in BOTH runs.
//     3. If avg dropped by more than threshold (default 0.05 / 5
//        percentage points absolute), emit a signal naming the axis.
type RetrievalRegressionDetector struct {
	// Threshold is the maximum acceptable absolute drop in a
	// retrieval-quality score (e.g. 0.05 = 5 percentage points).
	// 0 uses the default of 0.05.
	Threshold float64
}

// retrievalScoreKeys is the list of NamedScore keys recognised as
// retrieval-quality axes. Lowercased for matching; we accept a few
// common naming variants.
var retrievalScoreKeys = []string{
	// Ragas modern (mid-2024+) — the actual keys current Ragas emits.
	// Without these, aiRetrievalRegression silently fires zero signals
	// on real Ragas runs, defeating the headline use case of the Ragas
	// adapter.
	"context_precision", "context-precision", "contextprecision",
	"context_recall", "context-recall", "contextrecall",
	"context_entity_recall", "context-entity-recall",
	// Ragas legacy + community variants.
	"context_relevance", "context-relevance", "contextrelevance",
	"ndcg", "ndcg@10", "ndcg@5",
	"coverage",
	"faithfulness",
	"answer_relevancy", "answer-relevancy", "answerrelevancy",
	"retrieval_score", "retrievalscore",
	// LangSmith-style relevance.
	"relevance_score", "relevance-score", "relevancescore",
}

// Detect emits SignalAIRetrievalRegression per regressed axis per run.
func (d *RetrievalRegressionDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil || snap.Baseline == nil {
		return nil
	}
	threshold := d.Threshold
	if threshold <= 0 {
		threshold = 0.05
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

		baseByID := indexCasesByID(baseline.Cases)
		for _, key := range retrievalScoreKeys {
			curAvg, baseAvg, paired := pairedAverageNamedScore(current.Cases, baseByID, key)
			if paired == 0 {
				continue
			}
			drop := baseAvg - curAvg
			if drop <= threshold {
				continue
			}
			// 0.2.0 final-polish: scale confidence by paired-case count
			// (shared helper). Single-paired-case retrieval drops are
			// not the same evidence quality as 100-case drops; without
			// scaling, both fired at 0.9.
			confidence := pairedConfidence(paired)
			out = append(out, models.Signal{
				Type:        signals.SignalAIRetrievalRegression,
				Category:    models.CategoryAI,
				Severity:    models.SeverityHigh,
				Confidence:  confidence,
				Location:    models.SignalLocation{File: env.SourcePath, ScenarioID: env.RunID},
				Explanation: fmt.Sprintf("Retrieval score `%s` dropped %.3f → %.3f (Δ %.3f) across %d paired cases. Threshold: %.3f.",
					key, baseAvg, curAvg, drop, paired, threshold),
				SuggestedAction: "Investigate the regression; revert the offending change or re-tune retrieval before merging.",

				SeverityClauses: []string{"sev-high-007"},
				Actionability:   models.ActionabilityImmediate,
				LifecycleStages: []models.LifecycleStage{models.StageMaintenance, models.StageCIRun},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-111",
				RuleURI:         "docs/rules/ai/retrieval-regression.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        confidence,
					IntervalLow:  confidence - 0.05,
					IntervalHigh: confidence + 0.05,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceRuntime},
				},
				EvidenceSource:   models.SourceRuntime,
				EvidenceStrength: models.EvidenceStrong,
				Metadata: map[string]any{
					"framework":     env.Framework,
					"runId":         env.RunID,
					"baselineRunId": baseEnv.RunID,
					"scoreKey":      key,
					"currentAvg":    curAvg,
					"baselineAvg":   baseAvg,
					"drop":          drop,
					"threshold":     threshold,
					"pairedCases":   paired,
				},
			})
		}
	}
	return out
}

// indexCasesByID builds a lookup from CaseID to EvalCase. Cases
// without an ID are skipped — we can't reliably pair them.
func indexCasesByID(cases []airun.EvalCase) map[string]airun.EvalCase {
	out := make(map[string]airun.EvalCase, len(cases))
	for _, c := range cases {
		if c.CaseID != "" {
			out[c.CaseID] = c
		}
	}
	return out
}

// pairedAverageNamedScore returns the avg score for `key` across cases
// present in both maps. Returns the current avg, baseline avg, and the
// count of pairs. Case-insensitive on the key, and cases that don't
// contain the key in either side are skipped.
func pairedAverageNamedScore(currentCases []airun.EvalCase, baseByID map[string]airun.EvalCase, key string) (curAvg, baseAvg float64, paired int) {
	keyLower := strings.ToLower(key)
	var sumCur, sumBase float64
	for _, c := range currentCases {
		if c.CaseID == "" {
			continue
		}
		base, ok := baseByID[c.CaseID]
		if !ok {
			continue
		}
		curScore, curOK := lookupScoreLower(c.NamedScores, keyLower)
		baseScore, baseOK := lookupScoreLower(base.NamedScores, keyLower)
		if !curOK || !baseOK {
			continue
		}
		sumCur += curScore
		sumBase += baseScore
		paired++
	}
	if paired == 0 {
		return 0, 0, 0
	}
	return sumCur / float64(paired), sumBase / float64(paired), paired
}

// lookupScoreLower searches a case-insensitive named-score map.
func lookupScoreLower(scores map[string]float64, keyLower string) (float64, bool) {
	for k, v := range scores {
		if strings.ToLower(k) == keyLower {
			return v, true
		}
	}
	return 0, false
}
