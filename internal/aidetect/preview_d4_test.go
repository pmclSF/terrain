package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestPromptWithoutTemperatureDetector_NoPanic: the detector must not panic.
// It previously passed a nil context.Context to DetectContext, whose first
// action is ctx.Err() — a nil-interface dereference that panicked on every run
// and surfaced as a spurious Critical detectorPanic finding. Detect now passes
// context.Background(); this test fails (panics) if the regression returns.
func TestPromptWithoutTemperatureDetector_NoPanic(t *testing.T) {
	t.Parallel()
	d := &PromptWithoutTemperatureDetector{Root: t.TempDir()}
	// Empty repo → no AI call sites → nil result, and crucially no panic.
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Errorf("empty repo should yield no signals, got %d", len(got))
	}
}
