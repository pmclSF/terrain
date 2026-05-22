package evaladapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptfooAdapter_CanIngest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "results.json")
	if err := os.WriteFile(good, []byte(`{
  "evalId": "abc123",
  "results": {
    "version": 3,
    "timestamp": "2099-05-01T12:00:00Z",
    "results": [
      {"id": "x", "success": true, "score": 1.0, "testCase": {"description": "x"}}
    ],
    "stats": {"successes": 1, "failures": 0}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	bad := filepath.Join(dir, "notpromptfoo.json")
	if err := os.WriteFile(bad, []byte(`{"results": [{"name": "x"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	notJSON := filepath.Join(dir, "log.txt")
	_ = os.WriteFile(notJSON, []byte("hello"), 0o644)

	a := PromptfooAdapter{}
	if !a.CanIngest(good) {
		t.Error("expected CanIngest=true on well-formed promptfoo results")
	}
	if a.CanIngest(bad) {
		t.Error("expected CanIngest=false on non-promptfoo JSON")
	}
	if a.CanIngest(notJSON) {
		t.Error("expected CanIngest=false on non-JSON file")
	}
}

func TestPromptfooAdapter_Ingest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	if err := os.WriteFile(path, []byte(`{
  "evalId": "abc123",
  "results": {
    "version": 3,
    "timestamp": "2099-05-01T12:00:00Z",
    "results": [
      {
        "id": "safety-1",
        "success": true,
        "score": 0.95,
        "namedScores": {"safety": 1.0, "fluency": 0.9},
        "testCase": {"description": "Refuses harmful request", "threshold": 0.8}
      },
      {
        "id": "safety-2",
        "success": false,
        "score": 0.4,
        "namedScores": {"safety": 0.5, "fluency": 0.3},
        "testCase": {"description": "Refuses borderline request", "threshold": 0.8},
        "gradingResult": {"pass": false, "score": 0.4, "reason": "Did not refuse; responded with template"}
      }
    ],
    "stats": {"successes": 1, "failures": 1}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := PromptfooAdapter{}
	run, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if run.Framework != FrameworkPromptfoo {
		t.Errorf("Framework = %q", run.Framework)
	}
	if run.Source != path {
		t.Errorf("Source = %q", run.Source)
	}
	if run.Timestamp != "2099-05-01T12:00:00Z" {
		t.Errorf("Timestamp = %q", run.Timestamp)
	}
	if len(run.Cases) != 2 {
		t.Fatalf("Cases = %d, want 2", len(run.Cases))
	}

	c1 := run.Cases[0]
	if c1.ID != "safety-1" || c1.Name != "Refuses harmful request" {
		t.Errorf("case 0 id/name: %q / %q", c1.ID, c1.Name)
	}
	if !c1.Success || c1.Score != 0.95 {
		t.Errorf("case 0 success/score: %v / %v", c1.Success, c1.Score)
	}
	if c1.Threshold != 0.8 {
		t.Errorf("case 0 threshold: %v", c1.Threshold)
	}
	if c1.Metrics["safety"] != 1.0 {
		t.Errorf("case 0 metrics.safety = %v", c1.Metrics["safety"])
	}

	c2 := run.Cases[1]
	if c2.Success || c2.Reason == "" {
		t.Errorf("case 1 should be failed with reason: %+v", c2)
	}
	if !contains(c2.Reason, "Did not refuse") {
		t.Errorf("case 1 reason: %q", c2.Reason)
	}

	if run.Stats.Total != 2 || run.Stats.Successes != 1 || run.Stats.Failures != 1 {
		t.Errorf("stats: %+v", run.Stats)
	}
	// PrimaryMetric should be the mean of 0.95 and 0.4 = 0.675.
	if run.Stats.PrimaryMetric < 0.674 || run.Stats.PrimaryMetric > 0.676 {
		t.Errorf("PrimaryMetric = %v, want ~0.675", run.Stats.PrimaryMetric)
	}
}

func TestPromptfooAdapter_Ingest_MissingFile(t *testing.T) {
	t.Parallel()
	a := PromptfooAdapter{}
	if _, err := a.Ingest("/nonexistent/path.json"); err == nil {
		t.Error("expected error on missing file")
	}
}

func TestPromptfooAdapter_Ingest_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	_ = os.WriteFile(path, []byte("not json {"), 0o644)
	a := PromptfooAdapter{}
	if _, err := a.Ingest(path); err == nil {
		t.Error("expected parse error")
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
