// Package preview implements the §9 preview-tier detectors. These
// rules ship default-off and are pending LB-5 / LB-6 calibration on
// the dogfood corpus before promotion to Stable. The detection logic
// here is the minimum needed to surface the pattern; thresholds and
// edge-case handling refine as calibration data arrives.
package preview

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// signal is a tiny helper for the compact one-shape-per-rule detectors
// in this package. Each detector emits Signals with consistent
// metadata layout.
func signal(t models.SignalType, severity models.SignalSeverity, ruleID, docsURI string, loc models.SignalLocation, explanation, action string, metadata map[string]any) models.Signal {
	return models.Signal{
		Type:             t,
		Category:         models.CategoryAI,
		Severity:         severity,
		Confidence:       0.75,
		EvidenceStrength: models.EvidenceModerate,
		EvidenceSource:   models.SourceStructuralPattern,
		Location:         loc,
		Explanation:      explanation,
		SuggestedAction:  action,
		RuleID:           ruleID,
		RuleURI:          docsURI,
		DetectorVersion:  "0.2.0-preview",
		Metadata:         metadata,
	}
}

// --- evals coverage ---

// DetectOrphanedEval fires when an Eval has empty CoveredSurfaceIDs.
// Implements terrain/coverage/orphaned-eval.
func DetectOrphanedEval(evals []models.Eval) []models.Signal {
	var out []models.Signal
	for _, e := range evals {
		if len(e.CoveredSurfaceIDs) > 0 {
			continue
		}
		out = append(out, signal(
			signals.SignalOrphanedEval, models.SeverityLow,
			"terrain/coverage/orphaned-eval",
			"docs/rules/coverage/orphaned-eval.md",
			models.SignalLocation{File: e.Path, Symbol: e.Name},
			"Eval "+e.Name+" references no AI surface. The eval runs but contributes nothing to coverage tracking.",
			"Add coveredSurfaceIds to the eval YAML or move the eval out of evals/ if it's a fixture, not a real eval.",
			map[string]any{"evalId": e.EvalID, "framework": e.Framework},
		))
	}
	return out
}

// DetectMissingEvalCategories fires when the eval suite has no
// adversarial / edge_case / safety categories despite having
// happy_path coverage. Implements terrain/coverage/missing-eval-categories.
func DetectMissingEvalCategories(evals []models.Eval) []models.Signal {
	if len(evals) == 0 {
		return nil
	}
	have := map[string]bool{}
	for _, e := range evals {
		have[e.Category] = true
	}
	// Only fire when the suite has SOME categorized evals — otherwise
	// a brand-new project's empty eval-categories field is signal
	// noise, not missing-category coverage.
	if !have["happy_path"] && !have["accuracy"] {
		return nil
	}
	missing := []string{}
	for _, want := range []string{"adversarial", "edge_case", "safety"} {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []models.Signal{signal(
		signals.SignalMissingEvalCategories, models.SeverityLow,
		"terrain/coverage/missing-eval-categories",
		"docs/rules/coverage/missing-eval-categories.md",
		models.SignalLocation{File: "evals/"},
		"Eval suite has happy_path coverage but no "+joinList(missing)+" categories.",
		"Add adversarial / edge_case / safety scenarios. Tag each eval with its category in the YAML so coverage tracking can surface the breakdown.",
		map[string]any{"missing": missing},
	)}
}

func joinList(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	case 2:
		return xs[0] + " or " + xs[1]
	}
	out := ""
	for i, x := range xs {
		switch {
		case i == 0:
			out = x
		case i == len(xs)-1:
			out += ", or " + x
		default:
			out += ", " + x
		}
	}
	return out
}
