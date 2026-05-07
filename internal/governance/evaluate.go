// Package governance evaluates repository state against local policy
// and emits canonical governance signals for violations.
package governance

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/policy"
	sigtypes "github.com/pmclSF/terrain/internal/signals"
)

// Result holds the outcome of a policy evaluation.
type Result struct {
	Violations []models.Signal
	Pass       bool

	// Diagnostics records, per active rule, what was checked and
	// what was found — even when the rule passed. Audit-named gap
	// (policy_governance.E3): adopters needed visibility into which
	// rules ran, what they evaluated against, and why they did or
	// didn't fire. Empty when no policy is configured.
	Diagnostics []RuleDiagnostic
}

// RuleDiagnostic records one rule's evaluation outcome.
type RuleDiagnostic struct {
	// Rule is the policy rule's canonical name (e.g.
	// "disallow_skipped_tests"). Stable per release.
	Rule string

	// Status is "pass", "violated", "skipped" (rule wasn't active
	// or had no inputs to check), or "warn" (rule fired with
	// non-blocking severity).
	Status string

	// Detail is the one-sentence reason. Renders in
	// `terrain policy check --verbose`.
	Detail string

	// ViolationCount is the number of violations this rule
	// produced. Zero for pass / skipped statuses.
	ViolationCount int
}

// Evaluate checks the snapshot against the given policy and returns
// governance signals for any violations found.
//
// The evaluation is deterministic and transparent — each violation
// explains exactly what policy was violated and what evidence triggered it.
func Evaluate(snap *models.TestSuiteSnapshot, cfg *policy.Config) *Result {
	var violations []models.Signal
	var diagnostics []RuleDiagnostic

	if cfg == nil || cfg.IsEmpty() {
		return &Result{Pass: true}
	}

	checks := []struct {
		rule string
		fn   func(*models.TestSuiteSnapshot, *policy.Config) []models.Signal
		// active reports whether the rule has any input from the
		// policy file. Non-active rules emit a "skipped" diagnostic
		// rather than running so the diagnostic surface is honest
		// about which rules actually evaluated.
		active func(*policy.Config) bool
	}{
		{"disallow_frameworks", checkDisallowedFrameworks, func(c *policy.Config) bool { return len(c.Rules.DisallowFrameworks) > 0 }},
		{"disallow_skipped_tests", checkSkippedTests, func(c *policy.Config) bool { return c.Rules.DisallowSkippedTests != nil }},
		{"max_test_runtime_ms", checkRuntimeBudget, func(c *policy.Config) bool { return c.Rules.MaxTestRuntimeMs != nil }},
		{"minimum_coverage_percent", checkCoverageThreshold, func(c *policy.Config) bool { return c.Rules.MinimumCoveragePercent != nil }},
		{"max_weak_assertions", checkWeakAssertionThreshold, func(c *policy.Config) bool { return c.Rules.MaxWeakAssertions != nil }},
		{"max_mock_heavy_tests", checkMockHeavyThreshold, func(c *policy.Config) bool { return c.Rules.MaxMockHeavyTests != nil }},
		{"ai", checkAIPolicy, func(c *policy.Config) bool { return c.Rules.AI != nil }},
	}

	for _, ch := range checks {
		if !ch.active(cfg) {
			diagnostics = append(diagnostics, RuleDiagnostic{
				Rule:   ch.rule,
				Status: "skipped",
				Detail: "rule not configured in .terrain/policy.yaml",
			})
			continue
		}
		ruleViolations := ch.fn(snap, cfg)
		violations = append(violations, ruleViolations...)
		switch len(ruleViolations) {
		case 0:
			diagnostics = append(diagnostics, RuleDiagnostic{
				Rule:   ch.rule,
				Status: "pass",
				Detail: "no violations",
			})
		default:
			diagnostics = append(diagnostics, RuleDiagnostic{
				Rule:           ch.rule,
				Status:         "violated",
				Detail:         fmt.Sprintf("%d violation(s) emitted", len(ruleViolations)),
				ViolationCount: len(ruleViolations),
			})
		}
	}

	return &Result{
		Violations:  violations,
		Pass:        len(violations) == 0,
		Diagnostics: diagnostics,
	}
}

