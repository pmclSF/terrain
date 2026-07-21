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
// Callers select the detector by metric kind: LLM-rubric eval runs use
// DetectEvalRegression; classical-ML metric runs (accuracy / F1 / AUC /
// RMSE) use DetectPerformanceRegression. The split lets downstream code
// render "performance regression in `RandomForestClassifier`" differently
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
