# Signal Catalog

Terrain's signal system operates in four tiers, each requiring different data sources. All signals share the same structure: type, category, severity, confidence, evidence strength, location, explanation, and suggested remediation.

## Why tiers?

Signals are discovered progressively as more data becomes available:

- **Tier 1** requires only source code (always available on every `terrain analyze` run)
- **Tier 2** requires test execution data (JUnit XML, Jest JSON — provide via `--runtime`)
- **Tier 3** is automatic (dependency graph analysis runs alongside Tier 1)
- **Tier 4** requires AI evaluation artifacts (Gauntlet format — provide via `--gauntlet`)

Most teams get value from Tier 1 alone. Tiers 2–4 add depth without requiring additional setup.

## Tier 1: Core Static Signals (20 types)

Emitted by static analysis alone. No runtime artifacts required. Available on every `terrain analyze` run.

### Health (static)

#### staticSkippedTest
Category: quality

Tests contain `.skip`, `.only`, `xit`, `xdescribe`, or equivalent markers in source code.

Why it matters: Skip markers accumulate silently and hide coverage gaps.

Remediation: Restore, remove, or justify skipped tests in policy.

---

#### assertionFreeTest
Category: quality

Test files contain test function signatures but no detectable assertions.

Why it matters: Tests without assertions execute code but verify nothing.

Remediation: Add assertions to validate behavior.

---

#### orphanedTestFile
Category: quality

Test files do not import any source modules from the repository.

Why it matters: Orphaned tests may validate nothing or test deleted code.

Remediation: Connect to source code or remove if obsolete.

---

### Quality (7 types)

#### untestedExport
Category: quality

An exported function, method, or public code unit appears to have weak or missing direct test coverage.

Why it matters: Public APIs with weak coverage create high change risk.

Remediation: Add direct tests, improve code-to-test linkage, prioritize frequently changed or critical exports.

---

#### weakAssertion
Category: quality

A test file or suite has low or weak assertion strength relative to its scope.

Why it matters: Code may execute without meaningfully verifying behavior.

Remediation: Assert on outputs, state changes, side effects, or user-visible behavior.

---

#### mockHeavyTest
Category: quality

A test relies heavily on mocks relative to real interactions.

Why it matters: Mock-heavy tests can overstate confidence while missing real integration behavior.

Remediation: Reduce unnecessary mocking, add assertions on real system behavior.

---

#### testsOnlyMocks
Category: quality

Assertions primarily or exclusively validate mock interactions rather than business outcomes.

Why it matters: These tests verify implementation details rather than meaningful behavior.

Remediation: Assert on returned values, persisted state, domain events, or side effects.

---

#### snapshotHeavyTest
Category: quality

A file depends heavily on snapshots relative to direct semantic assertions.

Why it matters: Snapshot overuse can hide weak behavioral coverage and create brittle review noise.

Remediation: Replace some snapshots with targeted assertions, reduce low-value snapshot churn.

---

#### coverageBlindSpot
Category: quality

Coverage exists, but high-risk paths or code units remain weakly exercised.

Why it matters: Raw coverage percentages can hide meaningful quality gaps.

Remediation: Improve branch/path coverage, focus on high-complexity or critical modules.

---

#### coverageThresholdBreak
Category: quality

Coverage is below a declared threshold.

Why it matters: A threshold break is a concrete signal of degraded test effectiveness.

Remediation: Target high-risk modules first, distinguish broad threshold issues from critical blind spots.

---

### Migration (6 types)

#### frameworkMigration
Category: migration

The repository or package appears suitable for migration from one framework to another.

Remediation: Review representative examples, estimate blockers before conversion.

---

#### migrationBlocker
Category: migration

A pattern makes automated or safe migration difficult.

Remediation: Group by blocker type, address high-frequency blockers first.

---

#### deprecatedTestPattern
Category: migration

A test pattern is outdated or poorly aligned with target framework standards.

Remediation: Update patterns early, include in modernization backlog.

---

#### dynamicTestGeneration
Category: migration

Dynamic generation patterns reduce migration predictability.

Remediation: Isolate generation logic, review manually, simplify when possible.

---

#### customMatcherRisk
Category: migration

Custom matchers or helper abstractions complicate portability.

Remediation: Inventory wrappers, add mapping support or refactor to standard assertions.

---

#### unsupportedSetup
Category: migration

Framework-specific setup or fixture patterns that may not have equivalents in the target framework.

Remediation: Catalog framework-specific patterns, identify equivalent mechanisms in the target.

---

### Governance (4 types)

#### policyViolation
Category: governance

Current repository state violates declared Terrain policy.

Remediation: Review local policy configuration, fix or explicitly waive with rationale.

---

#### legacyFrameworkUsage
Category: governance

Legacy or disallowed framework usage persists or is reintroduced.

