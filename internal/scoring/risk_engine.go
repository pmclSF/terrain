// Package scoring implements Terrain's explainable risk engine.
//
// Risk surfaces are derived from concrete signals. Every risk surface
// must be explainable, transparent, and actionable.
//
// The risk model is intentionally simple:
//   - risk is computed from signal severity weights
//   - weights are explicit and inspectable
//   - bands are computed from normalized scores
//   - contributing signals are listed for transparency
package scoring

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// RiskModelVersion increments when scoring methodology changes.
const RiskModelVersion = "2.0.0"

// Severity weights are the multipliers applied to each finding when summing
// per-dimension risk. They are NOT corpus-calibrated — values were chosen by
// hand so that one Critical finding outweighs ~1.3 High findings, and one
// High outweighs 1.5 Medium findings, which roughly matches reviewer intuition
// on a sample of customer repos. Calibration against a labeled corpus lands
// in 0.3 (see docs/release/0.2.md → 0.3 plan); when it does, both these
// weights and the band thresholds below will shift. The current values
// represent the "best guess" that 0.1.0 shipped with; we are documenting
// them here, not changing them, to preserve back-compat in 0.1.2.
//
// Rationale per level:
//   - Critical (4.0): user-facing safety/security risk; fail-the-PR severity.
//   - High (3.0): bug-escape risk OR meaningful CI cost.
//   - Medium (2.0): maintenance pain or moderate efficacy gap.
//   - Low (1.0): code smell or cleanup opportunity.
//   - Info (0.5): observation; counted for visibility, not scored.
const (
	severityWeightCritical = 4.0
	severityWeightHigh     = 3.0
	severityWeightMedium   = 2.0
	severityWeightLow      = 1.0
	severityWeightInfo     = 0.5
)

var severityWeight = map[models.SignalSeverity]float64{
	models.SeverityCritical: severityWeightCritical,
	models.SeverityHigh:     severityWeightHigh,
	models.SeverityMedium:   severityWeightMedium,
	models.SeverityLow:      severityWeightLow,
	models.SeverityInfo:     severityWeightInfo,
}

// Risk band thresholds map a weighted score to a qualitative band.
//
// These four constants are the single source of truth for the band
// boundaries; the deadband logic in scoreToBandWithHysteresis derives its
// hysteresis values from them. They are intentionally NOT calibrated —
// 4 / 9 / 16 are gut-feel breakpoints chosen during 0.1.0 design and
// preserved through 0.1.2 for back-compat. 0.3 replaces them with corpus-
// percentile-derived values; see docs/scoring-rubric.md for the full
// methodology.
const (
	riskBandLowUpper      = 4.0  // score < 4 → Low
	riskBandMediumUpper   = 9.0  // 4 ≤ score < 9 → Medium
	riskBandHighUpper     = 16.0 // 9 ≤ score < 16 → High; ≥16 → Critical
	riskBandHysteresis    = 0.5  // deadband around each boundary
	governanceFloorScore  = 4.0  // governance violations don't drop below Medium
	densityScoreScale     = 10.0 // density score = (weight / files) × this
	absoluteWeightScale   = 1.2  // log-scaled weight component multiplier
	absoluteCountScale    = 0.8  // log-scaled count component multiplier
)

// Signal types that feed each risk dimension.
var reliabilitySignals = map[models.SignalType]bool{
	"flakyTest":     true,
	"skippedTest":   true,
	"deadTest":      true,
	"unstableSuite": true,
	"slowTest":      true,
}

var changeRiskSignals = map[models.SignalType]bool{
	"weakAssertion":          true,
	"untestedExport":         true,
	"mockHeavyTest":          true,
	"testsOnlyMocks":         true,
	"coverageBlindSpot":      true,
	"coverageThresholdBreak": true,
	"migrationBlocker":       true,
	"deprecatedTestPattern":  true,
	"dynamicTestGeneration":  true,
	"customMatcherRisk":      true,
	"unsupportedSetup":       true,
}

var speedSignals = map[models.SignalType]bool{
	"slowTest":              true,
	"runtimeBudgetExceeded": true,
}

var governanceSignals = map[models.SignalType]bool{
	"policyViolation":       true,
	"legacyFrameworkUsage":  true,
	"skippedTestsInCI":      true,
	"runtimeBudgetExceeded": true,
}

