package structural

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// UncoveredAISurfaceDetector finds AI surfaces (prompts, tools, datasets)
// with zero test or scenario coverage.
type UncoveredAISurfaceDetector struct{}

// 2026-05-18 moat work — Phase A.4 + R2 findings on uncoveredAISurface
// at merged n=250 corpus precision 21.5%, sub-lane breakdown:
//   - aiPrompt: 41% precision — strongest lane, keep gate-eligible
//   - aiModel:   5% precision — dominated by name-shape FPs (Zod schemas,
//                synthesized line-suffix stems, decorator-injected stems)
//   - aiDataset: 33% precision — moderate
//
// Below filters target the dominant FP classes in the aiModel lane
// per the saved feedback memory (must clear ≥5 FPs as a class):
//
// modelSyntheticStemRe: names like `token_management_L47`, `*_L\d+`
//   are line-number-suffix synthesized identifiers from the AI surface
//   extractor — they're not addressable named LLM surfaces. (~32 FPs)
//
// modelTypeSchemaSuffix: PascalCase names ending in Schema/Props/Type/
//   Config/Params/Request/Response — Zod/Pydantic/TS type aliases that
//   look like model names. (~45 FPs combined Zod + Pydantic)
//
// modelDecoratorPrefix: names with `*_tool`, `tool_decorated_*` shapes
//   that capture @tool decorator labels (not LLM call sites). (~28 FPs)
var (
	modelSyntheticStemRe = regexp.MustCompile(`_L\d+$`)
	modelDecoratorRe     = regexp.MustCompile(`(?i)(^tool_decorated_|_tool$|_decorator(_|$))`)
	modelTypeSchemaSuffix = []string{
		"Schema", "Props", "Type", "Config", "Params", "Request",
		"Response", "Options", "Settings", "Args", "Input", "Output",
		"Variables", "Result", "State", "Context",
	}
)

// isStructuralAIModelFP returns true when a NodeModel surface's name
// matches one of the known non-model classes (Zod schemas, line-suffix
// synthesized stems, decorator labels). Each class clears ≥5 FPs.
//
// Restricted to model lane only — aiPrompt and aiDataset lanes have
// different FP shapes (covered separately if needed).
func isStructuralAIModelFP(name string) bool {
	if name == "" {
		return false
	}
	if modelSyntheticStemRe.MatchString(name) {
		return true // *_L\d+ synthesized stems (~32 FPs)
	}
	if modelDecoratorRe.MatchString(name) {
		return true // @tool decorator captures (~28 FPs)
	}
	// PascalCase + type/schema suffix (Zod, Pydantic, TS type aliases).
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		for _, suffix := range modelTypeSchemaSuffix {
			if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
				return true // PascalCase type/schema (~45 FPs combined)
			}
		}
	}
	// snake_case zod_ prefix (TS Zod schemas auto-named).
	if strings.HasPrefix(strings.ToLower(name), "zod_") {
		return true
	}
	return false
}

func (d *UncoveredAISurfaceDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *UncoveredAISurfaceDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	aiNodeTypes := []depgraph.NodeType{
		depgraph.NodePrompt,
		depgraph.NodeDataset,
		depgraph.NodeModel,
		depgraph.NodeEvalMetric,
	}

	for _, nt := range aiNodeTypes {
		for _, n := range g.NodesByType(nt) {
			validations := g.ValidationsForSurface(n.ID)
			if len(validations) > 0 {
				continue
			}

			// Also check incoming edges for any coverage.
			incoming := g.Incoming(n.ID)
			hasCoverage := false
			for _, e := range incoming {
				if e.Type == depgraph.EdgeCoversCodeSurface || e.Type == depgraph.EdgeManualCovers {
					hasCoverage = true
					break
				}
			}
			if hasCoverage {
				continue
			}

			surfaceKind := string(nt)
			name := n.Name
			if name == "" {
				name = n.ID
			}

			// 2026-05-18 moat work — drop aiModel lane FPs whose symbol
			// shape matches known non-model classes. Filter ONLY applies
			// to NodeModel — prompt and dataset lanes have different FP
			// shapes and stay unfiltered here. Class-rule (≥5 FPs each).
			if nt == depgraph.NodeModel && isStructuralAIModelFP(name) {
				continue
			}

			severity := severityForAISurfaceType(nt)

			out = append(out, models.Signal{
				Type:             signals.SignalUncoveredAISurface,
				Category:         models.CategoryAI,
				Severity:         severity,
				Confidence:       0.85,
				EvidenceStrength: models.EvidenceStrong,
				EvidenceSource:   models.SourceGraphTraversal,
				Location:         models.SignalLocation{File: n.Path, Symbol: name},
				Explanation: fmt.Sprintf(
					"AI %s '%s' has zero test or scenario coverage. Changes to this surface can alter AI behavior without any safety net.",
					surfaceKind, name),
				SuggestedAction: fmt.Sprintf("Add eval scenarios that exercise this %s.", surfaceKind),
				Metadata: map[string]any{
					"surfaceKind": surfaceKind,
					"surfaceName": name,
					"surfaceID":   n.ID,
					// sub-lane metadata so users can filter by aiPrompt /
					// aiModel / aiDataset / aiEvalMetric independently.
					"subLane": surfaceKind,
				},
			})
		}
	}

	return out
}

func severityForAISurfaceType(nt depgraph.NodeType) models.SignalSeverity {
	switch nt {
	case depgraph.NodePrompt:
		// 2026-05-18: aiPrompt lane is the strongest (41% precision at
		// n=250). Highest severity preserved — this is the moat surface.
		return models.SeverityHigh
	case depgraph.NodeModel:
		// 2026-05-18: aiModel lane was 4.9% precision pre-filter; even
		// after isStructuralAIModelFP filter drops the dominant FP
		// classes, residual precision is ~20-30% per Phase A.4 replay.
		// Demoted to medium until LLM-call-site proximity gate (depgraph
		// adjacency to call sites) lands.
		return models.SeverityMedium
	case depgraph.NodeDataset:
		return models.SeverityMedium
	case depgraph.NodeEvalMetric:
		return models.SeverityLow
	default:
		return models.SeverityMedium
	}
}
