package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestAblation_LLMCallSiteFiresUncoveredAISurface is the durable
// regression-test version of the Track 2 ablation experiment that
// surfaced on 2026-05-12: when a new LLM-using source file is added
// without eval coverage, the analyzer must produce an
// `uncoveredAISurface` signal on that file.
//
// The original experiment is in scripts/track2-ablation.sh. This test
// makes that guarantee durable — future detector refactors can't
// silently break the ablation-confirmed behavior.
func TestAblation_LLMCallSiteFiresUncoveredAISurface(t *testing.T) {
	t.Parallel()

	// Hand-built snapshot mimicking what the analyzer would produce for
	// a TS file that imports OpenAI and makes an LLM call WITHOUT a
	// covering eval. The ablation harness creates the file on disk;
	// here we exercise the detector directly with the expected surface.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{
				SurfaceID: "surface-ablation",
				Path:      "src/ablation-test-callsite.ts",
				Name:      "ablationCall",
				Kind:      models.SurfacePrompt,
			},
		},
		// No Evals — surface is uncovered.
	}

	det := &PromptFileMissingEvalDetector{}
	emitted := det.Detect(snap)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 prompt-file-missing-eval signal, got %d", len(emitted))
	}
	if emitted[0].Type != signals.SignalPromptFileMissingEval {
		t.Errorf("expected signal type %q, got %q", signals.SignalPromptFileMissingEval, emitted[0].Type)
	}
	if emitted[0].Location.File != "src/ablation-test-callsite.ts" {
		t.Errorf("expected signal on ablation file, got %q", emitted[0].Location.File)
	}
}

// TestAblation_CoveredSurfaceDoesNotFire ensures the detector stays
// silent when an eval covers the surface. This is the negative case —
// equally important as the positive case for false-positive prevention.
func TestAblation_CoveredSurfaceDoesNotFire(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{
				SurfaceID: "surface-covered",
				Path:      "src/covered-callsite.ts",
				Name:      "coveredCall",
				Kind:      models.SurfacePrompt,
			},
		},
		Evals: []models.Eval{
			{
				EvalID:            "eval-1",
				Path:              "evals/covered.yaml",
				CoveredSurfaceIDs: []string{"surface-covered"},
			},
		},
	}

	det := &PromptFileMissingEvalDetector{}
	emitted := det.Detect(snap)
	if len(emitted) != 0 {
		t.Errorf("expected 0 signals on covered surface, got %d", len(emitted))
	}
}

// TestAblation_NonPromptKindIgnored verifies the detector only fires on
// the AI surface kinds in aiSurfaceKinds — not on every CodeSurface.
// Per Track 2 ablation finding, the detector scope is narrower than the
// "AI surfaces without eval coverage" framing implies.
func TestAblation_NonPromptKindIgnored(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{
				SurfaceID: "data-only",
				Path:      "data/dataset.csv",
				Name:      "trainingData",
				Kind:      models.SurfaceDataset, // NOT in aiSurfaceKinds
			},
		},
	}
	det := &PromptFileMissingEvalDetector{}
	if got := det.Detect(snap); len(got) != 0 {
		t.Errorf("expected 0 signals for non-aiSurfaceKind, got %d", len(got))
	}
}

// TestAblation_HarnessFixtureExists is a smoke check that the Track 2
// ablation script still exists and is executable. Future refactors that
// delete or rename it will trip this test, prompting the developer to
// also update this integration test suite.
func TestAblation_HarnessFixtureExists(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up to repo root from internal/aidetect.
	root := filepath.Join(wd, "..", "..")
	script := filepath.Join(root, "scripts", "track2-ablation.sh")
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("scripts/track2-ablation.sh missing — Track 2 ablation harness deleted? err=%v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("scripts/track2-ablation.sh exists but is not executable (mode=%v)", info.Mode())
	}
}
