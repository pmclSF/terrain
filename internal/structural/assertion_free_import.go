package structural

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// AssertionFreeImportDetector finds test files that import production code
// but contain zero assertions — exercising code without verifying behavior.
type AssertionFreeImportDetector struct{}

func (d *AssertionFreeImportDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil // Requires graph; implemented in DetectWithGraph.
}

func (d *AssertionFreeImportDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	testFileNodes := g.NodesByType(depgraph.NodeTestFile)
	for _, n := range testFileNodes {
		// Count outgoing imports to source files.
		outgoing := g.Outgoing(n.ID)
		var importedSources []string
		for _, e := range outgoing {
			if e.Type == depgraph.EdgeImportsModule {
				target := g.Node(e.To)
				if target != nil && target.Type == depgraph.NodeSourceFile {
					importedSources = append(importedSources, e.To)
				}
			}
		}
		if len(importedSources) == 0 {
			continue
		}

		// Find the matching TestFile in the snapshot to check assertion count.
		var assertionCount int
		var testCount int
		for i := range snap.TestFiles {
			tf := &snap.TestFiles[i]
			if n.ID == "test_file:"+tf.Path || strings.HasSuffix(n.ID, ":"+tf.Path) {
				assertionCount = tf.AssertionCount
				testCount = tf.TestCount
				break
			}
		}

		if testCount > 0 && assertionCount == 0 {
			severity := models.SeverityMedium
			if len(importedSources) >= 3 {
				severity = models.SeverityHigh
			}

			out = append(out, models.Signal{
				Type:             signals.SignalAssertionFreeImport,
				Category:         models.CategoryQuality,
				Severity:         severity,
				Confidence:       0.80,
				EvidenceStrength: models.EvidenceStrong,
				EvidenceSource:   models.SourceGraphTraversal,
				Location:         models.SignalLocation{File: n.Path},
				Explanation: fmt.Sprintf(
					"Test file imports %d source modules but contains zero assertions — exercises code without verifying behavior.",
					len(importedSources)),
				SuggestedAction: "Add assertions to validate behavior or remove tests that verify nothing.",
				Metadata: map[string]any{
					"importedSources": importedSources,
					"importCount":     len(importedSources),
					"testCount":       testCount,
				},
			})
		}
	}

	return out
}
