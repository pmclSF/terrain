package structural

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// CapabilityValidationGapDetector finds inferred AI capabilities with no
// scenario validation.
type CapabilityValidationGapDetector struct{}

func (d *CapabilityValidationGapDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *CapabilityValidationGapDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	for _, n := range g.NodesByType(depgraph.NodeCapability) {
		incoming := g.Incoming(n.ID)

		scenarioCount := 0
		executableCount := 0
		for _, e := range incoming {
			if e.Type == depgraph.EdgeScenarioValidatesCapability {
				scenarioCount++
				source := g.Node(e.From)
				if source != nil && source.Metadata["executable"] == "true" {
					executableCount++
				}
			}
		}

		if scenarioCount > 0 && executableCount > 0 {
			continue // Covered by executable scenarios.
		}

		name := n.Name
		if name == "" {
			name = n.ID
		}

		var severity models.SignalSeverity
		var confidence float64
		var explanation string

		if scenarioCount == 0 {
			severity = models.SeverityHigh
			confidence = 0.80
			explanation = fmt.Sprintf(
				"AI capability '%s' has no eval scenarios validating it. Behavioral regressions are undetectable.", name)
		} else {
			severity = models.SeverityMedium
			confidence = 0.65
			explanation = fmt.Sprintf(
				"AI capability '%s' has %d scenario(s) but none are executable — validation exists only on paper.", name, scenarioCount)
		}

		out = append(out, models.Signal{
			Type:             signals.SignalCapabilityValidationGap,
			Category:         models.CategoryAI,
			Severity:         severity,
			Confidence:       confidence,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceGraphTraversal,
			Location:         models.SignalLocation{Capability: name},
			Explanation:      explanation,
			SuggestedAction:  "Add eval scenarios that exercise this capability to ensure behavioral regression detection.",
			Metadata: map[string]any{
				"capabilityName":        name,
				"scenarioCount":         scenarioCount,
				"executableScenarios":   executableCount,
			},
		})
	}

	return out
}
