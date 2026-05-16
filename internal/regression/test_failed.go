package regression

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestResult is the normalized shape a test-runner emits per case.
// Sourced from JUnit XML parsing in internal/junit/ (or equivalent
// for jest --json, pytest --junitxml, go test -json).
type TestResult struct {
	// Path is the repo-relative path to the test file.
	Path string

	// Name is the test case identifier (e.g., "test_summarize_refusal"
	// or "describe>it" composite).
	Name string

	// Suite is the containing class / describe block when applicable.
	Suite string

	// Passed is true when the test succeeded.
	Passed bool

	// FailureMessage carries the assertion error / exception text.
	FailureMessage string

	// SystemErr / SystemOut capture stderr / stdout for diagnostic context.
	SystemErr string
	SystemOut string

	// DurationMs is the wall-clock duration in milliseconds.
	DurationMs int64

	// ImpactedBy lists the changed files / units that caused this test
	// to be selected. Populated by the impact engine; empty when the
	// runner emits test results outside of an impact-driven run.
	ImpactedBy []string

	// ImpactConfidence is the confidence tier of the impact-edge match
	// (exact / inferred). Mirrors impact.ImpactConfidence.
	ImpactConfidence string
}

// DetectTestFailed emits a Signal for every failing TestResult.
// Implements terrain/regression/test-failed.
//
// The detector consumes whatever the runtime layer produces — JUnit
// XML, jest --json, go test -json — through the shared TestResult
// shape. Per-case parameterized enumeration is the runner's job;
// this detector reports what it's given.
//
// Severity defaults to high; an impact-confidence-exact match boosts
// to critical because the failure is causally tied to the diff.
func DetectTestFailed(results []TestResult) []models.Signal {
	var out []models.Signal
	for _, r := range results {
		if r.Passed {
			continue
		}
		out = append(out, buildTestFailedSignal(r))
	}
	return out
}

func buildTestFailedSignal(r TestResult) models.Signal {
	severity := models.SeverityHigh
	if r.ImpactConfidence == "exact" {
		severity = models.SeverityCritical
	}

	displayName := r.Name
	if r.Suite != "" {
		displayName = r.Suite + " > " + r.Name
	}

	explanation := fmt.Sprintf("Test %q failed.", displayName)
	if r.FailureMessage != "" {
		explanation = fmt.Sprintf("Test %q failed: %s", displayName, truncate(r.FailureMessage, 240))
	}

	suggested := fmt.Sprintf(
		"Reproduce locally with `terrain test --selector regression/test-failed --filter %q`. Fix the failure or update the test deliberately if the new behavior is intended.",
		r.Name,
	)

	return models.Signal{
		Type:             signals.SignalTestFailed,
		Category:         models.CategoryHealth,
		Severity:         severity,
		Confidence:       1.0,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceRuntime,
		Location: models.SignalLocation{
			File:   r.Path,
			Symbol: r.Name,
		},
		Explanation:     explanation,
		SuggestedAction: suggested,
		RuleID:          "terrain/regression/test-failed",
		RuleURI:         "docs/rules/regression/test-failed.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"suite":            r.Suite,
			"failureMessage":   r.FailureMessage,
			"durationMs":       r.DurationMs,
			"impactedBy":       r.ImpactedBy,
			"impactConfidence": r.ImpactConfidence,
		},
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
