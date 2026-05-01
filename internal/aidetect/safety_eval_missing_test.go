package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestSafetyEvalMissing_FlagsUnprotectedPrompt(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/agent.py:promptBuilder", Name: "promptBuilder", Path: "src/agent.py", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "scenario:happy-1", Name: "happy path", Category: "happy_path",
				CoveredSurfaceIDs: []string{"surface:src/agent.py:promptBuilder"}},
		},
	}
	got := (&SafetyEvalMissingDetector{}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAISafetyEvalMissing {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q", got[0].Severity)
	}
}

func TestSafetyEvalMissing_AcceptsExplicitSafetyCategory(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/agent.py:p", Name: "p", Path: "src/agent.py", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "scenario:safety-1", Name: "jailbreak attempts", Category: "safety",
				CoveredSurfaceIDs: []string{"surface:src/agent.py:p"}},
		},
	}
	got := (&SafetyEvalMissingDetector{}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("expected no signals when safety scenario exists, got %d", len(got))
	}
}

func TestSafetyEvalMissing_AcceptsAdversarialAlias(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/agent.py:p", Name: "p", Path: "src/agent.py", Kind: models.SurfaceAgent},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "scenario:adv-1", Name: "adversarial inputs", Category: "adversarial",
				CoveredSurfaceIDs: []string{"surface:src/agent.py:p"}},
		},
	}
	got := (&SafetyEvalMissingDetector{}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("adversarial alias should pass, got %d signals", len(got))
	}
}

func TestSafetyEvalMissing_IgnoresNonSafetySurfaces(t *testing.T) {
	t.Parallel()

	// A regular function surface is not safety-critical and should
	// not fire even with no scenarios at all.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "surface:src/util.go:Sum", Name: "Sum", Path: "src/util.go", Kind: models.SurfaceFunction},
		},
	}
	got := (&SafetyEvalMissingDetector{}).Detect(snap)
	if len(got) != 0 {
		t.Errorf("regular function should not fire, got %d signals", len(got))
	}
}

func TestSafetyEvalMissing_FlagsPerSurface(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Name: "promptA", Path: "a.py", Kind: models.SurfacePrompt},
			{SurfaceID: "s2", Name: "agentB", Path: "b.py", Kind: models.SurfaceAgent},
			{SurfaceID: "s3", Name: "toolC", Path: "c.py", Kind: models.SurfaceToolDef},
		},
		Scenarios: []models.Scenario{
			{ScenarioID: "sc-safety", Name: "safety covers s1", Category: "safety",
				CoveredSurfaceIDs: []string{"s1"}},
		},
	}
	got := (&SafetyEvalMissingDetector{}).Detect(snap)
	// s1 is covered; s2 and s3 are not → 2 findings.
	if len(got) != 2 {
		t.Fatalf("got %d signals, want 2", len(got))
	}
}
