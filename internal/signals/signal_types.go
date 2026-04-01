package signals

import "github.com/pmclSF/terrain/internal/models"

// Convenience aliases so callers can use signals.SignalSlowTest etc.
// The underlying type is models.SignalType.
const (
	SignalSlowTest               models.SignalType = "slowTest"
	SignalFlakyTest              models.SignalType = "flakyTest"
	SignalSkippedTest            models.SignalType = "skippedTest"
	SignalDeadTest               models.SignalType = "deadTest"
	SignalUnstableSuite          models.SignalType = "unstableSuite"
	SignalUntestedExport         models.SignalType = "untestedExport"
	SignalWeakAssertion          models.SignalType = "weakAssertion"
	SignalMockHeavyTest          models.SignalType = "mockHeavyTest"
	SignalTestsOnlyMocks         models.SignalType = "testsOnlyMocks"
	SignalSnapshotHeavyTest      models.SignalType = "snapshotHeavyTest"
	SignalCoverageBlindSpot      models.SignalType = "coverageBlindSpot"
	SignalCoverageThresholdBreak models.SignalType = "coverageThresholdBreak"
	SignalFrameworkMigration     models.SignalType = "frameworkMigration"
	SignalMigrationBlocker       models.SignalType = "migrationBlocker"
	SignalDeprecatedTestPattern  models.SignalType = "deprecatedTestPattern"
	SignalDynamicTestGeneration  models.SignalType = "dynamicTestGeneration"
	SignalCustomMatcherRisk      models.SignalType = "customMatcherRisk"
	SignalUnsupportedSetup       models.SignalType = "unsupportedSetup"
	SignalPolicyViolation        models.SignalType = "policyViolation"
	SignalLegacyFrameworkUsage   models.SignalType = "legacyFrameworkUsage"
	SignalSkippedTestsInCI       models.SignalType = "skippedTestsInCI"
	SignalRuntimeBudgetExceeded  models.SignalType = "runtimeBudgetExceeded"
	SignalStaticSkippedTest     models.SignalType = "staticSkippedTest"
	SignalAssertionFreeTest    models.SignalType = "assertionFreeTest"
	SignalOrphanedTestFile     models.SignalType = "orphanedTestFile"

	// Graph-powered structural signal types.
	SignalUncoveredAISurface      models.SignalType = "uncoveredAISurface"
	SignalPhantomEvalScenario     models.SignalType = "phantomEvalScenario"
	SignalUntestedPromptFlow      models.SignalType = "untestedPromptFlow"
	SignalBlastRadiusHotspot      models.SignalType = "blastRadiusHotspot"
	SignalFixtureFragilityHotspot models.SignalType = "fixtureFragilityHotspot"
	SignalAssertionFreeImport     models.SignalType = "assertionFreeImport"
	SignalCapabilityValidationGap models.SignalType = "capabilityValidationGap"

	// AI/eval signal types.
	SignalEvalFailure            models.SignalType = "evalFailure"
	SignalEvalRegression         models.SignalType = "evalRegression"
	SignalAccuracyRegression     models.SignalType = "accuracyRegression"
	SignalCitationMissing        models.SignalType = "citationMissing"
	SignalRetrievalMiss          models.SignalType = "retrievalMiss"
	SignalAnswerGroundingFailure models.SignalType = "answerGroundingFailure"
	SignalToolSelectionError     models.SignalType = "toolSelectionError"
	SignalSchemaParseFailure     models.SignalType = "schemaParseFailure"
	SignalSafetyFailure          models.SignalType = "safetyFailure"
	SignalAIPolicyViolation      models.SignalType = "aiPolicyViolation"
	SignalHallucinationDetected  models.SignalType = "hallucinationDetected"
	SignalLatencyRegression      models.SignalType = "latencyRegression"
	SignalCostRegression         models.SignalType = "costRegression"
	SignalContextOverflowRisk    models.SignalType = "contextOverflowRisk"
	SignalWrongSourceSelected   models.SignalType = "wrongSourceSelected"
	SignalCitationMismatch      models.SignalType = "citationMismatch"
	SignalStaleSourceRisk       models.SignalType = "staleSourceRisk"
	SignalChunkingRegression    models.SignalType = "chunkingRegression"
	SignalRerankerRegression    models.SignalType = "rerankerRegression"
	SignalTopKRegression        models.SignalType = "topKRegression"
	SignalToolRoutingError      models.SignalType = "toolRoutingError"
	SignalToolGuardrailViolation models.SignalType = "toolGuardrailViolation"
	SignalToolBudgetExceeded    models.SignalType = "toolBudgetExceeded"
	SignalAgentFallbackTriggered models.SignalType = "agentFallbackTriggered"
)

