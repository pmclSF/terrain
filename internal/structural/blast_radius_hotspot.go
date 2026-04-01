package structural

import (
	"fmt"
	"sort"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// BlastRadiusHotspotDetector finds source files where a change would
// impact an unusually large number of tests.
type BlastRadiusHotspotDetector struct{}

func (d *BlastRadiusHotspotDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *BlastRadiusHotspotDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	cov := depgraph.AnalyzeCoverage(g)
	if len(cov.Sources) == 0 {
		return nil
	}

	// Collect (file, testCount) pairs and sort by test count descending.
	type entry struct {
		path      string
		direct    int
		indirect  int
		total     int
	}
	var entries []entry
	for _, sc := range cov.Sources {
		direct := len(sc.DirectTests)
		indirect := len(sc.IndirectTests)
		total := sc.TestCount
		if total >= 10 {
			entries = append(entries, entry{
				path:     sc.Path,
				direct:   direct,
				indirect: indirect,
				total:    total,
			})
		}
	}

	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].total > entries[j].total
	})

	// Flag top 5% or minimum threshold of 20 tests.
	cutoff := len(entries) / 20
	if cutoff < 1 {
		cutoff = 1
	}

	var out []models.Signal
	for i, e := range entries {
		if i >= cutoff && e.total < 20 {
			break
		}

		severity := models.SeverityLow
		if e.total > 50 {
			severity = models.SeverityHigh
		} else if e.total > 20 {
			severity = models.SeverityMedium
		}

		out = append(out, models.Signal{
			Type:             signals.SignalBlastRadiusHotspot,
			Category:         models.CategoryStructure,
			Severity:         severity,
			Confidence:       0.90,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location:         models.SignalLocation{File: e.path},
			Explanation: fmt.Sprintf(
				"Changes to this file propagate to %d tests (%d direct, %d indirect). High blast radius increases regression risk.",
				e.total, e.direct, e.indirect),
			SuggestedAction: "Ensure high direct test coverage and consider adding contract tests at interface boundaries.",
			Metadata: map[string]any{
				"directTestCount":   e.direct,
				"indirectTestCount": e.indirect,
				"totalImpactedTests": e.total,
			},
		})
	}

	return out
}
