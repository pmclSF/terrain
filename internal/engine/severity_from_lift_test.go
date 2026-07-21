package engine

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestApplyEvidenceBasedSeverity_NilSnapshot guards the no-op path.
// applyEvidenceBasedSeverity is called unconditionally by the pipeline,
// so it has to handle nil + empty without panicking.
func TestApplyEvidenceBasedSeverity_NilSnapshot(t *testing.T) {
	applyEvidenceBasedSeverity(nil)
	applyEvidenceBasedSeverity(&models.TestSuiteSnapshot{})
}

// TestApplyEvidenceBasedSeverity_UnknownTypeCappedAtMedium pins the
// fail-closed contract: a signal type the evidence ledger has never
// heard of must not ride through at Critical / High. The signal here
// is a synthetic type with no manifest entry.
func TestApplyEvidenceBasedSeverity_UnknownTypeCappedAtMedium(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "syntheticTypeWithoutEvidence",
				Severity: models.SeverityCritical,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Severity != models.SeverityMedium {
		t.Errorf("unknown type Severity = %q, want %q (cap-at-medium)",
			got.Severity, models.SeverityMedium)
	}
	if got.Metadata == nil {
		t.Fatal("Metadata not populated")
	}
	if got.Metadata["declared_severity"] != "critical" {
		t.Errorf("declared_severity = %v, want %q", got.Metadata["declared_severity"], "critical")
	}
	if got.Metadata["effective_severity_lowered"] != true {
		t.Errorf("effective_severity_lowered = %v, want true", got.Metadata["effective_severity_lowered"])
	}
}

// TestApplyEvidenceBasedSeverity_ObservabilityCappedAtMedium confirms
// that an observability-tier signal at declared Critical lands at
// Medium with the declared_severity preserved, regardless of evidence.
func TestApplyEvidenceBasedSeverity_ObservabilityCappedAtMedium(t *testing.T) {
	// SignalMockHeavyTest is TierObservability per manifest.go.
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     signals.SignalMockHeavyTest,
				Severity: models.SeverityCritical,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Severity != models.SeverityMedium {
		t.Errorf("observability Severity = %q, want %q", got.Severity, models.SeverityMedium)
	}
	if got.Metadata["declared_severity"] != "critical" {
		t.Errorf("declared_severity = %v, want %q", got.Metadata["declared_severity"], "critical")
	}
}

// TestApplyEvidenceBasedSeverity_PreservesLowDeclared confirms that
// a declared-low signal isn't promoted by evidence (evidence demotes
// only — it never raises), and that a signal which is not demoted
// carries no demotion metadata.
func TestApplyEvidenceBasedSeverity_PreservesLowDeclared(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "syntheticTypeWithoutEvidence",
				Severity: models.SeverityLow,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Severity != models.SeverityLow {
		t.Errorf("low-declared Severity = %q, want %q (not raised)", got.Severity, models.SeverityLow)
	}
	if got.Metadata["effective_severity_lowered"] == true {
		t.Errorf("effective_severity_lowered = true on a non-demoted signal")
	}
}

// TestApplyEvidenceBasedSeverity_KnownTypeWithoutLift exercises the
// `ev.GlobalLift == nil` branch — the entry is in the evidence
// ledger but its confidence interval is absent. Should still cap at
// Medium and record the demotion.
func TestApplyEvidenceBasedSeverity_KnownTypeWithoutLift(t *testing.T) {
	// SignalAIPromptVersioning is in the manifest with no GlobalLift
	// wired in the evidence ledger today.
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     signals.SignalAIPromptVersioning,
				Severity: models.SeverityHigh,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Severity != models.SeverityMedium {
		t.Errorf("Severity = %q, want %q (cap at medium when lift unavailable)",
			got.Severity, models.SeverityMedium)
	}
	if got.Metadata == nil {
		t.Fatal("Metadata not populated")
	}
}

// TestApplyEvidenceBasedSeverity_DemoteOnly confirms evidence never
// raises severity above what the detector declared.
func TestApplyEvidenceBasedSeverity_DemoteOnly(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     signals.SignalUntestedExport,
				Severity: models.SeverityMedium,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Severity != models.SeverityMedium && got.Severity != models.SeverityLow {
		t.Errorf("Severity = %q, want Medium or lower (evidence demotes only)", got.Severity)
	}
	if got.Metadata["effective_severity_lowered"] == true && got.Metadata["declared_severity"] != "medium" {
		t.Errorf("demotion metadata inconsistent: %v", got.Metadata)
	}
}
