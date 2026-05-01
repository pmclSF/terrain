package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestRetrievalRegression_FiresOnContextRelevanceDrop(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"context_relevance": 0.92}},
		{CaseID: "b", NamedScores: map[string]float64{"context_relevance": 0.88}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"context_relevance": 0.70}}, // -0.22
		{CaseID: "b", NamedScores: map[string]float64{"context_relevance": 0.65}}, // -0.23
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	got := (&RetrievalRegressionDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAIRetrievalRegression {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want high", got[0].Severity)
	}
	if got[0].Metadata["scoreKey"] != "context_relevance" {
		t.Errorf("scoreKey = %v", got[0].Metadata["scoreKey"])
	}
}

func TestRetrievalRegression_FiresOnNDCGDrop(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"nDCG": 0.85}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"nDCG": 0.70}}, // -0.15
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	got := (&RetrievalRegressionDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	// Case-insensitive match: "nDCG" → "ndcg" key lookup.
	if got[0].Metadata["scoreKey"] != "ndcg" {
		t.Errorf("scoreKey = %v, want ndcg", got[0].Metadata["scoreKey"])
	}
}

func TestRetrievalRegression_StaysQuietBelowThreshold(t *testing.T) {
	t.Parallel()

	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"coverage": 0.90}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"coverage": 0.88}}, // -0.02
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	if got := (&RetrievalRegressionDetector{}).Detect(snap); len(got) != 0 {
		t.Errorf("expected no signals at -0.02, got %d", len(got))
	}
}

func TestRetrievalRegression_FiresPerAxis(t *testing.T) {
	t.Parallel()

	// Both axes regress → two signals.
	baseline := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{
			"context_relevance": 0.90,
			"faithfulness":      0.85,
		}},
	})
	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{
			"context_relevance": 0.70,
			"faithfulness":      0.65,
		}},
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
		Baseline: &models.TestSuiteSnapshot{
			EvalRuns: []models.EvalRunEnvelope{baseline},
		},
	}
	got := (&RetrievalRegressionDetector{}).Detect(snap)
	if len(got) != 2 {
		t.Fatalf("got %d signals, want 2 (one per axis)", len(got))
	}
}

func TestRetrievalRegression_RequiresBaseline(t *testing.T) {
	t.Parallel()

	current := envelopeWithCases(t, "run-1", []airun.EvalCase{
		{CaseID: "a", NamedScores: map[string]float64{"context_relevance": 0.10}},
	})
	snap := &models.TestSuiteSnapshot{
		EvalRuns: []models.EvalRunEnvelope{current},
	}
	if got := (&RetrievalRegressionDetector{}).Detect(snap); len(got) != 0 {
		t.Errorf("expected no signals without baseline, got %d", len(got))
	}
}

// TestRetrievalRegression_FiresOnRagasModernKeys locks in the 0.2
// ship-blocker fix — Ragas's current key (`context_precision`,
// `context_recall`, `context_entity_recall`) and LangSmith's
// `relevance_score` must trigger the regression detector. Pre-0.2.x
// only the legacy `context_relevance` was in the allowlist; against a
// real Ragas run the detector fired zero signals.
func TestRetrievalRegression_FiresOnRagasModernKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  string
	}{
		{"context_precision", "context_precision"},
		{"context_recall", "context_recall"},
		{"context_entity_recall", "context_entity_recall"},
		{"relevance_score", "relevance_score"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline := envelopeWithCases(t, "ragas-modern", []airun.EvalCase{
				{CaseID: "a", NamedScores: map[string]float64{tc.key: 0.92}},
				{CaseID: "b", NamedScores: map[string]float64{tc.key: 0.88}},
			})
			current := envelopeWithCases(t, "ragas-modern", []airun.EvalCase{
				{CaseID: "a", NamedScores: map[string]float64{tc.key: 0.70}},
				{CaseID: "b", NamedScores: map[string]float64{tc.key: 0.65}},
			})
			snap := &models.TestSuiteSnapshot{
				EvalRuns: []models.EvalRunEnvelope{current},
				Baseline: &models.TestSuiteSnapshot{
					EvalRuns: []models.EvalRunEnvelope{baseline},
				},
			}
			got := (&RetrievalRegressionDetector{}).Detect(snap)
			if len(got) == 0 {
				t.Fatalf("expected at least 1 signal for %s drop, got none", tc.key)
			}
		})
	}
}