// Canonical signal type sets. Import these rather than duplicating
// signal type maps across packages.

// MigrationSignalTypes is the canonical set of migration-related signal types.
var MigrationSignalTypes = map[models.SignalType]bool{
	SignalFrameworkMigration:    true,
	SignalMigrationBlocker:      true,
	SignalDeprecatedTestPattern: true,
	SignalDynamicTestGeneration: true,
	SignalCustomMatcherRisk:     true,
	SignalUnsupportedSetup:      true,
}

// QualitySignalTypes is the canonical set of quality-related signal types.
var QualitySignalTypes = map[models.SignalType]bool{
	SignalWeakAssertion:          true,
	SignalMockHeavyTest:          true,
	SignalTestsOnlyMocks:         true,
	SignalSnapshotHeavyTest:      true,
	SignalUntestedExport:         true,
	SignalCoverageThresholdBreak: true,
	SignalCoverageBlindSpot:      true,
}

// StructuralSignalTypes is the canonical set of graph-powered structural signal types.
var StructuralSignalTypes = map[models.SignalType]bool{
	SignalUncoveredAISurface:      true,
	SignalPhantomEvalScenario:     true,
	SignalUntestedPromptFlow:      true,
	SignalBlastRadiusHotspot:      true,
	SignalFixtureFragilityHotspot: true,
	SignalAssertionFreeImport:     true,
	SignalCapabilityValidationGap: true,
}

// IsMigrationSignal returns true if the signal type is migration-related.
func IsMigrationSignal(t models.SignalType) bool {
	return MigrationSignalTypes[t]
}

// IsQualitySignal returns true if the signal type is quality-related.
func IsQualitySignal(t models.SignalType) bool {
	return QualitySignalTypes[t]
}

// TypeInfo describes user-facing semantics for a signal type.
type TypeInfo struct {
	Description string
	Remediation string
}

