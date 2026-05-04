package signals

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// noopDetector returns one signal so we can distinguish "ran" from
// "missing-input diagnostic emitted" in test output.
type noopDetector struct{}

func (noopDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return []models.Signal{{
		Type:        models.SignalType("test.ran"),
		Category:    models.CategoryQuality,
		Severity:    models.SeverityLow,
		Confidence:  1.0,
		Explanation: "noop detector executed",
	}}
}

// TestSafeDetectChecked_RunsWhenInputsPresent verifies the happy
// path: a detector whose inputs are satisfied runs normally and
// returns its own signals (no missing-input diagnostic).
func TestSafeDetectChecked_RunsWhenInputsPresent(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:              "test.no-required-inputs",
			Domain:          DomainQuality,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{}

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 || got[0].Type != "test.ran" {
		t.Errorf("expected detector to run; got: %+v", got)
	}
}

// TestSafeDetectChecked_MissingRuntime emits the diagnostic when
// RequiresRuntime is set but the snapshot has no runtime stats.
func TestSafeDetectChecked_MissingRuntime(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:              "test.needs-runtime",
			Domain:          DomainHealth,
			RequiresRuntime: true,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{
		// Test files present but no RuntimeStats on any of them.
		TestFiles: []models.TestFile{{Path: "a.test.ts"}},
	}

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(got))
	}
	if got[0].Type != signalTypeMissingInputDiagnostic {
		t.Errorf("Type = %q, want detectorMissingInput", got[0].Type)
	}
	if !strings.Contains(got[0].Explanation, "test.needs-runtime") {
		t.Errorf("explanation should name the detector: %q", got[0].Explanation)
	}
	if !strings.Contains(got[0].Explanation, "--runtime") {
		t.Errorf("explanation should name the missing flag: %q", got[0].Explanation)
	}
}

// TestSafeDetectChecked_RuntimePresent runs the detector when at
// least one test file has runtime stats — meeting the
// RequiresRuntime contract.
func TestSafeDetectChecked_RuntimePresent(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:              "test.needs-runtime",
			Domain:          DomainHealth,
			RequiresRuntime: true,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "a.test.ts", RuntimeStats: &models.RuntimeStats{PassRate: 1.0}},
		},
	}

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 || got[0].Type != "test.ran" {
		t.Errorf("expected detector to run with runtime present; got: %+v", got)
	}
}

// TestSafeDetectChecked_MissingBaseline emits the diagnostic when
// RequiresBaseline is set but Baseline is nil.
func TestSafeDetectChecked_MissingBaseline(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:               "test.needs-baseline",
			Domain:           DomainAI,
			RequiresBaseline: true,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{} // Baseline nil

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 || got[0].Type != signalTypeMissingInputDiagnostic {
		t.Errorf("expected missing-input diagnostic, got: %+v", got)
	}
	if !strings.Contains(got[0].Explanation, "--baseline") {
		t.Errorf("explanation should name --baseline: %q", got[0].Explanation)
	}
}

// TestSafeDetectChecked_MissingEvalArtifact emits the diagnostic when
// RequiresEvalArtifact is set but EvalRuns is empty.
func TestSafeDetectChecked_MissingEvalArtifact(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:                   "test.needs-eval",
			Domain:               DomainAI,
			RequiresEvalArtifact: true,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{} // EvalRuns nil

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 || got[0].Type != signalTypeMissingInputDiagnostic {
		t.Errorf("expected missing-input diagnostic, got: %+v", got)
	}
	if !strings.Contains(got[0].Explanation, "promptfoo-results") {
		t.Errorf("explanation should mention promptfoo-results / deepeval-results / ragas-results: %q",
			got[0].Explanation)
	}
}

// TestSafeDetectChecked_MultipleMissingInputs emits one diagnostic
// listing all missing inputs (Oxford-comma joined). Adopters who
// run analyze without runtime AND baseline AND eval data should see
// one diagnostic per affected detector citing all three needs.
func TestSafeDetectChecked_MultipleMissingInputs(t *testing.T) {
	t.Parallel()
	reg := DetectorRegistration{
		Meta: DetectorMeta{
			ID:                   "test.needs-everything",
			Domain:               DomainAI,
			RequiresRuntime:      true,
			RequiresBaseline:     true,
			RequiresEvalArtifact: true,
		},
		Detector: noopDetector{},
	}
	snap := &models.TestSuiteSnapshot{}

	got := safeDetectChecked(reg, snap, func() []models.Signal {
		return reg.Detector.Detect(snap)
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic listing all missing inputs, got %d", len(got))
	}
	for _, marker := range []string{"--runtime", "--baseline", "promptfoo-results"} {
		if !strings.Contains(got[0].Explanation, marker) {
			t.Errorf("explanation missing %q: %q", marker, got[0].Explanation)
		}
	}
}

// TestJoinInputNames covers the Oxford-comma formatting for 1, 2,
// and 3+ inputs.
func TestJoinInputNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"runtime"}, "runtime"},
		{[]string{"runtime", "baseline"}, "runtime and baseline"},
		{[]string{"runtime", "baseline", "eval"}, "runtime, baseline, and eval"},
		{[]string{"a", "b", "c", "d"}, "a, b, c, and d"},
	}
	for _, tt := range tests {
		if got := joinInputNames(tt.in); got != tt.want {
			t.Errorf("joinInputNames(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
