package truthcheck

import (
	"fmt"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/engine"
)

// RunCalibration runs the analysis pipeline against each fixture directory,
// loads its terrain_truth.yaml, and computes calibration metrics.
func RunCalibration(fixtureDirs []string) (*CalibrationResult, error) {
	var inputs []CalibrationInput

	for _, dir := range fixtureDirs {
		truthPath := filepath.Join(dir, "tests", "truth", "terrain_truth.yaml")
		spec, err := LoadTruthSpec(truthPath)
		if err != nil {
			return nil, fmt.Errorf("load truth spec for %s: %w", dir, err)
		}

		result, err := engine.RunPipeline(dir)
		if err != nil {
			return nil, fmt.Errorf("run pipeline on %s: %w", dir, err)
		}

		inputs = append(inputs, CalibrationInput{
			Name:     filepath.Base(dir),
			Snapshot: result.Snapshot,
			Truth:    spec,
		})
	}

	return CalibrateFromFixtures(inputs), nil
}
