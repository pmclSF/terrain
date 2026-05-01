package signals

import (
	"sort"

	"github.com/pmclSF/terrain/internal/models"
)

// SignalStatus expresses the lifecycle stage of a signal type.
type SignalStatus string

const (
	// StatusStable: at least one production detector emits this signal,
	// it has documented severity/confidence semantics, and the schema is locked.
	StatusStable SignalStatus = "stable"

	// StatusExperimental: detector exists and may emit, but precision/recall
	// are not yet calibrated against a labeled corpus and the schema may
	// evolve before 1.0.
	StatusExperimental SignalStatus = "experimental"

	// StatusPlanned: signal type is declared but no detector emits it today.
	// Documented to reserve the name and shape so future detectors don't
	// invent overlapping types. References from policy or measurement code
	// short-circuit to zero counts.
	StatusPlanned SignalStatus = "planned"
)

// ManifestEntry is the canonical record for a signal type. Every signal
// declared in signal_types.go must have a matching entry here, and every
// entry here must reference a real signal-type constant. Drift between the
// two is caught by TestManifest_MatchesSignalTypes in 0.1.2 and becomes a
// release-gate failure once the doc-generation pipeline lands in 0.2.
//
// The manifest replaces three older mechanisms over time:
//   - Registry (registry.go): superset; will be regenerated from this manifest
//   - typeInfoBySignal (signal_types.go): description/remediation pairs
//   - docs/signal-catalog.md: hand-edited list with persistent drift
type ManifestEntry struct {
	// Type is the canonical signal type string emitted in snapshots and JSON.
	Type models.SignalType

	// ConstName is the Go constant name (e.g. "SignalWeakAssertion"). Used by
	// the drift linter to validate one-to-one mapping with signal_types.go.
	ConstName string

	// Domain is the high-level category the signal belongs to. Maps to the
	// long-standing models.SignalCategory enum.
	Domain models.SignalCategory

	// Status: stable / experimental / planned.
	Status SignalStatus

	// Title is a short human-readable name (Title Case).
	Title string

	// Description is the one-line user-facing explanation. Pulled from
	// signal_types.go's typeInfoBySignal where present.
	Description string

	// Remediation is the suggested action to take.
	Remediation string

	// DefaultSeverity is the severity the producing detector emits in the
	// typical case. Detectors retain authority to escalate or de-escalate
	// per finding; this field documents the expected baseline.
	DefaultSeverity models.SignalSeverity

	// ConfidenceMin / ConfidenceMax bracket the typical confidence range
	// the detector emits. 0.1.2 values are descriptive (sourced from
	// detector code review), not calibrated. Calibration arrives in 0.3
	// alongside the corpus work.
	ConfidenceMin float64
	ConfidenceMax float64

	// EvidenceSources lists the data inputs the detector consults.
	// Values: structural-pattern, path-name, runtime, coverage,
	// policy, codeowners, graph-traversal.
	EvidenceSources []string

	// RuleID is a stable identifier for documentation cross-references and
	// SARIF emission. Format: TER-<DOMAIN>-<3-digit-number>.
	RuleID string

	// RuleURI points to the canonical rule documentation page. The path is
	// resolved relative to docs.terrain.dev once that domain is live; today
	// it resolves to the in-repo docs/ rules/ tree.
	RuleURI string

	// PromotionPlan describes what is required to advance the entry's
	// status. Populated for experimental and planned entries; empty for
	// stable.
	PromotionPlan string
}

