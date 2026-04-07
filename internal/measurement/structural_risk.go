package measurement

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// StructuralRiskMeasurements returns measurement definitions for the
// structural_risk dimension.
func StructuralRiskMeasurements() []Definition {
	return []Definition{
		{
			ID:          "structural_risk.migration_blocker_density",
			Dimension:   DimensionStructuralRisk,
			Description: "Density of migration blockers relative to test files.",
			Units:       UnitsRatio,
			Inputs: []string{
				string(signals.SignalMigrationBlocker),
				string(signals.SignalCustomMatcherRisk),
			},
			Compute: computeMigrationBlockerDensity,
		},
		{
			ID:          "structural_risk.deprecated_pattern_share",
			Dimension:   DimensionStructuralRisk,
			Description: "Share of test files using deprecated patterns.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalDeprecatedTestPattern)},
			Compute:     computeDeprecatedPatternShare,
		},
		{
			ID:          "structural_risk.dynamic_generation_share",
			Dimension:   DimensionStructuralRisk,
			Description: "Share of test files using dynamic test generation.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalDynamicTestGeneration)},
			Compute:     computeDynamicGenerationShare,
		},
	}
}

func computeMigrationBlockerDensity(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "structural_risk.migration_blocker_density", Dimension: DimensionStructuralRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	// Only count signals that are NOT already covered by their own dedicated
	// measurements (deprecated_pattern_share, dynamic_generation_share) to
	// avoid double-penalizing the structural risk dimension.
	count := countFileSignals(snap,
		signals.SignalMigrationBlocker,
		signals.SignalCustomMatcherRisk,
	)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.15, 0.30)

	return Result{
		ID: "structural_risk.migration_blocker_density", Dimension: DimensionStructuralRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    EvidenceStrong,
		Explanation: fmt.Sprintf("%d test file(s) with migration blocker(s) out of %d (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalMigrationBlocker), string(signals.SignalCustomMatcherRisk)},
	}
}

func computeDeprecatedPatternShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "structural_risk.deprecated_pattern_share", Dimension: DimensionStructuralRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalDeprecatedTestPattern)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.15, 0.30)

	return Result{
		ID: "structural_risk.deprecated_pattern_share", Dimension: DimensionStructuralRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    EvidenceStrong,
		Explanation: fmt.Sprintf("%d of %d test file(s) use deprecated patterns (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalDeprecatedTestPattern)},
	}
}

func computeDynamicGenerationShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "structural_risk.dynamic_generation_share", Dimension: DimensionStructuralRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalDynamicTestGeneration)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.10, 0.20)

	return Result{
		ID: "structural_risk.dynamic_generation_share", Dimension: DimensionStructuralRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    EvidenceStrong,
		Explanation: fmt.Sprintf("%d of %d test file(s) use dynamic test generation (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalDynamicTestGeneration)},
	}
}
