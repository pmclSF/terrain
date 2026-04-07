package measurement

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/skipstats"
)

// HealthMeasurements returns measurement definitions for the health dimension.
func HealthMeasurements() []Definition {
	return []Definition{
		{
			ID:          "health.flaky_share",
			Dimension:   DimensionHealth,
			Description: "Share of test files flagged as flaky or unstable.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalFlakyTest), string(signals.SignalUnstableSuite)},
			Compute:     computeFlakyShare,
		},
		{
			ID:          "health.skip_density",
			Dimension:   DimensionHealth,
			Description: "Share of test files with skipped tests.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalSkippedTest), string(signals.SignalStaticSkippedTest)},
			Compute:     computeSkipDensity,
		},
		{
			ID:          "health.dead_test_share",
			Dimension:   DimensionHealth,
			Description: "Share of test files containing dead or unreachable tests.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalDeadTest)},
			Compute:     computeDeadTestShare,
		},
		{
			ID:          "health.slow_test_share",
			Dimension:   DimensionHealth,
			Description: "Share of test files flagged as slow.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalSlowTest)},
			Compute:     computeSlowTestShare,
		},
	}
}

func computeFlakyShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "health.flaky_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalFlakyTest, signals.SignalUnstableSuite)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.15, 0.30)
	evidence := runtimeEvidence(snap)

	// Without runtime data and no signals, flakiness cannot be assessed —
	// report unknown rather than a false "strong".
	if evidence == EvidenceWeak && count == 0 {
		return Result{
			ID: "health.flaky_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence:    evidence,
			Explanation: "No runtime data available; flakiness cannot be assessed from static analysis alone.",
			Inputs:      []string{string(signals.SignalFlakyTest), string(signals.SignalUnstableSuite)},
			Limitations: evidenceLimitations(evidence),
		}
	}

	return Result{
		ID: "health.flaky_share", Dimension: DimensionHealth,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    evidence,
		Explanation: fmt.Sprintf("%d of %d test file(s) flagged as flaky or unstable (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalFlakyTest), string(signals.SignalUnstableSuite)},
		Limitations: evidenceLimitations(evidence),
	}
}

func computeSkipDensity(snap *models.TestSuiteSnapshot) Result {
	stats := skipstats.Summarize(snap)
	if stats.TotalFiles == 0 {
		return Result{
			ID: "health.skip_density", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}
	// Partial by default: skip markers can be detected statically (e.g. .skip(),
	// xit(), @Disabled) so some evidence exists without runtime data. Upgraded
	// to Strong when runtime data confirms the static findings.
	evidence := EvidencePartial
	if runtimeEvidence(snap) == EvidenceStrong {
		evidence = EvidenceStrong
	}

	return Result{
		ID: "health.skip_density", Dimension: DimensionHealth,
		Value: stats.FileRatio, Units: UnitsRatio, Band: ratioToBand(stats.FileRatio, 0.05, 0.15, 0.30),
		Evidence:    evidence,
		Explanation: fmt.Sprintf("%d of %d test file(s) contain skipped tests (%.0f%%).", stats.FilesWithSkips, stats.TotalFiles, stats.FileRatio*100),
		Inputs:      []string{string(signals.SignalSkippedTest), string(signals.SignalStaticSkippedTest)},
		Limitations: evidenceLimitations(evidence),
	}
}

func computeDeadTestShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "health.dead_test_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalDeadTest)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.02, 0.10, 0.20)
	evidence := runtimeEvidence(snap)

	// Dead test detection requires runtime data (tests observed only in
	// skipped/pending state with no pass/fail evidence). Without runtime
	// data, dead tests cannot be identified — report unknown.
	if evidence == EvidenceWeak && count == 0 {
		return Result{
			ID: "health.dead_test_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence:    evidence,
			Explanation: "No runtime data available; dead tests cannot be identified without test execution results.",
			Inputs:      []string{string(signals.SignalDeadTest)},
			Limitations: evidenceLimitations(evidence),
		}
	}

	return Result{
		ID: "health.dead_test_share", Dimension: DimensionHealth,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    evidence,
		Explanation: fmt.Sprintf("%d of %d test file(s) contain dead tests (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalDeadTest)},
		Limitations: evidenceLimitations(evidence),
	}
}

func computeSlowTestShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "health.slow_test_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalSlowTest)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.10, 0.25, 0.50)
	evidence := runtimeEvidence(snap)

	// Without runtime data and no signals, slowness cannot be assessed —
	// report unknown rather than a false "strong".
	if evidence == EvidenceWeak && count == 0 {
		return Result{
			ID: "health.slow_test_share", Dimension: DimensionHealth,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence:    evidence,
			Explanation: "No runtime data available; test speed cannot be assessed from static analysis alone.",
			Inputs:      []string{string(signals.SignalSlowTest)},
			Limitations: evidenceLimitations(evidence),
		}
	}

	return Result{
		ID: "health.slow_test_share", Dimension: DimensionHealth,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    evidence,
		Explanation: fmt.Sprintf("%d of %d test file(s) flagged as slow (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalSlowTest)},
		Limitations: evidenceLimitations(evidence),
	}
}
