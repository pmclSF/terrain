package measurement

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// OperationalRiskMeasurements returns measurement definitions for the
// operational_risk dimension.
func OperationalRiskMeasurements() []Definition {
	return []Definition{
		{
			ID:          "operational_risk.policy_violation_density",
			Dimension:   DimensionOperationalRisk,
			Description: "Density of policy violations relative to test files.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalPolicyViolation)},
			Compute:     computePolicyViolationDensity,
		},
		{
			ID:          "operational_risk.legacy_framework_share",
			Dimension:   DimensionOperationalRisk,
			Description: "Share of test files using legacy frameworks.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalLegacyFrameworkUsage)},
			Compute:     computeLegacyFrameworkShare,
		},
		{
			ID:          "operational_risk.runtime_budget_breach_share",
			Dimension:   DimensionOperationalRisk,
			Description: "Share of test files exceeding runtime budgets.",
			Units:       UnitsRatio,
			Inputs:      []string{string(signals.SignalRuntimeBudgetExceeded)},
			Compute:     computeRuntimeBudgetBreachShare,
		},
	}
}

func computePolicyViolationDensity(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "operational_risk.policy_violation_density", Dimension: DimensionOperationalRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalPolicyViolation)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.0, 0.05, 0.15)

	return Result{
		ID: "operational_risk.policy_violation_density", Dimension: DimensionOperationalRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    EvidenceStrong,
		Explanation: fmt.Sprintf("%d test file(s) with policy violation(s) out of %d (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalPolicyViolation)},
	}
}

func computeLegacyFrameworkShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "operational_risk.legacy_framework_share", Dimension: DimensionOperationalRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalLegacyFrameworkUsage)
	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.15, 0.30)

	return Result{
		ID: "operational_risk.legacy_framework_share", Dimension: DimensionOperationalRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    EvidenceStrong,
		Explanation: fmt.Sprintf("%d of %d test file(s) use legacy frameworks (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalLegacyFrameworkUsage)},
	}
}

func computeRuntimeBudgetBreachShare(snap *models.TestSuiteSnapshot) Result {
	total := len(snap.TestFiles)
	if total == 0 {
		return Result{
			ID: "operational_risk.runtime_budget_breach_share", Dimension: DimensionOperationalRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence: EvidenceNone, Explanation: "No test files detected.",
		}
	}

	count := countFileSignals(snap, signals.SignalRuntimeBudgetExceeded)
	evidence := runtimeEvidence(snap)

	// Without runtime data, budget breaches cannot be detected — report unknown.
	if evidence == EvidenceWeak && count == 0 {
		return Result{
			ID: "operational_risk.runtime_budget_breach_share", Dimension: DimensionOperationalRisk,
			Value: 0, Units: UnitsRatio, Band: "unknown",
			Evidence:    evidence,
			Explanation: "No runtime data available; cannot assess budget compliance.",
			Inputs:      []string{string(signals.SignalRuntimeBudgetExceeded)},
			Limitations: evidenceLimitations(evidence),
		}
	}

	ratio := float64(count) / float64(total)
	band := ratioToBand(ratio, 0.05, 0.15, 0.30)

	return Result{
		ID: "operational_risk.runtime_budget_breach_share", Dimension: DimensionOperationalRisk,
		Value: ratio, Units: UnitsRatio, Band: band,
		Evidence:    evidence,
		Explanation: fmt.Sprintf("%d of %d test file(s) exceed runtime budgets (%.0f%%).", count, total, ratio*100),
		Inputs:      []string{string(signals.SignalRuntimeBudgetExceeded)},
		Limitations: evidenceLimitations(evidence),
	}
}
