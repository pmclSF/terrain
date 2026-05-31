package engine

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// applyEvidenceBasedSeverity walks every signal in the snapshot and
// re-evaluates its severity against the validation evidence (lift CI +
// validated precision). Detectors with weak empirical lift get
// demoted; detectors with no evidence at all are capped to Medium
// (fail-closed — absence of evidence is not promotion to High/Critical).
//
// Side effects per signal:
//   - sig.Severity may be lowered (never raised — evidence demotes only)
//   - sig.Metadata["effective_severity_lowered"] = true when demoted
//   - sig.Metadata["declared_severity"] preserves the original
//   - sig.Metadata["corpus_lift"], "corpus_lift_ci_low", "corpus_lift_ci_high"
//     populated when evidence exists; consumed by `terrain explain`
//   - sig.Metadata["validated_precision"] populated when present
//
// This is the centralized point where severity rebasing on
// corpus lift) is enforced. Detectors continue to emit their declared
// severity; this pass is the single source of truth for what users see.
func applyEvidenceBasedSeverity(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil || len(snapshot.Signals) == 0 {
		return
	}
	for i := range snapshot.Signals {
		applyEvidenceBasedSeverityToSignal(&snapshot.Signals[i])
	}
}

func applyEvidenceBasedSeverityToSignal(sig *models.Signal) {
	declared := sig.Severity
	effective, adjusted := signals.EffectiveSeverity(sig.Type, declared)
	if sig.Metadata == nil {
		sig.Metadata = map[string]any{}
	}
	if adjusted {
		sig.Severity = effective
		sig.Metadata["declared_severity"] = string(declared)
		sig.Metadata["effective_severity_lowered"] = true
	}
	// Always populate the lift point + CI when we have it so callers
	// can filter on effect size. Methodology-revealing fields
	// (sample-size n, formatted summary strings) are not exposed in
	// the public metadata — `terrain explain` surfaces them in its
	// own rendering when verbose is requested.
	if ev, ok := signals.LookupEvidence(sig.Type); ok {
		if ev.GlobalLift != nil {
			sig.Metadata["corpus_lift"] = roundTo2(ev.GlobalLift.LiftPoint())
			if ev.GlobalLift.Low95 != 0 || ev.GlobalLift.High95 != 0 {
				sig.Metadata["corpus_lift_ci_low"] = roundTo2(ev.GlobalLift.Low95)
				sig.Metadata["corpus_lift_ci_high"] = roundTo2(ev.GlobalLift.High95)
			}
		}
	} else {
		sig.Metadata["corpus_lift"] = "unmeasured"
	}
}

// roundTo2 rounds a float to 2 decimal places. Kept as a small helper so
// the metadata stays readable in JSON output.
func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
