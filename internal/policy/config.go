// Package policy implements Terrain's local, repo-native policy system.
//
// A policy is loaded from .terrain/policy.yaml in the analyzed repository.
// It declares constraints that the repository's test suite should satisfy.
// Policy evaluation produces governance signals when violations are found.
//
// The policy system is intentionally simple and explicit:
//   - no magic defaults
//   - no central management
//   - no complex DSL
//
// The local policy file is the ground truth for a single repository.
package policy

import "strings"

// Config represents a local Terrain policy loaded from .terrain/policy.yaml.
//
// All fields are pointers or zero-value-safe so that a partial policy
// file works correctly — only explicitly set rules are enforced.
type Config struct {
	Rules Rules `yaml:"rules"`
}

// Rules contains the individual policy rules.
//
// Each rule is a pointer type so we can distinguish "not set" from
// "set to zero/false". A nil pointer means the rule is not active.
type Rules struct {
	// DisallowSkippedTests, when true, flags any skipped/pending tests
	// as a policy violation.
	DisallowSkippedTests *bool `yaml:"disallow_skipped_tests"`

	// DisallowFrameworks lists framework names that are not permitted.
	// If the repository contains test files using any of these frameworks,
	// a legacyFrameworkUsage governance signal is emitted.
	DisallowFrameworks []string `yaml:"disallow_frameworks"`

	// MaxTestRuntimeMs sets the maximum allowed average test runtime in
	// milliseconds. If any test file's average runtime exceeds this,
	// a runtimeBudgetExceeded governance signal is emitted.
	MaxTestRuntimeMs *float64 `yaml:"max_test_runtime_ms"`

	// MinimumCoveragePercent sets the minimum required coverage percentage.
	// This is evaluated against existing coverage threshold signals.
	// Value should be 0-100 (e.g., 80 means 80%).
	MinimumCoveragePercent *float64 `yaml:"minimum_coverage_percent"`

	// MaxWeakAssertions sets the maximum allowed number of weakAssertion
	// signals before a policy violation is raised.
	MaxWeakAssertions *int `yaml:"max_weak_assertions"`

	// MaxMockHeavyTests sets the maximum allowed number of mockHeavyTest
	// signals before a policy violation is raised.
	MaxMockHeavyTests *int `yaml:"max_mock_heavy_tests"`

	// DisabledDetectors lists detector signal types (e.g., "weakAssertion",
	// "mockHeavyTest") that should be suppressed entirely — no findings
	// emitted, no CI gating. Use when a detector has been demoted to
	// observability tier but its findings are still cluttering output,
	// OR when an adopter has confirmed the detector's signal isn't useful
	// for their codebase.
	//
	// Safety machinery: every CI-blocking detector has a per-rule kill
	// switch. Equivalent to `--disable-rule` on the CLI.
	//
	// Alias expansion: a bare rule_id that's registered as an alias old-ID
	// expands through the alias registry. E.g. `aiHardcodedAPIKey` disables
	// the back-compat shim AND every split half (literal-shape +
	// secret-scanner-coverage-degraded). To disable JUST the back-compat
	// rule and keep the split halves firing, prefix the name with `=`:
	//
	//   disabled_detectors:
	//     - "=aiHardcodedAPIKey"         # literal: only the back-compat shim
	//     - aiHardcodedAPIKey-literal-shape  # explicit new ID
	//
	// Example .terrain/policy.yaml:
	//   rules:
	//     disabled_detectors:
	//       - mockHeavyTest
	//       - weakAssertion
	DisabledDetectors []string `yaml:"disabled_detectors"`

	// EnabledDetectors opts back IN to detectors that are marked
	// DisabledByDefault in the manifest. Example: an adopter wants to
	// run aiPromptInjectionRisk on their codebase even though the
	// default config has it off. The list takes plain signal-type
	// names (no alias prefix). Has no effect on detectors that are
	// not in the default-disabled set.
	//
	// Alias expansion: a name listed here is matched as-is against the
	// manifest signal type. If a detector ships under a deprecated
	// rule_id with an alias entry, use the canonical new ID (the alias
	// registry does NOT expand opt-ins; expansion only applies to the
	// `disabled_detectors` opt-out list).
	//
	// Example .terrain/policy.yaml:
	//   rules:
	//     enabled_detectors:
	//       - aiPromptInjectionRisk
	EnabledDetectors []string `yaml:"enabled_detectors"`

	// AI holds AI/eval-specific CI policy rules.
	AI *AIRules `yaml:"ai"`
}

