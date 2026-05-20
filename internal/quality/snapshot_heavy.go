package quality

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
)

// SnapshotHeavyDetector identifies test files that rely heavily on snapshots
// relative to direct semantic assertions.
type SnapshotHeavyDetector struct{}

// Detect emits snapshotHeavyTest signals for files where snapshot usage is
// high enough to indicate brittle, low-semantic verification.
func (d *SnapshotHeavyDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var out []models.Signal

	for _, tf := range snap.TestFiles {
		if tf.SnapshotCount <= 0 {
			continue
		}

		ratio := 0.0
		if tf.AssertionCount > 0 {
			ratio = float64(tf.SnapshotCount) / float64(tf.AssertionCount)
		}

		// 2026-05-18 (Phase A.5 + L2 review): replace the SnapshotCount<3
		// AND ratio<1.0 filter with a disjunction that catches two distinct
		// "incidental snapshot" shapes without killing canonical TPs:
		//
		//   (snap≥2 AND ratio≥0.3): heavy snapshot pattern with ratio that
		//     dominates direct assertions
		//   (snap≥1 AND direct≤2): the snapshot IS the whole test (Claude-
		//     confirmed TP class — single big snapshot covering everything)
		//
		// Anything outside both shapes is incidental snapshot use:
		//   - snap=1 with direct=20+ → one incidental snapshot in a real test
		//   - snap=4, direct=164, ratio=0.02 → snapshot is supplementary, not
		//     dominant (the n=50 FP that the SnapshotCount<3 filter missed)
		keepHeavyRatio := tf.SnapshotCount >= 2 && ratio >= 0.3
		keepDominantSnap := tf.SnapshotCount >= 1 && tf.AssertionCount <= 2
		if !keepHeavyRatio && !keepDominantSnap {
			continue
		}

		severity := models.SeverityMedium
		if tf.AssertionCount == 0 || ratio >= 2.0 || tf.SnapshotCount >= 10 {
			severity = models.SeverityHigh
		}

		explanation := fmt.Sprintf(
			"%s uses %d snapshot assertion(s) with %d direct assertion(s) (snapshot/assertion ratio %.2f).",
			tf.Path, tf.SnapshotCount, tf.AssertionCount, ratio,
		)

		out = append(out, models.Signal{
			Type:             "snapshotHeavyTest",
			Category:         models.CategoryQuality,
			Severity:         severity,
			Confidence:       0.8,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceStructuralPattern,
			Location:         models.SignalLocation{File: tf.Path},
			Owner:            tf.Owner,
			Explanation:      explanation,
			SuggestedAction:  "Replace broad snapshot assertions with targeted semantic assertions for critical behavior.",
			Metadata: map[string]any{
				"snapshotCount":  tf.SnapshotCount,
				"assertionCount": tf.AssertionCount,
				"snapshotRatio":  ratio,
			},
		})
	}

	return out
}
