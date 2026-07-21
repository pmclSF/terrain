package regression

import (
	"testing"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectEvalRegression_CaseDropExceedsThreshold(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Framework: evaladapter.FrameworkPromptfoo,
		Source:    "baseline.json",
		Cases: []evaladapter.EvalCaseResult{
			{ID: "safety-1", Name: "safety", Score: 0.95},
			{ID: "safety-2", Name: "borderline", Score: 0.80},
		},
		Stats: evaladapter.EvalRunStats{Total: 2, PrimaryMetric: 0.875, HasPrimaryMetric: true},
	}
	current := &evaladapter.EvalRun{
		Framework: evaladapter.FrameworkPromptfoo,
		Source:    "current.json",
		Cases: []evaladapter.EvalCaseResult{
			{ID: "safety-1", Name: "safety", Score: 0.50, Reason: "safety check failed on adversarial input"},
			{ID: "safety-2", Name: "borderline", Score: 0.80},
		},
		Stats: evaladapter.EvalRunStats{Total: 2, PrimaryMetric: 0.65, HasPrimaryMetric: true},
	}
	sigs := DetectEvalRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) < 1 {
		t.Fatalf("expected ≥1 signal, got %d", len(sigs))
	}

	// First signal should be the case regression for safety-1.
	caseHit := findRegressionByScope(sigs, "case")
	if caseHit == nil {
		t.Fatalf("missing case-scope signal: %+v", sigs)
	}
	if caseHit.Metadata["caseId"] != "safety-1" {
		t.Errorf("case id = %v", caseHit.Metadata["caseId"])
	}
	if caseHit.Severity != models.SeverityCritical {
		t.Errorf("severity = %q, want critical (delta 0.45 > 0.25)", caseHit.Severity)
	}

	// And a run-level signal because the run primary metric also dropped.
	runHit := findRegressionByScope(sigs, "run")
	if runHit == nil {
		t.Fatalf("missing run-scope signal: %+v", sigs)
	}
}

func TestDetectEvalRegression_SmallDropSuppressed(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "x", Score: 0.95},
		},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "x", Score: 0.93},
		},
	}
	sigs := DetectEvalRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) != 0 {
		t.Errorf("0.02 delta should be below 0.05 threshold, got %+v", sigs)
	}
}

func TestDetectEvalRegression_NoBaselineCase(t *testing.T) {
	t.Parallel()
	// Current has a case absent from baseline — should be skipped.
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "existing", Score: 0.9},
		},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{
			{ID: "existing", Score: 0.91},
			{ID: "new", Score: 0.3}, // no baseline → not flagged here
		},
	}
	sigs := DetectEvalRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) != 0 {
		t.Errorf("new case without baseline should not fire, got %+v", sigs)
	}
}

func TestDetectEvalRegression_CustomThreshold(t *testing.T) {
	t.Parallel()
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 1.0}},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.97}},
	}
	cfg := EvalRegressionConfig{Threshold: 0.02} // tighter
	sigs := DetectEvalRegression(baseline, current, cfg)
	if len(sigs) != 1 {
		t.Errorf("0.03 delta should exceed 0.02 threshold, got %+v", sigs)
	}
}

func TestDetectEvalRegression_NilInputs(t *testing.T) {
	t.Parallel()
	if got := DetectEvalRegression(nil, nil, DefaultEvalRegressionConfig()); got != nil {
		t.Errorf("nil inputs should yield nil, got %+v", got)
	}
	if got := DetectEvalRegression(&evaladapter.EvalRun{}, nil, DefaultEvalRegressionConfig()); got != nil {
		t.Errorf("nil current should yield nil")
	}
}

func TestDetectEvalRegression_RunLevelOnly(t *testing.T) {
	t.Parallel()
	// No per-case regression, but PrimaryMetric dropped overall (e.g., due to
	// new failing cases added to baseline that aren't in current). The
	// detector still surfaces a run-level signal.
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.95}},
		Stats: evaladapter.EvalRunStats{PrimaryMetric: 0.95, HasPrimaryMetric: true},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.95}},
		Stats: evaladapter.EvalRunStats{PrimaryMetric: 0.70, HasPrimaryMetric: true},
	}
	sigs := DetectEvalRegression(baseline, current, DefaultEvalRegressionConfig())
	if len(sigs) != 1 {
		t.Fatalf("expected 1 run-level signal, got %d", len(sigs))
	}
	if sigs[0].Metadata["scope"] != "run" {
		t.Errorf("expected run-scope, got %v", sigs[0].Metadata["scope"])
	}
}

// TestDetectEvalRegression_RunMeanToZero guards the worst-case run-level
// regression: the current run mean collapses to exactly 0.0. A guard that
// required the current metric to be > 0 would silently skip this; gating on
// HasPrimaryMetric keeps it reported.
func TestDetectEvalRegression_RunMeanToZero(t *testing.T) {
	baseline := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.90}},
		Stats: evaladapter.EvalRunStats{PrimaryMetric: 0.90, HasPrimaryMetric: true},
	}
	current := &evaladapter.EvalRun{
		Cases: []evaladapter.EvalCaseResult{{ID: "x", Score: 0.90}},
		Stats: evaladapter.EvalRunStats{PrimaryMetric: 0.0, HasPrimaryMetric: true},
	}
	sigs := DetectEvalRegression(baseline, current, DefaultEvalRegressionConfig())
	if findRegressionByScope(sigs, "run") == nil {
		t.Fatal("run mean 0.9 -> 0.0 must produce a run-level regression signal")
	}
}

func findRegressionByScope(sigs []models.Signal, scope string) *models.Signal {
	for i, s := range sigs {
		if s.Metadata["scope"] == scope {
			return &sigs[i]
		}
	}
	return nil
}
