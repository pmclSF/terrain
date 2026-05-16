package evaladapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGEAdapter_CanIngest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "ge_validation.json")
	_ = os.WriteFile(good, []byte(`{
  "success": true,
  "meta": {"great_expectations_version": "0.18.4"},
  "statistics": {
    "evaluated_expectations": 3,
    "successful_expectations": 3,
    "unsuccessful_expectations": 0,
    "success_percent": 100
  },
  "results": [
    {
      "success": true,
      "expectation_config": {
        "expectation_type": "expect_column_values_to_not_be_null",
        "kwargs": {"column": "user_id"}
      }
    }
  ]
}`), 0o644)

	a := GreatExpectationsAdapter{}
	if !a.CanIngest(good) {
		t.Error("expected CanIngest=true on GE validation result")
	}

	bad := filepath.Join(dir, "promptfoo.json")
	_ = os.WriteFile(bad, []byte(`{"evalId": "x", "results": {"version": 3, "results": [], "stats": {}}}`), 0o644)
	if a.CanIngest(bad) {
		t.Error("expected CanIngest=false on promptfoo JSON")
	}
}

func TestGEAdapter_Ingest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "validation.json")
	_ = os.WriteFile(path, []byte(`{
  "success": false,
  "meta": {
    "great_expectations_version": "0.18.4",
    "run_id": {"run_time": "2026-05-01T12:00:00Z"}
  },
  "statistics": {
    "evaluated_expectations": 3,
    "successful_expectations": 2,
    "unsuccessful_expectations": 1,
    "success_percent": 66.67
  },
  "results": [
    {
      "success": true,
      "expectation_config": {
        "expectation_type": "expect_column_values_to_not_be_null",
        "kwargs": {"column": "user_id"}
      }
    },
    {
      "success": true,
      "expectation_config": {
        "expectation_type": "expect_table_row_count_to_be_between",
        "kwargs": {"min_value": 1, "max_value": 1000}
      }
    },
    {
      "success": false,
      "expectation_config": {
        "expectation_type": "expect_column_values_to_be_in_set",
        "kwargs": {"column": "status", "value_set": ["active", "pending"]}
      },
      "result": {
        "unexpected_count": 42,
        "unexpected_percent": 4.2
      }
    }
  ]
}`), 0o644)

	a := GreatExpectationsAdapter{}
	run, err := a.Ingest(path)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if run.Framework != FrameworkGreatExpectations {
		t.Errorf("framework = %q", run.Framework)
	}
	if run.Timestamp != "2026-05-01T12:00:00Z" {
		t.Errorf("timestamp = %q", run.Timestamp)
	}
	if len(run.Cases) != 3 {
		t.Fatalf("cases = %d, want 3", len(run.Cases))
	}
	if run.Cases[0].ID != "expect_column_values_to_not_be_null:user_id" {
		t.Errorf("case 0 id = %q", run.Cases[0].ID)
	}
	if !run.Cases[0].Success || run.Cases[0].Score != 1.0 {
		t.Errorf("case 0 success/score: %v / %v", run.Cases[0].Success, run.Cases[0].Score)
	}
	if run.Cases[2].Success || run.Cases[2].Score != 0.0 {
		t.Errorf("case 2 should fail with score 0: %+v", run.Cases[2])
	}
	if !contains(run.Cases[2].Reason, "unexpected_count=42") {
		t.Errorf("case 2 reason: %q", run.Cases[2].Reason)
	}

	if run.Stats.Total != 3 || run.Stats.Successes != 2 || run.Stats.Failures != 1 {
		t.Errorf("stats: %+v", run.Stats)
	}
	// PrimaryMetric = pass rate = 2/3 ≈ 0.6667
	if run.Stats.PrimaryMetric < 0.66 || run.Stats.PrimaryMetric > 0.67 {
		t.Errorf("PrimaryMetric = %v, want ~0.667", run.Stats.PrimaryMetric)
	}
}
