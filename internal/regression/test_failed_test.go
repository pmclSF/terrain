package regression

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestDetectTestFailed_FiresOnFailures(t *testing.T) {
	t.Parallel()
	results := []TestResult{
		{Path: "tests/a_test.go", Name: "TestA", Passed: true},
		{Path: "tests/b_test.go", Name: "TestB", Passed: false, FailureMessage: "expected 1, got 2"},
	}
	sigs := DetectTestFailed(results)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Location.Symbol != "TestB" {
		t.Errorf("name = %q", sigs[0].Location.Symbol)
	}
}

func TestDetectTestFailed_ExactImpactRaisesSeverity(t *testing.T) {
	t.Parallel()
	results := []TestResult{
		{Path: "tests/x_test.go", Name: "TestX", Passed: false, ImpactConfidence: "exact"},
	}
	sigs := DetectTestFailed(results)
	if len(sigs) != 1 {
		t.Fatal("expected 1 signal")
	}
	if sigs[0].Severity != models.SeverityCritical {
		t.Errorf("severity = %q, want critical (exact-impact)", sigs[0].Severity)
	}
}

func TestDetectTestFailed_LongMessageTruncated(t *testing.T) {
	t.Parallel()
	long := ""
	for i := 0; i < 500; i++ {
		long += "x"
	}
	results := []TestResult{
		{Path: "tests/x_test.go", Name: "TestX", Passed: false, FailureMessage: long},
	}
	sigs := DetectTestFailed(results)
	if len(sigs) != 1 {
		t.Fatal("expected 1 signal")
	}
	if len(sigs[0].Explanation) > 400 {
		t.Errorf("explanation not truncated: %d chars", len(sigs[0].Explanation))
	}
}
