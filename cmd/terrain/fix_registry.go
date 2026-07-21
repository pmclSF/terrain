package main

import (
	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/deps"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/promptcontract"
	"github.com/pmclSF/terrain/internal/remediate"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

// resolveTrustFloor decides whether the remediation-validity gate is active.
// The trust floor is the 0.4.0 default (on): a finding may block CI only when
// its remediation is closed-loop validated. Resolution order, later wins:
//
//	default (on) → config trust_floor → --trust-floor (force on) →
//	--no-trust-floor (force off)
//
// so the CLI always overrides the config, and the opt-out overrides the
// opt-in when a user contradictorily passes both.
func resolveTrustFloor(optTrustFloor, optNoTrustFloor bool, cfg *terrainconfig.Config) bool {
	trustFloor := true
	if cfg != nil && cfg.TrustFloor != nil {
		trustFloor = *cfg.TrustFloor
	}
	if optTrustFloor {
		trustFloor = true
	}
	if optNoTrustFloor {
		trustFloor = false
	}
	return trustFloor
}

// defaultFixRegistry wires the signal-side remediation producers — the
// canonical-path analog of the AI composer's fixscaffold registry. Each
// producer attaches a structured, mechanically-applicable Fix to a finding
// when one exists; rules without a producer (or whose producer declines)
// stay judge-only. New detector families register here as they are taken
// through the closed loop.
func defaultFixRegistry() *remediate.FixRegistry {
	r := remediate.NewFixRegistry()
	pinCarets := func(root string, f findings.Finding) *findings.Fix {
		fix, ok := deps.PinCaretsFix(root, f.PrimaryLoc.Path)
		if !ok {
			return nil
		}
		return fix
	}
	// Register under the base drift-risk rule and the two split rule IDs
	// (deps_drift_risk_split): when the split mechanism is on, findings carry
	// the caret-policy / strict-pin rule ID, and they need the same producer
	// so the fix still attaches.
	r.Register("terrain/deps/drift-risk", pinCarets)
	r.Register("terrain/deps/drift-caret-policy", pinCarets)
	r.Register("terrain/deps/drift-strict-pin", pinCarets)
	// Prompt→schema drift: correct the prompt's field reference to the nearest
	// existing schema field when there is a confident (typo-distance) match;
	// declines otherwise, leaving the finding judge-only.
	r.Register("terrain/ai/prompt-schema-drift", promptcontract.DriftFix)
	return r
}

// ruleIDForSignalType maps a signal type to its canonical terrain ruleID,
// using the manifest — the same mapping writeFindingsJSON uses, so the gate
// and the artifact agree on rule identity.
func ruleIDForSignalType() func(models.SignalType) string {
	typeToRuleID := map[models.SignalType]string{}
	for _, entry := range signals.Manifest() {
		if entry.RuleID != "" {
			typeToRuleID[entry.Type] = entry.RuleID
		}
	}
	return func(t models.SignalType) string { return typeToRuleID[t] }
}

// alwaysGate is the set of signal types that ALWAYS fail CI regardless of the
// trust floor — blocking on them is never a false-positive risk an auto-fix
// would justify. Membership is restricted to DETERMINISTIC failures/regressions,
// exact security/safety detections, and user-authored policy. Heuristic
// detectors (e.g. substring-based data-leakage guesses) are deliberately NOT
// here — always-gating a heuristic would break builds on a false positive, the
// exact thing the trust floor exists to prevent. A build must never silently
// pass one of these.
var alwaysGate = map[models.SignalType]bool{
	// Definite failures / regressions (deterministic; the regressions need a
	// recorded baseline to fire, so they can't false-fire from nothing).
	signals.SignalTestFailed:            true,
	signals.SignalEvalFailure:           true,
	signals.SignalEvalRegression:        true,
	signals.SignalPassRateDrop:          true,
	signals.SignalPerformanceRegression: true,
	// Security / safety leaks — an exact detection must block (opt-in detectors;
	// the security bias favours failing CI over letting a real leak through).
	signals.SignalSecretsInPrompt:        true,
	signals.SignalPIIInEval:              true,
	signals.SignalInsecureDeserialize:    true,
	signals.SignalSafetyFailure:          true,
	signals.SignalToolGuardrailViolation: true,
	// User-authored policy — the user explicitly configured it to fail CI.
	signals.SignalPolicyViolation:   true,
	signals.SignalAIPolicyViolation: true,
	// NOTE: targetLeakage / dataLeakageSuspected are intentionally EXCLUDED —
	// they are substring/pattern heuristics; always-gating them would break CI
	// on a false positive. They remain trust-floor-governed. snapshotMismatch is
	// excluded too (it ships at observability tier, so it never gates anyway).
}

// trustFloorApplies reports whether the trust floor's "needs a validated fix to
// block CI" rule governs this signal. It governs the heuristic detectors where a
// false positive would wrongly break a build (the risk the trust floor exists to
// manage), but NEVER a Critical finding or an alwaysGate type — those fail CI
// regardless, because "we couldn't auto-fix it" is not a reason to pass a
// Critical, a leaked secret, a failing test, or the user's own policy.
func trustFloorApplies(s models.Signal) bool {
	if s.Severity == models.SeverityCritical {
		return false
	}
	if alwaysGate[s.Type] {
		return false
	}
	return true
}

// gateBlockable returns the predicate every CI-blocking surface must apply to
// decide whether a signal may fail the build. A gate-relevant signal blocks
// when EITHER the trust floor does not govern it (see trustFloorApplies) OR its
// remediation is closed-loop validated. Returns nil when the trust floor is off,
// meaning "gate-relevance alone decides" (the caller's IsGateRelevant stands).
//
// This is the ONE definition of "may block CI"; analyze, test, report pr, and
// the required check-run all route through it so they never disagree.
func gateBlockable(root string, trustFloor bool) func(models.Signal) bool {
	if !trustFloor {
		return nil
	}
	lookup := ruleIDForSignalType()
	fixReg := defaultFixRegistry()
	vReg := remediate.DefaultValidityRegistry()
	return func(s models.Signal) bool {
		if !signals.IsGateRelevant(s.Type) {
			return false
		}
		if !trustFloorApplies(s) {
			return true // deterministic / Critical / user-policy: gates on severity
		}
		f := findings.FromSignal(s, lookup(s.Type))
		fs := []findings.Finding{f}
		fixReg.Attach(root, fs)
		return remediate.GateEligible(fs[0], vReg)
	}
}

// trustFloorGateBreakdown recomputes the gate-relevant severity breakdown
// under the trust floor: a gate-relevant signal counts toward `--fail-on`
// only when its finding carries a closed-loop-validated remediation. Signals
// whose remediation is unproven or judge-only are excluded — they surface in
// the report but cannot block CI. This is the remediation-validity axis
// applied to the gate decision.
func trustFloorGateBreakdown(root string, sigs []models.Signal) analyze.SignalBreakdown {
	blockable := gateBlockable(root, true)
	severities := make([]string, 0, len(sigs))
	for _, s := range sigs {
		if !blockable(s) {
			continue
		}
		severities = append(severities, string(s.Severity))
	}
	return prSeverityBreakdown(severities)
}
