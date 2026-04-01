package quality

import (
	"fmt"
	"sort"

	"github.com/pmclSF/terrain/internal/models"
)

// AssertionFreeDetector identifies test files that contain test function
// signatures but no detectable assertion patterns. These tests verify
// nothing and mask gaps in real coverage.
//
// This is a static detector: it uses pre-computed assertion counts from
// the analysis phase, requiring no runtime data.
type AssertionFreeDetector struct{}

// Detect scans test files for those with tests but zero assertions.
func (d *AssertionFreeDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
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
		if tf.AssertionCount == 0 {
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

	// Sort by test count descending for deterministic output.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].tests != candidates[j].tests {
			return candidates[i].tests > candidates[j].tests
		}
		return candidates[i].path < candidates[j].path
	})

	// Repository-level summary signal.
	total := len(snap.TestFiles)
	ratio := float64(len(candidates)) / float64(total)
	sev := models.SeverityLow
	if ratio > 0.2 {
		sev = models.SeverityHigh
	} else if ratio > 0.1 {
		sev = models.SeverityMedium
	}

	sigs = append(sigs, models.Signal{
		Type:     "assertionFreeTest",
		Category: models.CategoryHealth,
		Severity: sev,
		Confidence: 0.7,
		Location: models.SignalLocation{Repository: "static"},
		Explanation: fmt.Sprintf(
			"%d of %d test files have tests but no detectable assertions (%.0f%%).",
			len(candidates), total, ratio*100,
		),
		SuggestedAction:  "Add assertions to validate real behavior — tests without assertions verify nothing.",
		EvidenceStrength: models.EvidencePartial,
		EvidenceSource:   models.SourceStructuralPattern,
		Metadata: map[string]any{
			"affectedFiles": len(candidates),
			"totalFiles":    total,
			"ratio":         ratio,
			"scope":         "repository",
		},
	})

	// Per-file signals (capped at 10 to avoid noise).
	limit := 10
	if len(candidates) < limit {
		limit = len(candidates)
	}
	for _, c := range candidates[:limit] {
		sigs = append(sigs, models.Signal{
			Type:     "assertionFreeTest",
			Category: models.CategoryHealth,
			Severity: models.SeverityMedium,
			Confidence: 0.7,
			Location: models.SignalLocation{File: c.path},
			Explanation: fmt.Sprintf(
				"%s has %d test(s) but no detectable assertions.",
				c.path, c.tests,
			),
			SuggestedAction:  "Add assertions to validate behavior.",
			EvidenceStrength: models.EvidencePartial,
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
