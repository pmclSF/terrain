package truthcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCalibration_Fixtures(t *testing.T) {
	// Find the repo root by walking up from the test file.
	root := findRepoRoot(t)

	fixtureDirs := []string{
		filepath.Join(root, "tests", "fixtures", "ai-prompt-only"),
		filepath.Join(root, "tests", "fixtures", "ai-mixed-traditional"),
		filepath.Join(root, "tests", "fixtures", "ai-rag-pipeline"),
		filepath.Join(root, "tests", "fixtures", "terrain-world"),
	}

	// Skip if fixtures aren't available (e.g., shallow clone).
	for _, dir := range fixtureDirs {
		truthPath := filepath.Join(dir, "tests", "truth", "terrain_truth.yaml")
		if _, err := os.Stat(truthPath); os.IsNotExist(err) {
			t.Skipf("fixture truth spec not found: %s", truthPath)
		}
	}

	result, err := RunCalibration(fixtureDirs)
	if err != nil {
		t.Fatalf("RunCalibration failed: %v", err)
	}

	if result.FixtureCount != 4 {
		t.Errorf("expected 4 fixtures, got %d", result.FixtureCount)
	}

	if result.TotalSurfaces == 0 {
		t.Error("expected TotalSurfaces > 0")
	}

	// At least one kind should be present.
	if len(result.ByKind) == 0 {
		t.Error("expected at least one kind in calibration results")
	}

	// Verify calibration report formats without error.
	report := FormatCalibrationReport(result)
	if len(report) == 0 {
		t.Error("expected non-empty calibration report")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}
