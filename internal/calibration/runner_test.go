package calibration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestRunner_MatchesExpectedSignals exercises the full happy path:
// load labels, run a stub analyser, compute TP/FP/FN.
func TestRunner_MatchesExpectedSignals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLabels(t, dir, `
schemaVersion: 1
fixture: test-fixture
description: integration test fixture
expected:
  - type: weakAssertion
    file: src/auth.test.js
    notes: uses toBeTruthy
  - type: skippedTest
    file: src/db.test.js
    notes: skipped without ticket
expectedAbsent:
  - type: aiHardcodedAPIKey
    file: src/auth.test.js
    notes: placeholder string, not a real key
`)

	stub := func(string) ([]models.Signal, error) {
		return []models.Signal{
			// Match the first expected.
			{Type: "weakAssertion", Location: models.SignalLocation{File: "src/auth.test.js"}},
			// Match the false-positive guard → counts as FP.
			{Type: "aiHardcodedAPIKey", Location: models.SignalLocation{File: "src/auth.test.js"}},
			// Out-of-scope — corpus doesn't label flakyTest, silent.
			{Type: "flakyTest", Location: models.SignalLocation{File: "src/queue.test.js"}},
			// skippedTest is missing → FN.
		}, nil
	}

	result, err := Run(dir, stub)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(result.Fixtures))
	}
	counts := result.Fixtures[0].CountByOutcome()
	if counts[OutcomeTruePositive] != 1 {
		t.Errorf("TP = %d, want 1", counts[OutcomeTruePositive])
	}
	if counts[OutcomeFalsePositive] != 1 {
		t.Errorf("FP = %d, want 1", counts[OutcomeFalsePositive])
	}
	if counts[OutcomeFalseNegative] != 1 {
		t.Errorf("FN = %d, want 1", counts[OutcomeFalseNegative])
	}

	prec := result.PrecisionByType()
	if prec["weakAssertion"] != 1.0 {
		t.Errorf("precision[weakAssertion] = %v, want 1.0", prec["weakAssertion"])
	}
	if prec["aiHardcodedAPIKey"] != 0.0 {
		t.Errorf("precision[aiHardcodedAPIKey] = %v, want 0.0", prec["aiHardcodedAPIKey"])
	}
	rec := result.RecallByType()
	if rec["skippedTest"] != 0.0 {
		t.Errorf("recall[skippedTest] = %v, want 0.0", rec["skippedTest"])
	}
}

// TestRunner_PrecisionInterval gives a non-degenerate Wilson interval
// when the corpus has measurable data, and an empty map for detectors
// with no positive samples.
func TestRunner_PrecisionInterval(t *testing.T) {
	t.Parallel()

	c := CorpusResult{
		TP: map[models.SignalType]int{"weakAssertion": 19},
		FP: map[models.SignalType]int{"weakAssertion": 1},
		FN: map[models.SignalType]int{"weakAssertion": 0},
	}
	intervals := c.PrecisionByTypeInterval()
	mi, ok := intervals["weakAssertion"]
	if !ok {
		t.Fatal("expected interval for weakAssertion")
	}
	if mi.Value < 0.93 || mi.Value > 0.96 {
		t.Errorf("Value = %.3f, want ~0.95", mi.Value)
	}
	if mi.IntervalLow >= mi.Value || mi.IntervalHigh <= mi.Value {
		t.Errorf("interval [%.3f, %.3f] does not bracket Value %.3f",
			mi.IntervalLow, mi.IntervalHigh, mi.Value)
	}
	// No samples → omitted from result.
	c2 := CorpusResult{TP: map[models.SignalType]int{}, FP: map[models.SignalType]int{}}
	if got := c2.PrecisionByTypeInterval(); len(got) != 0 {
		t.Errorf("expected empty result for empty corpus, got %d entries", len(got))
	}
}

// TestRunner_NoFixtures returns an empty corpus result without error.
func TestRunner_NoFixtures(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stub := func(string) ([]models.Signal, error) { return nil, nil }
	result, err := Run(dir, stub)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Fixtures) != 0 {
		t.Errorf("expected 0 fixtures, got %d", len(result.Fixtures))
	}
}

// TestLoadLabels_RejectsBadSchemaVersion guards against silently
// honouring an old or new label format.
func TestLoadLabels_RejectsBadSchemaVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLabels(t, dir, `
schemaVersion: 99
fixture: bogus
expected: []
`)
	if _, err := LoadLabels(dir); err == nil {
		t.Error("expected schemaVersion 99 to be rejected")
	}
}

// TestLoadLabels_RejectsEmptyFixture protects report rendering from
// "fixture:" key with a blank value.
func TestLoadLabels_RejectsEmptyFixture(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLabels(t, dir, `
schemaVersion: 1
fixture: ""
expected: []
`)
	if _, err := LoadLabels(dir); err == nil {
		t.Error("expected empty fixture name to be rejected")
	}
}

func writeLabels(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "labels.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write labels: %v", err)
	}
}
