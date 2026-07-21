package aipipeline

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline/fixscaffold"
)

// TestComposer_AttachScaffold_Contract pins the scaffold-attachment contract
// of Composer.Compose (which calls attachScaffold). This is the unit-level
// contract behind the "findings carry their fix" feature; the integration
// test in aipiperun exercises only the wired-registry happy path.
//
// Contract:
//
//	C1. No Scaffolds generator        → finding carries no fix (empty body+path).
//	C2. Registry set, rule unhandled  → no fix (a generator must exist for the rule).
//	C3. Registry set, rule handled    → body present + target path derived
//	                                    from the surface basename.
func TestComposer_AttachScaffold_Contract(t *testing.T) {
	cand := &Candidate{RuleID: "ai.surface.missing_eval", Path: "src/handler.ts"}

	// C1: nil Scaffolds → no scaffold, and Compose must not panic.
	bare := NewComposer(nil, PostureObservability)
	if f := bare.Compose(cand); f.FixScaffold != "" || f.FixScaffoldPath != "" {
		t.Errorf("C1 nil Scaffolds: want empty, got body=%q path=%q", f.FixScaffold, f.FixScaffoldPath)
	}

	withReg := NewComposer(nil, PostureObservability)
	withReg.Scaffolds = fixscaffold.NewRegistryAdapter(fixscaffold.NewRegistry())

	// C3: rule WITH a generator → non-empty body + exact derived path.
	f := withReg.Compose(cand)
	if f.FixScaffold == "" {
		t.Error("C3: rule with a generator must attach a non-empty scaffold body")
	}
	if f.FixScaffoldPath != "evals/promptfoo/handler.yaml" {
		t.Errorf("C3: path = %q, want evals/promptfoo/handler.yaml", f.FixScaffoldPath)
	}

	// C2: rule WITHOUT a generator → no scaffold, even with the registry set.
	// Guards against a regression that attaches a generic/wrong scaffold to
	// every rule.
	unknown := &Candidate{RuleID: "ai.no.such.generator", Path: "src/handler.ts"}
	if f := withReg.Compose(unknown); f.FixScaffold != "" || f.FixScaffoldPath != "" {
		t.Errorf("C2 rule without generator: want empty, got body=%q path=%q", f.FixScaffold, f.FixScaffoldPath)
	}
}
