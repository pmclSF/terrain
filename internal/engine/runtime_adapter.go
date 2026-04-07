package engine

import (
	"github.com/pmclSF/terrain/internal/health"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/runtime"
)

// RuntimeDetectorAdapter bridges a health.HealthDetector (which takes
// []runtime.TestResult) to the signals.Detector interface (which takes
// *models.TestSuiteSnapshot).
//
// The adapter holds a pointer to the runtime results slice. When the
// pipeline ingests runtime artifacts, it populates this slice before
// the detector registry runs. If no runtime data is available, the
// adapter returns nil signals — the health detector is simply silent.
type RuntimeDetectorAdapter struct {
	Health  health.HealthDetector
	Results *[]runtime.TestResult
}

// Detect implements signals.Detector by delegating to the wrapped
// health detector with the runtime results.
func (a *RuntimeDetectorAdapter) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	if a.Results == nil || len(*a.Results) == 0 {
		return nil
	}
	return a.Health.Detect(*a.Results)
}
