package structural

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// Fixture fragility thresholds.
//
// The pre-calibration thresholds fired on fixtures used by >=5 tests
// in a single file — which is normal DRY test design, not fragility.
// The current shape requires fan-out *across files* before flagging.
const (
	// minFixtureDependents is the minimum direct-test count before a
	// fixture is even considered. Raised from 5 to 10 — fewer than 10
	// tests sharing a fixture is normal DRY design, not a cascade risk.
	minFixtureDependents = 10

	// minFixtureTestFiles is the minimum *file* span — a fixture used
	// by 20 tests in a single file is local DRY; it becomes fragile
	// only when many independent test groups depend on it.
	minFixtureTestFiles = 2

	// fixtureHighTestThreshold flags SeverityHigh when direct tests exceed this.
	fixtureHighTestThreshold = 25

	// fixtureMediumTestThreshold flags SeverityMedium when direct tests exceed this.
	fixtureMediumTestThreshold = 15

	// fixtureHighFileThreshold flags SeverityHigh when distinct test files exceed this.
	fixtureHighFileThreshold = 6

	// fixtureMediumFileThreshold flags SeverityMedium when distinct test files exceed this.
	fixtureMediumFileThreshold = 3
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

		if directTests < minFixtureDependents {
			continue
		}
		if len(testFiles) < minFixtureTestFiles {
			// File-local fixture — many tests in one file sharing
			// setup is normal DRY, not a cascade risk. Skip.
			continue
		}

		severity := models.SeverityLow
		if directTests > fixtureHighTestThreshold || len(testFiles) > fixtureHighFileThreshold {
			severity = models.SeverityHigh
		} else if directTests > fixtureMediumTestThreshold || len(testFiles) > fixtureMediumFileThreshold {
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
				"fixtureName":     name,
				"directTestCount": directTests,
				"testFileCount":   len(testFiles),
			},
		})
	}

	return out
}
