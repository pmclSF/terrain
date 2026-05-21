// Package migration implements detectors for migration-related signals.
//
// Migration signals help teams understand what makes framework migration
// difficult, what patterns are deprecated, and where modernization effort
// should be concentrated.
//
// These detectors analyze test file content for patterns that indicate
// migration complexity. They are heuristic-based and conservative.
package framework_migration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/looppredicate"
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/triggergate"
)

// Blocker taxonomy categories.
// These help group migration issues for prioritization.
const (
	BlockerCustomMatcher     = "custom-matcher"
	BlockerDynamicGeneration = "dynamic-generation"
	BlockerDeprecatedPattern = "deprecated-pattern"
	BlockerFrameworkHelper   = "framework-helper"
	BlockerUnsupportedSetup  = "unsupported-setup"
)

// Blocker severity tiers control how signals affect migration readiness.
const (
	// TierHardBlocker: genuinely prevents migration without significant rework.
	// Examples: Enzyme usage (unmaintained library), Cypress plugin internals.
	TierHardBlocker = "hard-blocker"

	// TierSoftBlocker: requires effort but has a clear migration path.
	// Examples: custom matchers (need rewriting), done-callbacks (need async/await).
	TierSoftBlocker = "soft-blocker"

	// TierAdvisory: worth knowing about but doesn't affect migration readiness.
	// Examples: setTimeout in tests (works everywhere), dynamic test generation
	// (standard practice), snapshot testing (supported by all modern frameworks).
	TierAdvisory = "advisory"
)

// patternToTier maps specific deprecated patterns to their severity tier.
var patternToTier = map[string]string{
	// Hard blockers — genuinely block migration.
	"enzyme-usage":            TierHardBlocker,
	"cypress-custom-commands": TierHardBlocker,
	"cypress-plugin-events":   TierHardBlocker,

	// Soft blockers — require effort but have clear migration paths.
	"done-callback":          TierSoftBlocker,
	"sinon-standalone":       TierSoftBlocker,
	"jest-global-setup":      TierSoftBlocker,
	"mocha-root-hooks":       TierSoftBlocker,
	"framework-test-context": TierSoftBlocker,

	// Advisories — valid patterns that work across frameworks.
	"setTimeout-in-test": TierAdvisory,
}

// blockerTypeToDefaultTier maps blocker categories to their default tier
// when no specific pattern-level override exists.
var blockerTypeToDefaultTier = map[string]string{
	BlockerCustomMatcher:     TierSoftBlocker,
	BlockerDynamicGeneration: TierAdvisory,
	BlockerDeprecatedPattern: TierSoftBlocker,
	BlockerFrameworkHelper:   TierSoftBlocker,
	BlockerUnsupportedSetup:  TierSoftBlocker,
}

// TierForSignal determines the blocker tier for a migration signal
// based on its metadata.
func TierForSignal(s models.Signal) string {
	if s.Type == "frameworkMigration" {
		return TierAdvisory
	}
	// Check for explicit pattern-level tier first.
	if pattern, ok := s.Metadata["pattern"].(string); ok {
		if tier, ok := patternToTier[pattern]; ok {
			return tier
		}
	}
	// Fall back to blocker-type-level default.
	if bt, ok := s.Metadata["blockerType"].(string); ok {
		if tier, ok := blockerTypeToDefaultTier[bt]; ok {
			return tier
		}
	}
	return TierSoftBlocker // conservative default
}

// DeprecatedPatternDetector identifies deprecated or outdated test patterns
// that complicate migration and should be modernized.
type DeprecatedPatternDetector struct {
	RepoRoot string
}