Remediation: Prevent new usage, prioritize migration hotspots.

---

#### skippedTestsInCI
Category: governance

Skipped tests are present where CI policy disallows them.

Remediation: Remove or restore skipped tests, use limited exceptions explicitly.

---

#### runtimeBudgetExceeded
Category: governance

Tests or suites exceed configured runtime budgets.

Remediation: Isolate hotspots, refactor test setup, adjust policy only with explicit intent.

---

## Tier 2: Runtime Health Signals (5 types)

Emitted when runtime artifacts are provided (JUnit XML, Jest JSON, etc.). These signals require observed test execution data.

#### slowTest
Category: health | Evidence: runtime

A test or suite consistently exceeds an expected runtime threshold.

Why it matters: Slow tests create CI bottlenecks, slow feedback loops, and increase migration validation cost.

Remediation: Reduce setup overhead, isolate expensive integration behavior, split large runtime hotspots.

---

#### flakyTest
Category: health | Evidence: runtime

A test demonstrates intermittent failures or elevated retry behavior.

Why it matters: Flakes reduce trust in the test suite and make changes harder to validate.

Remediation: Identify nondeterministic dependencies, reduce timing assumptions, isolate unstable fixtures.

---

#### skippedTest
Category: health | Evidence: runtime

A test is disabled, skipped, or pending in runtime results.

Why it matters: Skipped tests create false confidence and often conceal degraded quality.

Remediation: Restore or remove intentionally, track stale skips, prevent accumulation in CI.

---

#### deadTest
Category: health | Evidence: runtime

A test appears disconnected from live behavior, observed only in skipped state.

Why it matters: Dead tests increase maintenance cost while providing little or no confidence.

Remediation: Delete if obsolete, reconnect if intended to remain active.

---

#### unstableSuite
Category: health | Evidence: runtime

A suite exhibits unusually high variance, retries, or inconsistency as a group.

Why it matters: Suite-level instability often indicates shared fixture or infrastructure problems.

Remediation: Inspect common setup/teardown, isolate environmental dependencies, reduce shared mutable state.

---

## Tier 3: Structural Graph Signals (7 types)

Emitted from dependency graph analysis. These signals use cross-file relationship traversal to find patterns invisible to per-file analysis.

#### blastRadiusHotspot
Category: structure | Evidence: graph-traversal

Source files where a change would impact an unusually large number of tests.

Remediation: Ensure high direct test coverage and consider adding contract tests at interface boundaries.

---

#### fixtureFragilityHotspot
Category: structure | Evidence: graph-traversal

Fixtures depended on by many tests, where a single change cascades widely.

Remediation: Extract smaller, focused fixtures to reduce cascading test failures.

---

#### assertionFreeImport
Category: quality | Evidence: graph-traversal

Test files that import production code but contain zero assertions.

Remediation: Add assertions to validate behavior or remove tests that verify nothing.

---

#### uncoveredAISurface
Category: ai | Evidence: graph-traversal

AI surfaces (prompts, tools, datasets) with zero test or scenario coverage.

Remediation: Add eval scenarios that exercise this AI surface.

---

#### phantomEvalScenario
Category: ai | Evidence: graph-traversal

Eval scenarios that claim to validate AI surfaces but have no import-graph path to those surfaces.

Remediation: Verify the test file imports and exercises the target code, or correct the surface mapping.

---

#### untestedPromptFlow
Category: ai | Evidence: graph-traversal

A prompt flows through multiple source files via imports with zero test coverage anywhere in the chain.

Remediation: Add integration tests at the prompt's consumption points.

---

#### capabilityValidationGap
Category: ai | Evidence: graph-traversal

Inferred AI capabilities have no eval scenarios validating them.

Remediation: Add eval scenarios that exercise this capability to ensure behavioral regression detection.

---

## Tier 4: AI/Eval Signals (24 types)

Emitted when Gauntlet evaluation artifacts are provided. These signals represent observed evaluation failures and regressions from actual AI system execution. They cannot be produced by static analysis.

**Eval execution:**
`evalFailure`, `evalRegression`, `accuracyRegression`

**Safety:**
`safetyFailure`, `aiPolicyViolation`, `hallucinationDetected`

**Grounding and citation:**
`answerGroundingFailure`, `citationMissing`, `citationMismatch`

**RAG pipeline:**
`retrievalMiss`, `wrongSourceSelected`, `staleSourceRisk`, `contextOverflowRisk`, `chunkingRegression`, `rerankerRegression`, `topKRegression`

**Tool and agent:**
`toolSelectionError`, `schemaParseFailure`, `toolRoutingError`, `toolGuardrailViolation`, `toolBudgetExceeded`, `agentFallbackTriggered`

**Performance:**
`latencyRegression`, `costRegression`
