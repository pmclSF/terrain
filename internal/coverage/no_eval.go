package coverage

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectNoEvalForAISurface walks AI-typed CodeSurfaces and emits a
// Signal for any surface that no Eval claims to cover. Implements
// terrain/coverage/no-eval.
//
// "AI-typed" means a CodeSurface whose Kind is one of:
//   - SurfacePrompt
//   - SurfaceContext
//   - SurfaceDataset
//   - SurfaceToolDef
//   - SurfaceRetrieval
//   - SurfaceAgent
//   - SurfaceEvalDef
//   - SurfaceModel  (added in Tier 1)
//
// The eval-side coverage check uses Eval.CoveredSurfaceIDs.
//
// Severity defaults to high — uncovered AI surface means model
// behavior can shift without any eval surfacing the change.
func DetectNoEvalForAISurface(snap *models.TestSuiteSnapshot) []models.Signal {
	if snap == nil {
		return nil
	}

	covered := buildEvalCoverageIndex(snap.Evals)

	var out []models.Signal
	for _, cs := range snap.CodeSurfaces {
		if !isAISurface(cs.Kind) {
			continue
		}
		if covered[cs.SurfaceID] {
			continue
		}
		out = append(out, models.Signal{
			Type:             signals.SignalNoEvalForAISurface,
			Category:         models.CategoryAI,
			Severity:         models.SeverityHigh,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location: models.SignalLocation{
				File:   cs.Path,
				Symbol: cs.Name,
			},
			Explanation: fmt.Sprintf(
				"AI surface %q (kind=%s) in %s has no Eval that exercises it. Model behavior on this surface can shift in production without any eval surfacing the regression.",
				cs.Name, cs.Kind, cs.Path,
			),
			SuggestedAction: fmt.Sprintf(
				"Add an eval scenario that exercises %q and asserts on its output / output-shape / metric.",
				cs.Name,
			),
			RuleID:          "terrain/coverage/no-eval",
			RuleURI:         "docs/rules/coverage/no-eval.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"surfaceId":   cs.SurfaceID,
				"surfaceKind": string(cs.Kind),
			},
		})
	}
	return out
}

func buildEvalCoverageIndex(evals []models.Eval) map[string]bool {
	idx := make(map[string]bool)
	for _, e := range evals {
		for _, sid := range e.CoveredSurfaceIDs {
			idx[sid] = true
		}
	}
	return idx
}

func isAISurface(k models.CodeSurfaceKind) bool {
	switch k {
	case models.SurfacePrompt, models.SurfaceContext,
		models.SurfaceDataset, models.SurfaceToolDef,
		models.SurfaceRetrieval, models.SurfaceAgent,
		models.SurfaceEvalDef, models.SurfaceModel:
		return true
	}
	return false
}
