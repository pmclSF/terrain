package aidetect

import (
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/ehr"
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/surfacelit"
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
type SafetyEvalMissingDetector struct {
	// Root is the repository root used to resolve surface paths for
	// the surface_literal_presence_gate mechanism. Empty defaults to
	// "."; the gate is a no-op when paths can't be read.
	Root string
}

// ehrKindFor maps the snapshot's CodeSurfaceKind to the ehr package's
// SurfaceKind. The ehr taxonomy is narrower (prompt/model/dataset);
// anything else falls back to "prompt" since the gate's job is to
// check whether a same-named surface appears in the eval's report.
func ehrKindFor(k models.CodeSurfaceKind) ehr.SurfaceKind {
	switch k {
	case models.SurfaceModel:
		return ehr.SurfaceModel
	case models.SurfaceDataset:
		return ehr.SurfaceDataset
	default:
		return ehr.SurfacePrompt
	}
}

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

	// Index scenarios by the surface IDs they cover, for scenarios
	// that look safety-shaped. Two paths:
	//
	//   1. Explicit: scenario.CoveredSurfaceIDs lists surface IDs.
	//   2. Implicit: scenario sits in an eval directory with empty
	//      CoveredSurfaceIDs (the common shape produced by
	//      DeriveEvals). Pre-0.2.x this case caused the detector
	//      to flood false positives on every safety-critical surface
	//      in repos using auto-derived scenarios — the default path.
	//      We now treat such scenarios as covering all
	//      safety-critical surfaces under the same top-level path
	//      directory as the scenario.
	safelyCoveredSurfaces := map[string]bool{}
	safelyCoveredDirs := map[string]bool{}
	for _, sc := range snap.Evals {
		if !scenarioLooksSafety(sc) {
			continue
		}
		if len(sc.CoveredSurfaceIDs) > 0 {
			for _, sid := range sc.CoveredSurfaceIDs {
				safelyCoveredSurfaces[sid] = true
			}
			continue
		}
		// Implicit path-based coverage — the scenario doesn't list
		// surface IDs, so any same-directory safety-critical surface
		// is treated as covered.
		if sc.Path == "" {
			continue
		}
		dir := topLevelDir(sc.Path)
		if dir != "" {
			safelyCoveredDirs[dir] = true
		}
	}

	// Mechanism gate: ehr_surfaces_covered.
	// Build ehr reports once per scan from eval config paths so the
	// gate can override the coarse safelyCoveredDirs heuristic on a
	// per-surface basis.
	var evalReports []*ehr.Report
	{
		root := d.Root
		if root == "" {
			root = "."
		}
		for _, sc := range snap.Evals {
			if !scenarioLooksSafety(sc) || sc.Path == "" {
				continue
			}
			rep, err := ehr.Recognize(filepath.Join(root, sc.Path))
			if err == nil && rep != nil {
				evalReports = append(evalReports, rep)
			}
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
		dirHit := false
		if dir := topLevelDir(surface.Path); dir != "" && safelyCoveredDirs[dir] {
			dirHit = true
		}
		// Mechanism gate: ehr_surfaces_covered. When ON, the per-eval
		// surface report can override the coarse dirHit heuristic. The
		// gate lifts suppression only when the eval doesn't actually
		// cover this specific surface name.
		root := d.Root
		if root == "" {
			root = "."
		}
		abs := filepath.Join(root, surface.Path)
		keep := ehr.GateSuppression(
			mechanisms.Default(), evalReports,
			ehrKindFor(surface.Kind),
			surface.Name, "aiSafetyEvalMissing", surface.Path,
			dirHit,
		)
		if !keep {
			continue
		}
		// Mechanism gate: surface_literal_presence_gate.
		if dec := surfacelit.Gate(mechanisms.Default(), surface.Name, abs, "aiSafetyEvalMissing"); !dec.Keep {
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
			RuleID:          "terrain/ai/safety-eval-missing",
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
func scenarioLooksSafety(sc models.Eval) bool {
	hay := strings.ToLower(sc.Category + " " + sc.Name + " " + sc.Description)
	for _, m := range safetyCategoryMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}

// topLevelDir returns the first directory segment of a repo-relative
// path (e.g. "internal/aidetect/foo.go" → "internal"). Used to
// approximate "same package" for implicit safety-coverage attribution
// when a scenario doesn't list specific surface IDs.
func topLevelDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	for i, c := range p {
		if c == '/' || c == '\\' {
			if i == 0 {
				continue
			}
			return p[:i]
		}
	}
	return ""
}
