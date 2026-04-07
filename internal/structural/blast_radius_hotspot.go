package structural

import (
	"fmt"
	"sort"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// Blast-radius thresholds.
const (
	// minBlastRadiusTests is the minimum test count for a source file to be
	// considered as a blast-radius candidate.
	minBlastRadiusTests = 10

	// blastRadiusTopPercentDivisor controls the top-N% cutoff. A value of 20
	// means the top 5% (1/20) of entries are always flagged.
	blastRadiusTopPercentDivisor = 20

	// blastRadiusHighThreshold is the test count above which a source file
	// receives SeverityHigh.
	blastRadiusHighThreshold = 50

	// blastRadiusMediumThreshold is the test count above which a source file
	// receives SeverityMedium (and serves as the floor for the cutoff loop).
	blastRadiusMediumThreshold = 20
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
		path     string
		direct   int
		indirect int
		total    int
	}
	var entries []entry
	for _, sc := range cov.Sources {
		direct := len(sc.DirectTests)
		indirect := len(sc.IndirectTests)
		total := sc.TestCount
		if total >= minBlastRadiusTests {
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

	// Flag top 5% or minimum threshold.
	cutoff := len(entries) / blastRadiusTopPercentDivisor
	if cutoff < 1 {
		cutoff = 1
	}

	var out []models.Signal
	for i, e := range entries {
		if i >= cutoff && e.total < blastRadiusMediumThreshold {
			break
		}

		severity := models.SeverityLow
		if e.total > blastRadiusHighThreshold {
			severity = models.SeverityHigh
		} else if e.total > blastRadiusMediumThreshold {
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
				"directTestCount":    e.direct,
				"indirectTestCount":  e.indirect,
				"totalImpactedTests": e.total,
			},
		})
	}

	return out
}
