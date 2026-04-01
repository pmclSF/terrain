package health

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/runtime"
)

// HealthDetector is the interface for runtime-backed health signal detectors.
//
// Unlike static detectors (which take *models.TestSuiteSnapshot), health
// detectors consume runtime test results and emit health signals.
// The RuntimeDetectorAdapter in internal/engine bridges this interface
// to the standard signals.Detector interface for registry integration.
type HealthDetector interface {
	Detect(results []runtime.TestResult) []models.Signal
}
