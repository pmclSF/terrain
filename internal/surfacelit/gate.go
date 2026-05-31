package surfacelit

import (
	"github.com/pmclSF/terrain/internal/mechanisms"
)

// Gate is the canonical wire-up helper used by AI surface-aware detectors. Given
// a mechanisms registry, a surface name, the file the finding will
// reference, and the rule_id, it:
//
//  1. Runs Check(name, file) to compute presence.
//  2. Routes through mechanisms.GateSuppress which handles the
//     state-machine (off/shadow/on) + shadow event emission uniformly
//     across every gate-helper in the codebase.
//  3. Returns a Decision telling the caller whether to keep emitting
//     the finding.
//
// When state=off (or the registry is nil), the gate is a no-op —
// Decision.Keep is always true and no shadow event is emitted. When
// state=on AND the surface name is absent, Decision.Keep is false
// (drop the finding). When state=shadow AND absent, Keep is true but
// a would-suppress event is emitted to the configured shadow sink.
//
// Callers pass the rule_id so the shadow event records which detector
// the finding originated from.
func Gate(reg *mechanisms.Registry, name, file, ruleID string) Decision {
	// Predicate: presence check. Fires (returns Fired=true) when the
	// surface name is ABSENT from the file's non-comment content —
	// that's the case where the gate would suppress the finding.
	var presence Result
	predicate := func() mechanisms.PredicateResult {
		res, _ := Check(name, file)
		presence = res
		return mechanisms.PredicateResult{
			Fired:   res == Absent,
			Reasons: []string{Reason(res, name, file)},
		}
	}

	keep := mechanisms.GateSuppress(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: file},
		true, predicate)

	dec := Decision{Result: presence, Keep: keep}
	if !keep {
		dec.ShadowAction = "would_suppress"
	}
	return dec
}
