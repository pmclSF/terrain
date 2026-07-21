package findings

import (
	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/models"
)

// FromSignal converts a snapshot Signal into the canonical Finding
// shape. Used by surfaces that re-emit pipeline output as artifacts
// (e.g., `terrain test --selector <rule>` filters the snapshot and
// renders JUnit / Step Summary via the Finding shape).
//
// ruleID is supplied by the caller because the Signal carries only
// the Type; the canonical RuleID lives in the manifest entry.
func FromSignal(s models.Signal, ruleID string) Finding {
	return Finding{
		Version:      1,
		RuleID:       ruleID,
		FindingID:    identity.BuildFindingID(string(s.Type), s.Location.File, s.Location.Symbol, s.Location.Line),
		Severity:     severityFromSignal(s.Severity),
		PrimaryLoc:   Location{Path: s.Location.File, Line: s.Location.Line},
		ShortMessage: shortMessage(s),
		LongMessage:  s.Explanation,
		Suggestions:  suggestionsFromSignal(s),
		DocsURL:      "https://github.com/pmclSF/terrain/blob/main/docs/rules/" + relRuleDoc(ruleID) + ".md",
		Metadata:     copyMetadata(s.Metadata),
	}
}

// suggestionsFromSignal lifts the detector's SuggestedAction onto the
// canonical finding. Historically this text was dropped in conversion, so
// the user-facing artifact carried no remediation at all. The text-only
// Suggestion is the judge-only floor; detectors that emit a structured,
// mechanically-applicable Fix populate Suggestion.Fix separately upstream.
func suggestionsFromSignal(s models.Signal) []Suggestion {
	if s.SuggestedAction == "" {
		return nil
	}
	sg := Suggestion{Text: s.SuggestedAction}
	if s.Location.File != "" {
		sg.AppliesTo = &Location{Path: s.Location.File, Line: s.Location.Line}
	}
	return []Suggestion{sg}
}

// FromSignals is a convenience over a slice; ruleIDLookup maps
// signal.Type → ruleID (provided by the caller from the manifest).
func FromSignals(signals []models.Signal, ruleIDLookup func(models.SignalType) string) []Finding {
	if len(signals) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(signals))
	for _, s := range signals {
		ruleID := ""
		if ruleIDLookup != nil {
			ruleID = ruleIDLookup(s.Type)
		}
		out = append(out, FromSignal(s, ruleID))
	}
	return out
}

// severityFromSignal maps the snapshot Severity vocabulary
// (Critical/High/Medium/Low/Info) to the artifact Severity vocabulary
// (error/warning/notice). Critical+High map to error; Medium maps to
// warning; Low+Info map to notice.
func severityFromSignal(sev models.SignalSeverity) Severity {
	switch sev {
	case models.SeverityCritical, models.SeverityHigh:
		return SeverityError
	case models.SeverityMedium:
		return SeverityWarning
	default:
		return SeverityNotice
	}
}

// shortMessage produces the single-line summary from a Signal. Prefers
// Explanation truncated to ~140 chars; falls back to "<Type> finding".
func shortMessage(s models.Signal) string {
	if s.Explanation == "" {
		return string(s.Type) + " finding"
	}
	r := []rune(s.Explanation)
	if len(r) <= 140 {
		return s.Explanation
	}
	return string(r[:137]) + "..."
}

// relRuleDoc returns the rule-doc path suffix for a ruleID. Input
// like "terrain/ai/prompt-injection-risk" → "ai/prompt-injection-risk".
func relRuleDoc(ruleID string) string {
	const prefix = "terrain/"
	if len(ruleID) > len(prefix) && ruleID[:len(prefix)] == prefix {
		return ruleID[len(prefix):]
	}
	return ruleID
}

func copyMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