func (d *DeprecatedPatternDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal
	// Track hard-blocker patterns to roll up into one migrationBlocker
	// per pattern across the project. Before this change, the detector
	// emitted a separate migrationBlocker per (pattern, file) — which
	// produced 1,097 duplicate "enzyme-usage" signals on AutoGPT alone
	// (verified on the 80-repo OSS corpus). Now the per-file signal is
	// `deprecatedTestPattern` (still per-file, actionable) and the
	// project-wide migrationBlocker carries the file list in metadata.
	blockerFiles := map[string][]string{}      // pattern -> file list
	blockerOwners := map[string]string{}       // pattern -> first owner seen

	for _, tf := range snap.TestFiles {
		lang := frameworkLanguage(tf.Framework)
		if lang != "js" {
			continue
		}

		content := readFile(d.RepoRoot, tf.Path)
		if content == "" {
			continue
		}

		found := detectDeprecatedJS(content, tf.Framework)
		for _, pattern := range found {
			// Mechanism gate: deprecated_test_pattern_trigger_gate.
			// When ON, only fire enzyme-usage on files that actually
			// import enzyme. The legacy regex match alone produces
			// framework-mismatch false positives (e.g. matching
			// `mount(<Foo>)` in vitest tests).
			if pattern == "enzyme-usage" {
				absPath := filepath.Join(d.RepoRoot, tf.Path)
				keep := triggergate.GateImports(
					mechanisms.Default(), absPath, "deprecatedTestPattern",
					[]string{"enzyme", "enzyme-adapter-*"},
				)
				if !keep {
					continue
				}
			}
			tier := patternToTier[pattern]
			if tier == "" {
				tier = blockerTypeToDefaultTier[BlockerDeprecatedPattern]
			}
			signals = append(signals, models.Signal{
				Type:             "deprecatedTestPattern",
				Category:         models.CategoryMigration,
				Severity:         models.SeverityMedium,
				Confidence:       0.7,
				EvidenceStrength: models.EvidenceModerate,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Owner:            tf.Owner,
				Explanation: fmt.Sprintf(
					"Deprecated pattern '%s' found in %s.",
					pattern, tf.Path,
				),
				SuggestedAction: "Update to modern testing patterns before migration.",
				Metadata: map[string]any{
					"pattern":     pattern,
					"blockerType": BlockerDeprecatedPattern,
				},
			})

			if tier == TierHardBlocker {
				blockerFiles[pattern] = append(blockerFiles[pattern], tf.Path)
				if _, ok := blockerOwners[pattern]; !ok {
					blockerOwners[pattern] = tf.Owner
				}
			}
		}
	}

	// Emit one migrationBlocker per pattern (project-wide rollup).
	signals = append(signals, rolledUpMigrationBlockers(blockerFiles, blockerOwners, BlockerDeprecatedPattern, 0.8)...)
	return signals
}

// rolledUpMigrationBlockers emits one migrationBlocker signal per
// distinct pattern, with the affected file list captured in metadata
// (full list + count + first 5 as a sample for the explanation).
// Shared helper so both DeprecatedPatternDetector and
// UnsupportedSetupDetector aggregate the same way.
func rolledUpMigrationBlockers(
	blockerFiles map[string][]string,
	blockerOwners map[string]string,
	blockerType string,
	confidence float64,
) []models.Signal {
	if len(blockerFiles) == 0 {
		return nil
	}
	// Deterministic ordering by pattern name.
	patterns := make([]string, 0, len(blockerFiles))
	for p := range blockerFiles {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)

	var out []models.Signal
	for _, pattern := range patterns {
		files := blockerFiles[pattern]
		sort.Strings(files)
		sample := files
		if len(sample) > 5 {
			sample = sample[:5]
		}
		// Representative location = first affected file.
		repLocation := models.SignalLocation{File: files[0]}
		out = append(out, models.Signal{
			Type:             "migrationBlocker",
			Category:         models.CategoryMigration,
			Severity:         models.SeverityHigh,
			Confidence:       confidence,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceStructuralPattern,
			Location:         repLocation,
			Owner:            blockerOwners[pattern],
			Explanation: fmt.Sprintf(
				"Hard migration blocker '%s' affects %d file(s); each requires rework before migration. Sample: %s",
				pattern, len(files), strings.Join(sample, ", "),
			),
			SuggestedAction: "Address this blocker (across all affected files) before attempting framework migration.",
			Metadata: map[string]any{
				"pattern":       pattern,
				"blockerType":   blockerType,
				"blockerTier":   TierHardBlocker,
				"file_count":    len(files),
				"files":         files,
				"sample_files":  sample,
			},
		})
	}
	return out
}

// DynamicTestGenerationDetector identifies tests that use dynamic test
// generation patterns, which reduce migration predictability.
type DynamicTestGenerationDetector struct {
	RepoRoot string
}

