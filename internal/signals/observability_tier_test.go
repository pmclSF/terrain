package signals

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestIsObservabilityTier pins the gate/observability split. Every
// manifest entry has an explicit Tier (enforced by
// TestManifest_AllEntriesHaveExplicitTier). Signals without a manifest
// entry (runtime-derived, no static registration) keep the legacy
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

		// Explicit Tier: TierObservability on health-tier hygiene rules.
		{"slowTest is observability", SignalSlowTest, true},
		{"flakyTest is observability", SignalFlakyTest, true},

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

// TestManifest_AllEntriesHaveExplicitTier asserts that every manifest
// entry sets Tier explicitly. Empty Tier would silently flow through
// IsObservabilityTier (returns true) and silently demote any High/
// Critical detector to capped-Medium observability. The contract is
// "no implicit default": every entry chooses gate or observability at
// declaration time.
func TestManifest_AllEntriesHaveExplicitTier(t *testing.T) {
	var missing []string
	for _, e := range allSignalManifest {
		if e.Tier == "" {
			missing = append(missing, string(e.Type))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("manifest entries missing explicit Tier (%d): %v", len(missing), missing)
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
