package engine

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/signals"
)

// TestDefaultDisabledTypes_ContainsExpected pins the manifest contract
// that aiPromptInjectionRisk and the aiHardcodedAPIKey back-compat shim
// remain disabled by default. A regression that removes the
// DisabledByDefault flag from either entry would silently re-enable
// noisy detectors on every adopter's first analyze.
func TestDefaultDisabledTypes_ContainsExpected(t *testing.T) {
	disabled := signals.DefaultDisabledTypes()
	for _, expect := range []string{
		"aiPromptInjectionRisk",
		"aiHardcodedAPIKey",
	} {
		if !disabled[expect] {
			t.Errorf("DefaultDisabledTypes missing %q — disabled-by-default flag dropped from manifest?\n  current set: %v",
				expect, disabled)
		}
	}
}

// TestDefaultDisabledTypes_DoesNotIncludeWellValidated ensures the
// disable-by-default flag is reserved for detectors with poor evidence.
// Well-validated detectors must NOT be off by default.
func TestDefaultDisabledTypes_DoesNotIncludeWellValidated(t *testing.T) {
	disabled := signals.DefaultDisabledTypes()
	for _, wellValidated := range []string{
		"untestedExport",
		"depsDriftRisk",
		"configSchemaDrift",
		"frameworkMigration",
		"staticSkippedTest",
		"blastRadiusHotspot",
	} {
		if disabled[wellValidated] {
			t.Errorf("%q must NOT be DisabledByDefault — well-validated detectors stay on", wellValidated)
		}
	}
}

// TestDefaultDisabledTypes_StablePerCall returns a fresh map on each
// call so callers can mutate it without affecting future readers.
func TestDefaultDisabledTypes_StablePerCall(t *testing.T) {
	a := signals.DefaultDisabledTypes()
	b := signals.DefaultDisabledTypes()
	if len(a) != len(b) {
		t.Fatalf("len mismatch a=%d b=%d", len(a), len(b))
	}
	// Mutating a must not affect b.
	a["sentinel"] = true
	if b["sentinel"] {
		t.Errorf("DefaultDisabledTypes returns a shared map — mutating one call affects another")
	}
}

// TestDisabledByDefaultReason validates the manifest entry copy talks
// about an opt-in path so adopters know how to re-enable.
func TestDisabledByDefaultReason(t *testing.T) {
	for _, sig := range signals.Manifest() {
		if !sig.DisabledByDefault {
			continue
		}
		if !strings.Contains(strings.ToLower(sig.PromotionPlan), "opt") &&
			!strings.Contains(strings.ToLower(sig.PromotionPlan), "off by default") {
			t.Errorf("manifest entry %q is DisabledByDefault but its PromotionPlan does not document the opt-in story: %q",
				sig.Type, sig.PromotionPlan)
		}
	}
}