func (d *DynamicTestGenerationDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal

	for _, tf := range snap.TestFiles {
		lang := frameworkLanguage(tf.Framework)
		if lang != "js" {
			continue
		}

		content := readFile(d.RepoRoot, tf.Path)
		if content == "" {
			continue
		}

		if line, ok := dynamicTestGenerationLine(content); ok {
			// Mechanism gate: a3_loop_predicate. When ON, the
			// AST-precise looppredicate verifies the it/test/describe
			// call at `line` is actually wrapped by a loop construct.
			absPath := filepath.Join(d.RepoRoot, tf.Path)
			if !looppredicate.Gate(mechanisms.Default(), absPath, line, "dynamicTestGeneration") {
				continue
			}
			signals = append(signals, models.Signal{
				Type:             "dynamicTestGeneration",
				Category:         models.CategoryMigration,
				Severity:         models.SeverityLow,
				Confidence:       0.6,
				EvidenceStrength: models.EvidenceModerate,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Owner:            tf.Owner,
				Explanation: fmt.Sprintf(
					"Dynamic test generation detected in %s. Parameterized tests are standard practice but may need syntax adjustment during migration.",
					tf.Path,
				),
				SuggestedAction: "Review dynamic test generation for migration compatibility.",
				Metadata: map[string]any{
					"blockerType": BlockerDynamicGeneration,
					"blockerTier": TierAdvisory,
				},
			})
		}
	}
	return signals
}

// CustomMatcherDetector identifies custom matcher/helper patterns that
// complicate framework migration.
type CustomMatcherDetector struct {
	RepoRoot string
}

func (d *CustomMatcherDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal

	for _, tf := range snap.TestFiles {
		lang := frameworkLanguage(tf.Framework)
		if lang != "js" {
			continue
		}

		content := readFile(d.RepoRoot, tf.Path)
		if content == "" {
			continue
		}

		if hasCustomMatchers(content) {
			signals = append(signals, models.Signal{
				Type:             "customMatcherRisk",
				Category:         models.CategoryMigration,
				Severity:         models.SeverityLow,
				Confidence:       0.5,
				EvidenceStrength: models.EvidenceWeak,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Owner:            tf.Owner,
				Explanation: fmt.Sprintf(
					"Custom matcher or assertion helper usage in %s may need manual migration.",
					tf.Path,
				),
				SuggestedAction: "Document custom matchers and plan manual migration strategy.",
				Metadata: map[string]any{
					"blockerType": BlockerCustomMatcher,
				},
			})
		}
	}
	return signals
}

// Deprecated JS patterns
var (
	// done() callback pattern (use async/await instead)
	doneCallbackPattern = regexp.MustCompile(`\b(it|test)\s*\(\s*[^,]*,\s*(?:async\s*)?(?:function\s*\(\s*done\s*\)|\(\s*done\s*\)\s*=>|done\s*=>)`)
	// setTimeout in tests (fragile timing)
	setTimeoutInTest = regexp.MustCompile(`setTimeout\s*\(`)
	// Enzyme-specific patterns (deprecated React testing)
	enzymePattern = regexp.MustCompile(`(?s)\b(shallow|mount|render)\s*\([^)]*<`)
	// sinon standalone usage (older mocking)
	sinonPattern = regexp.MustCompile(`\bsinon\.(stub|mock|spy)\s*\(`)
)

func detectDeprecatedJS(content string, framework string) []string {
	content = stripJSNoise(content)
	var found []string
	// done-callback is idiomatic in mocha; only flag for other frameworks
	if framework != "mocha" && doneCallbackPattern.MatchString(content) {
		found = append(found, "done-callback")
	}
	if setTimeoutInTest.MatchString(content) {
		found = append(found, "setTimeout-in-test")
	}
	if enzymePattern.MatchString(content) {
		found = append(found, "enzyme-usage")
	}
	if sinonPattern.MatchString(content) {
		found = append(found, "sinon-standalone")
	}
	return found
}

// Dynamic test generation patterns
var (
	forEachTestPattern = regexp.MustCompile(`\.\s*forEach\s*\([^)]*\)\s*=>\s*\{[^}]*(it|test|describe)\s*\(`)
	mapTestPattern     = regexp.MustCompile(`\.\s*map\s*\([^)]*\)\s*=>\s*\{[^}]*(it|test|describe)\s*\(`)
	testEachPattern    = regexp.MustCompile(`\b(it|test|describe)\.each\s*[\(\[]`)
	forLoopTestPattern = regexp.MustCompile(`for\s*\([^)]+\)\s*\{[^}]*(it|test|describe)\s*\(`)
)