// checkDisallowedFrameworks emits legacyFrameworkUsage signals for each
// disallowed framework found in the repository.
func checkDisallowedFrameworks(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if len(cfg.Rules.DisallowFrameworks) == 0 {
		return nil
	}

	disallowed := map[string]bool{}
	for _, f := range cfg.Rules.DisallowFrameworks {
		disallowed[strings.ToLower(f)] = true
	}

	var signals []models.Signal
	for _, fw := range snap.Frameworks {
		if disallowed[strings.ToLower(fw.Name)] {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalLegacyFrameworkUsage,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityHigh,
				Confidence: 1.0,
				Location: models.SignalLocation{
					Repository: snap.Repository.Name,
				},
				Explanation: fmt.Sprintf(
					"Policy disallows framework '%s', but %d test files were detected.",
					fw.Name, fw.FileCount,
				),
				SuggestedAction: fmt.Sprintf(
					"Migrate or remove '%s' framework usage.", fw.Name,
				),
				Metadata: map[string]any{
					"framework": fw.Name,
					"fileCount": fw.FileCount,
					"rule":      "disallow_frameworks",
				},
			})
		}
	}
	return signals
}

// checkSkippedTests emits a skippedTestsInCI governance signal if skipped tests
// are found when policy disallows them.
func checkSkippedTests(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if cfg.Rules.DisallowSkippedTests == nil || !*cfg.Rules.DisallowSkippedTests {
		return nil
	}

	topFiles := topFilesForType(snap.Signals, sigtypes.SignalSkippedTest, 5)
	if len(topFiles) == 0 {
		return nil
	}

	// Count across all skippedTest signals, not just top files.
	skippedCount := countSignalsForType(snap.Signals, sigtypes.SignalSkippedTest)

	return []models.Signal{{
		Type:       sigtypes.SignalSkippedTestsInCI,
		Category:   models.CategoryGovernance,
		Severity:   models.SeverityMedium,
		Confidence: 1.0,
		Location: models.SignalLocation{
			Repository: snap.Repository.Name,
		},
		Explanation: fmt.Sprintf(
			"Policy disallows skipped tests, but %d skipped test signal(s) were detected. Top files: %s.",
			skippedCount, formatTopFiles(topFiles),
		),
		SuggestedAction: "Restore or remove skipped tests.",
		Metadata: map[string]any{
			"skippedCount": skippedCount,
			"rule":         "disallow_skipped_tests",
			"topFiles":     topFileNames(topFiles),
		},
	}}
}

// checkRuntimeBudget emits runtimeBudgetExceeded signals for test files
// whose average runtime exceeds the configured maximum.
func checkRuntimeBudget(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if cfg.Rules.MaxTestRuntimeMs == nil {
		return nil
	}
	maxMs := *cfg.Rules.MaxTestRuntimeMs

	var signals []models.Signal
	for _, tf := range snap.TestFiles {
		if tf.RuntimeStats != nil && tf.RuntimeStats.AvgRuntimeMs > 0 && tf.RuntimeStats.AvgRuntimeMs > maxMs {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalRuntimeBudgetExceeded,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityMedium,
				Confidence: 1.0,
				Location: models.SignalLocation{
					File: tf.Path,
				},
				Explanation: fmt.Sprintf(
					"Observed runtime %.0fms exceeds configured max_test_runtime_ms of %.0f.",
					tf.RuntimeStats.AvgRuntimeMs, maxMs,
				),
				SuggestedAction: "Reduce runtime hotspots or adjust policy intentionally.",
				Metadata: map[string]any{
					"observedMs": tf.RuntimeStats.AvgRuntimeMs,
					"budgetMs":   maxMs,
					"rule":       "max_test_runtime_ms",
				},
			})
		}
	}
	return signals
}

