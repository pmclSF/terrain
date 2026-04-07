package measurement

import "github.com/pmclSF/terrain/internal/models"

// countSignals counts the total number of signals in the snapshot matching any
// of the given signal types. Use this for density-style measurements where
// multiple signals per file are meaningful.
func countSignals(snap *models.TestSuiteSnapshot, types ...models.SignalType) int {
	typeSet := make(map[models.SignalType]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	count := 0
	for _, s := range snap.Signals {
		if typeSet[s.Type] {
			count++
		}
	}
	return count
}

// countFileSignals counts the number of unique files that contain at least one
// signal matching the given types. Use this for share-style measurements where
// the unit is "files affected" rather than "total occurrences".
func countFileSignals(snap *models.TestSuiteSnapshot, types ...models.SignalType) int {
	typeSet := make(map[models.SignalType]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	files := make(map[string]bool)
	for _, s := range snap.Signals {
		if typeSet[s.Type] && s.Location.File != "" {
			files[s.Location.File] = true
		}
	}
	return len(files)
}

// ratioToBand maps a ratio to a qualitative band using three thresholds.
//   - ratio <= low  → strong
//   - ratio <= mid  → moderate
//   - ratio <= high → weak
//   - ratio >  high → critical
func ratioToBand(ratio, low, mid, high float64) string {
	switch {
	case ratio <= low:
		return string(PostureStrong)
	case ratio <= mid:
		return string(PostureModerate)
	case ratio <= high:
		return string(PostureWeak)
	default:
		return string(PostureCritical)
	}
}

// runtimeEvidence returns the evidence strength based on whether runtime data
// is available in the snapshot.
func runtimeEvidence(snap *models.TestSuiteSnapshot) EvidenceStrength {
	for _, tf := range snap.TestFiles {
		if tf.RuntimeStats != nil && tf.RuntimeStats.AvgRuntimeMs > 0 {
			return EvidenceStrong
		}
	}
	return EvidenceWeak
}

// evidenceLimitations returns standard limitation strings for a given evidence level.
func evidenceLimitations(evidence EvidenceStrength) []string {
	switch evidence {
	case EvidenceWeak:
		return []string{"No runtime data available; result is based on static analysis only."}
	case EvidencePartial:
		return []string{"Partial data available; result may improve with more runtime artifacts."}
	case EvidenceNone:
		return []string{"No data available for this measurement."}
	default:
		return nil
	}
}
