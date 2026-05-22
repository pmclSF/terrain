package signals

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestIsObservabilityTier pins the empty-tier-defaults-to-observability
// contract: every detector with a manifest entry MUST opt in to
// TierGate explicitly to block CI. Signals without a manifest entry
// (runtime-derived, no static registration) keep the legacy
// "treat as gate-relevant" behavior.
func TestIsObservabilityTier(t *testing.T) {
	cases := []struct {
		name string
		t    models.SignalType
		want bool
	}{
		// Explicit Tier: TierObservability — observability.
		{"mockHeavyTest is observability", SignalMockHeavyTest, true},
		{"testsOnlyMocks is observability", SignalTestsOnlyMocks, true},

		// Explicit Tier: TierGate — gate.
		{"configSchemaDrift is gate", SignalConfigSchemaDrift, false},
		{"depsDriftRisk is gate", SignalDepsDriftRisk, false},
		{"untestedExport is gate", SignalUntestedExport, false},
		{"frameworkMigration is gate", SignalFrameworkMigration, false},
		{"safetyFailure is gate", SignalSafetyFailure, false},
		{"hallucinationDetected is gate", SignalHallucinationDetected, false},

		// Empty Tier — defaults to observability under the flipped rule.
		{"slowTest (empty tier) is observability", SignalSlowTest, true},
		{"flakyTest (empty tier) is observability", SignalFlakyTest, true},

		// Unknown type — runtime / ingestion derived; legacy gate-relevant.
		{"unknown type is gate-relevant", models.SignalType("unknownRuntimeSignal"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsObservabilityTier(tc.t); got != tc.want {
				t.Errorf("IsObservabilityTier(%q) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}

// TestIsGateRelevant is the dual of TestIsObservabilityTier.
func TestIsGateRelevant(t *testing.T) {
	cases := []struct {
		name string
		t    models.SignalType
		want bool
	}{
		{"untestedExport (gate)", SignalUntestedExport, true},
		{"mockHeavyTest (observability)", SignalMockHeavyTest, false},
		{"unknown type (legacy gate)", models.SignalType("foo"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsGateRelevant(tc.t); got != tc.want {
				t.Errorf("IsGateRelevant(%q) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}
