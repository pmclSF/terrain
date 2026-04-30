package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func envelopeForCases(t *testing.T, cases []airun.EvalCase) models.EvalRunEnvelope {
	t.Helper()
	r := &airun.EvalRunResult{
		Framework: "promptfoo",
		RunID:     "test-run",
		Cases:     cases,
	}
	for _, c := range cases {
		if c.Success {
			r.Aggregates.Successes++
		} else {
			r.Aggregates.Failures++
		}
	}
	env, err := r.ToEnvelope("evals/run.json")
	if err != nil {
		t.Fatalf("ToEnvelope: %v", err)
	}
	return env
}

func TestHallucinationRate_FiresOnLowFaithfulness(t *testing.T) {
	t.Parallel()

	cases := make([]airun.EvalCase, 20)
	for i := range cases {
		cases[i] = airun.EvalCase{
			CaseID:      string(rune('a' + i)),
			Success:     true,
			NamedScores: map[string]float64{"faithfulness": 0.95},
		}
	}
	// 3 of 20 = 15% hallucinated → above 5% threshold.
	cases[0].NamedScores = map[string]float64{"faithfulness": 0.2}
	cases[1].NamedScores = map[string]float64{"faithfulness": 0.3}
	cases[2].NamedScores = map[string]float64{"faithfulness": 0.4}

	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{envelopeForCases(t, cases)},
	}
	got := (&HallucinationRateDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1: %+v", len(got), got)
	}
	if got[0].Type != signals.SignalAIHallucinationRate {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want high", got[0].Severity)
	}
	if rate, _ := got[0].Metadata["hallucinationRate"].(float64); rate < 0.14 || rate > 0.16 {
		t.Errorf("hallucinationRate = %v, want ~0.15", rate)
	}
}

func TestHallucinationRate_StaysQuietBelowThreshold(t *testing.T) {
	t.Parallel()

	cases := make([]airun.EvalCase, 100)
	for i := range cases {
		cases[i] = airun.EvalCase{
			Success:     true,
			NamedScores: map[string]float64{"faithfulness": 0.95},
		}
	}
	// 2 of 100 = 2% — below the 5% default threshold.
	cases[0].NamedScores = map[string]float64{"faithfulness": 0.1}
	cases[1].NamedScores = map[string]float64{"faithfulness": 0.2}

	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{envelopeForCases(t, cases)},
	}
	got := (&HallucinationRateDetector{}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("expected no signals at 2%% rate, got %d", len(got))
	}
}

func TestHallucinationRate_FiresOnFailureKeywords(t *testing.T) {
	t.Parallel()

	cases := []airun.EvalCase{
		{Success: true},
		{Success: true},
		{Success: false, FailureReason: "model fabricated a citation"},
		{Success: false, FailureReason: "ungrounded answer detected"},
		{Success: false, FailureReason: "wrong answer (factual error)"},
		{Success: false, FailureReason: "wrong answer"}, // no halluc keyword
		{Success: false, FailureReason: "wrong"},        // no halluc keyword
	}
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{envelopeForCases(t, cases)},
	}
	got := (&HallucinationRateDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	// 2 keyword-shaped (fabricated, ungrounded) of 7 → ~28%.
	rate, _ := got[0].Metadata["hallucinationRate"].(float64)
	if rate < 0.25 || rate > 0.31 {
		t.Errorf("hallucinationRate = %v, want ~0.28", rate)
	}
}

func TestHallucinationRate_HandlesInversePolarity(t *testing.T) {
	t.Parallel()

	// "hallucination" score is high = bad. Inverse of faithfulness.
	cases := make([]airun.EvalCase, 10)
	for i := range cases {
		cases[i] = airun.EvalCase{
			Success:     true,
			NamedScores: map[string]float64{"hallucination": 0.05},
		}
	}
	cases[0].NamedScores = map[string]float64{"hallucination": 0.9}
	cases[1].NamedScores = map[string]float64{"hallucination": 0.7}

	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{envelopeForCases(t, cases)},
	}
	got := (&HallucinationRateDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
}

func TestHallucinationRate_EmptySnap(t *testing.T) {
	t.Parallel()
	if got := (&HallucinationRateDetector{}).Detect(&models.TestSuiteSnapshot{}); len(got) != 0 {
		t.Errorf("got %d signals, want 0 on empty snapshot", len(got))
	}
}

func TestHallucinationRate_RespectsCustomThreshold(t *testing.T) {
	t.Parallel()

	cases := make([]airun.EvalCase, 100)
	for i := range cases {
		cases[i] = airun.EvalCase{Success: true, NamedScores: map[string]float64{"faithfulness": 0.95}}
	}
	// 6% rate.
	for i := 0; i < 6; i++ {
		cases[i].NamedScores = map[string]float64{"faithfulness": 0.1}
	}

	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{envelopeForCases(t, cases)},
	}
	// Default threshold (5%): fires.
	if got := (&HallucinationRateDetector{}).Detect(snap); len(got) != 1 {
		t.Errorf("default threshold should fire, got %d", len(got))
	}
	// Custom threshold (10%): stays quiet.
	if got := (&HallucinationRateDetector{Threshold: 0.10}).Detect(snap); len(got) != 0 {
		t.Errorf("custom 10%% threshold should not fire on 6%% rate, got %d", len(got))
	}
}