// checkCoverageThreshold emits a policyViolation if coverage threshold
// break signals exist and policy requires minimum coverage.
func checkCoverageThreshold(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if cfg.Rules.MinimumCoveragePercent == nil {
		return nil
	}

	var coverageBreaks int
	for _, s := range snap.Signals {
		if s.Type == sigtypes.SignalCoverageThresholdBreak {
			coverageBreaks++
		}
	}

	if coverageBreaks == 0 {
		return nil
	}

	return []models.Signal{{
		Type:       sigtypes.SignalPolicyViolation,
		Category:   models.CategoryGovernance,
		Severity:   models.SeverityHigh,
		Confidence: 1.0,
		Location: models.SignalLocation{
			Repository: snap.Repository.Name,
		},
		Explanation: fmt.Sprintf(
			"Policy requires minimum %.0f%% coverage, but %d coverage threshold break(s) detected.",
			*cfg.Rules.MinimumCoveragePercent, coverageBreaks,
		),
		SuggestedAction: "Increase test coverage to meet the configured threshold.",
		Metadata: map[string]any{
			"coverageBreaks": coverageBreaks,
			"threshold":      *cfg.Rules.MinimumCoveragePercent,
			"rule":           "minimum_coverage_percent",
		},
	}}
}

// checkWeakAssertionThreshold emits a policyViolation if weakAssertion
// signal count exceeds the configured maximum.
func checkWeakAssertionThreshold(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if cfg.Rules.MaxWeakAssertions == nil {
		return nil
	}

	count := countSignalsForType(snap.Signals, sigtypes.SignalWeakAssertion)
	topFiles := topFilesForType(snap.Signals, sigtypes.SignalWeakAssertion, 5)
	baseMax := *cfg.Rules.MaxWeakAssertions
	effectiveMax := sizeAdjustedThreshold(baseMax, len(snap.TestFiles))
	if count <= effectiveMax {
		return nil
	}

	return []models.Signal{{
		Type:       sigtypes.SignalPolicyViolation,
		Category:   models.CategoryGovernance,
		Severity:   models.SeverityMedium,
		Confidence: 1.0,
		Location: models.SignalLocation{
			Repository: snap.Repository.Name,
		},
		Explanation: fmt.Sprintf(
			"Found %d weakAssertion signal(s), exceeding effective policy maximum of %d (base %d, suite size %d files). Top files: %s.",
			count, effectiveMax, baseMax, len(snap.TestFiles), formatTopFiles(topFiles),
		),
		SuggestedAction: "Add meaningful assertions to test files with weak or missing assertions.",
		Metadata: map[string]any{
			"count":        count,
			"max":          effectiveMax,
			"baseMax":      baseMax,
			"totalFiles":   len(snap.TestFiles),
			"rule":         "max_weak_assertions",
			"sizeAdjusted": effectiveMax != baseMax,
			"topFiles":     topFileNames(topFiles),
		},
	}}
}

// checkMockHeavyThreshold emits a policyViolation if mockHeavyTest
// signal count exceeds the configured maximum.
func checkMockHeavyThreshold(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	if cfg.Rules.MaxMockHeavyTests == nil {
		return nil
	}

	count := countSignalsForType(snap.Signals, sigtypes.SignalMockHeavyTest)
	topFiles := topFilesForType(snap.Signals, sigtypes.SignalMockHeavyTest, 5)
	baseMax := *cfg.Rules.MaxMockHeavyTests
	effectiveMax := sizeAdjustedThreshold(baseMax, len(snap.TestFiles))
	if count <= effectiveMax {
		return nil
	}

	return []models.Signal{{
		Type:       sigtypes.SignalPolicyViolation,
		Category:   models.CategoryGovernance,
		Severity:   models.SeverityMedium,
		Confidence: 1.0,
		Location: models.SignalLocation{
			Repository: snap.Repository.Name,
		},
		Explanation: fmt.Sprintf(
			"Found %d mockHeavyTest signal(s), exceeding effective policy maximum of %d (base %d, suite size %d files). Top files: %s.",
			count, effectiveMax, baseMax, len(snap.TestFiles), formatTopFiles(topFiles),
		),
		SuggestedAction: "Reduce mock usage in favor of real implementations in tests.",
		Metadata: map[string]any{
			"count":        count,
			"max":          effectiveMax,
			"baseMax":      baseMax,
			"totalFiles":   len(snap.TestFiles),
			"rule":         "max_mock_heavy_tests",
			"sizeAdjusted": effectiveMax != baseMax,
			"topFiles":     topFileNames(topFiles),
		},
	}}
}

