package mechanisms

import (
	"testing"

	"github.com/pmclSF/terrain/internal/shadow"
)

// fixedPredicate returns a PredicateResult-producing closure that
// always returns the supplied Fired + Reasons; useful for table tests.
func fixedPredicate(fired bool, reasons ...string) func() PredicateResult {
	return func() PredicateResult {
		return PredicateResult{Fired: fired, Reasons: reasons}
	}
}

func loadTestReg(t *testing.T, name string, state State) *Registry {
	t.Helper()
	reg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Override(name, state); err != nil {
		t.Fatal(err)
	}
	return reg
}

const testMechanism = "surface_literal_presence_gate"

// ── GateSuppress ───────────────────────────────────────────────────

func TestGateSuppress_StateOff_PredicateNotInvoked(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOff)
	called := false
	pred := func() PredicateResult {
		called = true
		return PredicateResult{Fired: true}
	}
	keep := GateSuppress(reg, testMechanism, EventContext{RuleID: "r"}, true, pred)
	if !keep {
		t.Errorf("state=off + keepLegacy=true should keep")
	}
	if called {
		t.Errorf("predicate must not be invoked when state=off")
	}
}

func TestGateSuppress_StateShadow_FiredEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadTestReg(t, testMechanism, StateShadow)
	keep := GateSuppress(reg, testMechanism,
		EventContext{RuleID: "r", File: "f.py", Line: 12},
		true, fixedPredicate(true, "demo reason"))
	if !keep {
		t.Errorf("state=shadow should keep")
	}
	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 shadow event, got %d", len(events))
	}
	e := events[0]
	if e.Mechanism != testMechanism || e.RuleID != "r" || e.File != "f.py" || e.Line != 12 {
		t.Errorf("event metadata wrong: %+v", e)
	}
	if e.Action != shadow.ActionSuppress {
		t.Errorf("event action = %v, want would_suppress", e.Action)
	}
	if len(e.Reasons) != 1 || e.Reasons[0] != "demo reason" {
		t.Errorf("event reasons = %v", e.Reasons)
	}
}

func TestGateSuppress_StateShadow_NotFiredEmitsNothing(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadTestReg(t, testMechanism, StateShadow)
	keep := GateSuppress(reg, testMechanism, EventContext{RuleID: "r"}, true, fixedPredicate(false))
	if !keep {
		t.Errorf("shadow + not fired should keep")
	}
	if len(sink.Events()) != 0 {
		t.Errorf("not-fired predicate should emit zero events")
	}
}

func TestGateSuppress_StateOn_FiredDrops(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOn)
	keep := GateSuppress(reg, testMechanism, EventContext{RuleID: "r"}, true, fixedPredicate(true))
	if keep {
		t.Errorf("state=on + fired should drop (keep=false)")
	}
}

func TestGateSuppress_StateOn_NotFiredKeeps(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOn)
	keep := GateSuppress(reg, testMechanism, EventContext{RuleID: "r"}, true, fixedPredicate(false))
	if !keep {
		t.Errorf("state=on + not fired should keep")
	}
}

func TestGateSuppress_KeepLegacyFalse_FiredStillKeepsInShadow(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	// keepLegacy=false case: legacy was "suppress by default", gate
	// might "keep" instead. Shadow should preserve legacy verdict.
	reg := loadTestReg(t, testMechanism, StateShadow)
	keep := GateSuppress(reg, testMechanism, EventContext{RuleID: "r"}, false, fixedPredicate(true))
	if keep {
		t.Errorf("shadow should preserve legacy verdict; got keep=true with keepLegacy=false")
	}
}

// ── GateDemote ─────────────────────────────────────────────────────

func TestGateDemote_StateOnFiredDemotes(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOn)
	demote := GateDemote(reg, testMechanism, EventContext{RuleID: "r"}, fixedPredicate(true))
	if !demote {
		t.Errorf("state=on + fired should demote")
	}
}

func TestGateDemote_StateShadowEmitsDemoteEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadTestReg(t, testMechanism, StateShadow)
	demote := GateDemote(reg, testMechanism, EventContext{RuleID: "r"}, fixedPredicate(true, "catalog"))
	if demote {
		t.Errorf("shadow should not demote user-visible findings")
	}
	if len(sink.Events()) != 1 || sink.Events()[0].Action != shadow.ActionDemoteSeverity {
		t.Errorf("expected one would_demote_severity event")
	}
}

func TestGateDemote_StateOffNoEffect(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOff)
	if GateDemote(reg, testMechanism, EventContext{RuleID: "r"}, fixedPredicate(true)) {
		t.Errorf("state=off should never demote")
	}
}

// ── GateAdd ────────────────────────────────────────────────────────

func TestGateAdd_StateOnAdds(t *testing.T) {
	reg := loadTestReg(t, testMechanism, StateOn)
	if !GateAdd(reg, testMechanism, EventContext{}, fixedPredicate(true)) {
		t.Errorf("state=on + fired should add")
	}
}

func TestGateAdd_StateShadowEmitsAddEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadTestReg(t, testMechanism, StateShadow)
	add := GateAdd(reg, testMechanism, EventContext{RuleID: "r"}, fixedPredicate(true))
	if add {
		t.Errorf("shadow should not add user-visible findings")
	}
	if len(sink.Events()) != 1 || sink.Events()[0].Action != shadow.ActionAdd {
		t.Errorf("expected one would_add event")
	}
}

func TestGate_NilRegistrySafe(t *testing.T) {
	// All three helpers fail open on nil registry.
	if !GateSuppress(nil, testMechanism, EventContext{}, true, fixedPredicate(true)) {
		t.Errorf("nil registry should preserve keepLegacy")
	}
	if GateDemote(nil, testMechanism, EventContext{}, fixedPredicate(true)) {
		t.Errorf("nil registry should not demote")
	}
	if GateAdd(nil, testMechanism, EventContext{}, fixedPredicate(true)) {
		t.Errorf("nil registry should not add")
	}
}
