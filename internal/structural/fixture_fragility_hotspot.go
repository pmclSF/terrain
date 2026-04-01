package structural

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// FixtureFragilityHotspotDetector finds fixtures depended on by many tests,
// where a single fixture change cascades widely.
type FixtureFragilityHotspotDetector struct{}

func (d *FixtureFragilityHotspotDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *FixtureFragilityHotspotDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	for _, n := range g.NodesByType(depgraph.NodeFixture) {
		incoming := g.Incoming(n.ID)

		// Count direct test dependents.
		directTests := 0
		testFiles := map[string]bool{}
		for _, e := range incoming {
			if e.Type == depgraph.EdgeTestUsesFixture {
				directTests++
				source := g.Node(e.From)
				if source != nil {
					if p := source.Path; p != "" {
						testFiles[p] = true
					}
				}
			}
		}

		if directTests < 5 {
			continue
		}

		severity := models.SeverityLow
		if directTests > 20 || len(testFiles) > 5 {
			severity = models.SeverityHigh
		} else if directTests > 10 || len(testFiles) > 3 {
			severity = models.SeverityMedium
		}

		name := n.Name
		if name == "" {
			name = n.ID
		}

		out = append(out, models.Signal{
			Type:             signals.SignalFixtureFragilityHotspot,
			Category:         models.CategoryStructure,
			Severity:         severity,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location:         models.SignalLocation{File: n.Path, Symbol: name},
			Explanation: fmt.Sprintf(
				"Fixture '%s' is used by %d tests across %d files. A single change cascades widely.",
				name, directTests, len(testFiles)),
			SuggestedAction: "Extract smaller, focused fixtures to reduce cascading test failures.",
			Metadata: map[string]any{
				"fixtureName":    name,
				"directTestCount": directTests,
				"testFileCount":  len(testFiles),
			},
		})
	}

	return out
}