func hasDynamicTestGeneration(content string) bool {
	content = stripJSNoise(content)
	return forEachTestPattern.MatchString(content) ||
		mapTestPattern.MatchString(content) ||
		testEachPattern.MatchString(content) ||
		forLoopTestPattern.MatchString(content)
}

// dynamicTestGenerationLine returns the 1-based line number of the
// first dynamic-test-gen pattern match, plus a presence bool. Used
// by the a3_loop_predicate gate to test whether the test builder at
// that line is actually inside a loop construct (AST-precise).
func dynamicTestGenerationLine(content string) (int, bool) {
	stripped := stripJSNoise(content)
	var bestIdx = -1
	for _, re := range []*regexp.Regexp{
		forEachTestPattern, mapTestPattern, testEachPattern, forLoopTestPattern,
	} {
		if loc := re.FindStringIndex(stripped); loc != nil {
			if bestIdx == -1 || loc[0] < bestIdx {
				bestIdx = loc[0]
			}
		}
	}
	if bestIdx == -1 {
		return 0, false
	}
	// Convert byte offset in stripped → 1-based line number in stripped.
	// stripJSNoise preserves newlines (it replaces comments/strings with
	// spaces) so line numbers are stable.
	return 1 + strings.Count(stripped[:bestIdx], "\n"), true
}

// Custom matcher patterns
var (
	addMatcherPattern    = regexp.MustCompile(`\bexpect\.extend\s*\(`)
	customMatcherPattern = regexp.MustCompile(`\bjest\.addMatchers?\s*\(`)
	chaiUsePattern       = regexp.MustCompile(`\bchai\.use\s*\(`)
	customAssertHelper   = regexp.MustCompile(`\bfunction\s+assert[A-Z]\w+\s*\(`)
)

func hasCustomMatchers(content string) bool {
	content = stripJSNoise(content)
	return addMatcherPattern.MatchString(content) ||
		customMatcherPattern.MatchString(content) ||
		chaiUsePattern.MatchString(content) ||
		customAssertHelper.MatchString(content)
}

func readFile(root, relPath string) string {
	data, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		return ""
	}
	return string(data)
}

func frameworkLanguage(framework string) string {
	switch framework {
	case "jest", "vitest", "mocha", "jasmine", "cypress", "playwright", "puppeteer", "webdriverio", "testcafe", "node-test":
		return "js"
	case "go-testing":
		return "go"
	case "pytest", "unittest", "nose2":
		return "python"
	case "junit4", "junit5", "testng":
		return "java"
	default:
		return ""
	}
}

// UnsupportedSetupDetector identifies framework-specific setup and fixture
// patterns that may not have equivalents in a target framework.
type UnsupportedSetupDetector struct {
	RepoRoot string
}

func (d *UnsupportedSetupDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal
	// Same rollup pattern as DeprecatedPatternDetector — see comment there.
	blockerFiles := map[string][]string{}
	blockerOwners := map[string]string{}

	for _, tf := range snap.TestFiles {
		lang := frameworkLanguage(tf.Framework)
		if lang != "js" {
			continue
		}

		content := readFile(d.RepoRoot, tf.Path)
		if content == "" {
			continue
		}

		found := detectUnsupportedSetup(content, tf.Framework)
		for _, pattern := range found {
			tier := patternToTier[pattern]
			if tier == "" {
				tier = blockerTypeToDefaultTier[BlockerUnsupportedSetup]
			}
			signals = append(signals, models.Signal{
				Type:             "unsupportedSetup",
				Category:         models.CategoryMigration,
				Severity:         models.SeverityMedium,
				Confidence:       0.6,
				EvidenceStrength: models.EvidenceModerate,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Owner:            tf.Owner,
				Explanation: fmt.Sprintf(
					"Framework-specific setup pattern '%s' in %s may not have a direct equivalent in other frameworks.",
					pattern, tf.Path,
				),
				SuggestedAction: "Review setup/fixture patterns for migration compatibility.",
				Metadata: map[string]any{
					"pattern":     pattern,
					"blockerType": BlockerUnsupportedSetup,
				},
			})

			if tier == TierHardBlocker {
				blockerFiles[pattern] = append(blockerFiles[pattern], tf.Path)
				if _, ok := blockerOwners[pattern]; !ok {
					blockerOwners[pattern] = tf.Owner
				}
			}
		}
	}
	signals = append(signals, rolledUpMigrationBlockers(blockerFiles, blockerOwners, BlockerUnsupportedSetup, 0.7)...)
	return signals
}