// allSignalManifest is the canonical inventory. Order is significant for
// generated docs; do not sort. New entries go at the end of their domain
// section to keep RuleIDs stable.
//
// Convention for adding entries:
//  1. Add the constant to signal_types.go.
//  2. Add the manifest entry below in domain order.
//  3. If a detector emits it, set Status = StatusStable.
//  4. Otherwise mark StatusPlanned with a PromotionPlan that names the
//     milestone in docs/release/ that will ship the detector.
//  5. Run `go test ./internal/signals/... -run TestManifest`.
var allSignalManifest = []ManifestEntry{
	// ── Health ─────────────────────────────────────────────────
	{
		Type: SignalSlowTest, ConstName: "SignalSlowTest",
		Domain: models.CategoryHealth, Status: StatusStable,
		Title:           "Slow Test",
		Description:     "Tests exceed expected runtime budget and slow feedback loops.",
		Remediation:     "Profile slow paths and split or optimize expensive tests.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-HEALTH-001",
		RuleURI:         "docs/rules/health/slow-test.md",
	},
	{
		Type: SignalFlakyTest, ConstName: "SignalFlakyTest",
		Domain: models.CategoryHealth, Status: StatusStable,
		Title:           "Flaky Test",
		Description:     "Tests exhibit inconsistent pass/fail behavior across runs.",
		Remediation:     "Stabilize timing, shared state, and external dependency handling.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.85,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-HEALTH-002",
		RuleURI:         "docs/rules/health/flaky-test.md",
		PromotionPlan: "Today's detector is retry-based, not statistical failure-rate. " +
			"Statistical detection lands in 0.3 with the calibration corpus.",
	},
	{
		Type: SignalSkippedTest, ConstName: "SignalSkippedTest",
		Domain: models.CategoryHealth, Status: StatusStable,
		Title:           "Skipped Test",
		Description:     "Tests are skipped and may hide latent regressions.",
		Remediation:     "Unskip, remove, or explicitly justify skipped tests in policy.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime", "structural-pattern"},
		RuleID:          "TER-HEALTH-003",
		RuleURI:         "docs/rules/health/skipped-test.md",
	},
	{
		Type: SignalDeadTest, ConstName: "SignalDeadTest",
		Domain: models.CategoryHealth, Status: StatusStable,
		Title:           "Dead Test",
		Description:     "Tests may no longer validate meaningful behavior.",
		Remediation:     "Remove obsolete tests or reconnect them to active behavior.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.6, ConfidenceMax: 0.8,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-HEALTH-004",
		RuleURI:         "docs/rules/health/dead-test.md",
	},
	{
		Type: SignalUnstableSuite, ConstName: "SignalUnstableSuite",
		Domain: models.CategoryHealth, Status: StatusStable,
		Title:           "Unstable Suite",
		Description:     "The suite has concentrated instability signals.",
		Remediation:     "Prioritize stabilization in the highest-instability areas.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.85,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-HEALTH-005",
		RuleURI:         "docs/rules/health/unstable-suite.md",
	},

	// ── Quality ────────────────────────────────────────────────
	{
		Type: SignalUntestedExport, ConstName: "SignalUntestedExport",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Untested Export",
		Description:     "Exported code units are not directly covered by tests.",
		Remediation:     "Add direct tests for public exports to protect API behavior.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.5, ConfidenceMax: 0.7,
		EvidenceSources: []string{"path-name", "graph-traversal"},
		RuleID:          "TER-QUAL-001",
		RuleURI:         "docs/rules/quality/untested-export.md",
	},
	{
		Type: SignalWeakAssertion, ConstName: "SignalWeakAssertion",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Weak Assertion",
		Description:     "Tests use weak or low-density assertions, reducing defect-catching power.",
		Remediation:     "Add behavior-focused assertions on outputs, state transitions, and side effects.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.4, ConfidenceMax: 0.8,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-002",
		RuleURI:         "docs/rules/quality/weak-assertion.md",
		PromotionPlan: "Detector is regex/density-based; AST-based semantic scoring lands in 0.3 " +
			"alongside the calibration corpus.",
	},
	{
		Type: SignalMockHeavyTest, ConstName: "SignalMockHeavyTest",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Mock-Heavy Test",
		Description:     "Tests rely heavily on mocks and may miss integration-level regressions.",
		Remediation:     "Replace brittle mocks with real collaborators where practical.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.6, ConfidenceMax: 0.8,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-003",
		RuleURI:         "docs/rules/quality/mock-heavy.md",
	},
	{
		Type: SignalTestsOnlyMocks, ConstName: "SignalTestsOnlyMocks",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Tests Only Mocks",
		Description:     "Test files contain mock setup but zero assertions, verifying wiring only.",
		Remediation:     "Add assertions on outputs, state changes, or side effects to validate real behavior.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-004",
		RuleURI:         "docs/rules/quality/tests-only-mocks.md",
	},
	{
		Type: SignalSnapshotHeavyTest, ConstName: "SignalSnapshotHeavyTest",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Snapshot-Heavy Test",
		Description:     "Test files over-rely on snapshot assertions, reducing defect specificity.",
		Remediation:     "Supplement snapshots with targeted assertions on critical behavior.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.5, ConfidenceMax: 0.75,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-005",
		RuleURI:         "docs/rules/quality/snapshot-heavy.md",
	},
	{
		Type: SignalCoverageBlindSpot, ConstName: "SignalCoverageBlindSpot",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Coverage Blind Spot",
		Description:     "Code units appear unprotected or weakly protected by current coverage mix.",
		Remediation:     "Add unit/integration tests where only broad or indirect coverage exists.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.5, ConfidenceMax: 0.8,
		EvidenceSources: []string{"coverage", "graph-traversal"},
		RuleID:          "TER-QUAL-006",
		RuleURI:         "docs/rules/quality/coverage-blind-spot.md",
	},
	{
		Type: SignalCoverageThresholdBreak, ConstName: "SignalCoverageThresholdBreak",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Coverage Threshold Break",
		Description:     "Measured coverage falls below configured thresholds.",
		Remediation:     "Target low-coverage, high-risk areas and raise meaningful coverage first.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.9, ConfidenceMax: 0.99,
		EvidenceSources: []string{"coverage"},
		RuleID:          "TER-QUAL-007",
		RuleURI:         "docs/rules/quality/coverage-threshold.md",
		PromotionPlan: "Severity flips at hard 100%-gap boundary; smooth gradient lands in 0.3 " +
			"per docs/scoring-rubric.md.",
	},
	{
		Type: SignalStaticSkippedTest, ConstName: "SignalStaticSkippedTest",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Static Skipped Test",
		Description:     "Tests are statically marked as skipped (it.skip, xit, @skip, etc.).",
		Remediation:     "Re-enable, replace, or document skip markers older than the policy threshold.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-008",
		RuleURI:         "docs/rules/quality/static-skip.md",
	},
	{
		Type: SignalAssertionFreeTest, ConstName: "SignalAssertionFreeTest",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Assertion-Free Test",
		Description:     "Test files contain test function signatures but no detectable assertions.",
		Remediation:     "Add assertions to validate behavior — tests without assertions verify nothing.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.75, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-QUAL-009",
		RuleURI:         "docs/rules/quality/assertion-free.md",
	},
	{
		Type: SignalOrphanedTestFile, ConstName: "SignalOrphanedTestFile",
		Domain: models.CategoryQuality, Status: StatusStable,
		Title:           "Orphaned Test File",
		Description:     "Test files do not import any source modules from the repository.",
		Remediation:     "Connect orphaned tests to source code or remove if obsolete.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.4, ConfidenceMax: 0.7,
		EvidenceSources: []string{"graph-traversal"},
		RuleID:          "TER-QUAL-010",
		RuleURI:         "docs/rules/quality/orphaned-test.md",
	},

	// ── Migration ──────────────────────────────────────────────
	{
		Type: SignalFrameworkMigration, ConstName: "SignalFrameworkMigration",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Framework Migration Opportunity",
		Description:     "The repository or package appears suitable for migration to a target framework.",
		Remediation:     "Evaluate candidates with `terrain migration readiness` and plan staged migration.",
		DefaultSeverity: models.SeverityInfo,
		ConfidenceMin:   0.5, ConfidenceMax: 0.8,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-001",
		RuleURI:         "docs/rules/migration/framework-migration.md",
	},
	{
		Type: SignalMigrationBlocker, ConstName: "SignalMigrationBlocker",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Migration Blocker",
		Description:     "Detected patterns will complicate framework migration.",
		Remediation:     "Address blockers incrementally before broad migration changes.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-002",
		RuleURI:         "docs/rules/migration/migration-blocker.md",
	},
	{
		Type: SignalDeprecatedTestPattern, ConstName: "SignalDeprecatedTestPattern",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Deprecated Test Pattern",
		Description:     "Deprecated test patterns increase migration and maintenance risk.",
		Remediation:     "Replace deprecated APIs with supported alternatives.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-003",
		RuleURI:         "docs/rules/migration/deprecated-pattern.md",
	},
	{
		Type: SignalDynamicTestGeneration, ConstName: "SignalDynamicTestGeneration",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Dynamic Test Generation",
		Description:     "Dynamic test generation may reduce migration and analysis confidence.",
		Remediation:     "Prefer explicit, static test declarations for critical paths.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.5, ConfidenceMax: 0.75,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-004",
		RuleURI:         "docs/rules/migration/dynamic-generation.md",
	},
	{
		Type: SignalCustomMatcherRisk, ConstName: "SignalCustomMatcherRisk",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Custom Matcher Risk",
		Description:     "Custom matcher behavior can be difficult to migrate safely.",
		Remediation:     "Audit matcher semantics and provide migration-safe equivalents.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.4, ConfidenceMax: 0.7,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-005",
		RuleURI:         "docs/rules/migration/custom-matcher.md",
	},
	{
		Type: SignalUnsupportedSetup, ConstName: "SignalUnsupportedSetup",
		Domain: models.CategoryMigration, Status: StatusStable,
		Title:           "Unsupported Setup",
		Description:     "Setup/teardown patterns may not port cleanly to target frameworks.",
		Remediation:     "Refactor setup boundaries toward framework-agnostic patterns.",
		DefaultSeverity: models.SeverityLow,
		ConfidenceMin:   0.4, ConfidenceMax: 0.7,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-MIG-006",
		RuleURI:         "docs/rules/migration/unsupported-setup.md",
	},

	// ── Governance ─────────────────────────────────────────────
	{
		Type: SignalPolicyViolation, ConstName: "SignalPolicyViolation",
		Domain: models.CategoryGovernance, Status: StatusStable,
		Title:           "Policy Violation",
		Description:     "Repository state violates configured Terrain policy rules.",
		Remediation:     "Resolve violations or intentionally update policy thresholds.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   1.0, ConfidenceMax: 1.0,
		EvidenceSources: []string{"policy"},
		RuleID:          "TER-GOV-001",
		RuleURI:         "docs/rules/governance/policy-violation.md",
	},
	{
		Type: SignalLegacyFrameworkUsage, ConstName: "SignalLegacyFrameworkUsage",
		Domain: models.CategoryGovernance, Status: StatusStable,
		Title:           "Legacy Framework Usage",
		Description:     "Legacy framework usage remains where policy discourages it.",
		Remediation:     "Plan and execute incremental migration away from legacy frameworks.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   1.0, ConfidenceMax: 1.0,
		EvidenceSources: []string{"policy", "structural-pattern"},
		RuleID:          "TER-GOV-002",
		RuleURI:         "docs/rules/governance/legacy-framework.md",
	},
	{
		Type: SignalSkippedTestsInCI, ConstName: "SignalSkippedTestsInCI",
		Domain: models.CategoryGovernance, Status: StatusStable,
		Title:           "Skipped Tests In CI",
		Description:     "Skipped tests are present where CI policy disallows them.",
		Remediation:     "Investigate skip conditions and re-enable tests or replace with targeted alternatives.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   1.0, ConfidenceMax: 1.0,
		EvidenceSources: []string{"policy", "structural-pattern"},
		RuleID:          "TER-GOV-003",
		RuleURI:         "docs/rules/governance/skipped-in-ci.md",
	},
	{
		Type: SignalRuntimeBudgetExceeded, ConstName: "SignalRuntimeBudgetExceeded",
		Domain: models.CategoryGovernance, Status: StatusStable,
		Title:           "Runtime Budget Exceeded",
		Description:     "Observed runtimes exceed configured policy budget.",
		Remediation:     "Reduce runtime hotspots or adjust policy to reflect intentional tradeoffs.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   1.0, ConfidenceMax: 1.0,
		EvidenceSources: []string{"policy", "runtime"},
		RuleID:          "TER-GOV-004",
		RuleURI:         "docs/rules/governance/runtime-budget.md",
	},

	// ── Structural (graph-powered) ─────────────────────────────
	{
		Type: SignalUncoveredAISurface, ConstName: "SignalUncoveredAISurface",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Uncovered AI Surface",
		Description:     "AI surfaces (prompts, tools, datasets) have zero test or scenario coverage.",
		Remediation:     "Add eval scenarios that exercise this AI surface — untested prompts and tools can change behavior silently.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"graph-traversal", "structural-pattern"},
		RuleID:          "TER-STRUCT-001",
		RuleURI:         "docs/rules/structural/uncovered-ai-surface.md",
		PromotionPlan: "Coverage attribution depends on .terrain/terrain.yaml scenario " +
			"declarations; precision/recall calibrated in 0.2 against the AI fixture corpus.",
	},
	{
		Type: SignalPhantomEvalScenario, ConstName: "SignalPhantomEvalScenario",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Phantom Eval Scenario",
		Description:     "Eval scenarios claim to validate AI surfaces but have no import-graph path to those surfaces.",
		Remediation:     "Verify the test file actually imports and exercises the target code, or correct the surface mapping.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"graph-traversal"},
		RuleID:          "TER-STRUCT-002",
		RuleURI:         "docs/rules/structural/phantom-eval.md",
		PromotionPlan: "Promote once .terrain/terrain.yaml scenario declarations are validated " +
			"against the AI fixture corpus in 0.2. Today's traversal can miss surfaces declared " +
			"by ID without a corresponding code path; calibration in 0.3 closes the gap.",
	},
	{
		Type: SignalUntestedPromptFlow, ConstName: "SignalUntestedPromptFlow",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Untested Prompt Flow",
		Description:     "A prompt flows through multiple source files via imports with zero test coverage at any point in the chain.",
		Remediation:     "Add integration tests at the prompt's consumption points to catch behavioral regressions.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"graph-traversal"},
		RuleID:          "TER-STRUCT-003",
		RuleURI:         "docs/rules/structural/untested-prompt-flow.md",
		PromotionPlan: "Detection currently misses prompt flows that go through framework " +
			"abstractions (LangChain runnables, LlamaIndex query engines). 0.2 ships AST-based " +
			"prompt-flow tracing; promote once recall measures >=0.8 on the AI fixture corpus.",
	},
	{
		Type: SignalBlastRadiusHotspot, ConstName: "SignalBlastRadiusHotspot",
		Domain: models.CategoryStructure, Status: StatusStable,
		Title:           "Blast-Radius Hotspot",
		Description:     "Source files where a change would impact an unusually large number of tests.",
		Remediation:     "Ensure high direct test coverage and consider adding contract tests at interface boundaries.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"graph-traversal"},
		RuleID:          "TER-STRUCT-004",
		RuleURI:         "docs/rules/structural/blast-radius.md",
	},
	{
		Type: SignalFixtureFragilityHotspot, ConstName: "SignalFixtureFragilityHotspot",
		Domain: models.CategoryStructure, Status: StatusStable,
		Title:           "Fixture Fragility Hotspot",
		Description:     "Fixtures depended on by many tests, where a single change cascades widely.",
		Remediation:     "Extract smaller, focused fixtures to reduce cascading test failures.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"graph-traversal"},
		RuleID:          "TER-STRUCT-005",
		RuleURI:         "docs/rules/structural/fixture-fragility.md",
	},
	{
		Type: SignalAssertionFreeImport, ConstName: "SignalAssertionFreeImport",
		Domain: models.CategoryStructure, Status: StatusStable,
		Title:           "Assertion-Free Import",
		Description:     "Test files import production code but contain zero assertions — exercising code without verifying behavior.",
		Remediation:     "Add assertions to validate behavior or remove tests that verify nothing.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.8, ConfidenceMax: 0.95,
		EvidenceSources: []string{"graph-traversal", "structural-pattern"},
		RuleID:          "TER-STRUCT-006",
		RuleURI:         "docs/rules/structural/assertion-free-import.md",
	},
	{
		Type: SignalCapabilityValidationGap, ConstName: "SignalCapabilityValidationGap",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Capability Validation Gap",
		Description:     "Inferred AI capabilities have no eval scenarios validating them.",
		Remediation:     "Add eval scenarios that exercise this capability to ensure behavioral regression detection.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.5, ConfidenceMax: 0.8,
		EvidenceSources: []string{"graph-traversal", "structural-pattern"},
		RuleID:          "TER-STRUCT-007",
		RuleURI:         "docs/rules/structural/capability-gap.md",
		PromotionPlan: "Capability inference is heuristic in 0.1.2; 0.2 introduces the AI " +
			"taxonomy v2 with explicit capability tags so this signal can fire only on declared " +
			"capabilities, eliminating false positives. Promote once precision >=0.8.",
	},

	// ── AI / Eval (planned in 0.1.2; ship in 0.2) ──────────────
	// All entries below are referenced by policy and measurement code so
	// that future detector wiring requires no plumbing change. Until then,
	// counts are zero and StatusPlanned is documented in feature-status.md.
	{
		Type: SignalEvalFailure, ConstName: "SignalEvalFailure",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title:           "Eval Failure",
		Description:     "An AI eval scenario reported a hard failure.",
		Remediation:     "Investigate the failing case in the eval framework's report and patch the prompt or guardrail.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.9, ConfidenceMax: 1.0,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-001",
		RuleURI:         "docs/rules/ai/eval-failure.md",
		PromotionPlan:   "Detector lands in 0.2 with eval-framework metric ingestion.",
	},
	{
		Type: SignalEvalRegression, ConstName: "SignalEvalRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title:           "Eval Regression",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-002", RuleURI: "docs/rules/ai/eval-regression.md",
		PromotionPlan: "0.2: ingest baseline-vs-current metrics from Promptfoo / DeepEval / Ragas.",
	},
	{
		Type: SignalAccuracyRegression, ConstName: "SignalAccuracyRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Accuracy Regression", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-003", RuleURI: "docs/rules/ai/accuracy-regression.md",
		PromotionPlan: "0.2",
	},
	{
		Type: SignalCitationMissing, ConstName: "SignalCitationMissing",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Citation Missing", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-004", RuleURI: "docs/rules/ai/citation-missing.md",
		PromotionPlan: "0.3 — RAG-specific detectors.",
	},
	{
		Type: SignalRetrievalMiss, ConstName: "SignalRetrievalMiss",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Retrieval Miss", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-005", RuleURI: "docs/rules/ai/retrieval-miss.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalAnswerGroundingFailure, ConstName: "SignalAnswerGroundingFailure",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Answer Grounding Failure", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-006", RuleURI: "docs/rules/ai/grounding-failure.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalToolSelectionError, ConstName: "SignalToolSelectionError",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Tool Selection Error", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-007", RuleURI: "docs/rules/ai/tool-selection-error.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalSchemaParseFailure, ConstName: "SignalSchemaParseFailure",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Schema Parse Failure", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-008", RuleURI: "docs/rules/ai/schema-parse-failure.md",
		PromotionPlan: "0.2",
	},
	{
		Type: SignalSafetyFailure, ConstName: "SignalSafetyFailure",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Safety Failure", DefaultSeverity: models.SeverityCritical,
		ConfidenceMin: 0.9, ConfidenceMax: 1.0,
		EvidenceSources: []string{"runtime", "policy"},
		RuleID:          "TER-AI-009", RuleURI: "docs/rules/ai/safety-failure.md",
		PromotionPlan: "0.2 — first-class safety eval signals.",
	},
	{
		Type: SignalAIPolicyViolation, ConstName: "SignalAIPolicyViolation",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "AI Policy Violation", DefaultSeverity: models.SeverityCritical,
		ConfidenceMin: 1.0, ConfidenceMax: 1.0,
		EvidenceSources: []string{"policy"},
		RuleID:          "TER-AI-010", RuleURI: "docs/rules/ai/ai-policy-violation.md",
		PromotionPlan: "0.2",
	},
	{
		Type: SignalHallucinationDetected, ConstName: "SignalHallucinationDetected",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Hallucination Detected", DefaultSeverity: models.SeverityCritical,
		ConfidenceMin: 0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-011", RuleURI: "docs/rules/ai/hallucination.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalLatencyRegression, ConstName: "SignalLatencyRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Latency Regression", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-012", RuleURI: "docs/rules/ai/latency-regression.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalCostRegression, ConstName: "SignalCostRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Cost Regression", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-013", RuleURI: "docs/rules/ai/cost-regression.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalContextOverflowRisk, ConstName: "SignalContextOverflowRisk",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Context Overflow Risk", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"structural-pattern", "runtime"},
		RuleID:          "TER-AI-014", RuleURI: "docs/rules/ai/context-overflow.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalWrongSourceSelected, ConstName: "SignalWrongSourceSelected",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Wrong Source Selected", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-015", RuleURI: "docs/rules/ai/wrong-source.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalCitationMismatch, ConstName: "SignalCitationMismatch",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Citation Mismatch", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-016", RuleURI: "docs/rules/ai/citation-mismatch.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalStaleSourceRisk, ConstName: "SignalStaleSourceRisk",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Stale Source Risk", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.5, ConfidenceMax: 0.8,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-017", RuleURI: "docs/rules/ai/stale-source.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalChunkingRegression, ConstName: "SignalChunkingRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Chunking Regression", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-018", RuleURI: "docs/rules/ai/chunking-regression.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalRerankerRegression, ConstName: "SignalRerankerRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Reranker Regression", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-019", RuleURI: "docs/rules/ai/reranker-regression.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalTopKRegression, ConstName: "SignalTopKRegression",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Top-K Regression", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-020", RuleURI: "docs/rules/ai/topk-regression.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalToolRoutingError, ConstName: "SignalToolRoutingError",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Tool Routing Error", DefaultSeverity: models.SeverityHigh,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-021", RuleURI: "docs/rules/ai/tool-routing-error.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalToolGuardrailViolation, ConstName: "SignalToolGuardrailViolation",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Tool Guardrail Violation", DefaultSeverity: models.SeverityCritical,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime", "policy"},
		RuleID:          "TER-AI-022", RuleURI: "docs/rules/ai/tool-guardrail.md",
		PromotionPlan: "0.2 — tools-without-sandbox detection.",
	},
	{
		Type: SignalToolBudgetExceeded, ConstName: "SignalToolBudgetExceeded",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Tool Budget Exceeded", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime", "policy"},
		RuleID:          "TER-AI-023", RuleURI: "docs/rules/ai/tool-budget.md",
		PromotionPlan: "0.3",
	},
	{
		Type: SignalAgentFallbackTriggered, ConstName: "SignalAgentFallbackTriggered",
		Domain: models.CategoryAI, Status: StatusPlanned,
		Title: "Agent Fallback Triggered", DefaultSeverity: models.SeverityMedium,
		ConfidenceMin: 0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-024", RuleURI: "docs/rules/ai/agent-fallback.md",
		PromotionPlan: "0.3",
	},

	// ── 0.2 AI signals (planned in 0.2, detectors land before 0.2 close) ──
	{
		Type: SignalAISafetyEvalMissing, ConstName: "SignalAISafetyEvalMissing",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "AI Safety Eval Missing",
		Description:     "Agent or prompt has no eval scenario covering the documented safety category (jailbreak, harm, leak).",
		Remediation:     "Add an eval scenario tagged with the missing safety category and re-run the gauntlet.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.75, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern", "graph-traversal"},
		RuleID:          "TER-AI-100", RuleURI: "docs/rules/ai/safety-eval-missing.md",
	},
	{
		Type: SignalAIPromptVersioning, ConstName: "SignalAIPromptVersioning",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Prompt Versioning",
		Description:     "Prompt-kind surface ships without a recognisable version marker (filename suffix, inline `version:` field, or `# version:` comment). Future content changes will silently drift; consumers can't detect the change.",
		Remediation:     "Add a `version:` field, a `_v<N>` filename suffix, or a `# version: ...` comment so downstream consumers can detect content drift.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.75, ConfidenceMax: 0.92,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-101", RuleURI: "docs/rules/ai/prompt-versioning.md",
	},
	{
		Type: SignalAIPromptInjectionRisk, ConstName: "SignalAIPromptInjectionRisk",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Prompt-Injection-Shaped Concatenation",
		Description:     "User-controlled input is concatenated into a prompt without escaping, system-prompt boundaries, or structured input boundaries.",
		Remediation:     "Use a prompt template with explicit user-content boundaries, or run user input through a sanitiser.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.6, ConfidenceMax: 0.85,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-102", RuleURI: "docs/rules/ai/prompt-injection-risk.md",
		PromotionPlan:   "0.2 ships heuristic regex detection. Promotes to stable in 0.3 when AST-precise taint-flow analysis lands.",
	},
	{
		Type: SignalAIHardcodedAPIKey, ConstName: "SignalAIHardcodedAPIKey",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Hard-Coded API Key in AI Configuration",
		Description:     "API-key-shaped string appears in an eval YAML, prompt config, or agent definition.",
		Remediation:     "Move the secret to an environment variable or secrets store and reference it through the runner's secret-resolution path.",
		DefaultSeverity: models.SeverityCritical,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-103", RuleURI: "docs/rules/ai/hardcoded-api-key.md",
	},
	{
		Type: SignalAIToolWithoutSandbox, ConstName: "SignalAIToolWithoutSandbox",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Destructive Tool Without Sandbox",
		Description:     "An agent tool definition can perform an irreversible operation (delete, drop, exec) without an explicit approval gate, sandbox, or dry-run mode.",
		Remediation:     "Wrap the tool in an approval gate or restrict its capability surface to a sandbox.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.7, ConfidenceMax: 0.9,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-104", RuleURI: "docs/rules/ai/tool-without-sandbox.md",
	},
	{
		Type: SignalAINonDeterministicEval, ConstName: "SignalAINonDeterministicEval",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Non-Deterministic Eval Configuration",
		Description:     "An LLM eval runs without temperature pinned to 0 or a deterministic seed, so re-runs produce noisy comparisons.",
		Remediation:     "Pin temperature: 0 and a seed in the eval config, or document the non-determinism budget.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.9, ConfidenceMax: 0.98,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-105", RuleURI: "docs/rules/ai/non-deterministic-eval.md",
	},
	{
		Type: SignalAIModelDeprecationRisk, ConstName: "SignalAIModelDeprecationRisk",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Model Pinned to Deprecated or Floating Tag",
		Description:     "Code references a model name that resolves to a deprecated version or a floating tag (e.g. `gpt-4`, `gpt-3.5-turbo`).",
		Remediation:     "Pin to a dated model variant or upgrade to a supported tier.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.8, ConfidenceMax: 0.95,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-106", RuleURI: "docs/rules/ai/model-deprecation-risk.md",
	},
	{
		Type: SignalAICostRegression, ConstName: "SignalAICostRegression",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Prompt Token-Cost Regression",
		Description:     "A prompt change increases the token count by more than 25% versus the recorded baseline.",
		Remediation:     "Investigate the change for unintended bloat; bump the baseline if the increase is intentional.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-107", RuleURI: "docs/rules/ai/cost-regression.md",
	},
	{
		Type: SignalAIHallucinationRate, ConstName: "SignalAIHallucinationRate",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Hallucination Rate Above Threshold",
		Description:     "An eval reports fabricated outputs at a rate above the project-configured threshold (default 5%).",
		Remediation:     "Investigate failing scenarios; tighten retrieval or grounding before merging.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.8, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-108", RuleURI: "docs/rules/ai/hallucination-rate.md",
	},
	{
		Type: SignalAIFewShotContamination, ConstName: "SignalAIFewShotContamination",
		Domain: models.CategoryAI, Status: StatusExperimental,
		Title:           "Few-Shot Contamination",
		Description:     "Few-shot examples in a prompt overlap verbatim with the inputs of eval scenarios that exercise that prompt, inflating reported scores.",
		Remediation:     "Hold out the contaminated examples from the prompt's few-shot block, or rewrite the eval input so it isn't a copy of an example. Re-run the eval after de-duplication.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.55, ConfidenceMax: 0.83,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-109", RuleURI: "docs/rules/ai/few-shot-contamination.md",
		PromotionPlan:   "Substring-overlap detector ships in 0.2; promotes to stable in 0.3 once the calibration corpus tunes the threshold and adds token-level n-gram + semantic-similarity passes.",
	},
	{
		Type: SignalAIEmbeddingModelChange, ConstName: "SignalAIEmbeddingModelChange",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Embedding Model Swap Without Re-Evaluation",
		Description:     "A repository references an embedding model in source code without a retrieval-shaped eval scenario, so a future model swap will silently change retrieval quality.",
		Remediation:     "Add a retrieval eval scenario (Ragas, Promptfoo, or DeepEval) that exercises this surface so embedding swaps surface as a measurable regression.",
		DefaultSeverity: models.SeverityMedium,
		ConfidenceMin:   0.7, ConfidenceMax: 0.88,
		EvidenceSources: []string{"structural-pattern"},
		RuleID:          "TER-AI-110", RuleURI: "docs/rules/ai/embedding-model-change.md",
		PromotionPlan:   "0.2 ships the static precondition (embedding referenced + no retrieval coverage). Cross-snapshot content-hash diff variant lands in 0.3 once snapshot fingerprints are recorded.",
	},
	{
		Type: SignalAIRetrievalRegression, ConstName: "SignalAIRetrievalRegression",
		Domain: models.CategoryAI, Status: StatusStable,
		Title:           "Retrieval Quality Regression",
		Description:     "Context relevance, nDCG, or coverage dropped versus the recorded baseline.",
		Remediation:     "Investigate the regression; revert the offending change or re-tune retrieval before merging.",
		DefaultSeverity: models.SeverityHigh,
		ConfidenceMin:   0.85, ConfidenceMax: 0.95,
		EvidenceSources: []string{"runtime"},
		RuleID:          "TER-AI-111", RuleURI: "docs/rules/ai/retrieval-regression.md",
	},
}

// Manifest returns a snapshot copy of the canonical signal manifest, sorted
// alphabetically by signal type. Callers should treat the result as read-only.
func Manifest() []ManifestEntry {
	out := make([]ManifestEntry, len(allSignalManifest))
	copy(out, allSignalManifest)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Type < out[j].Type
	})
	return out
}

// ManifestByType returns the manifest entry for a given signal type, or
// (zero, false) if no entry exists.
func ManifestByType(t models.SignalType) (ManifestEntry, bool) {
	for _, e := range allSignalManifest {
		if e.Type == t {
			return e, true
		}
	}
	return ManifestEntry{}, false
}

// AllSignalTypes returns every signal type currently declared in the manifest.
func AllSignalTypes() []models.SignalType {
	out := make([]models.SignalType, len(allSignalManifest))
	for i, e := range allSignalManifest {
		out[i] = e.Type
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SignalTypesByStatus returns every signal type with the given status.
func SignalTypesByStatus(status SignalStatus) []models.SignalType {
	var out []models.SignalType
	for _, e := range allSignalManifest {
		if e.Status == status {
			out = append(out, e.Type)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
