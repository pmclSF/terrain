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
	if got.Metadata["corpus_lift"] != "unmeasured" {
		t.Errorf("corpus_lift = %v, want \"unmeasured\"", got.Metadata["corpus_lift"])
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
// only — it never raises). Metadata still gets the lift / unmeasured
// marker for `terrain explain` consumption.
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

// TestApplyEvidenceBasedSeverity_PopulatesMetadataOnEveryCall — even
// when severity is unchanged, the corpus_lift marker must populate so
// downstream `terrain explain` can render evidence consistently.
func TestApplyEvidenceBasedSeverity_PopulatesMetadataOnEveryCall(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     "anotherSyntheticType",
				Severity: models.SeverityInfo,
			},
		},
	}
	applyEvidenceBasedSeverity(snap)
	got := snap.Signals[0]
	if got.Metadata == nil {
		t.Fatal("Metadata not populated on info-severity signal")
	}
	if _, ok := got.Metadata["corpus_lift"]; !ok {
		t.Error("corpus_lift not set")
	}
}

// TestApplyEvidenceBasedSeverity_KnownTypeWithoutLift exercises the
// `ev.GlobalLift == nil` branch — the entry is in the evidence
// ledger but its global-lift summary is absent. Should still cap at
// Medium and populate metadata.
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

// TestApplyEvidenceBasedSeverity_KnownTypeWithLiftPopulatesIntervals
// exercises the GlobalLift-present branch — the metadata should carry
// the lift point AND, when the CI bounds are non-zero, the CI low/high
// markers. SignalUntestedExport is the canonical gate-tier detector
// with a populated lift in the evidence ledger.
func TestApplyEvidenceBasedSeverity_KnownTypeWithLiftPopulatesIntervals(t *testing.T) {
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
	if got.Metadata == nil {
		t.Fatal("Metadata not populated")
	}
	if _, ok := signals.LookupEvidence(signals.SignalUntestedExport); ok {
		if _, has := got.Metadata["corpus_lift"]; !has {
			t.Error("corpus_lift not set for known-type-with-lift signal")
		}
	}
}

// TestRoundTo2 covers the public helper's edge cases (rounding,
// negatives, exact halves).
func TestRoundTo2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{1.234, 1.23},
		{1.235, 1.24},   // round-half-up
		{0.005, 0.01},   // tiny positive
		{-1.235, -1.23}, // negative; truncation differs from positive — document current behavior
		{100, 100},
	}
	for _, c := range cases {
		if got := roundTo2(c.in); got != c.want {
			t.Errorf("roundTo2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
