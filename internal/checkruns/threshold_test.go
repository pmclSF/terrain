package checkruns

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestBuildBundleAt_HonorsThreshold: the required gate check must fail only at
// or above the configured --fail-on threshold, so it agrees with the CLI
// verdict instead of hardcoding "medium". A medium finding fails at
// --fail-on=medium but NOT at --fail-on=high.
func TestBuildBundleAt_HonorsThreshold(t *testing.T) {
	t.Parallel()
	medium := &models.TestSuiteSnapshot{Signals: []models.Signal{{
		Type:     signals.SignalUntestedExport, // TierGate per manifest
		Severity: models.SeverityMedium,
		Location: models.SignalLocation{File: "src/exported.go", Line: 42},
	}}}

	if got := BuildBundleAt(medium, "sha", nil, models.SeverityMedium).Gate.Conclusion; got != "failure" {
		t.Errorf("medium finding at --fail-on=medium: gate = %q, want failure", got)
	}
	if got := BuildBundleAt(medium, "sha", nil, models.SeverityHigh).Gate.Conclusion; got != "success" {
		t.Errorf("medium finding at --fail-on=high: gate = %q, want success (below threshold)", got)
	}

	high := &models.TestSuiteSnapshot{Signals: []models.Signal{{
		Type:     signals.SignalUntestedExport,
		Severity: models.SeverityHigh,
		Location: models.SignalLocation{File: "src/exported.go", Line: 42},
	}}}
	if got := BuildBundleAt(high, "sha", nil, models.SeverityHigh).Gate.Conclusion; got != "failure" {
		t.Errorf("high finding at --fail-on=high: gate = %q, want failure", got)
	}
	// An empty/unset threshold configures no gate — the check never fails.
	if got := BuildBundleAt(high, "sha", nil, "").Gate.Conclusion; got != "success" {
		t.Errorf("empty threshold: gate = %q, want success (no gate configured)", got)
	}
}

// TestBuildBundleAtWithGate_TrustFloorDemotes proves the required-check surface
// honors the trust floor: a gate-tier finding whose remediation is NOT validated
// (blockable returns false) is demoted to the observability check and cannot
// fail the required gate — matching `terrain analyze --fail-on`. With a nil
// predicate (trust floor off) the same finding fails the gate.
func TestBuildBundleAtWithGate_TrustFloorDemotes(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{Signals: []models.Signal{{
		Type:     signals.SignalUntestedExport, // gate tier, High
		Severity: models.SeverityHigh,
		Location: models.SignalLocation{File: "src/exported.go", Line: 42},
	}}}

	// Trust floor off (nil predicate): the gate fails on the High finding.
	if got := BuildBundleAtWithGate(snap, "sha", nil, models.SeverityHigh, nil).Gate.Conclusion; got != "failure" {
		t.Errorf("trust floor off: gate = %q, want failure", got)
	}

	// Trust floor on, remediation NOT validated: demoted to observability; the
	// required gate must NOT fail — and the finding still surfaces in the obs check.
	notBlockable := func(models.Signal) bool { return false }
	bundle := BuildBundleAtWithGate(snap, "sha", nil, models.SeverityHigh, notBlockable)
	if got := bundle.Gate.Conclusion; got != "success" {
		t.Errorf("trust floor, unvalidated remediation: gate = %q, want success (demoted, not blocking)", got)
	}
	if bundle.Observability.Output.Summary == "" && len(bundle.Observability.Output.Text) == 0 {
		t.Error("demoted finding should still surface in the observability check, not vanish")
	}

	// Trust floor on, remediation validated (blockable true): still gates.
	blockable := func(models.Signal) bool { return true }
	if got := BuildBundleAtWithGate(snap, "sha", nil, models.SeverityHigh, blockable).Gate.Conclusion; got != "failure" {
		t.Errorf("trust floor, validated remediation: gate = %q, want failure", got)
	}
}