// AIRules defines CI policy for AI risk review.
//
// Example .terrain/policy.yaml:
//
//	rules:
//	  ai:
//	    block_on_safety_failure: true
//	    block_on_accuracy_regression: 0
//	    block_on_uncovered_context: true
//	    warn_on_latency_regression: true
//	    warn_on_cost_regression: true
//	    blocking_signal_types:
//	      - hallucinationDetected
//	      - aiPolicyViolation
type AIRules struct {
	// BlockOnSafetyFailure fails CI when any safetyFailure signal is present.
	BlockOnSafetyFailure *bool `yaml:"block_on_safety_failure"`

	// BlockOnAccuracyRegression fails CI when accuracyRegression signal count
	// exceeds this value. 0 means any regression blocks.
	BlockOnAccuracyRegression *int `yaml:"block_on_accuracy_regression"`

	// BlockOnUncoveredContext fails CI when a changed AI context surface has
	// no scenario coverage.
	BlockOnUncoveredContext *bool `yaml:"block_on_uncovered_context"`

	// WarnOnLatencyRegression emits a warning for latency regressions.
	WarnOnLatencyRegression *bool `yaml:"warn_on_latency_regression"`

	// WarnOnCostRegression emits a warning for cost regressions.
	WarnOnCostRegression *bool `yaml:"warn_on_cost_regression"`

	// WarnOnMissingCapabilityCoverage emits a warning when an AI capability
	// has no scenario coverage.
	WarnOnMissingCapabilityCoverage *bool `yaml:"warn_on_missing_capability_coverage"`

	// BlockingSignalTypes lists additional AI signal types that block CI.
	// e.g., ["hallucinationDetected", "aiPolicyViolation"]
	BlockingSignalTypes []string `yaml:"blocking_signal_types"`
}

// IsEmpty returns true if no rules are configured.
func (c *Config) IsEmpty() bool {
	r := c.Rules
	return r.DisallowSkippedTests == nil &&
		len(r.DisallowFrameworks) == 0 &&
		r.MaxTestRuntimeMs == nil &&
		r.MinimumCoveragePercent == nil &&
		r.MaxWeakAssertions == nil &&
		r.MaxMockHeavyTests == nil &&
		len(r.DisabledDetectors) == 0 &&
		len(r.EnabledDetectors) == 0 &&
		r.AI == nil
}

// DisabledDetectorSet returns the configured disabled detectors as a
// lookup set for O(1) membership checks during signal filtering. The
// set normalizes whitespace and is case-sensitive on signal-type names
// (the manifest uses camelCase signal types — we don't lowercase to
// avoid surprise when an adopter's typo silently disables nothing).
func (c *Config) DisabledDetectorSet() map[string]bool {
	if c == nil || len(c.Rules.DisabledDetectors) == 0 {
		return nil
	}
	out := make(map[string]bool, len(c.Rules.DisabledDetectors))
	for _, d := range c.Rules.DisabledDetectors {
		d = strings.TrimSpace(d)
		if d != "" {
			out[d] = true
		}
	}
	return out
}

// EnabledDetectorSet returns the configured enabled-detector opt-ins
// as a lookup set. Mirrors DisabledDetectorSet shape but expresses an
// opt-in: a detector that is disabled by default at the manifest
// level can be re-enabled by listing its rule_id here.
func (c *Config) EnabledDetectorSet() map[string]bool {
	if c == nil || len(c.Rules.EnabledDetectors) == 0 {
		return nil
	}
	out := make(map[string]bool, len(c.Rules.EnabledDetectors))
	for _, d := range c.Rules.EnabledDetectors {
		d = strings.TrimSpace(d)
		if d != "" {
			out[d] = true
		}
	}
	return out
}
