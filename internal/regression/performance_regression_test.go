package regression

import (
	"testing"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestDetectPerformanceRegression_Fires(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "acc", Name: "accuracy", Score: 0.91},
		},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "acc", Name: "accuracy", Score: 0.78},
		},
	}
	sigs := DetectPerformanceRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Type != signals.SignalPerformanceRegression {
		t.Errorf("type = %q, want performanceRegression", sigs[0].Type)
	}
	if sigs[0].RuleID != "terrain/regression/performance-regression" {
		t.Errorf("ruleID = %q", sigs[0].RuleID)
	}
}

func TestDetectPerformanceRegression_NoChangeSuppressed(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "acc", Score: 0.91}},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "acc", Score: 0.90}},
	}
	sigs := DetectPerformanceRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) != 0 {
		t.Errorf("0.01 drop suppressed, got %+v", sigs)
	}
}
