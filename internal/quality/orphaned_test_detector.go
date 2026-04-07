package quality

import (
	"fmt"
	"sort"

	"github.com/pmclSF/terrain/internal/models"
)

// OrphanedTestDetector identifies test files that have no linked code units —
// meaning they don't appear to test any source code in the repository. These
// tests may be obsolete, testing external dependencies, or simply disconnected
// from the import graph.
//
// This is a static detector: it uses structural analysis data (linked code
// units), requiring no runtime or coverage artifacts.
type OrphanedTestDetector struct{}

// Detect scans test files for those with zero linked code units.
func (d *OrphanedTestDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var sigs []models.Signal

	type candidate struct {
		path      string
		tests     int
		framework string
	}
	var candidates []candidate

	for _, tf := range snap.TestFiles {
		if tf.TestCount == 0 {
			continue
		}
		if len(tf.LinkedCodeUnits) == 0 {
			candidates = append(candidates, candidate{
				path:      tf.Path,
				tests:     tf.TestCount,
				framework: tf.Framework,
			})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by path for deterministic output.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].path < candidates[j].path
	})

	// Repository-level summary signal.
	total := len(snap.TestFiles)
	ratio := float64(len(candidates)) / float64(total)
	sev := models.SeverityLow
	if ratio > 0.3 {
		sev = models.SeverityMedium
	}

	sigs = append(sigs, models.Signal{
		Type:       "orphanedTestFile",
		Category:   models.CategoryHealth,
		Severity:   sev,
		Confidence: 0.5,
		Location:   models.SignalLocation{Repository: "static"},
		Explanation: fmt.Sprintf(
			"%d of %d test files have no linked source code units (%.0f%%).",
			len(candidates), total, ratio*100,
		),
		SuggestedAction:  "Verify orphaned tests are still relevant or remove them to reduce CI burden.",
		EvidenceStrength: models.EvidenceWeak,
		EvidenceSource:   models.SourceStructuralPattern,
		Metadata: map[string]any{
			"orphanedFiles": len(candidates),
			"totalFiles":    total,
			"ratio":         ratio,
			"scope":         "repository",
		},
	})

	// Per-file signals (capped at 10).
	limit := 10
	if len(candidates) < limit {
		limit = len(candidates)
	}
	for _, c := range candidates[:limit] {
		sigs = append(sigs, models.Signal{
			Type:       "orphanedTestFile",
			Category:   models.CategoryHealth,
			Severity:   models.SeverityLow,
			Confidence: 0.5,
			Location:   models.SignalLocation{File: c.path},
			Explanation: fmt.Sprintf(
				"%s has %d test(s) but no linked source code units.",
				c.path, c.tests,
			),
			SuggestedAction:  "Verify the test is still relevant or remove it.",
			EvidenceStrength: models.EvidenceWeak,
			EvidenceSource:   models.SourceStructuralPattern,
			Metadata: map[string]any{
				"testCount": c.tests,
				"framework": c.framework,
				"scope":     "file",
			},
		})
	}

	return sigs
}
