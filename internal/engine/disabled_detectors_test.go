package engine

import (
	"testing"

	"github.com/pmclSF/terrain/internal/aliases"
)

func TestExpandDisabledDetectors_BareExpandsThroughAlias(t *testing.T) {
	reg, _ := aliases.Load()
	disabled := map[string]bool{"aiHardcodedAPIKey": true}
	expanded, hit := expandDisabledDetectors(disabled, reg)

	for _, want := range []string{
		"aiHardcodedAPIKey",
		"aiHardcodedAPIKey-literal-shape",
		"secretScannerCoverageDegraded",
	} {
		if !expanded[want] {
			t.Errorf("bare aiHardcodedAPIKey should expand to include %q; expanded=%v", want, expanded)
		}
	}
	if !hit["aiHardcodedAPIKey"] {
		t.Errorf("aliasesHit should record aiHardcodedAPIKey; got %v", hit)
	}
}

func TestExpandDisabledDetectors_LiteralPrefixOptsOut(t *testing.T) {
	reg, _ := aliases.Load()
	// "=" prefix means: disable ONLY this rule_id, don't expand through aliases.
	disabled := map[string]bool{"=aiHardcodedAPIKey": true}
	expanded, hit := expandDisabledDetectors(disabled, reg)

	if !expanded["aiHardcodedAPIKey"] {
		t.Errorf("literal entry should disable the bare rule_id; got %v", expanded)
	}
	if expanded["aiHardcodedAPIKey-literal-shape"] {
		t.Errorf("literal entry should NOT disable split halves; got %v", expanded)
	}
	if expanded["secretScannerCoverageDegraded"] {
		t.Errorf("literal entry should NOT disable split halves; got %v", expanded)
	}
	if len(hit) != 0 {
		t.Errorf("literal entry should NOT emit alias NOTE; got hit=%v", hit)
	}
}

func TestExpandDisabledDetectors_MixedLiteralAndBare(t *testing.T) {
	reg, _ := aliases.Load()
	disabled := map[string]bool{
		"=aiHardcodedAPIKey": true, // back-compat only
		"weakAssertion":      true, // unaliased — pass through
	}
	expanded, _ := expandDisabledDetectors(disabled, reg)

	if !expanded["aiHardcodedAPIKey"] {
		t.Errorf("literal entry should disable bare ID")
	}
	if expanded["aiHardcodedAPIKey-literal-shape"] {
		t.Errorf("literal entry should not pull in split halves")
	}
	if !expanded["weakAssertion"] {
		t.Errorf("bare unaliased entry should pass through")
	}
}

func TestExpandDisabledDetectors_EmptyLiteralIgnored(t *testing.T) {
	reg, _ := aliases.Load()
	disabled := map[string]bool{"=": true}
	expanded, _ := expandDisabledDetectors(disabled, reg)
	if len(expanded) != 0 {
		t.Errorf("empty literal should produce nothing; got %v", expanded)
	}
}

func TestExpandDisabledDetectors_NilRegistryDoesNotPanic(t *testing.T) {
	disabled := map[string]bool{"=aiHardcodedAPIKey": true}
	// Literal path should work even with nil registry.
	expanded, _ := expandDisabledDetectors(disabled, nil)
	if !expanded["aiHardcodedAPIKey"] {
		t.Errorf("nil registry should still allow literal-prefix entries")
	}
}