var typeInfoBySignal = map[models.SignalType]TypeInfo{
	SignalWeakAssertion: {
		Description: "Tests use weak or low-density assertions, reducing defect-catching power.",
		Remediation: "Add behavior-focused assertions on outputs, state transitions, and side effects.",
	},
	SignalMockHeavyTest: {
		Description: "Tests rely heavily on mocks and may miss integration-level regressions.",
		Remediation: "Replace brittle mocks with real collaborators where practical.",
	},
	SignalTestsOnlyMocks: {
		Description: "Test files contain mock setup but zero assertions, verifying wiring only.",
		Remediation: "Add assertions on outputs, state changes, or side effects to validate real behavior.",
	},
	SignalSnapshotHeavyTest: {
		Description: "Test files over-rely on snapshot assertions, reducing defect specificity.",
		Remediation: "Supplement snapshots with targeted assertions on critical behavior.",
	},
	SignalSkippedTestsInCI: {
		Description: "Tests are conditionally skipped in CI, potentially hiding regressions.",
		Remediation: "Investigate skip conditions and re-enable tests or replace with targeted alternatives.",
	},
	SignalUntestedExport: {
		Description: "Exported code units are not directly covered by tests.",
		Remediation: "Add direct tests for public exports to protect API behavior.",
	},
	SignalCoverageThresholdBreak: {
		Description: "Measured coverage falls below configured thresholds.",
		Remediation: "Target low-coverage, high-risk areas and raise meaningful coverage first.",
	},
	SignalCoverageBlindSpot: {
		Description: "Code units appear unprotected or weakly protected by current coverage mix.",
		Remediation: "Add unit/integration tests where only broad or indirect coverage exists.",
	},
	SignalFlakyTest: {
		Description: "Tests exhibit inconsistent pass/fail behavior across runs.",
		Remediation: "Stabilize timing, shared state, and external dependency handling.",
	},
	SignalSlowTest: {
		Description: "Tests exceed expected runtime budget and slow feedback loops.",
		Remediation: "Profile slow paths and split or optimize expensive tests.",
	},
	SignalSkippedTest: {
		Description: "Tests are skipped and may hide latent regressions.",
		Remediation: "Unskip, remove, or explicitly justify skipped tests in policy.",
	},
	SignalDeadTest: {
		Description: "Tests may no longer validate meaningful behavior.",
		Remediation: "Remove obsolete tests or reconnect them to active behavior.",
	},
	SignalUnstableSuite: {
		Description: "The suite has concentrated instability signals.",
		Remediation: "Prioritize stabilization in the highest-instability areas.",
	},
	SignalMigrationBlocker: {
		Description: "Detected patterns will complicate framework migration.",
		Remediation: "Address blockers incrementally before broad migration changes.",
	},
	SignalDeprecatedTestPattern: {
		Description: "Deprecated test patterns increase migration and maintenance risk.",
		Remediation: "Replace deprecated APIs with supported alternatives.",
	},
	SignalDynamicTestGeneration: {
		Description: "Dynamic test generation may reduce migration and analysis confidence.",
		Remediation: "Prefer explicit, static test declarations for critical paths.",
	},
	SignalCustomMatcherRisk: {
		Description: "Custom matcher behavior can be difficult to migrate safely.",
		Remediation: "Audit matcher semantics and provide migration-safe equivalents.",
	},
	SignalUnsupportedSetup: {
		Description: "Setup/teardown patterns may not port cleanly to target frameworks.",
		Remediation: "Refactor setup boundaries toward framework-agnostic patterns.",
	},
	SignalPolicyViolation: {
		Description: "Repository state violates configured Terrain policy rules.",
		Remediation: "Resolve violations or intentionally update policy thresholds.",
	},
	SignalLegacyFrameworkUsage: {
		Description: "Legacy framework usage remains where policy discourages it.",
		Remediation: "Plan and execute incremental migration away from legacy frameworks.",
	},
	SignalRuntimeBudgetExceeded: {
		Description: "Observed runtimes exceed configured policy budget.",
		Remediation: "Reduce runtime hotspots or adjust policy to reflect intentional tradeoffs.",
	},
	SignalAssertionFreeTest: {
		Description: "Test files contain test function signatures but no detectable assertions.",
		Remediation: "Add assertions to validate behavior — tests without assertions verify nothing.",
	},
	SignalOrphanedTestFile: {
		Description: "Test files do not import any source modules from the repository.",
		Remediation: "Connect orphaned tests to source code or remove if obsolete.",
	},

	// Graph-powered structural signals.
	SignalUncoveredAISurface: {
		Description: "AI surfaces (prompts, tools, datasets) have zero test or scenario coverage.",
		Remediation: "Add eval scenarios that exercise this AI surface — untested prompts and tools can change behavior silently.",
	},
	SignalPhantomEvalScenario: {
		Description: "Eval scenarios claim to validate AI surfaces but have no import-graph path to those surfaces.",
		Remediation: "Verify the test file actually imports and exercises the target code, or correct the surface mapping.",
	},
	SignalUntestedPromptFlow: {
		Description: "A prompt flows through multiple source files via imports with zero test coverage at any point in the chain.",
		Remediation: "Add integration tests at the prompt's consumption points to catch behavioral regressions.",
	},
	SignalBlastRadiusHotspot: {
		Description: "Source files where a change would impact an unusually large number of tests.",
		Remediation: "Ensure high direct test coverage and consider adding contract tests at interface boundaries.",
	},
	SignalFixtureFragilityHotspot: {
		Description: "Fixtures depended on by many tests, where a single change cascades widely.",
		Remediation: "Extract smaller, focused fixtures to reduce cascading test failures.",
	},
	SignalAssertionFreeImport: {
		Description: "Test files import production code but contain zero assertions — exercising code without verifying behavior.",
		Remediation: "Add assertions to validate behavior or remove tests that verify nothing.",
	},
	SignalCapabilityValidationGap: {
		Description: "Inferred AI capabilities have no eval scenarios validating them.",
		Remediation: "Add eval scenarios that exercise this capability to ensure behavioral regression detection.",
	},
}

// Info returns user-facing metadata for a signal type.
func Info(t models.SignalType) (TypeInfo, bool) {
	info, ok := typeInfoBySignal[t]
	return info, ok
}
