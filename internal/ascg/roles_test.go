package ascg

import (
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

func TestRoleOf_Live_ExportedAndConsumed(t *testing.T) {
	facts := GraphFacts{
		IsExported:         true,
		ConsumedInSameFile: true,
	}
	if got := RoleOf(facts); got != RoleLiveConfigValue {
		t.Errorf("RoleOf = %v, want RoleLiveConfigValue", got)
	}
}

func TestRoleOf_Live_ExportedAndImportReached(t *testing.T) {
	facts := GraphFacts{
		IsExported:                 true,
		ImportReachedFromCallSites: true,
	}
	if got := RoleOf(facts); got != RoleLiveConfigValue {
		t.Errorf("RoleOf = %v, want RoleLiveConfigValue", got)
	}
}

func TestRoleOf_Catalog_FullSignal(t *testing.T) {
	facts := GraphFacts{
		InMapLiteralOfSize:         5,
		ConsumedInSameFile:         false,
		MapKeysReferencedElsewhere: true,
	}
	if got := RoleOf(facts); got != RoleCatalogOrExampleString {
		t.Errorf("RoleOf = %v, want RoleCatalogOrExampleString", got)
	}
}

func TestRoleOf_Catalog_RequiresMin3Entries(t *testing.T) {
	facts := GraphFacts{
		InMapLiteralOfSize:         2,
		ConsumedInSameFile:         false,
		MapKeysReferencedElsewhere: true,
	}
	if got := RoleOf(facts); got == RoleCatalogOrExampleString {
		t.Errorf("2-entry map should not classify as catalog")
	}
}

func TestRoleOf_Catalog_RequiresKeyReferencesElsewhere(t *testing.T) {
	facts := GraphFacts{
		InMapLiteralOfSize:         5,
		ConsumedInSameFile:         false,
		MapKeysReferencedElsewhere: false, // no external consumers
	}
	if got := RoleOf(facts); got == RoleCatalogOrExampleString {
		t.Errorf("map with no external key references should not be catalog")
	}
}

func TestRoleOf_NoEvidence_Unknown(t *testing.T) {
	facts := GraphFacts{}
	if got := RoleOf(facts); got != RoleUnknown {
		t.Errorf("empty facts should yield RoleUnknown, got %v", got)
	}
}

func TestRoleOf_LiveWinsOverCatalogWhenBothApply(t *testing.T) {
	facts := GraphFacts{
		IsExported:                 true,
		ConsumedInSameFile:         true,
		InMapLiteralOfSize:         5,
		MapKeysReferencedElsewhere: true,
	}
	if got := RoleOf(facts); got != RoleLiveConfigValue {
		t.Errorf("direct consumption should beat map-literal context, got %v", got)
	}
}

// ── GateRole ────────────────────────────────────────────────────────

func loadRoleReg(t *testing.T, state mechanisms.State) *mechanisms.Registry {
	t.Helper()
	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Override(RoleMechanismName, state); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestGateRole_Off_AlwaysKeepsNoDemote(t *testing.T) {
	reg := loadRoleReg(t, mechanisms.StateOff)
	facts := GraphFacts{InMapLiteralOfSize: 5, MapKeysReferencedElsewhere: true}
	dec := GateRole(reg, facts, Location{Path: "x.py"}, "ruleA")
	if !dec.Keep || dec.Demote {
		t.Errorf("state=off should keep, not demote; got %+v", dec)
	}
}

func TestGateRole_On_CatalogDemotesNotDropsKeep(t *testing.T) {
	reg := loadRoleReg(t, mechanisms.StateOn)
	facts := GraphFacts{InMapLiteralOfSize: 5, MapKeysReferencedElsewhere: true}
	dec := GateRole(reg, facts, Location{Path: "x.py"}, "ruleA")
	if !dec.Keep || !dec.Demote {
		t.Errorf("state=on + catalog should keep+demote; got %+v", dec)
	}
}

func TestGateRole_On_LiveJustKeeps(t *testing.T) {
	reg := loadRoleReg(t, mechanisms.StateOn)
	facts := GraphFacts{IsExported: true, ConsumedInSameFile: true}
	dec := GateRole(reg, facts, Location{Path: "x.py"}, "ruleA")
	if !dec.Keep || dec.Demote {
		t.Errorf("state=on + live should keep, not demote; got %+v", dec)
	}
	if dec.Role != RoleLiveConfigValue {
		t.Errorf("dec.Role = %v, want RoleLiveConfigValue", dec.Role)
	}
}

func TestGateRole_Shadow_CatalogEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadRoleReg(t, mechanisms.StateShadow)
	facts := GraphFacts{InMapLiteralOfSize: 5, MapKeysReferencedElsewhere: true}
	dec := GateRole(reg, facts, Location{Path: "x.py", Line: 12}, "ruleA")
	if dec.Demote {
		t.Errorf("shadow should not demote user-visible findings, got Demote=true")
	}
	if len(sink.Events()) != 1 {
		t.Errorf("expected 1 shadow event, got %d", len(sink.Events()))
	}
	if len(sink.Events()) == 1 && sink.Events()[0].Action != shadow.ActionDemoteSeverity {
		t.Errorf("event action = %v, want would_demote_severity", sink.Events()[0].Action)
	}
}

// ── CombineWithClassify ────────────────────────────────────────────

func TestCombineWithClassify_RoleWinsWhenStrong(t *testing.T) {
	roleDec := RoleDecision{Role: RoleLiveConfigValue, Keep: true}
	res := Result{Class: CatalogOrExample}
	merged := CombineWithClassify(roleDec, res)
	if merged.Role != RoleLiveConfigValue {
		t.Errorf("strong role decision should win, got %v", merged.Role)
	}
}

func TestCombineWithClassify_FallsBackToClassify(t *testing.T) {
	roleDec := RoleDecision{Role: RoleUnknown, Keep: true}
	res := Result{Class: CatalogOrExample}
	merged := CombineWithClassify(roleDec, res)
	if merged.Role != RoleCatalogOrExampleString {
		t.Errorf("unknown role + catalog classify should yield catalog role, got %v", merged.Role)
	}
	if !merged.Demote {
		t.Errorf("catalog from classify should demote")
	}
}

func TestCombineWithClassify_UnknownStays(t *testing.T) {
	roleDec := RoleDecision{Role: RoleUnknown, Keep: true}
	res := Result{Class: Unknown}
	merged := CombineWithClassify(roleDec, res)
	if merged.Role != RoleUnknown {
		t.Errorf("unknown + unknown should stay unknown, got %v", merged.Role)
	}
}

func TestRoleString(t *testing.T) {
	cases := map[Role]string{
		RoleLiveConfigValue:        "live_config_value",
		RoleCatalogOrExampleString: "catalog_or_example_string",
		RoleUnknown:                "unknown_role",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", r, got, want)
		}
	}
}
