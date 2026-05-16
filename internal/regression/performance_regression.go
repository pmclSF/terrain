package regression

import (
	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectPerformanceRegression is the classical-ML twin of
// DetectEvalRegression. Same detection mechanism (per-case Score
// delta exceeds threshold, optional run-level signal); different
// rule ID and signal type so consumers can branch the rendering.
//
// Routing convention at 0.2.0:
//
//   - EvalRun.Framework == promptfoo / deepeval / ragas → use
//     DetectEvalRegression (LLM rubric scores).
//   - EvalRun.Framework == great_expectations or framework annotation
//     indicates classical ML metric (accuracy / F1 / AUC / RMSE) →
//     use DetectPerformanceRegression.
//
// The split serves rule-layer downstream code that wants to display
// "performance regression in `RandomForestClassifier`" differently
// from "eval regression in `summarize_refusal`."
func DetectPerformanceRegression(baseline, current *evaladapter.EvalRun, cfg EvalRegressionConfig) []models.Signal {
	sigs := DetectEvalRegression(baseline, current, cfg)
	for i := range sigs {
		sigs[i].Type = signals.SignalPerformanceRegression
		sigs[i].RuleID = "terrain/regression/performance-regression"
		sigs[i].RuleURI = "docs/rules/regression/performance-regression.md"
	}
	return sigs
}
