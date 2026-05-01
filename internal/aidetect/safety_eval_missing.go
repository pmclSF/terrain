package aidetect

import (
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// SafetyEvalMissingDetector flags AI surfaces (prompt / agent / tool
// definition) that have no eval scenario covering the documented
// safety category (jailbreak / harm / leak / abuse / pii).
//
// Detection logic:
//
//   1. Walk every CodeSurface whose Kind is in safetyCriticalSurfaceKinds.
//   2. For each surface, check whether ANY scenario in the snapshot
//      covers it AND has a safety-shaped category or name.
//   3. Emit one signal per surface that lacks safety coverage.
//
// "Safety-shaped" is matched against the scenario's Category, Name,
// and Description to allow projects that don't standardise on a
// `category: safety` field. The match list lives in
// safetyCategoryMarkers.
type SafetyEvalMissingDetector struct{}

var safetyCriticalSurfaceKinds = map[models.CodeSurfaceKind]bool{
	models.SurfacePrompt:   true,
	models.SurfaceAgent:    true,
	models.SurfaceToolDef:  true,
	models.SurfaceContext:  true,
}

// safetyCategoryMarkers are case-insensitive substrings that indicate
// a scenario is exercising a safety concern. We're generous about
// matching here — a project saying "adversarial" or "jailbreak" or
// "harm" all count.
var safetyCategoryMarkers = []string{
	"safety", "jailbreak", "adversarial", "harm", "abuse",
	"injection", "leak", "pii", "redteam", "red-team", "red_team",
	"abuse", "toxic", "policy_violation",
}

// Detect emits SignalAISafetyEvalMissing for each safety-critical
// surface that has no safety-shaped scenario covering it.
func (d *SafetyEvalMissingDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}

	// Index scenarios by the surface IDs they cover, for surfaces
	// with at least one safety-shaped scenario.
	safelyCoveredSurfaces := map[string]bool{}
	for _, sc := range snap.Scenarios {
		if !scenarioLooksSafety(sc) {
			continue
		}
		for _, sid := range sc.CoveredSurfaceIDs {
			safelyCoveredSurfaces[sid] = true
		}
	}

	var out []models.Signal
	for _, surface := range snap.CodeSurfaces {
		if !safetyCriticalSurfaceKinds[surface.Kind] {
			continue
		}
		if safelyCoveredSurfaces[surface.SurfaceID] {
			continue
		}
		out = append(out, models.Signal{
			Type:        signals.SignalAISafetyEvalMissing,
			Category:    models.CategoryAI,
			Severity:    models.SeverityHigh,
			Confidence:  0.82,
			Location:    models.SignalLocation{File: surface.Path, Symbol: surface.Name},
			Explanation: "Surface `" + surface.Name + "` (kind=" + string(surface.Kind) + ") has no eval scenario covering a safety category (jailbreak / harm / injection / leak / pii).",
			SuggestedAction: "Add a scenario tagged with `category: safety` (or jailbreak / adversarial / harm) that exercises this surface, then re-run the eval gauntlet.",

			SeverityClauses: []string{"sev-high-004"},
			Actionability:   models.ActionabilityScheduled,
			LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageTestAuthoring},
			AIRelevance:     models.AIRelevanceHigh,
			RuleID:          "TER-AI-100",
			RuleURI:         "docs/rules/ai/safety-eval-missing.md",
			DetectorVersion: "0.2.0",
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.82,
				IntervalLow:  0.7,
				IntervalHigh: 0.9,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceStructuralPattern, models.SourceGraphTraversal},
			},
			EvidenceSource:   models.SourceGraphTraversal,
			EvidenceStrength: models.EvidenceModerate,
			Metadata: map[string]any{
				"surfaceId":   surface.SurfaceID,
				"surfaceKind": string(surface.Kind),
			},
		})
	}
	return out
}

// scenarioLooksSafety returns true when the scenario's Category, Name,
// or Description contains a safety-shaped marker.
func scenarioLooksSafety(sc models.Scenario) bool {
	hay := strings.ToLower(sc.Category + " " + sc.Name + " " + sc.Description)
	for _, m := range safetyCategoryMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}
