package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/policy"
)

// ---------------------------------------------------------------------------
// policyConfigMap
// ---------------------------------------------------------------------------

func TestPolicyConfigMap_Nil(t *testing.T) {
	t.Parallel()
	if got := policyConfigMap(nil); got != nil {
		t.Errorf("expected nil for nil config, got %v", got)
	}
}

func TestPolicyConfigMap_EmptyRules(t *testing.T) {
	t.Parallel()
	cfg := &policy.Config{}
	if got := policyConfigMap(cfg); got != nil {
		t.Errorf("expected nil for empty rules, got %v", got)
	}
}

func TestPolicyConfigMap_PopulatesRules(t *testing.T) {
	t.Parallel()
	boolTrue := true
	maxRuntime := 10000.0
	minCov := 80.0
	maxWeak := 5
	maxMock := 3
	cfg := &policy.Config{
		Rules: policy.Rules{
			DisallowSkippedTests:   &boolTrue,
			DisallowFrameworks:     []string{"mocha", "jasmine"},
			MaxTestRuntimeMs:       &maxRuntime,
			MinimumCoveragePercent: &minCov,
			MaxWeakAssertions:      &maxWeak,
			MaxMockHeavyTests:      &maxMock,
		},
	}

	got := policyConfigMap(cfg)
	if got == nil {
		t.Fatal("expected non-nil config map")
	}
	rules, ok := got["rules"].(map[string]any)
	if !ok {
		t.Fatal("expected rules key with map value")
	}

	if rules["disallow_skipped_tests"] != true {
		t.Error("expected disallow_skipped_tests = true")
	}
	fws, ok := rules["disallow_frameworks"].([]string)
	if !ok || len(fws) != 2 {
		t.Errorf("expected 2 disallowed frameworks, got %v", rules["disallow_frameworks"])
	}
	// Verify frameworks are sorted.
	if fws[0] != "jasmine" || fws[1] != "mocha" {
		t.Errorf("expected sorted frameworks [jasmine, mocha], got %v", fws)
	}
	if rules["max_test_runtime_ms"] != 10000.0 {
		t.Errorf("max_test_runtime_ms = %v", rules["max_test_runtime_ms"])
	}
	if rules["minimum_coverage_percent"] != 80.0 {
		t.Errorf("minimum_coverage_percent = %v", rules["minimum_coverage_percent"])
	}
	if rules["max_weak_assertions"] != 5 {
		t.Errorf("max_weak_assertions = %v", rules["max_weak_assertions"])
	}
	if rules["max_mock_heavy_tests"] != 3 {
		t.Errorf("max_mock_heavy_tests = %v", rules["max_mock_heavy_tests"])
	}
}

func TestPolicyConfigMap_PartialRules(t *testing.T) {
	t.Parallel()
	maxRuntime := 5000.0
	cfg := &policy.Config{
		Rules: policy.Rules{
			MaxTestRuntimeMs: &maxRuntime,
		},
	}

	got := policyConfigMap(cfg)
	if got == nil {
		t.Fatal("expected non-nil for partial rules")
	}
	rules := got["rules"].(map[string]any)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

// ---------------------------------------------------------------------------
// ingestGauntletArtifacts
// ---------------------------------------------------------------------------

func TestIngestGauntletArtifacts_EmptyPaths(t *testing.T) {
	t.Parallel()
	arts, err := ingestGauntletArtifacts(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(arts))
	}
}

func TestIngestGauntletArtifacts_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := ingestGauntletArtifacts([]string{"/nonexistent/gauntlet.json"})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestIngestGauntletArtifacts_ValidArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	artPath := filepath.Join(dir, "gauntlet.json")
	art := map[string]any{
		"version":    "1",
		"provider":   "gauntlet",
		"timestamp":  "2099-01-01T00:00:00Z",
		"repository": "test/repo",
		"scenarios": []any{
			map[string]any{"scenarioId": "sc:test", "name": "test", "status": "passed", "durationMs": 100},
		},
		"summary": map[string]any{
			"total":    0,
			"passed":   0,
			"failed":   0,
			"skipped":  0,
			"duration": 0,
		},
	}
	data, _ := json.MarshalIndent(art, "", "  ")
	os.WriteFile(artPath, data, 0o644)

	arts, err := ingestGauntletArtifacts([]string{artPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(arts))
	}
}

// ---------------------------------------------------------------------------
// ingestGreatExpectationsArtifacts
// ---------------------------------------------------------------------------

