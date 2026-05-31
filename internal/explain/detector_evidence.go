package explain

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// Per-detector validation evidence (precision floors, PR-lift,
// recall, validated samples) shipped embedded in the binary so
// `terrain explain` can surface a trust line alongside each finding.

//go:embed data/detector-evidence.json
var detectorEvidenceJSON []byte

// DetectorEvidence is the embedded JSON's top-level shape.
type DetectorEvidence struct {
	SchemaVersion string                           `json:"schema_version"`
	GeneratedAt   string                           `json:"generated_at"`
	Methodology   string                           `json:"methodology"`
	Detectors     map[string]DetectorEvidenceEntry `json:"detectors"`
}

// DetectorEvidenceEntry is the corpus evidence for one detector.
type DetectorEvidenceEntry struct {
	HeuristicPrecision *HeuristicPrecision         `json:"heuristic_precision,omitempty"`
	GlobalLift         *LiftMeasurement            `json:"global_lift,omitempty"`
	Recall             *RecallMeasurement          `json:"recall,omitempty"`
	PerCorpusLift      map[string]*LiftMeasurement `json:"per_corpus_lift,omitempty"`
	HandValidated      *HandValidatedSample        `json:"hand_validated,omitempty"`
}

type HeuristicPrecision struct {
	Point      float64 `json:"point"`
	Low95      float64 `json:"low_95"`
	High95     float64 `json:"high_95"`
	SampleSize int     `json:"sample_size"`
}

type LiftMeasurement struct {
	Lift     float64 `json:"lift"`
	Low95    float64 `json:"low_95"`
	High95   float64 `json:"high_95"`
	RegHits  int     `json:"reg_hits"`
	SafeHits int     `json:"safe_hits"`
}

type RecallMeasurement struct {
	Marginal float64 `json:"marginal"`
	Unique   float64 `json:"unique"`
}

type HandValidatedSample struct {
	TP             int      `json:"tp"`
	FP             int      `json:"fp"`
	Unknown        int      `json:"unknown"`
	PointPrecision *float64 `json:"point_precision,omitempty"`
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

// DetectorEvidenceFor returns the corpus-measured evidence for the
// given detector type, or nil if not in the bundle.
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

// FormatTrustLine renders a one-line "trust this detector" summary
// suitable for inline rendering in `terrain explain` output. Picks
// the strongest available evidence (validated sample > global lift >
// heuristic precision).
func (e *DetectorEvidenceEntry) FormatTrustLine() string {
	if e == nil {
		return ""
	}
	if e.HandValidated != nil && e.HandValidated.PointPrecision != nil {
		return fmt.Sprintf("Validated precision %.0f%%.",
			*e.HandValidated.PointPrecision*100)
	}
	if e.GlobalLift != nil && e.GlobalLift.RegHits+e.GlobalLift.SafeHits > 30 {
		return fmt.Sprintf("PR-lift %.2fx [95%% CI %.2f, %.2f].",
			e.GlobalLift.Lift, e.GlobalLift.Low95, e.GlobalLift.High95)
	}
	if e.HeuristicPrecision != nil && e.HeuristicPrecision.SampleSize > 0 {
		return fmt.Sprintf("Heuristic precision %.0f%% lower bound.",
			e.HeuristicPrecision.Low95*100)
	}
	return ""
}

// FormatLiftLine renders a corpus-lift line for `terrain explain`. Surfaces
// the predictive-power evidence even when FormatTrustLine picks hand-
// validated precision (which alone doesn't tell users whether a firing
// is *predictive* of regressions). Returns "" when no lift data exists.
//
// Honest framing: if lift CI upper bound < 1.0 the detector is actively
// *anti-predictive* on the corpus — we say so plainly.
func (e *DetectorEvidenceEntry) FormatLiftLine() string {
	if e == nil || e.GlobalLift == nil {
		return ""
	}
	gl := e.GlobalLift
	if gl.RegHits+gl.SafeHits < 30 {
		return ""
	}
	verdict := ""
	switch {
	case gl.High95 < 1.0:
		verdict = " — flagged files were LESS likely than baseline to contain regressions in the corpus (anti-predictive)."
	case gl.Low95 > 1.5:
		verdict = " — flagged files were materially more likely to contain regressions."
	case gl.Low95 > 1.0:
		verdict = " — flagged files were modestly more likely to contain regressions."
	default:
		verdict = " — CI crosses 1.0; lift is consistent with chance on the corpus."
	}
	return fmt.Sprintf("PR-lift %.2fx (95%% CI %.2f-%.2f)%s",
		gl.Lift, gl.Low95, gl.High95, verdict)
}
