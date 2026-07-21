package remediate

import "github.com/pmclSF/terrain/internal/findings"

// ValidityRegistry records which (rule, fix-kind) remediations have cleared
// the closed-loop bar — i.e. applying the fix has been shown to resolve the
// finding with no regressions. A finding may block CI only if its remediation
// can be proven to resolve the finding, not merely advised.
//
// Keyed by canonical ruleID + fix kind so it is pipeline-agnostic (both the AI
// composer and the signal detectors land on findings.Finding). The seed below
// reflects rules with a passing in-repo closed-loop test; new entries are
// added as detector families are taken through the loop.
type ValidityRegistry struct {
	validated map[string]bool
}

func validityKey(ruleID string, kind findings.FixKind) string {
	return ruleID + "\x00" + string(kind)
}

// Validated reports whether the (ruleID, kind) remediation has cleared the
// closed loop.
func (r *ValidityRegistry) Validated(ruleID string, kind findings.FixKind) bool {
	if r == nil {
		return false
	}
	return r.validated[validityKey(ruleID, kind)]
}

// MarkValidated records a (ruleID, kind) pair as closed-loop validated.
func (r *ValidityRegistry) MarkValidated(ruleID string, kind findings.FixKind) {
	if r.validated == nil {
		r.validated = map[string]bool{}
	}
	r.validated[validityKey(ruleID, kind)] = true
}

// DefaultValidityRegistry seeds the registry from the remediations proven by
// the in-repo closed-loop tests. New entries are added here as detector
// families are taken through the loop.
func DefaultValidityRegistry() *ValidityRegistry {
	r := &ValidityRegistry{}
	r.MarkValidated("terrain/ai/surface-missing-eval", findings.FixNewFile)
	r.MarkValidated("terrain/deps/drift-risk", findings.FixEditInPlace)
	// Proven by TestPromptSchemaDrift_RemediationClosesTheLoop: the correct-side
	// prompt-reference correction clears the drift with no regressions.
	r.MarkValidated("terrain/ai/prompt-schema-drift", findings.FixEditInPlace)
	return r
}

// GateMetadataKey marks a finding that was demoted from gate-blocking to
// observability because its remediation is not closed-loop validated.
const GateMetadataKey = "remediation_unvalidated"

// EnforceGate applies the remediation-validity axis to a finding set in
// place: any gate-blocking finding (SeverityError) whose remediation has not
// been closed-loop validated is demoted to observability (SeverityWarning)
// and annotated. A finding is gate-eligible only when it carries a
// structured Fix whose (ruleID, kind) is in reg — judge-only findings (no
// Fix) are never gate-blocking until the judge fallback validates them.
//
// Returns the number of findings demoted.
func EnforceGate(fs []findings.Finding, reg *ValidityRegistry) int {
	demoted := 0
	for i := range fs {
		if fs[i].Severity != findings.SeverityError {
			continue
		}
		if GateEligible(fs[i], reg) {
			continue
		}
		fs[i].Severity = findings.SeverityWarning
		if fs[i].Metadata == nil {
			fs[i].Metadata = map[string]any{}
		}
		fs[i].Metadata[GateMetadataKey] = true
		demoted++
	}
	return demoted
}

// GateEligible reports whether a finding's remediation has been closed-loop
// validated and may therefore block CI: it must carry a structured Fix whose
// (ruleID, kind) is in reg. Judge-only findings (no Fix) are never eligible.
func GateEligible(f findings.Finding, reg *ValidityRegistry) bool {
	fix := firstFix(f)
	if fix == nil {
		return false
	}
	return reg.Validated(f.RuleID, fix.Kind)
}
