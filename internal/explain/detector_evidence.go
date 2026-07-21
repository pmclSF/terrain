package explain

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// Loads the embedded per-detector confidence labels that `terrain explain`
// surfaces alongside a finding.

//go:embed data/detector-evidence.json
var detectorEvidenceJSON []byte

// DetectorEvidence is the embedded JSON's top-level shape.
type DetectorEvidence struct {
	SchemaVersion string                           `json:"schema_version"`
	Detectors     map[string]DetectorEvidenceEntry `json:"detectors"`
}

// DetectorEvidenceEntry holds the confidence signal for one detector.
type DetectorEvidenceEntry struct {
	GlobalLift *LiftCI `json:"global_lift,omitempty"`
}

// LiftCI is the confidence interval that drives the qualitative label.
type LiftCI struct {
	Low95  float64 `json:"low_95"`
	High95 float64 `json:"high_95"`
}

var (
	evidenceOnce sync.Once
	evidenceData *DetectorEvidence
	evidenceErr  error
)

func loadDetectorEvidence() (*DetectorEvidence, error) {
	evidenceOnce.Do(func() {
		var d DetectorEvidence
		if err := json.Unmarshal(detectorEvidenceJSON, &d); err != nil {
			evidenceErr = err
			return
		}
		evidenceData = &d
	})
	return evidenceData, evidenceErr
}

// DetectorEvidenceFor returns the confidence entry for the given detector
// type, or nil if the detector has none.
func DetectorEvidenceFor(detectorType string) *DetectorEvidenceEntry {
	d, err := loadDetectorEvidence()
	if err != nil || d == nil {
		return nil
	}
	if e, ok := d.Detectors[detectorType]; ok {
		return &e
	}
	return nil
}

// FormatTrustLine renders a one-line qualitative confidence label for
// `terrain explain`, derived from the detector's confidence interval. The
// ladder is one-way and mirrors the severity ladder: a lower bound above 1
// reads as confident, an interval that dips at or below 1 reads as advisory.
// Returns "" when there is no confidence interval to report.
func (e *DetectorEvidenceEntry) FormatTrustLine() string {
	if e == nil || e.GlobalLift == nil {
		return ""
	}
	switch {
	case e.GlobalLift.Low95 > 1.5:
		return "High-confidence detector — findings have been reliable in practice."
	case e.GlobalLift.Low95 > 1.0:
		return "Moderate-confidence detector — findings are usually actionable."
	case e.GlobalLift.High95 >= 1.0:
		return "Directional detector — treat findings as advisory."
	default:
		return "Experimental detector — advisory only."
	}
}
