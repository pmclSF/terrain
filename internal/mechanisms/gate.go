package mechanisms

import "github.com/pmclSF/terrain/internal/shadow"

// PredicateResult is what a gate's predicate closure returns. Fired
// indicates whether the structural test matched; Reasons is the list
// of human-readable justifications attached to the shadow event when
// the gate is in shadow state.
//
// Predicates are expected to be cheap to call when the mechanism is
// off — the GateSuppress / GateDemote helpers short-circuit before
// invoking the predicate in that case.
type PredicateResult struct {
	Fired   bool
	Reasons []string
}

// EventContext is the location metadata attached to shadow events.
// All fields are optional — File/Line are commonly set from a finding's
// Location, RuleID from the consumer detector's rule_id.
type EventContext struct {
	RuleID string
	File   string
	Line   int
}

// GateSuppress runs the canonical mechanism-state machine for a
// suppression-gate consumer:
//
//   - state=off    → caller's keepLegacy verdict wins, predicate is
//     NOT invoked.
//   - state=shadow → caller's keepLegacy verdict wins, predicate IS
//     invoked, and a would-suppress shadow event is
//     emitted when the predicate fired.
//   - state=on     → predicate IS invoked, and Keep=false when fired
//     (the caller drops the finding).
//
// keepLegacy is what the legacy code path would do absent the gate.
// Typically `true` (keep the finding by default; the gate may suppress).
// Pass `false` when legacy was "suppress unless something says keep".
//
// The predicate closure is called at most once per invocation. Its
// PredicateResult.Reasons is forwarded verbatim into the shadow event.
func GateSuppress(reg *Registry, name string, ctx EventContext, keepLegacy bool, predicate func() PredicateResult) bool {
	state := reg.State(name)
	if state == StateOff {
		return keepLegacy
	}
	res := predicate()
	if !res.Fired {
		return keepLegacy
	}
	if state == StateOn {
		return false
	}
	// Shadow: keep the legacy verdict on the user-visible finding
	// while emitting a would-suppress event to telemetry.
	shadow.Emit(shadow.Event{
		Mechanism: name,
		RuleID:    ctx.RuleID,
		Action:    shadow.ActionSuppress,
		File:      ctx.File,
		Line:      ctx.Line,
		Reasons:   res.Reasons,
	})
	return keepLegacy
}

// GateDemote runs the canonical mechanism-state machine for a severity-
// demotion consumer:
//
//   - state=off    → demote=false, predicate NOT invoked.
//   - state=shadow → demote=false (no user-visible change), predicate IS
//     invoked, and a would-demote-severity event is
//     emitted when the predicate fired.
//   - state=on     → predicate IS invoked, and demote=true when fired
//     (the caller demotes the finding's severity).
//
// Mirror of GateSuppress for the demote path. Used by consumer
// detectors that demote findings (e.g. ASCG catalog/example role,
// runtimeconfig).
func GateDemote(reg *Registry, name string, ctx EventContext, predicate func() PredicateResult) bool {
	state := reg.State(name)
	if state == StateOff {
		return false
	}
	res := predicate()
	if !res.Fired {
		return false
	}
	if state == StateOn {
		return true
	}
	shadow.Emit(shadow.Event{
		Mechanism: name,
		RuleID:    ctx.RuleID,
		Action:    shadow.ActionDemoteSeverity,
		File:      ctx.File,
		Line:      ctx.Line,
		Reasons:   res.Reasons,
	})
	return false
}

// GateAdd is the symmetric "would-add" variant: a gate that would
// surface a NEW finding when fired. state=on adds; state=shadow emits
// a would-add event and returns false; state=off returns false.
//
// Used by mechanisms that lift recall — e.g. barrelresolver pushing
// resolutions the legacy resolver missed.
func GateAdd(reg *Registry, name string, ctx EventContext, predicate func() PredicateResult) bool {
	state := reg.State(name)
	if state == StateOff {
		return false
	}
	res := predicate()
	if !res.Fired {
		return false
	}
	if state == StateOn {
		return true
	}
	shadow.Emit(shadow.Event{
		Mechanism: name,
		RuleID:    ctx.RuleID,
		Action:    shadow.ActionAdd,
		File:      ctx.File,
		Line:      ctx.Line,
		Reasons:   res.Reasons,
	})
	return false
}