// ComputeRisk generates risk surfaces from the signals in the snapshot.
//
// Risk uses a hybrid score:
//   - density score (weighted issues per 10 files)
//   - absolute burden score (log-scaled weight and count)
//
// The final score is the maximum of those two signals, which preserves
// local concentration while avoiding under-reporting severe absolute burden
// in very large repositories.
//
// Currently computes:
//   - repository-level reliability, change, speed, and governance risk
//   - directory-level change risk rollups
func ComputeRisk(snap *models.TestSuiteSnapshot) []models.RiskSurface {
	var surfaces []models.RiskSurface

	totalFiles := len(snap.TestFiles)
	previousRepoBands := previousRepositoryBands(snap.Risk)

	// Repository-level risk
	surfaces = append(surfaces, computeRepoRisk(
		snap.Signals, "reliability", reliabilitySignals, totalFiles, previousRepoBands["reliability"],
	)...)
	surfaces = append(surfaces, computeRepoRisk(
		snap.Signals, "change", changeRiskSignals, totalFiles, previousRepoBands["change"],
	)...)
	surfaces = append(surfaces, computeRepoRisk(
		snap.Signals, "speed", speedSignals, totalFiles, previousRepoBands["speed"],
	)...)
	surfaces = append(surfaces, computeRepoRisk(
		snap.Signals, "governance", governanceSignals, totalFiles, previousRepoBands["governance"],
	)...)

	// Directory-level change risk rollups
	surfaces = append(surfaces, computeDirectoryRisk(snap)...)

	return surfaces
}

// computeRepoRisk computes a single risk dimension at repo scope.
// totalFiles is used for density normalization; absolute burden is always
// considered to avoid masking severe issues in large suites.
func computeRepoRisk(
	signals []models.Signal,
	riskType string,
	relevant map[models.SignalType]bool,
	totalFiles int,
	previousBand models.RiskBand,
) []models.RiskSurface {
	var contributing []models.Signal
	var totalWeight float64

	for _, s := range signals {
		if relevant[s.Type] {
			contributing = append(contributing, s)
			totalWeight += severityWeight[s.Severity]
		}
	}

	if len(contributing) == 0 {
		return nil
	}

	score := computeHybridScore(totalWeight, len(contributing), totalFiles)
	// Governance floor: when a hard policy violation or a Critical/High
	// signal exists in the governance dimension, the score is floored at
	// the Medium-band boundary so a small repo with a single but
	// significant policy violation still lands in Medium rather than Low.
	// This is documented in docs/scoring-rubric.md as the only case where
	// the band is not a pure function of the score.
	if riskType == "governance" && score < governanceFloorScore && hasGovernanceFloorTrigger(contributing) {
		score = governanceFloorScore
	}
	band := scoreToBandWithHysteresis(score, previousBand)

	return []models.RiskSurface{{
		Type:                riskType,
		Scope:               "repository",
		ScopeName:           "repo",
		Band:                band,
		Score:               score,
		ContributingSignals: contributing,
		Explanation:         buildExplanation(riskType, band, contributing, totalFiles, totalWeight, score),
		SuggestedAction:     buildSuggestedAction(riskType, band),
	}}
}

func previousRepositoryBands(risk []models.RiskSurface) map[string]models.RiskBand {
	byType := map[string]models.RiskBand{}
	for _, r := range risk {
		if r.Scope != "repository" || r.Type == "" {
			continue
		}
		byType[r.Type] = r.Band
	}
	return byType
}

func hasGovernanceFloorTrigger(signals []models.Signal) bool {
	for _, s := range signals {
		if s.Type == "policyViolation" || severityRank(s.Severity) >= severityRank(models.SeverityHigh) {
			return true
		}
	}
	return false
}

func severityRank(sev models.SignalSeverity) int {
	switch sev {
	case models.SeverityCritical:
		return 5
	case models.SeverityHigh:
		return 4
	case models.SeverityMedium:
		return 3
	case models.SeverityLow:
		return 2
	case models.SeverityInfo:
		return 1
	default:
		return 0
	}
}

