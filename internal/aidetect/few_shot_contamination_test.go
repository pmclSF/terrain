package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func writeFewShotPrompt(t *testing.T, root, rel, content string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestFewShotContamination_FiresOnVerbatimOverlap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFewShotPrompt(t, root, "prompts/classifier.yaml", `
role: system
content: |
  You are a classifier.

  Examples:
  Input: The customer reports the device overheats during gameplay sessions
  Output: hardware-issue

  Input: The order shipped to the wrong address last week
  Output: shipping-issue
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "classifier", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "scenario:1",
				Name:              "device overheats",
				Description:       "The customer reports the device overheats during gameplay sessions",
				CoveredSurfaceIDs: []string{"s1"},
			},
		},
	}
	got := (&FewShotContaminationDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAIFewShotContamination {
		t.Errorf("type = %q", got[0].Type)
	}
}

func TestFewShotContamination_StaysQuietBelowThreshold(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFewShotPrompt(t, root, "prompts/classifier.yaml", `
role: system
content: |
  Classify the input.
`)
	// Description is short ("happy path") — under default threshold.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "classifier", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "scenario:1",
				Name:              "happy path",
				Description:       "happy path",
				CoveredSurfaceIDs: []string{"s1"},
			},
		},
	}
	if got := (&FewShotContaminationDetector{}).Detect(snap); len(got) != 0 {
		t.Errorf("short scenario description should not fire, got %d", len(got))
	}
}

func TestFewShotContamination_NoOverlap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFewShotPrompt(t, root, "prompts/classifier.yaml", `
role: system
content: |
  Examples:
  Input: alpha bravo charlie delta echo foxtrot golf hotel india juliet
  Output: phonetic
`)
	// Scenario uses different long-enough text — no overlap.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "classifier", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:        "scenario:1",
				Name:              "kilo lima",
				Description:       "kilo lima mike november oscar papa quebec romeo sierra tango",
				CoveredSurfaceIDs: []string{"s1"},
			},
		},
	}
	if got := (&FewShotContaminationDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("disjoint texts should not fire, got %d", len(got))
	}
}

// TestFewShotContamination_FiresOnImplicitCoverage_AutoDerivedScenario
// locks in the 0.2.0 final-polish fix: pre-fix, a scenario with empty
// `CoveredSurfaceIDs` (the default for auto-derived scenarios — the
// dominant shape in the wild) silently disabled the detector. The fix
// adds path-based implicit coverage (matching the same pattern
// aiSafetyEvalMissing already uses). The detector should fire when the
// scenario file and prompt file share a top-level directory, OR when
// the scenario has no Path at all (whole-repo fallback).
func TestFewShotContamination_FiresOnImplicitCoverage_AutoDerivedScenario(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFewShotPrompt(t, root, "prompts/classifier.yaml", `
role: system
content: |
  The customer reports the device overheats during gameplay sessions.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "classifier", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:  "scenario:1",
				Name:        "device overheats",
				Description: "The customer reports the device overheats during gameplay sessions",
				// CoveredSurfaceIDs intentionally empty (auto-derived shape).
				// Path empty too → whole-repo fallback should apply.
			},
		},
	}
	got := (&FewShotContaminationDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("auto-derived scenario should fire under implicit coverage, got %d", len(got))
	}
	if got[0].Type != signals.SignalAIFewShotContamination {
		t.Errorf("type = %q", got[0].Type)
	}
}

// TestFewShotContamination_ImplicitCoverage_RespectsTopLevelDir
// verifies that when a scenario DOES have a Path, only prompts under
// the same top-level directory are checked — not prompts in unrelated
// subprojects. Without this scope, a scenario in `service-a/` could
// match a prompt in `service-b/`, generating cross-project noise.
func TestFewShotContamination_ImplicitCoverage_RespectsTopLevelDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Prompt in service-b — should NOT be matched against a scenario
	// rooted in service-a, even though the text overlaps.
	relB := writeFewShotPrompt(t, root, "service-b/prompts/classifier.yaml", `
role: system
content: |
  The customer reports the device overheats during gameplay sessions.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: relB, Name: "classifier", Kind: models.SurfacePrompt},
		},
		Scenarios: []models.Scenario{
			{
				ScenarioID:  "scenario:1",
				Name:        "device overheats",
				Description: "The customer reports the device overheats during gameplay sessions",
				Path:        "service-a/scenarios/overheat.yaml",
				// CoveredSurfaceIDs empty; implicit coverage should
				// scope to service-a/* prompts only.
			},
		},
	}
	if got := (&FewShotContaminationDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("scenario in service-a should not match prompt in service-b under implicit coverage, got %d", len(got))
	}
}