// checkAIPolicy evaluates AI-specific policy rules against snapshot signals.
func checkAIPolicy(snap *models.TestSuiteSnapshot, cfg *policy.Config) []models.Signal {
	ai := cfg.Rules.AI
	if ai == nil {
		return nil
	}

	var signals []models.Signal

	// Count AI signals by type.
	aiSignalCounts := map[models.SignalType]int{}
	for _, s := range snap.Signals {
		if s.Category == models.CategoryAI {
			aiSignalCounts[s.Type]++
		}
	}

	// Rule: block on safety failure.
	if ai.BlockOnSafetyFailure != nil && *ai.BlockOnSafetyFailure {
		if count := aiSignalCounts[sigtypes.SignalSafetyFailure]; count > 0 {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityCritical,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy requires zero safety failures, but %d safetyFailure signal(s) detected.",
					count,
				),
				SuggestedAction: "Fix all safety evaluation failures before merging.",
				Metadata: map[string]any{
					"rule":  "block_on_safety_failure",
					"count": count,
				},
			})
		}
	}

	// Rule: block on accuracy regression above threshold.
	if ai.BlockOnAccuracyRegression != nil {
		threshold := *ai.BlockOnAccuracyRegression
		count := aiSignalCounts[sigtypes.SignalAccuracyRegression]
		if count > threshold {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityHigh,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy allows at most %d accuracy regression(s), but %d detected.",
					threshold, count,
				),
				SuggestedAction: "Investigate accuracy regressions and update baselines if intentional.",
				Metadata: map[string]any{
					"rule":      "block_on_accuracy_regression",
					"threshold": threshold,
					"count":     count,
				},
			})
		}
	}

	// Rule: block on uncovered AI context surfaces.
	if ai.BlockOnUncoveredContext != nil && *ai.BlockOnUncoveredContext {
		coveredIDs := map[string]bool{}
		for _, sc := range snap.Scenarios {
			for _, sid := range sc.CoveredSurfaceIDs {
				coveredIDs[sid] = true
			}
		}
		uncoveredCount := 0
		for _, cs := range snap.CodeSurfaces {
			if cs.Kind == models.SurfaceContext && !coveredIDs[cs.SurfaceID] {
				uncoveredCount++
			}
		}
		if uncoveredCount > 0 {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityHigh,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy requires all context surfaces to have scenario coverage, but %d context surface(s) are uncovered.",
					uncoveredCount,
				),
				SuggestedAction: "Add eval scenarios covering uncovered context surfaces.",
				Metadata: map[string]any{
					"rule":           "block_on_uncovered_context",
					"uncoveredCount": uncoveredCount,
				},
			})
		}
	}

	// Rule: warn on latency regression.
	if ai.WarnOnLatencyRegression == nil || *ai.WarnOnLatencyRegression {
		if count := aiSignalCounts[sigtypes.SignalLatencyRegression]; count > 0 {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityMedium,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy warning: %d latency regression(s) detected.",
					count,
				),
				SuggestedAction: "Review latency regressions — they may impact user experience.",
				Metadata: map[string]any{
					"rule":  "warn_on_latency_regression",
					"count": count,
				},
			})
		}
	}

	// Rule: warn on cost regression.
	if ai.WarnOnCostRegression == nil || *ai.WarnOnCostRegression {
		if count := aiSignalCounts[sigtypes.SignalCostRegression]; count > 0 {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityMedium,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy warning: %d cost regression(s) detected.",
					count,
				),
				SuggestedAction: "Review cost regressions — they may impact operational budget.",
				Metadata: map[string]any{
					"rule":  "warn_on_cost_regression",
					"count": count,
				},
			})
		}
	}

	// Rule: custom blocking signal types.
	blockSet := map[string]bool{}
	for _, t := range ai.BlockingSignalTypes {
		blockSet[t] = true
	}
	// Sort signal types for deterministic violation order.
	var sortedSigTypes []models.SignalType
	for sigType := range aiSignalCounts {
		sortedSigTypes = append(sortedSigTypes, sigType)
	}
	sort.Slice(sortedSigTypes, func(i, j int) bool {
		return string(sortedSigTypes[i]) < string(sortedSigTypes[j])
	})
	for _, sigType := range sortedSigTypes {
		count := aiSignalCounts[sigType]
		if blockSet[string(sigType)] {
			signals = append(signals, models.Signal{
				Type:       sigtypes.SignalPolicyViolation,
				Category:   models.CategoryGovernance,
				Severity:   models.SeverityHigh,
				Confidence: 1.0,
				Explanation: fmt.Sprintf(
					"AI policy blocks on %s: %d signal(s) detected.",
					sigType, count,
				),
				SuggestedAction: fmt.Sprintf("Resolve all %s signals before merging.", sigType),
				Metadata: map[string]any{
					"rule":       "blocking_signal_types",
					"signalType": string(sigType),
					"count":      count,
				},
			})
		}
	}

	return signals
}

