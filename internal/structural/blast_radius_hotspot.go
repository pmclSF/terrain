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

		// 2026-05-11 corpus-driven severity refinement: global PR-lift on
		// 5 clean corpora is 1.06x (essentially base rate), but lift
		// climbs to 1.76x in non-AI mainstream OSS. The discriminator
		// that matters is the *direct-test ratio*: a file with 100
		// indirect tests but only 2 direct tests is genuinely
		// regression-prone; a file with 50 direct + 50 indirect is
		// likely well-covered.
		//
		// Severity now factors both blast radius AND direct-test
		// inverse ratio so high-severity firings concentrate on the
		// truly under-tested hotspots.
		directRatio := 0.0
		if e.total > 0 {
			directRatio = float64(e.direct) / float64(e.total)
		}
		severity := models.SeverityLow
		switch {
		case e.total > blastRadiusHighThreshold && directRatio < 0.20:
			// Big blast, weak direct coverage → critical-quality concern.
			severity = models.SeverityHigh
		case e.total > blastRadiusMediumThreshold && directRatio < 0.30:
			severity = models.SeverityMedium
		case directRatio >= 0.50:
			// Well-tested directly — keep the signal as informational
			// (some adopters still want the topology view) but don't
			// gate on it.
			severity = models.SeverityInfo
		}

		out = append(out, models.Signal{
			Type:             signals.SignalBlastRadiusHotspot,
			Category:         models.CategoryStructure,
			Severity:         severity,
			Confidence:       0.85, // demoted from 0.90: per-corpus lift is mixed
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location:         models.SignalLocation{File: e.path},
			Explanation: fmt.Sprintf(
				"Changes to this file propagate to %d tests (%d direct, %d indirect; direct ratio %.0f%%). %s",
				e.total, e.direct, e.indirect, directRatio*100,
				blastRadiusRiskCommentary(directRatio)),
			SuggestedAction: "Ensure high direct test coverage and consider adding contract tests at interface boundaries.",
			Metadata: map[string]any{
				"directTestCount":    e.direct,
				"indirectTestCount":  e.indirect,
				"totalImpactedTests": e.total,
				"directRatio":        directRatio,
			},
		})
	}

	return out
}

// blastRadiusRiskCommentary returns a human-readable risk commentary
// based on the direct-test ratio. Used in the signal explanation so
// adopters can immediately see whether the hotspot is a real concern
// or a topology curiosity.
func blastRadiusRiskCommentary(directRatio float64) string {
	switch {
	case directRatio < 0.20:
		return "High blast radius + low direct-test ratio — changes here likely cause regressions covered only via transitive paths."
	case directRatio < 0.50:
		return "High blast radius — review whether direct tests catch the surface fully."
	default:
		return "High blast radius but strong direct-test coverage — informational, not a gating concern."
	}
}
