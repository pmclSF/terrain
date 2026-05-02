package signals

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
)

// RunDetectors runs all provided detectors against the snapshot and
// appends the resulting signals to snap.Signals.
//
// 0.2.0 final-polish: each detector now runs through `safeDetect` so
// a panic in one detector emits the standard `detectorPanic` sentinel
// instead of unwinding the call stack and tanking the rest of the
// pass. Pre-fix `RunDetectors` was the only signal-producing entry
// point that bypassed panic recovery — tests calling it directly
// (rather than via DetectorRegistry.Run) had no protection.
func RunDetectors(snap *models.TestSuiteSnapshot, detectors ...Detector) {
	for _, d := range detectors {
		// Synthesize a minimal registration so safeDetect's panic
		// message can name the detector. The Meta.ID is the Go type
		// name as seen by reflection's fmt-format; good enough for a
		// post-mortem hint.
		reg := DetectorRegistration{Meta: DetectorMeta{ID: detectorTypeName(d)}}
		found := safeDetect(reg, func() []models.Signal { return d.Detect(snap) })
		snap.Signals = append(snap.Signals, found...)
	}
}

// detectorTypeName returns a human-readable identifier for a Detector
// implementation. Used by RunDetectors when no DetectorRegistration
// is available, so the synthesized Meta.ID still names something
// useful in panic-recovery diagnostics.
func detectorTypeName(d Detector) string {
	if d == nil {
		return "<nil-detector>"
	}
	// Format `*pkg.TypeName` is what fmt's %T produces; we keep it
	// as-is rather than trying to strip pointer prefix because the
	// extra `*` is a useful hint that the registration likely
	// referred to a pointer receiver.
	return fmt.Sprintf("%T", d)
}
