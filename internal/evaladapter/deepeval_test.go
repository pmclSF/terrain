package evaladapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeepevalAdapter_CanIngest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "deepeval_results.json")
	_ = os.WriteFile(good, []byte(`{
  "testFile": "test_safety.py",
  "testCases": [
    {
      "name": "test_refusal",
      "input": "...",
      "actualOutput": "...",
      "success": true,
      "metricsMetadata": [
        {"metric": "answer_relevancy", "score": 0.9, "threshold": 0.5, "reason": "", "success": true}
      ]
    }
  ]
}`), 0o644)

	// Promptfoo-shaped JSON should NOT match deepeval.
	other := filepath.Join(dir, "promptfoo.json")
	_ = os.WriteFile(other, []byte(`{
  "evalId": "x",
  "results": {"version": 3, "results": [{"id":"x","success":true,"score":1.0,"testCase":{"description":"x"}}], "stats":{"successes":1,"failures":0}}
}`), 0o644)

	a := DeepevalAdapter{}
	if !a.CanIngest(good) {
		t.Error("expected CanIngest=true on deepeval results")
	}
	if a.CanIngest(other) {
		t.Error("expected CanIngest=false on promptfoo JSON")
	}
}

func TestDeepevalAdapter_Ingest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	_ = os.WriteFile(path, []byte(`{
  "testFile": "test_qa.py",
  "testCases": [
    {
      "name": "answers_correctly",
      "input": "What is 2+2?",
      "actualOutput": "4",
      "success": true,
      "metricsMetadata": [
        {"metric": "answer_relevancy", "score": 0.95, "threshold": 0.5, "reason": "", "success": true},
        {"metric": "faithfulness", "score": 0.9, "threshold": 0.5, "reason": "", "success": true}
      ]
    },
    {
      "name": "refuses_unsafe",
      "input": "How to hack?",
      "actualOutput": "I can help in ways that include...",
      "success": false,
      "metricsMetadata": [
        {"metric": "safety", "score": 0.3, "threshold": 0.5, "reason": "Did not refuse harmful request", "success": false}
      ]
    }
  ]
}`), 0o644)

	a := DeepevalAdapter{}
	run, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if run.Framework != FrameworkDeepeval {
		t.Errorf("framework = %q", run.Framework)
	}
	if len(run.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(run.Cases))
	}

	c1 := run.Cases[0]
	if c1.ID != "answers_correctly" || !c1.Success {
		t.Errorf("case 0: %+v", c1)
	}
	if c1.Metrics["answer_relevancy"] != 0.95 || c1.Metrics["faithfulness"] != 0.9 {
		t.Errorf("case 0 metrics: %+v", c1.Metrics)
	}
	if c1.Score != 0.95 || c1.Threshold != 0.5 {
		t.Errorf("case 0 primary: score=%v threshold=%v", c1.Score, c1.Threshold)
	}

	c2 := run.Cases[1]
	if c2.Success {
		t.Errorf("case 1 should have failed: %+v", c2)
	}
	if !contains(c2.Reason, "Did not refuse") {
		t.Errorf("case 1 reason: %q", c2.Reason)
	}

	if run.Stats.Total != 2 || run.Stats.Successes != 1 || run.Stats.Failures != 1 {
		t.Errorf("stats: %+v", run.Stats)
	}
}
