package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// envelopeWithCases builds a Promptfoo-shape envelope with the given
// cases. Each case's TokenUsage is taken at face value.
func envelopeWithCases(t *testing.T, runID string, cases []airun.EvalCase) models.EvalRunEnvelope {
	t.Helper()
	r := &airun.EvalRunResult{
		Framework: "promptfoo",
		RunID:     runID,
		Cases:     cases,
	}
	env, err := r.ToEnvelope("evals/run.json")
	if err != nil {
		t.Fatalf("ToEnvelope: %v", err)
	}
	return env
}

func TestCostRegression_FiresOnIncrease(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.001}},
		{CaseID: "b", TokenUsage: airun.TokenUsage{Cost: 0.002}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.0015}}, // +50%
		{CaseID: "b", TokenUsage: airun.TokenUsage{Cost: 0.003}},  // +50%
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	got := (&CostRegressionDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAICostRegression {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q, want medium", got[0].Severity)
	}
	if delta, _ := got[0].Metadata["deltaPct"].(float64); delta < 0.49 || delta > 0.51 {
		t.Errorf("deltaPct = %v, want ~0.5", delta)
	}
}

func TestCostRegression_StaysQuietBelowThreshold(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.001}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.0011}}, // +10%, below 25%
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	if got := (&CostRegressionDetector{}).Detect(snap); len(got) != 0 {
		t.Errorf("expected no signals at +10%%, got %d", len(got))
	}
}

func TestCostRegression_RequiresBaseline(t *testing.T) {
	t.Parallel()

	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.10}},
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		// No Baseline.
	}
	if got := (&CostRegressionDetector{}).Detect(snap); len(got) != 0 {
		t.Errorf("expected no signals without baseline, got %d", len(got))
	}
}

func TestCostRegression_SkipsUnpairedCases(t *testing.T) {
	t.Parallel()

	// Baseline: a, b. Current: a, c. Only "a" is paired.
	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.001}},
		{CaseID: "b", TokenUsage: airun.TokenUsage{Cost: 0.001}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.002}}, // +100% on the only paired case
		{CaseID: "c", TokenUsage: airun.TokenUsage{Cost: 0.005}}, // new case — ignored
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	got := (&CostRegressionDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if paired, _ := got[0].Metadata["pairedCases"].(int); paired != 1 {
		t.Errorf("pairedCases = %v, want 1", paired)
	}
}

func TestCostRegression_RespectsCustomThreshold(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.001}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", TokenUsage: airun.TokenUsage{Cost: 0.0015}}, // +50%
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	// Default threshold (25%): fires.
	if got := (&CostRegressionDetector{}).Detect(snap); len(got) != 1 {
		t.Errorf("default threshold should fire on +50%%, got %d", len(got))
	}
	// Custom threshold (60%): does not fire.
	if got := (&CostRegressionDetector{Threshold: 0.60}).Detect(snap); len(got) != 0 {
		t.Errorf("60%% threshold should not fire on +50%%, got %d", len(got))
	}
}
