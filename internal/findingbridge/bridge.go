// Package findingbridge converts an aipipeline.Finding (the scaffold-bearing
// output of the AI composer) into the canonical findings.Finding shape,
// carrying any fix scaffold onto the resulting Suggestion.Fix so it appears
// in findings.json alongside signal-detector findings.
package findingbridge

import (
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/identity"
)

// docsBaseURL is the absolute rule-doc URL prefix, matching the base
// findings.FromSignal emits so every finding source (signal detectors
// and the AI pipeline) presents the same absolute, schema-valid
// docs_url.
const docsBaseURL = "https://github.com/pmclSF/terrain/blob/main/docs/rules/"

// FromAIPipeline converts one aipipeline.Finding into a findings.Finding.
//
// ruleID is the canonical terrain/<category>/<rule-name> id, supplied by
// the caller (mirroring findings.FromSignal): aipipeline carries a dotted
// rule id in a different vocabulary, and the canonical id lives with the
// detector. When ruleID is empty, NormalizeRuleID(f.RuleID) is used as a
// deterministic fallback.
//
// A non-empty FixScaffold/FixScaffoldPath becomes a Suggestion bearing a
// structured new_file Fix, so the remediation lands on the canonical
// findings.Finding artifact.
func FromAIPipeline(f aipipeline.Finding, ruleID string) findings.Finding {
	if ruleID == "" {
		ruleID = NormalizeRuleID(f.RuleID)
	}
	if ruleID == "" {
		// An unmappable AI rule id (empty after normalization) would
		// otherwise yield a schema-invalid finding (empty rule_id and a
		// bare "docs/rules/.md" docs_url). Fall back to a stable,
		// valid canonical id so the artifact is always well-formed.
		ruleID = "terrain/ai/unmapped"
	}

	line := firstAtomLine(f)
	out := findings.Finding{
		Version:      findings.SchemaVersion,
		RuleID:       ruleID,
		FindingID:    identity.BuildFindingID(ruleID, f.Path, "", line),
		Severity:     severityFromAI(f.Severity),
		PrimaryLoc:   findings.Location{Path: f.Path, Line: line},
		ShortMessage: shortMessage(ruleID, f.Path),
		DocsURL:      docsBaseURL + relRuleDoc(ruleID) + ".md",
		Metadata:     aiMetadata(f),
	}
	if sg := scaffoldSuggestion(f); sg != nil {
		out.Suggestions = []findings.Suggestion{*sg}
	}
	return out
}

// scaffoldSuggestion turns the finding's fix scaffold into a structured,
// mechanically-applicable Suggestion. Returns nil when the rule shipped no
// scaffold (the finding is then diagnostic-only on the canonical surface).
func scaffoldSuggestion(f aipipeline.Finding) *findings.Suggestion {
	if f.FixScaffold == "" || f.FixScaffoldPath == "" {
		return nil
	}
	return &findings.Suggestion{
		Text:      "Create " + f.FixScaffoldPath + " to resolve this finding.",
		AppliesTo: &findings.Location{Path: f.FixScaffoldPath},
		Fix: &findings.Fix{
			Kind:    findings.FixNewFile,
			Path:    f.FixScaffoldPath,
			Content: f.FixScaffold,
		},
	}
}

// severityFromAI maps the AI composer's severity vocabulary
// (low/medium/high/critical) onto the artifact vocabulary
// (notice/warning/error), matching findings.severityFromSignal: critical
// and high are gate-blocking errors, medium is a warning, low is a notice.
func severityFromAI(s aipipeline.Severity) findings.Severity {
	switch s {
	case aipipeline.SeverityCritical, aipipeline.SeverityHigh:
		return findings.SeverityError
	case aipipeline.SeverityMedium:
		return findings.SeverityWarning
	default:
		return findings.SeverityNotice
	}
}

// firstAtomLine recovers a line number for the file-level aipipeline.Finding
// from the first evidence atom that carries one. aipipeline.Finding is
// file-grained; the line lives on atom spans.
func firstAtomLine(f aipipeline.Finding) int {
	for _, a := range f.Atoms {
		if a.Span.Line > 0 {
			return a.Span.Line
		}
	}
	return 0
}

// aiMetadata preserves the composer's calibration signal so the canonical
// artifact does not lose the confidence/cohort/suppression context.
func aiMetadata(f aipipeline.Finding) map[string]any {
	m := map[string]any{
		"confidence": f.Confidence,
		"log_odds":   f.LogOdds,
	}
	if f.Cohort != "" {
		m["cohort"] = f.Cohort
	}
	if f.Suppressed {
		m["suppressed"] = true
		if f.SuppressedReason != "" {
			m["suppressed_reason"] = f.SuppressedReason
		}
	}
	return m
}

// shortMessage synthesizes a single-line summary. aipipeline.Finding has no
// human explanation field, so until the AI detectors carry one, the
// canonical message is derived from the rule and path.
func shortMessage(ruleID, path string) string {
	name := strings.TrimPrefix(ruleID, "terrain/")
	if path == "" {
		return name
	}
	return name + ": " + path
}

// relRuleDoc strips the canonical "terrain/" prefix to form the rule-doc
// path suffix, matching the convention in findings.FromSignal.
func relRuleDoc(ruleID string) string {
	return strings.TrimPrefix(ruleID, "terrain/")
}

// NormalizeRuleID converts a dotted aipipeline rule id
// ("ai.surface.missing_eval") into the canonical terrain form
// ("terrain/ai/surface-missing-eval"): the first segment is the category,
// the remainder is the rule name with dots and underscores folded to
// hyphens. Callers with an explicit mapping should pass ruleID directly to
// FromAIPipeline instead; this is the deterministic fallback.
func NormalizeRuleID(dotted string) string {
	dotted = strings.TrimSpace(dotted)
	if dotted == "" {
		return ""
	}
	category, rest, found := strings.Cut(dotted, ".")
	if !found {
		// No category segment; place it under "ai" as a safe default.
		return "terrain/ai/" + hyphenate(dotted)
	}
	return "terrain/" + hyphenate(category) + "/" + hyphenate(rest)
}

// hyphenate folds dots and underscores to hyphens and lowercases, producing
// the [a-z0-9-] alphabet validRuleID accepts.
func hyphenate(s string) string {
	return strings.ToLower(strings.NewReplacer(".", "-", "_", "-").Replace(s))
}