// computeDirectoryRisk groups signals by directory and computes per-directory risk.
// Uses test file count per directory for density normalization.
func computeDirectoryRisk(snap *models.TestSuiteSnapshot) []models.RiskSurface {
	riskType := "change"
	relevant := changeRiskSignals

	dirSignals := map[string][]models.Signal{}
	dirWeights := map[string]float64{}

	for _, s := range snap.Signals {
		if !relevant[s.Type] {
			continue
		}
		dir := filepath.Dir(s.Location.File)
		if dir == "" || dir == "." {
			continue
		}
		dirSignals[dir] = append(dirSignals[dir], s)
		dirWeights[dir] += severityWeight[s.Severity]
	}

	// Count test files per directory for normalization.
	dirFileCount := map[string]int{}
	for _, tf := range snap.TestFiles {
		dir := filepath.Dir(tf.Path)
		dirFileCount[dir]++
	}

	// Sort directory keys for deterministic output.
	dirs := make([]string, 0, len(dirSignals))
	for dir := range dirSignals {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var surfaces []models.RiskSurface
	for _, dir := range dirs {
		sigs := dirSignals[dir]
		// Flag directories with multiple signals, or single signals at
		// high/critical severity — a lone high-severity issue in a
		// directory still represents concentrated risk worth surfacing.
		if len(sigs) < 2 && !hasHighSeveritySignal(sigs) {
			continue
		}
		// Normalize by directory file count.
		fileCount := dirFileCount[dir]
		normalizedScore := dirWeights[dir]
		if fileCount > 0 {
			normalizedScore = (dirWeights[dir] / float64(fileCount)) * 10.0
		}
		band := scoreToBand(normalizedScore)
		surfaces = append(surfaces, models.RiskSurface{
			Type:                riskType,
			Scope:               "directory",
			ScopeName:           dir,
			Band:                band,
			Score:               normalizedScore,
			ContributingSignals: sigs,
			Explanation: fmt.Sprintf("%s risk in %s: %d signals across %d test file(s).",
				titleRiskType(riskType), dir, len(sigs), fileCount),
			SuggestedAction: fmt.Sprintf("Review %s for concentrated test quality issues.", dir),
		})
	}

	// Sort by score descending, then by name for deterministic output.
	sort.Slice(surfaces, func(i, j int) bool {
		if surfaces[i].Score != surfaces[j].Score {
			return surfaces[i].Score > surfaces[j].Score
		}
		return surfaces[i].ScopeName < surfaces[j].ScopeName
	})

	return surfaces
}

// scoreToBand maps a weighted signal score to a qualitative risk band.
//
// Thresholds (derived from riskBand* constants above):
//   - score < 4:   Low
//   - 4 ≤ x < 9:   Medium
//   - 9 ≤ x < 16:  High
//   - score ≥ 16:  Critical
//
// These thresholds are intentionally simple and inspectable. See
// docs/scoring-rubric.md for what changes when calibration lands in 0.3.
func scoreToBand(score float64) models.RiskBand {
	switch {
	case score >= riskBandHighUpper:
		return models.RiskBandCritical
	case score >= riskBandMediumUpper:
		return models.RiskBandHigh
	case score >= riskBandLowUpper:
		return models.RiskBandMedium
	default:
		return models.RiskBandLow
	}
}

func scoreToBandWithHysteresis(score float64, previousBand models.RiskBand) models.RiskBand {
	if previousBand == "" {
		return scoreToBand(score)
	}

	// Deadband around each threshold to reduce band flapping near boundaries.
	// All values derive from the constants above so adjusting the model is a
	// single-place edit.
	lowUp := riskBandLowUpper + riskBandHysteresis
	mediumDown := riskBandLowUpper - riskBandHysteresis
	mediumUp := riskBandMediumUpper + riskBandHysteresis
	highDown := riskBandMediumUpper - riskBandHysteresis
	highUp := riskBandHighUpper + riskBandHysteresis
	criticalDown := riskBandHighUpper - riskBandHysteresis

	switch previousBand {
	case models.RiskBandLow:
		switch {
		case score >= highUp:
			return models.RiskBandCritical
		case score >= mediumUp:
			return models.RiskBandHigh
		case score >= lowUp:
			return models.RiskBandMedium
		default:
			return models.RiskBandLow
		}
	case models.RiskBandMedium:
		switch {
		case score >= highUp:
			return models.RiskBandCritical
		case score >= mediumUp:
			return models.RiskBandHigh
		case score < mediumDown:
			return models.RiskBandLow
		default:
			return models.RiskBandMedium
		}
	case models.RiskBandHigh:
		switch {
		case score >= highUp:
			return models.RiskBandCritical
		case score < highDown:
			if score < mediumDown {
				return models.RiskBandLow
			}
			return models.RiskBandMedium
		default:
			return models.RiskBandHigh
		}
	case models.RiskBandCritical:
		if score < criticalDown {
			if score < highDown {
				if score < mediumDown {
					return models.RiskBandLow
				}
				return models.RiskBandMedium
			}
			return models.RiskBandHigh
		}
		return models.RiskBandCritical
	default:
		return scoreToBand(score)
	}
}

// computeHybridScore returns the larger of two scores so neither very small
// nor very large repos dominate the band assignment unfairly:
//
//   - density: (weighted-severity-sum / file-count) × densityScoreScale.
//     Captures concentration. A 10-file repo with 5 medium findings
//     (weight 10) yields density 10. A 1000-file repo with the same 5
//     findings yields 0.1 — too low to flag.
//
//   - absolute: log(1+weight) × W + log(1+count) × C. Captures sheer
//     volume even when density is low. A 1000-file repo with 200 medium
//     findings has density 4 (Medium boundary) but absolute burden ~7.5
//     (still Medium, but trending up).
//
// Taking the max means a repo can land in High either by being densely
// problematic or by accumulating absolute volume. Both axes are kept
// inspectable so future calibration can adjust them independently.
func computeHybridScore(totalWeight float64, signalCount, totalFiles int) float64 {
	densityScore := totalWeight
	if totalFiles > 0 {
		densityScore = (totalWeight / float64(totalFiles)) * densityScoreScale
	}

	absoluteScore := math.Log1p(totalWeight)*absoluteWeightScale +
		math.Log1p(float64(signalCount))*absoluteCountScale

	if densityScore > absoluteScore {
		return densityScore
	}
	return absoluteScore
}

func buildExplanation(riskType string, band models.RiskBand, signals []models.Signal, totalFiles int, totalWeight, score float64) string {
	// Count by type for a useful explanation
	typeCounts := map[models.SignalType]int{}
	for _, s := range signals {
		typeCounts[s.Type]++
	}

	parts := make([]string, 0, len(typeCounts))
	for t, c := range typeCounts {
		parts = append(parts, fmt.Sprintf("%d %s", c, t))
	}
	sort.Strings(parts)

	density := "n/a"
	if totalFiles > 0 {
		densityScore := (totalWeight / float64(totalFiles)) * 10.0
		density = fmt.Sprintf("%.2f/10 across %d test files", densityScore, totalFiles)
	}
	absoluteScore := (math.Log1p(totalWeight) * 1.2) + (math.Log1p(float64(len(signals))) * 0.8)

	return fmt.Sprintf("%s risk is %s (score %.2f) from %d signal(s): density=%s, absolute=%.2f, types=%s.",
		titleRiskType(riskType), band, score, len(signals), density, absoluteScore, strings.Join(parts, ", "))
}

func buildSuggestedAction(riskType string, band models.RiskBand) string {
	switch riskType {
	case "reliability":
		return "Investigate flaky, skipped, or dead tests to improve suite reliability."
	case "change":
		if band == models.RiskBandHigh || band == models.RiskBandCritical {
			return "Prioritize adding assertions and test coverage for high-risk areas before making changes."
		}
		return "Improve test coverage and assertion quality to reduce change risk."
	case "speed":
		return "Identify and optimize slow tests to maintain fast feedback loops."
	case "governance":
		return "Address policy violations and governance findings before expanding test investments."
	default:
		return "Review contributing signals and address highest-severity items first."
	}
}

func titleRiskType(riskType string) string {
	if riskType == "" {
		return riskType
	}
	return strings.ToUpper(riskType[:1]) + riskType[1:]
}

func hasHighSeveritySignal(signals []models.Signal) bool {
	for _, s := range signals {
		if s.Severity == models.SeverityHigh || s.Severity == models.SeverityCritical {
			return true
		}
	}
	return false
}
