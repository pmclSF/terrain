package aidetect

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// PromptFileMissingEvalDetector flags AI/ML surfaces (prompt, agent, tool,
// model call, training script) that have NO eval scenario covering them
// at all — the strategy-aligned headline detector.
//
// This is the broader, "boundary class" variant of SafetyEvalMissingDetector:
//   - safety_eval_missing requires a *safety-shaped* eval missing
//   - surface_missing_eval requires *any* eval missing
//
// Corpus evidence (tier-4/corpus-2000-summary.md, 2026-05-12):
// 18,761 `surface_missing_eval` edges across 2000 AI/ML repos vs 138
// `eval_covers_surface` edges. The 136:1 ratio in the wild OSS AI/ML
// codebase is the empirical case for this detector as the headline of
// Terrain's product strategy.
//
// Detection logic mirrors safety_eval_missing's structure:
//   1. Walk every CodeSurface whose Kind is an AI/ML surface kind.
//   2. For each surface, check whether ANY eval (regardless of category)
//      covers it — explicitly (CoveredSurfaceIDs) or implicitly (same
//      top-level directory).
//   3. Emit one signal per surface that has no covering eval.
//
// Severity ladder:
//   - LLM call sites + Agent + ToolDef: High (most exposure)
//   - Prompt + Context: Medium (data-contract drift risk)
//   - Model (training-script surfaces detected by ML pipelines): Medium
type PromptFileMissingEvalDetector struct{}

// aiSurfaceKinds is the set of CodeSurface kinds that the boundary-
// detector treats as AI/ML surfaces requiring eval coverage.
var aiSurfaceKinds = map[models.CodeSurfaceKind]bool{
	models.SurfacePrompt:  true,
	models.SurfaceAgent:   true,
	models.SurfaceToolDef: true,
	models.SurfaceContext: true,
	models.SurfaceModel:   true,
}

// surfaceMissingEvalSeverity maps a surface kind to the default
// severity for a missing-eval finding on that kind.
func surfaceMissingEvalSeverity(k models.CodeSurfaceKind) models.SignalSeverity {
	switch k {
	case models.SurfaceAgent, models.SurfaceToolDef:
		return models.SeverityHigh
	case models.SurfacePrompt, models.SurfaceContext, models.SurfaceModel:
		return models.SeverityMedium
	default:
		return models.SeverityLow
	}
}

// Detect emits SignalPromptFileMissingEval for each AI/ML surface with no
// covering eval (any kind, not just safety-shaped).
func (d *PromptFileMissingEvalDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}

	// Index every covered surface. Two paths — same as safety_eval_missing:
	//   1. Explicit: eval.CoveredSurfaceIDs lists surface IDs.
	//   2. Implicit: eval sits in an eval-directory shape with empty
	//      CoveredSurfaceIDs. Treat such evals as covering all AI/ML
	//      surfaces under the same top-level directory.
	coveredSurfaces := map[string]bool{}
	coveredDirs := map[string]bool{}
	for _, ev := range snap.Evals {
		if len(ev.CoveredSurfaceIDs) > 0 {
			for _, sid := range ev.CoveredSurfaceIDs {
				coveredSurfaces[sid] = true
			}
			continue
		}
		if ev.Path == "" {
			continue
		}
		if dir := topLevelDir(ev.Path); dir != "" {
			coveredDirs[dir] = true
		}
	}

	var out []models.Signal
	for _, surface := range snap.CodeSurfaces {
		if !aiSurfaceKinds[surface.Kind] {
			continue
		}
		if coveredSurfaces[surface.SurfaceID] {
			continue
		}
		if dir := topLevelDir(surface.Path); dir != "" && coveredDirs[dir] {
			continue
		}

		sev := surfaceMissingEvalSeverity(surface.Kind)
		out = append(out, models.Signal{
			Type:       signals.SignalPromptFileMissingEval,
			Category:   models.CategoryAI,
			Severity:   sev,
			Confidence: 0.7,
			Location: models.SignalLocation{
				File:   surface.Path,
				Symbol: surface.Name,
			},
			Explanation: "AI/ML surface `" + surface.Name + "` (kind=" + string(surface.Kind) +
				") has no eval scenario covering it. In a 2000-repo OSS corpus this gap appears " +
				"in 136 of every 137 detected surfaces — the dominant AI testing failure mode.",
			SuggestedAction: "Add an eval scenario (promptfoo, DeepEval, Ragas, or a framework-specific " +
				"format) that exercises this surface. Run `terrain ai list` to see what other surfaces " +
				"in this repo are uncovered.",
			Actionability:    models.ActionabilityScheduled,
			LifecycleStages:  []models.LifecycleStage{models.StageDesign, models.StageTestAuthoring},
			AIRelevance:      models.AIRelevanceHigh,
			RuleID:           "terrain/ai/surface-missing-eval",
			RuleURI:          "docs/rules/ai/surface-missing-eval.md",
			DetectorVersion:  "0.2.0",
			EvidenceSource:   models.SourceGraphTraversal,
			EvidenceStrength: models.EvidenceModerate,
			ConfidenceDetail: &models.ConfidenceDetail{
				Value:        0.7,
				IntervalLow:  0.55,
				IntervalHigh: 0.85,
				Quality:      "heuristic",
				Sources:      []models.EvidenceSource{models.SourceGraphTraversal},
			},
			Metadata: map[string]any{
				"surfaceId":   surface.SurfaceID,
				"surfaceKind": string(surface.Kind),
			},
		})
	}
	return out
}
