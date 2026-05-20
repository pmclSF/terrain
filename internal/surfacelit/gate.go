package surfacelit

import (
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// Gate is the canonical wire-up helper used by AI-moat detectors. Given
// a mechanisms registry, a surface name, the file the finding will
// reference, and the rule_id, it:
//
//  1. Runs Check(name, file) to compute presence.
//  2. Reads the surface_literal_presence_gate mechanism state.
//  3. Returns a Decision telling the caller whether to keep emitting
//     the finding.
//  4. When state=shadow AND the name is absent, emits a `would_suppress`
//     event to the configured shadow sink (if any).
//
// When state=off (or the mechanism is unknown), the gate is a no-op —
// Decision.Keep is always true and no shadow event is emitted. This
// keeps pre-cycle-2 behavior intact when the mechanism is disabled.
//
// When state=on AND the name is absent, Decision.Keep is false (drop
// the finding). No shadow event is emitted in state=on because the
// suppression is already user-visible.
//
// Callers pass the rule_id so the shadow event records which detector
// the finding originated from.
func Gate(reg *mechanisms.Registry, name, file, ruleID string) Decision {
	state := reg.State(MechanismName)

	// State=off: gate is fully disabled.
	if state == mechanisms.StateOff {
		return Decision{Result: Skipped, Keep: true}
	}

	// Run the check; failures (missing file, oversize) fail open.
	res, _ := Check(name, file)
	if res != Absent {
		return Decision{Result: res, Keep: true}
	}

	dec := Decision{Result: Absent, ShadowAction: string(shadow.ActionSuppress)}

	if state == mechanisms.StateOn {
		// Live: actually suppress the finding.
		dec.Keep = false
		return dec
	}

	// Shadow: keep the finding, emit a would-suppress event.
	dec.Keep = true
	shadow.Emit(shadow.Event{
		Mechanism: MechanismName,
		RuleID:    ruleID,
		Action:    shadow.ActionSuppress,
		File:      file,
		Reasons:   []string{Reason(res, name, file)},
	})
	return dec
}
