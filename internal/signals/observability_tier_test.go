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
// entry sets Tier to a known value (TierGate or TierObservability).
// Empty Tier would silently flow through IsObservabilityTier (returns
// true) and silently demote any High/Critical detector to capped-
// Medium observability. A typo like Tier: "Observability" (capital O)
// would do the same. The contract is "no implicit default and no
// unknown values": every entry chooses one of the two known tiers at
// declaration time.
func TestManifest_AllEntriesHaveExplicitTier(t *testing.T) {
	var missing []string
	var unknown []string
	for _, e := range allSignalManifest {
		switch e.Tier {
		case "":
			missing = append(missing, string(e.Type))
		case TierGate, TierObservability:
			// ok
		default:
			unknown = append(unknown, string(e.Type)+"="+string(e.Tier))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("manifest entries missing explicit Tier (%d): %v", len(missing), missing)
	}
	if len(unknown) > 0 {
		t.Fatalf("manifest entries with unknown Tier value (%d): %v (expected one of: %q, %q)",
			len(unknown), unknown, TierGate, TierObservability)
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
		// prompt-schema-drift was promoted to gate tier after a large real-world
		// corpus confirmed gate-ready precision; under the default trust floor it
		// still blocks CI only when its remediation is closed-loop validated.
		{"aiPromptSchemaDrift (promoted to gate)", SignalAIPromptSchemaDrift, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsGateRelevant(tc.t); got != tc.want {
				t.Errorf("IsGateRelevant(%q) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}