func sizeAdjustedThreshold(baseMax, totalFiles int) int {
	// Keep legacy behavior for missing suite size information.
	if baseMax <= 0 || totalFiles <= 0 {
		return baseMax
	}
	// Policy thresholds are interpreted as baseline limits for a 100-file suite.
	// Larger suites scale linearly to avoid penalizing healthy large repos.
	if totalFiles <= 100 {
		return baseMax
	}
	scaled := int(math.Ceil(float64(baseMax) * (float64(totalFiles) / 100.0)))
	if scaled < baseMax {
		return baseMax
	}
	return scaled
}

// fileCount tracks signal count per file for governance reporting.
type fileCount struct {
	file  string
	count int
}

func countSignalsForType(signals []models.Signal, signalType models.SignalType) int {
	total := 0
	for _, s := range signals {
		if s.Type == signalType {
			total++
		}
	}
	return total
}

// topFilesForType returns the top N files with the most signals of the given type.
func topFilesForType(signals []models.Signal, signalType models.SignalType, limit int) []fileCount {
	counts := map[string]int{}
	total := 0
	for _, s := range signals {
		if s.Type != signalType {
			continue
		}
		total++
		if s.Location.File != "" {
			counts[s.Location.File]++
		}
	}

	if total == 0 {
		return nil
	}

	// If no file-level signals, return a single entry with empty file.
	if len(counts) == 0 {
		return []fileCount{{file: "", count: total}}
	}

	result := make([]fileCount, 0, len(counts))
	for f, c := range counts {
		result = append(result, fileCount{file: f, count: c})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].count != result[j].count {
			return result[i].count > result[j].count
		}
		return result[i].file < result[j].file
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// formatTopFiles formats file counts for human-readable explanations.
func formatTopFiles(files []fileCount) string {
	parts := make([]string, 0, len(files))
	for _, f := range files {
		if f.file == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%d)", f.file, f.count))
	}
	if len(parts) == 0 {
		return "(no file-level detail)"
	}
	return strings.Join(parts, ", ")
}

// topFileNames extracts just the file paths for metadata.
func topFileNames(files []fileCount) []string {
	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.file != "" {
			names = append(names, f.file)
		}
	}
	return names
}
