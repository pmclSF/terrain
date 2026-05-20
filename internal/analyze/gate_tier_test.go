package analyze

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestBuildGateRelevantSummary_ExcludesObservabilityTier(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			// configSchemaDrift has no explicit Tier in the manifest →
			// default is gate-tier.
			{Type: "configSchemaDrift", Severity: models.SeverityHigh},
			{Type: "configSchemaDrift", Severity: models.SeverityMedium},
			// testsOnlyMocks ships at Tier: TierObservability.
			{Type: "testsOnlyMocks", Severity: models.SeverityCritical},
		},
	}

	full := buildSignalSummary(snap)
	if full.Total != 3 {
		t.Errorf("full summary should include all 3 signals, got %d", full.Total)
	}
	if full.Critical != 1 || full.High != 1 || full.Medium != 1 {
		t.Errorf("full breakdown wrong: %+v", full)
	}

	gate := BuildGateRelevantSummary(snap)
	if gate.Total != 2 {
		t.Errorf("gate-relevant summary should exclude observability-tier; got Total=%d", gate.Total)
	}
	if gate.Critical != 0 {
		t.Errorf("Critical testsOnlyMocks (observability) should NOT count toward gate; got %d", gate.Critical)
	}
	if gate.High != 1 || gate.Medium != 1 {
		t.Errorf("gate breakdown wrong: %+v", gate)
	}
}

func TestBuildGateRelevantSummary_EmptySignals(t *testing.T) {
	snap := &models.TestSuiteSnapshot{Signals: nil}
	gate := BuildGateRelevantSummary(snap)
	if gate.Total != 0 {
		t.Errorf("empty signals should yield zero Total, got %d", gate.Total)
	}
	if gate.ByCategory == nil {
		t.Errorf("ByCategory should be non-nil (empty map)")
	}
}

func TestBuildGateRelevantSummary_AllObservability(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "testsOnlyMocks", Severity: models.SeverityCritical},
			{Type: "aiPromptVersioning", Severity: models.SeverityHigh},
		},
	}
	gate := BuildGateRelevantSummary(snap)
	if gate.Total != 0 {
		t.Errorf("all-observability snapshot should yield Total=0, got %d", gate.Total)
	}
}
