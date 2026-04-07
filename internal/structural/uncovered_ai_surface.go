package structural

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// UncoveredAISurfaceDetector finds AI surfaces (prompts, tools, datasets)
// with zero test or scenario coverage.
type UncoveredAISurfaceDetector struct{}

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

			severity := severityForAISurfaceType(nt)
			surfaceKind := string(nt)
			name := n.Name
			if name == "" {
				name = n.ID
			}

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
				},
			})
		}
	}

	return out
}

func severityForAISurfaceType(nt depgraph.NodeType) models.SignalSeverity {
	switch nt {
	case depgraph.NodePrompt:
		return models.SeverityHigh
	case depgraph.NodeModel:
		return models.SeverityHigh
	case depgraph.NodeDataset:
		return models.SeverityMedium
	case depgraph.NodeEvalMetric:
		return models.SeverityLow
	default:
		return models.SeverityMedium
	}
}
