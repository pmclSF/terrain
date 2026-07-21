package engine

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// applyEvidenceBasedSeverity walks every signal in the snapshot and
// re-evaluates its severity against the detector's confidence interval.
// Detectors with weak empirical confidence get demoted; detectors with no
// evidence at all are capped to Medium (fail-closed — absence of evidence
// is not promotion to High/Critical).
//
// Side effects per signal (only when the severity is lowered):
//   - sig.Severity is set to the lowered value (never raised)
//   - sig.Metadata["effective_severity_lowered"] = true
//   - sig.Metadata["declared_severity"] preserves the original
//
// This is the single point where severity rebasing is enforced. Detectors
// continue to emit their declared severity; this pass is the source of
// truth for what users see.
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
	if !adjusted {
		return
	}
	if sig.Metadata == nil {
		sig.Metadata = map[string]any{}
	}
	sig.Severity = effective
	sig.Metadata["declared_severity"] = string(declared)
	sig.Metadata["effective_severity_lowered"] = true
}
