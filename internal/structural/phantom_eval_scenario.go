package structural

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// PhantomEvalScenarioDetector finds eval scenarios that claim to validate
// AI surfaces but have no import-graph path connecting them to those surfaces.
type PhantomEvalScenarioDetector struct{}

func (d *PhantomEvalScenarioDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *PhantomEvalScenarioDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	for _, scenario := range snap.Scenarios {
		if len(scenario.CoveredSurfaceIDs) == 0 {
			continue
		}

		// Find the scenario's test file via graph edges.
		scenarioNodeID := "scenario:" + scenario.ScenarioID
		sn := g.Node(scenarioNodeID)
		if sn == nil {
			continue
		}

		// Collect all source files reachable from the scenario's test file.
		reachable := collectReachableSourceFiles(g, scenario.Path)

		// Check which claimed surfaces are actually reachable.
		var unreachable []string
		for _, surfaceID := range scenario.CoveredSurfaceIDs {
			surfaceNode := g.Node(surfaceID)
			if surfaceNode == nil {
				continue
			}
			surfacePath := surfaceNode.Path
			if surfacePath == "" {
				continue
			}
			if !reachable[surfacePath] {
				unreachable = append(unreachable, surfaceID)
			}
		}

		if len(unreachable) == 0 {
			continue
		}

		severity := models.SeverityMedium
		if scenario.Executable {
			severity = models.SeverityHigh
		}

		out = append(out, models.Signal{
			Type:             signals.SignalPhantomEvalScenario,
			Category:         models.CategoryAI,
			Severity:         severity,
			Confidence:       0.75,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceGraphTraversal,
			Location: models.SignalLocation{
				File:       scenario.Path,
				ScenarioID: scenario.ScenarioID,
			},
			Explanation: fmt.Sprintf(
				"Eval scenario '%s' claims to cover %d surfaces but cannot reach %d of them through the import graph.",
				scenario.Name, len(scenario.CoveredSurfaceIDs), len(unreachable)),
			SuggestedAction: "Verify the test file imports and exercises the target code, or correct the surface mapping.",
			Metadata: map[string]any{
				"scenarioName":       scenario.Name,
				"claimedSurfaces":    len(scenario.CoveredSurfaceIDs),
				"unreachableSurfaces": unreachable,
			},
		})
	}

	return out
}

// collectReachableSourceFiles BFS from a test file through import edges.
func collectReachableSourceFiles(g *depgraph.Graph, testFilePath string) map[string]bool {
	reachable := map[string]bool{}

	// Find test file node.
	var startID string
	for _, n := range g.NodesByType(depgraph.NodeTestFile) {
		if n.Path == testFilePath || strings.HasSuffix(n.ID, ":"+testFilePath) {
			startID = n.ID
			break
		}
	}
	if startID == "" {
		return reachable
	}

	// BFS through import edges.
	queue := []string{startID}
	visited := map[string]bool{startID: true}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, e := range g.Outgoing(current) {
			if e.Type != depgraph.EdgeImportsModule && e.Type != depgraph.EdgeSourceImportsSource {
				continue
			}
			if visited[e.To] {
				continue
			}
			visited[e.To] = true

			target := g.Node(e.To)
			if target != nil {
				if p := target.Path; p != "" {
					reachable[p] = true
				}
			}
			queue = append(queue, e.To)
		}
	}

	return reachable
}