// Unsupported setup/fixture patterns
var (
	// Jest-specific global setup
	jestGlobalSetup = regexp.MustCompile(`\bglobalSetup\b|\bglobalTeardown\b`)
	// Mocha-specific root hooks
	mochaRootHooks = regexp.MustCompile(`\brootHooks?\b|\bexports\.mochaHooks\b`)
	// Cypress-specific commands and plugins
	cypressCommands = regexp.MustCompile(`\bCypress\.Commands\.add\s*\(`)
	cypressPlugins  = regexp.MustCompile(`\bCypress\.on\s*\(\s*['"]`)
	// Framework-specific test context
	testContextPattern = regexp.MustCompile(`\bthis\.timeout\s*\(|\bthis\.retries\s*\(|\bthis\.slow\s*\(`)
)

func detectUnsupportedSetup(content string, framework string) []string {
	content = stripJSNoise(content)
	var found []string
	if framework == "jest" && jestGlobalSetup.MatchString(content) {
		found = append(found, "jest-global-setup")
	}
	if framework == "mocha" && mochaRootHooks.MatchString(content) {
		found = append(found, "mocha-root-hooks")
	}
	if framework == "cypress" && cypressCommands.MatchString(content) {
		found = append(found, "cypress-custom-commands")
	}
	if framework == "cypress" && cypressPlugins.MatchString(content) {
		found = append(found, "cypress-plugin-events")
	}
	if framework == "mocha" && testContextPattern.MatchString(content) {
		found = append(found, "framework-test-context")
	}
	return found
}

// stripJSNoise removes JS comments and string/template literals from content
// so structural regex detectors avoid matching inside non-code text.
func stripJSNoise(content string) string {
	var out strings.Builder
	inLineComment := false
	inBlockComment := false
	inSingle := false
	inDouble := false
	inTemplate := false
	escaped := false

	for i := 0; i < len(content); i++ {
		ch := content[i]
		next := byte(0)
		if i+1 < len(content) {
			next = content[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out.WriteByte('\n')
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
				continue
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		if inSingle {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		if inTemplate {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '`' {
				inTemplate = false
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}

		if ch == '/' && next == '/' {
			inLineComment = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}
		if ch == '\'' {
			inSingle = true
			escaped = false
			continue
		}
		if ch == '"' {
			inDouble = true
			escaped = false
			continue
		}
		if ch == '`' {
			inTemplate = true
			escaped = false
			continue
		}

		out.WriteByte(ch)
	}

	return out.String()
}

// FrameworkMigrationDetector detects multi-framework situations that
// suggest migration opportunity.
type FrameworkMigrationDetector struct{}

func (d *FrameworkMigrationDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if len(snap.Frameworks) < 2 {
		return nil
	}

	// Only flag if there are multiple unit-test frameworks (not e2e + unit mix)
	var unitFrameworks []models.Framework
	for _, fw := range snap.Frameworks {
		if fw.Type == models.FrameworkTypeUnit || fw.Type == models.FrameworkTypeUnknown {
			unitFrameworks = append(unitFrameworks, fw)
		}
	}

	if len(unitFrameworks) < 2 {
		return nil
	}

	names := make([]string, len(unitFrameworks))
	for i, fw := range unitFrameworks {
		names[i] = fw.Name
	}

	return []models.Signal{{
		Type:             "frameworkMigration",
		Category:         models.CategoryMigration,
		Severity:         models.SeverityInfo,
		Confidence:       0.8,
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			Repository: snap.Repository.Name,
		},
		Explanation: fmt.Sprintf(
			"Multiple unit-test frameworks detected (%s). Consolidation may simplify maintenance.",
			strings.Join(names, ", "),
		),
		SuggestedAction: "Evaluate consolidating to a single test framework.",
		Metadata: map[string]any{
			"frameworks":     names,
			"frameworkCount": len(unitFrameworks),
		},
	}}
}
