package airun

import "testing"

const ragasSample = `{
  "run_id": "ragas-run-1",
  "created_at": "2026-04-30T12:00:00Z",
  "results": [
    {
      "id": "row-1",
      "question": "What is the capital of France?",
      "answer": "Paris",
      "context_relevance": 0.92,
      "faithfulness": 0.88,
      "answer_relevancy": 0.85
    },
    {
      "id": "row-2",
      "question": "Who painted the Mona Lisa?",
      "answer": "Leonardo da Vinci",
      "context_relevance": 0.30,
      "faithfulness": 0.20,
      "answer_relevancy": 0.40
    }
  ]
}`

func TestParseRagas_Roundtrip(t *testing.T) {
	t.Parallel()

	got, err := ParseRagasJSON([]byte(ragasSample))
	if err != nil {
		t.Fatalf("ParseRagasJSON: %v", err)
	}
	if got.Framework != "ragas" {
		t.Errorf("framework = %q", got.Framework)
	}
	if got.RunID != "ragas-run-1" {
		t.Errorf("runId = %q", got.RunID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if len(got.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(got.Cases))
	}

	// Case 1: all scores above 0.5 → success.
	if !got.Cases[0].Success {
		t.Errorf("case 0 should be success (all named scores >= 0.5)")
	}
	if got.Cases[0].NamedScores["context_relevance"] != 0.92 {
		t.Errorf("case 0 context_relevance = %v", got.Cases[0].NamedScores["context_relevance"])
	}
	if got.Cases[0].NamedScores["faithfulness"] != 0.88 {
		t.Errorf("case 0 faithfulness = %v", got.Cases[0].NamedScores["faithfulness"])
	}

	// Case 2: any below threshold → failure.
	if got.Cases[1].Success {
		t.Errorf("case 1 should fail (any named score < 0.5)")
	}

	if got.Aggregates.Successes != 1 || got.Aggregates.Failures != 1 {
		t.Errorf("aggregates = %+v", got.Aggregates)
	}
}

func TestParseRagas_Description(t *testing.T) {
	t.Parallel()
	got, _ := ParseRagasJSON([]byte(ragasSample))
	if got.Cases[0].Description != "What is the capital of France?" {
		t.Errorf("description = %q", got.Cases[0].Description)
	}
}

func TestParseRagas_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := ParseRagasJSON(nil); err == nil {
		t.Error("expected empty payload to be rejected")
	}
}

func TestParseRagas_NoResults(t *testing.T) {
	t.Parallel()
	if _, err := ParseRagasJSON([]byte(`{"results": []}`)); err == nil {
		t.Error("expected empty results to be rejected")
	}
}

func TestParseRagas_FallbackCaseID(t *testing.T) {
	t.Parallel()

	const noID = `{"results": [
      {"question": "q1", "faithfulness": 0.8},
      {"question": "q2", "faithfulness": 0.6}
   ]}`
	got, err := ParseRagasJSON([]byte(noID))
	if err != nil {
		t.Fatalf("ParseRagasJSON: %v", err)
	}
	if got.Cases[0].CaseID != "ragas-row-1" || got.Cases[1].CaseID != "ragas-row-2" {
		t.Errorf("expected fallback ids, got [%q, %q]", got.Cases[0].CaseID, got.Cases[1].CaseID)
	}
}
