package airun

import "testing"

const deepEvalSample = `{
  "testRunId": "run-deepeval-1",
  "createdAt": "2026-04-30T12:00:00Z",
  "testCases": [
    {
      "id": "tc-1",
      "name": "answers paris",
      "description": "happy path",
      "latencyMs": 950,
      "metricsData": [
        {"name": "AnswerRelevancy", "score": 0.92, "threshold": 0.5, "success": true},
        {"name": "Faithfulness",    "score": 0.88, "threshold": 0.5, "success": true}
      ],
      "tokenUsage": {"total": 80, "cost": 0.0024}
    },
    {
      "id": "tc-2",
      "name": "answers london",
      "description": "edge case",
      "latencyMs": 1500,
      "metricsData": [
        {"name": "AnswerRelevancy", "score": 0.40, "threshold": 0.5, "success": false},
        {"name": "Faithfulness",    "score": 0.20, "threshold": 0.5, "success": false}
      ],
      "tokenUsage": {"total": 65, "cost": 0.0019}
    }
  ]
}`

func TestParseDeepEval_Roundtrip(t *testing.T) {
	t.Parallel()

	got, err := ParseDeepEvalJSON([]byte(deepEvalSample))
	if err != nil {
		t.Fatalf("ParseDeepEvalJSON: %v", err)
	}
	if got.Framework != "deepeval" {
		t.Errorf("framework = %q", got.Framework)
	}
	if got.RunID != "run-deepeval-1" {
		t.Errorf("runId = %q", got.RunID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if len(got.Cases) != 2 {
		t.Fatalf("cases = %d", len(got.Cases))
	}
	if !got.Cases[0].Success {
		t.Errorf("case 0 should succeed (all metrics passed)")
	}
	if got.Cases[1].Success {
		t.Errorf("case 1 should fail (any metric failure flips success)")
	}
	if got.Cases[0].NamedScores["answerrelevancy"] != 0.92 {
		t.Errorf("case 0 answerrelevancy = %v", got.Cases[0].NamedScores["answerrelevancy"])
	}
	if got.Cases[0].NamedScores["faithfulness"] != 0.88 {
		t.Errorf("case 0 faithfulness = %v", got.Cases[0].NamedScores["faithfulness"])
	}
	if got.Cases[0].Score != 0.90 {
		t.Errorf("case 0 score (mean) = %v, want 0.90", got.Cases[0].Score)
	}
	if got.Cases[1].FailureReason == "" {
		t.Error("case 1 should carry a failure reason")
	}
	if got.Aggregates.Successes != 1 || got.Aggregates.Failures != 1 {
		t.Errorf("aggregates = %+v", got.Aggregates)
	}
	if got.Aggregates.TokenUsage.Total != 145 {
		t.Errorf("tokens.total = %d, want 145", got.Aggregates.TokenUsage.Total)
	}
}

func TestParseDeepEval_NoCases(t *testing.T) {
	t.Parallel()
	if _, err := ParseDeepEvalJSON([]byte(`{"testCases": []}`)); err == nil {
		t.Error("expected empty testCases to be rejected")
	}
}

func TestParseDeepEval_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := ParseDeepEvalJSON(nil); err == nil {
		t.Error("expected empty payload to be rejected")
	}
}