func TestIngestGreatExpectationsArtifacts_ValidArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifactPath := filepath.Join(root, "evals", "ge-results.json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	data := []byte(`{
  "success": false,
  "meta": {
    "great_expectations_version": "0.18.0",
    "run_id": {"run_time": "2099-01-01T00:00:00Z"}
  },
  "statistics": {
    "evaluated_expectations": 2,
    "successful_expectations": 1,
    "unsuccessful_expectations": 1,
    "success_percent": 50.0
  },
  "results": [
    {
      "success": true,
      "expectation_config": {
        "expectation_type": "expect_table_row_count_to_be_between",
        "kwargs": {}
      }
    },
    {
      "success": false,
      "expectation_config": {
        "expectation_type": "expect_column_values_to_not_be_null",
        "kwargs": {"column": "email"}
      },
      "result": {
        "unexpected_count": 3,
        "unexpected_percent": 12.5
      }
    }
  ]
}`)
	if err := os.WriteFile(artifactPath, data, 0o644); err != nil {
		t.Fatalf("write GE artifact: %v", err)
	}

	envs, err := ingestGreatExpectationsArtifacts(root, []string{artifactPath})
	if err != nil {
		t.Fatalf("ingest GE artifact: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}
	env := envs[0]
	if env.Framework != "great_expectations" {
		t.Fatalf("framework = %q, want great_expectations", env.Framework)
	}
	if env.SourcePath != "evals/ge-results.json" {
		t.Fatalf("sourcePath = %q, want evals/ge-results.json", env.SourcePath)
	}
	if env.Aggregates.Successes != 1 || env.Aggregates.Failures != 1 {
		t.Fatalf("aggregates = %+v, want 1 success and 1 failure", env.Aggregates)
	}
	result, err := airun.ParseEvalRunPayload(env)
	if err != nil {
		t.Fatalf("parse envelope payload: %v", err)
	}
	if len(result.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(result.Cases))
	}
	failed := result.Cases[1]
	if failed.CaseID != "expect_column_values_to_not_be_null:email" {
		t.Fatalf("failed case ID = %q", failed.CaseID)
	}
	if failed.Success {
		t.Fatal("expected second GE expectation to fail")
	}
	if failed.Score != 0 {
		t.Fatalf("failed score = %v, want 0", failed.Score)
	}
	if failed.FailureReason == "" {
		t.Fatal("expected failure reason from unexpected_count metadata")
	}
}

func TestIngestGreatExpectationsArtifacts_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := ingestGreatExpectationsArtifacts(t.TempDir(), []string{"/nonexistent/ge.json"})
	if err == nil {
		t.Fatal("expected error for nonexistent GE artifact")
	}
}

// ---------------------------------------------------------------------------
// ingestCoverageArtifacts
// ---------------------------------------------------------------------------

func TestIngestCoverageArtifacts_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := ingestCoverageArtifacts(context.Background(), "/nonexistent/cov", "")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestIngestCoverageArtifacts_LCOVFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lcovPath := filepath.Join(dir, "lcov.info")
	os.WriteFile(lcovPath, []byte("TN:\nSF:src/auth.ts\nDA:1,1\nDA:2,0\nend_of_record\n"), 0o644)

	arts, err := ingestCoverageArtifacts(context.Background(), lcovPath, "unit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arts) != 1 {
		t.Errorf("expected 1 coverage artifact, got %d", len(arts))
	}
}

func TestIngestCoverageArtifacts_Directory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lcov.info"), []byte("TN:\nSF:src/a.ts\nDA:1,1\nend_of_record\n"), 0o644)

	arts, err := ingestCoverageArtifacts(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arts) == 0 {
		t.Error("expected at least 1 coverage artifact from directory")
	}
}

func TestIngestCoverageArtifacts_CancelledContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lcovPath := filepath.Join(dir, "lcov.info")
	os.WriteFile(lcovPath, []byte("TN:\nSF:src/a.ts\nDA:1,1\nend_of_record\n"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := ingestCoverageArtifacts(ctx, lcovPath, "")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

// ---------------------------------------------------------------------------
// validatePipelineOptions
// ---------------------------------------------------------------------------

func TestValidatePipelineOptions_InvalidRunLabel(t *testing.T) {
	t.Parallel()
	err := validatePipelineOptions(".", PipelineOptions{CoverageRunLabel: "foobar"})
	if err == nil {
		t.Fatal("expected error for invalid run label")
	}
}

func TestValidatePipelineOptions_ValidRunLabels(t *testing.T) {
	t.Parallel()
	for _, label := range []string{"unit", "integration", "e2e", "Unit", "E2E", ""} {
		err := validatePipelineOptions(".", PipelineOptions{CoverageRunLabel: label})
		if err != nil {
			t.Errorf("unexpected error for label %q: %v", label, err)
		}
	}
}

func TestValidatePipelineOptions_NegativeThreshold(t *testing.T) {
	t.Parallel()
	err := validatePipelineOptions(".", PipelineOptions{SlowTestThresholdMs: -1})
	if err == nil {
		t.Fatal("expected error for negative threshold")
	}
}

func TestValidatePipelineOptions_EmptyRoot(t *testing.T) {
	t.Parallel()
	err := validatePipelineOptions("", PipelineOptions{})
	if err == nil {
		t.Fatal("expected error for empty root")
	}
}
