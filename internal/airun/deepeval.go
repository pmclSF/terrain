package airun

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ParseDeepEvalJSON parses a DeepEval `--export results.json` payload
// into a normalised EvalRunResult. Pairs with ParsePromptfooJSON; both
// adapters emit the same shape so the runtime-aware AI detectors
// (aiHallucinationRate, aiCostRegression, aiRetrievalRegression) work
// against either eval framework.
//
// DeepEval's JSON layout is roughly:
//
//	{
//	  "testRunId": "<id>",
//	  "createdAt": "2026-04-30T...",
//	  "testCases": [
//	    {
//	      "input": "...",
//	      "actualOutput": "...",
//	      "metricsData": [
//	        {"name": "AnswerRelevancy", "score": 0.85,
//	         "threshold": 0.5, "success": true},
//	        {"name": "Faithfulness", "score": 0.30,
//	         "threshold": 0.5, "success": false},
//	        ...
//	      ]
//	    }, ...
//	  ]
//	}
//
// We normalise as follows:
//   - one EvalCase per testCase
//   - Success := all metricsData entries' success==true (a single
//     metric failure flips the case to false)
//   - Score := average of metric scores (0..1); falls back to 1.0 / 0.0
//     based on Success when no scores are present
//   - NamedScores := each metric name → score (lowercased key)
//   - LatencyMs / TokenUsage taken when present
//
// DeepEval doesn't write a stats block; aggregates are derived from
// the cases.
func ParseDeepEvalJSON(data []byte) (*EvalRunResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	var raw deepEvalEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse deepeval payload: %w", err)
	}
	if len(raw.TestCases) == 0 {
		return nil, fmt.Errorf("deepeval payload has no testCases")
	}

	out := &EvalRunResult{
		Framework: "deepeval",
		RunID:     raw.TestRunID,
	}
	if t, err := time.Parse(time.RFC3339, raw.CreatedAt); err == nil {
		out.CreatedAt = t.UTC()
	}

	out.Cases = make([]EvalCase, 0, len(raw.TestCases))
	for _, tc := range raw.TestCases {
		c := EvalCase{
			CaseID:        firstNonEmpty(tc.ID, tc.Name),
			Description:   firstNonEmpty(tc.Description, tc.Name),
			LatencyMs:     tc.LatencyMs,
			FailureReason: deepEvalFailureReason(tc),
			TokenUsage: TokenUsage{
				Prompt:     tc.TokenUsage.Prompt,
				Completion: tc.TokenUsage.Completion,
				Total:      tc.TokenUsage.Total,
				Cost:       tc.TokenUsage.Cost,
			},
		}
		c.Success, c.Score, c.NamedScores = aggregateMetricsData(tc.MetricsData)
		out.Cases = append(out.Cases, c)

		if c.Success {
			out.Aggregates.Successes++
		} else {
			out.Aggregates.Failures++
		}
		out.Aggregates.TokenUsage.Total += c.TokenUsage.Total
		out.Aggregates.TokenUsage.Prompt += c.TokenUsage.Prompt
		out.Aggregates.TokenUsage.Completion += c.TokenUsage.Completion
		out.Aggregates.TokenUsage.Cost += c.TokenUsage.Cost
	}

	return out, nil
}

// LoadDeepEvalFile is the convenience wrapper around ParseDeepEvalJSON.
func LoadDeepEvalFile(path string) (*EvalRunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseDeepEvalJSON(data)
}

// aggregateMetricsData distills a DeepEval metricsData list into a
// (success, score, namedScores) triple. Success is the AND of every
// metric's success field. Score is the mean of the per-metric scores
// (so a case with mixed metrics scores ~0.5).
func aggregateMetricsData(metrics []deepEvalMetricEntry) (success bool, score float64, named map[string]float64) {
	if len(metrics) == 0 {
		return false, 0, nil
	}
	success = true
	var sum float64
	named = make(map[string]float64, len(metrics))
	for _, m := range metrics {
		if !m.Success {
			success = false
		}
		sum += m.Score
		key := strings.ToLower(strings.TrimSpace(m.Name))
		if key != "" {
			named[key] = m.Score
		}
	}
	score = sum / float64(len(metrics))
	return success, score, named
}

// deepEvalFailureReason produces a one-line summary of why the case
// failed by listing the metric names that flipped success=false.
func deepEvalFailureReason(tc deepEvalTestCase) string {
	if tc.FailureReason != "" {
		return tc.FailureReason
	}
	var failed []string
	for _, m := range tc.MetricsData {
		if !m.Success {
			failed = append(failed, m.Name)
		}
	}
	if len(failed) == 0 {
		return ""
	}
	return "metrics failed: " + strings.Join(failed, ", ")
}

// ── DeepEval wire shapes (the subset we consume) ────────────────────

type deepEvalEnvelope struct {
	TestRunID string             `json:"testRunId,omitempty"`
	CreatedAt string             `json:"createdAt,omitempty"`
	TestCases []deepEvalTestCase `json:"testCases"`
}

type deepEvalTestCase struct {
	ID            string                `json:"id,omitempty"`
	Name          string                `json:"name,omitempty"`
	Description   string                `json:"description,omitempty"`
	LatencyMs     int                   `json:"latencyMs,omitempty"`
	FailureReason string                `json:"failureReason,omitempty"`
	MetricsData   []deepEvalMetricEntry `json:"metricsData,omitempty"`
	TokenUsage    deepEvalTokenUsage    `json:"tokenUsage,omitempty"`
}

type deepEvalMetricEntry struct {
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Threshold float64 `json:"threshold,omitempty"`
	Success   bool    `json:"success"`
}

type deepEvalTokenUsage struct {
	Prompt     int     `json:"prompt,omitempty"`
	Completion int     `json:"completion,omitempty"`
	Total      int     `json:"total,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}
