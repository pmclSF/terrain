// Package engine provides the analysis orchestration layer.
//
// It wires together detectors, ownership resolution, risk scoring,
// and governance evaluation into a single reusable pipeline.
package engine

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/configdrift"
	"github.com/pmclSF/terrain/internal/deps"
	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/health"
	"github.com/pmclSF/terrain/internal/framework_migration"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/quality"
	"github.com/pmclSF/terrain/internal/runtime"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/structural"
)

// Config holds runtime configuration needed to construct detectors.
type Config struct {
	// RepoRoot is the repository root path (required for file-reading detectors).
	RepoRoot string

	// PolicyConfig is the loaded policy configuration (nil if no policy file).
	PolicyConfig *policy.Config

	// RuntimeResults holds ingested runtime data for health detectors.
	// Nil when no runtime artifacts were provided.
	RuntimeResults *[]runtime.TestResult

	// SlowTestThresholdMs is the threshold for slow test detection.
	// Zero uses the default (5000ms).
	SlowTestThresholdMs float64

	// EnablePreviewRules registers the §9 preview-tier AI detectors
	// alongside the stable batch. Default false — preview rules ship
	// default-off per §9 spec, pending LB-5 / LB-6 calibration on the
	// dogfood corpus. Set via `terrain analyze --preview` or
	// terrain.yaml: rules.preview.enabled.
	EnablePreviewRules bool
}

// DefaultRegistry returns a DetectorRegistry populated with all
// standard Terrain detectors in the correct execution order.
// Returns an error if any registration fails (duplicate ID, ordering violation).
//
// The order matters: governance detectors depend on signals from
// quality and migration detectors, so they are registered last.
func DefaultRegistry(cfg Config) (*signals.DetectorRegistry, error) {
	r := signals.NewRegistry()

	// reg is a helper that registers a detector and returns the first error.
	// This avoids 13 separate if-err-return blocks while still propagating errors.
	var firstErr error
	reg := func(registration signals.DetectorRegistration) {
		if firstErr != nil {
			return // already failed
		}
		if err := r.Register(registration); err != nil {
			firstErr = fmt.Errorf("detector registry: %w", err)
		}
	}

	// Quality detectors (no dependencies on other signals).
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "quality.weak-assertion",
			Domain:       signals.DomainQuality,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect test files with weak or missing assertions.",
			SignalTypes:  []models.SignalType{signals.SignalWeakAssertion},
		},
		Detector: &quality.WeakAssertionDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "quality.mock-heavy",
			Domain:       signals.DomainQuality,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect test files with excessive mock usage.",
			SignalTypes:  []models.SignalType{signals.SignalMockHeavyTest, signals.SignalTestsOnlyMocks},
		},
		Detector: &quality.MockHeavyDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "quality.snapshot-heavy",
			Domain:       signals.DomainQuality,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect test files that over-rely on snapshot assertions.",
			SignalTypes:  []models.SignalType{signals.SignalSnapshotHeavyTest},
		},
		Detector: &quality.SnapshotHeavyDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "quality.untested-export",
			Domain:       signals.DomainQuality,
			EvidenceType: signals.EvidencePathName,
			Description:  "Detect exported code units without matching test files.",
			SignalTypes:  []models.SignalType{signals.SignalUntestedExport},
		},
		Detector: &quality.UntestedExportDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "quality.coverage-threshold",
			Domain:         signals.DomainCoverage,
			EvidenceType:   signals.EvidenceCoverage,
			Description:    "Detect coverage below configured thresholds.",
			SignalTypes:    []models.SignalType{signals.SignalCoverageThresholdBreak},
			RequiresFileIO: true,
		},
		Detector: &quality.CoverageThresholdDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "coverage.blind-spot",
			Domain:       signals.DomainCoverage,
			EvidenceType: signals.EvidenceCoverage,
			Description:  "Detect coverage lineage blind spots across discovered code units.",
			SignalTypes:  []models.SignalType{signals.SignalCoverageBlindSpot},
		},
		Detector: &quality.CoverageBlindSpotDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "quality.static-skip",
			Domain:       signals.DomainQuality,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect statically skipped tests from source code patterns (.skip, xit, @skip, etc.).",
			SignalTypes:  []models.SignalType{signals.SignalStaticSkippedTest},
		},
		Detector: &quality.StaticSkipDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.assertion-free",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect test files with tests but no detectable assertions.",
			SignalTypes:  []models.SignalType{signals.SignalAssertionFreeTest},
		},
		Detector: &quality.AssertionFreeDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.orphaned-test",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect test files with no linked source code units.",
			SignalTypes:  []models.SignalType{signals.SignalOrphanedTestFile},
		},
		Detector: &quality.OrphanedTestDetector{RepoRoot: cfg.RepoRoot},
	})

	// Dependency-drift detector (Tier 5.1): targets the 35.2% of the
	// 0.2.0 recall gap attributable to bot-authored deps-bump PRs.
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "deps.drift-risk",
			Domain:         signals.DomainQuality,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect dependency manifests with a high share of moving-target version specs.",
			SignalTypes:    []models.SignalType{signals.SignalDepsDriftRisk},
			RequiresFileIO: true,
		},
		Detector: &deps.DriftRiskDetector{Root: cfg.RepoRoot},
	})

	// Config-schema-drift detector (Tier 5.2): targets the 5.7% of the
	// 0.2.0 recall gap that is config-only PRs (CI, docker-compose,
	// helm, k8s manifests).
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "config.schema-drift",
			Domain:         signals.DomainQuality,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect infra configs using forward-compat hazards (mutable action refs, :latest images, deprecated apiVersions).",
			SignalTypes:    []models.SignalType{signals.SignalConfigSchemaDrift},
			RequiresFileIO: true,
		},
		Detector: &configdrift.SchemaDriftDetector{Root: cfg.RepoRoot},
	})

	// Migration detectors (no dependencies on other signals).
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "framework_migration.deprecated-pattern",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect deprecated test patterns that block framework_migration.",
			SignalTypes:    []models.SignalType{signals.SignalDeprecatedTestPattern},
			RequiresFileIO: true,
		},
		Detector: &framework_migration.DeprecatedPatternDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "framework_migration.dynamic-test-generation",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect dynamic test generation patterns.",
			SignalTypes:    []models.SignalType{signals.SignalDynamicTestGeneration},
			RequiresFileIO: true,
		},
		Detector: &framework_migration.DynamicTestGenerationDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "framework_migration.custom-matcher",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect custom matchers that complicate framework_migration.",
			SignalTypes:    []models.SignalType{signals.SignalCustomMatcherRisk},
			RequiresFileIO: true,
		},
		Detector: &framework_migration.CustomMatcherDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "framework_migration.unsupported-setup",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect framework-specific setup/fixture patterns.",
			SignalTypes:    []models.SignalType{signals.SignalUnsupportedSetup},
			RequiresFileIO: true,
		},
		Detector: &framework_migration.UnsupportedSetupDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "framework_migration.framework-migration",
			Domain:       signals.DomainMigration,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect multi-framework repos suitable for framework_migration.",
			SignalTypes:  []models.SignalType{signals.SignalFrameworkMigration},
		},
		Detector: &framework_migration.FrameworkMigrationDetector{},
	})

	// Runtime health detectors (Phase 1: adapted from health.HealthDetector).
	// These are silent when no runtime data is provided.
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.slow-test",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Detect tests exceeding runtime threshold.",
			SignalTypes:  []models.SignalType{signals.SignalSlowTest},
		},
		Detector: &RuntimeDetectorAdapter{
			Health:  &health.SlowTestDetector{ThresholdMs: cfg.SlowTestThresholdMs},
			Results: cfg.RuntimeResults,
		},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.flaky-test",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Detect tests with intermittent failures or retry behavior.",
			SignalTypes:  []models.SignalType{signals.SignalFlakyTest},
		},
		Detector: &RuntimeDetectorAdapter{
			Health:  &health.FlakyTestDetector{},
			Results: cfg.RuntimeResults,
		},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.skipped-test",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Detect skipped tests from runtime results.",
			SignalTypes:  []models.SignalType{signals.SignalSkippedTest},
		},
		Detector: &RuntimeDetectorAdapter{
			Health:  &health.SkippedTestDetector{},
			Results: cfg.RuntimeResults,
		},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.dead-test",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Detect tests observed only in skipped state.",
			SignalTypes:  []models.SignalType{signals.SignalDeadTest},
		},
		Detector: &RuntimeDetectorAdapter{
			Health:  &health.DeadTestDetector{},
			Results: cfg.RuntimeResults,
		},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "health.unstable-suite",
			Domain:       signals.DomainHealth,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Detect suites with elevated failure rates and retries.",
			SignalTypes:  []models.SignalType{signals.SignalUnstableSuite},
		},
		Detector: &RuntimeDetectorAdapter{
			Health:  &health.UnstableSuiteDetector{},
			Results: cfg.RuntimeResults,
		},
	})

	// Graph-powered structural detectors (Phase 2: require dependency graph).
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.assertion-free-import",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect test files that import production code but never assert on it.",
			SignalTypes:   []models.SignalType{signals.SignalAssertionFreeImport},
			RequiresGraph: true,
		},
		Detector: &structural.AssertionFreeImportDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.blast-radius-hotspot",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect source files with high test blast radius.",
			SignalTypes:   []models.SignalType{signals.SignalBlastRadiusHotspot},
			RequiresGraph: true,
		},
		Detector: &structural.BlastRadiusHotspotDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.fixture-fragility-hotspot",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect fixtures with high test fanout creating fragility risk.",
			SignalTypes:   []models.SignalType{signals.SignalFixtureFragilityHotspot},
			RequiresGraph: true,
		},
		Detector: &structural.FixtureFragilityHotspotDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.uncovered-ai-surface",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect AI surfaces with zero test or scenario coverage.",
			SignalTypes:   []models.SignalType{signals.SignalUncoveredAISurface},
			RequiresGraph: true,
		},
		Detector: &structural.UncoveredAISurfaceDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.phantom-eval-scenario",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect eval scenarios that claim coverage they cannot reach.",
			SignalTypes:   []models.SignalType{signals.SignalPhantomEvalScenario},
			RequiresGraph: true,
		},
		Detector: &structural.PhantomEvalScenarioDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.untested-prompt-flow",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect prompts that flow through source files with zero test coverage.",
			SignalTypes:   []models.SignalType{signals.SignalUntestedPromptFlow},
			RequiresGraph: true,
		},
		Detector: &structural.UntestedPromptFlowDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:            "structural.capability-validation-gap",
			Domain:        signals.DomainStructural,
			EvidenceType:  signals.EvidenceGraphTraversal,
			Description:   "Detect AI capabilities with no eval scenario validation.",
			SignalTypes:   []models.SignalType{signals.SignalCapabilityValidationGap},
			RequiresGraph: true,
		},
		Detector: &structural.CapabilityValidationGapDetector{},
	})

	// AI detectors (0.2). Each reads files referenced by the snapshot
	// (TestFiles + Scenarios) and emits AI-domain signals. They run
	// after quality/migration so any signals they reference (when 0.3
	// adds compound-evidence) are already in the snapshot.
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.hardcoded-api-key",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect hard-coded API keys in AI configuration files.",
			SignalTypes:    []models.SignalType{signals.SignalAIHardcodedAPIKey},
			RequiresFileIO: true,
		},
		Detector: &aidetect.HardcodedAPIKeyDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.non-deterministic-eval",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect eval configs missing temperature: 0 / seed pin.",
			SignalTypes:    []models.SignalType{signals.SignalAINonDeterministicEval},
			RequiresFileIO: true,
		},
		Detector: &aidetect.NonDeterministicEvalDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.model-deprecation-risk",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect floating or deprecated model tags (gpt-4, text-davinci-003, ...).",
			SignalTypes:    []models.SignalType{signals.SignalAIModelDeprecationRisk},
			RequiresFileIO: true,
		},
		Detector: &aidetect.ModelDeprecationDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.prompt-injection-risk",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect prompt-injection-shaped concatenation of user input.",
			SignalTypes:    []models.SignalType{signals.SignalAIPromptInjectionRisk},
			RequiresFileIO: true,
		},
		Detector: &aidetect.PromptInjectionDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.tool-without-sandbox",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect destructive agent tools without an approval gate or sandbox.",
			SignalTypes:    []models.SignalType{signals.SignalAIToolWithoutSandbox},
			RequiresFileIO: true,
		},
		Detector: &aidetect.ToolWithoutSandboxDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "ai.safety-eval-missing",
			Domain:       signals.DomainAI,
			EvidenceType: signals.EvidenceGraphTraversal,
			Description:  "Detect safety-critical surfaces with no safety-shaped scenario coverage.",
			SignalTypes:  []models.SignalType{signals.SignalAISafetyEvalMissing},
		},
		Detector: &aidetect.SafetyEvalMissingDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "ai.surface-missing-eval",
			Domain:       signals.DomainAI,
			EvidenceType: signals.EvidenceGraphTraversal,
			Description:  "Detect AI/ML surfaces (prompt / agent / tool / context / model) with no eval coverage at all.",
			SignalTypes:  []models.SignalType{signals.SignalPromptFileMissingEval},
		},
		Detector: &aidetect.PromptFileMissingEvalDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "ai.hallucination-rate",
			Domain:       signals.DomainAI,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Flag eval runs whose hallucination-shaped failure rate exceeds the configured threshold.",
			SignalTypes:  []models.SignalType{signals.SignalAIHallucinationRate},
		},
		Detector: &aidetect.HallucinationRateDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "ai.cost-regression",
			Domain:       signals.DomainAI,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Flag avg cost-per-case rising more than the configured threshold against a baseline snapshot.",
			SignalTypes:  []models.SignalType{signals.SignalAICostRegression},
		},
		Detector: &aidetect.CostRegressionDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "ai.retrieval-regression",
			Domain:       signals.DomainAI,
			EvidenceType: signals.EvidenceRuntime,
			Description:  "Flag drops in retrieval-quality named-scores (context_relevance, nDCG, coverage, etc.) vs baseline.",
			SignalTypes:  []models.SignalType{signals.SignalAIRetrievalRegression},
		},
		Detector: &aidetect.RetrievalRegressionDetector{},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.prompt-versioning",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Flag prompt-kind surfaces with no recognizable version marker (filename, inline, or comment).",
			SignalTypes:    []models.SignalType{signals.SignalAIPromptVersioning},
			RequiresFileIO: true,
		},
		Detector: &aidetect.PromptVersioningDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.few-shot-contamination",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Flag prompts whose few-shot examples overlap verbatim with the inputs of eval scenarios that cover them.",
			SignalTypes:    []models.SignalType{signals.SignalAIFewShotContamination},
			RequiresFileIO: true,
		},
		Detector: &aidetect.FewShotContaminationDetector{Root: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "ai.embedding-model-change",
			Domain:         signals.DomainAI,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Flag repos that reference an embedding model in source code without any retrieval-shaped eval scenario.",
			SignalTypes:    []models.SignalType{signals.SignalAIEmbeddingModelChange},
			RequiresFileIO: true,
		},
		Detector: &aidetect.EmbeddingModelChangeDetector{Root: cfg.RepoRoot},
	})

	// Preview-tier AI/ML detectors (§9). These ship default-off and
	// are pending LB-5 / LB-6 calibration on the dogfood corpus before
	// promotion to Stable. Each adapter wraps a thin detector in
	// internal/preview that owns the rule semantics. Enabled via
	// Config.EnablePreviewRules.
	previewRegs := []signals.DetectorRegistration{
		{
			Meta: signals.DetectorMeta{
				ID:           "ai.orphaned-eval",
				Domain:       signals.DomainAI,
				EvidenceType: signals.EvidenceStructuralPattern,
				Description:  "Detect evals with no covered AI surface.",
				SignalTypes:  []models.SignalType{signals.SignalOrphanedEval},
			},
			Detector: &aidetect.OrphanedEvalDetector{},
		},
		{
			Meta: signals.DetectorMeta{
				ID:           "ai.missing-eval-categories",
				Domain:       signals.DomainAI,
				EvidenceType: signals.EvidenceStructuralPattern,
				Description:  "Detect eval suites missing adversarial / edge_case / safety categories.",
				SignalTypes:  []models.SignalType{signals.SignalMissingEvalCategories},
			},
			Detector: &aidetect.MissingEvalCategoriesDetector{},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.prompt-bloat",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect prompt files that exceed the configured size budget.",
				SignalTypes:    []models.SignalType{signals.SignalPromptBloat},
				RequiresFileIO: true,
			},
			Detector: &aidetect.PromptBloatDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.prompt-without-temperature",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect LLM SDK calls without an explicit temperature value.",
				SignalTypes:    []models.SignalType{signals.SignalPromptWithoutTemperature},
				RequiresFileIO: true,
			},
			Detector: &aidetect.PromptWithoutTemperatureDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.missing-prompt-validator",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect LLM call sites without a structured-output validator.",
				SignalTypes:    []models.SignalType{signals.SignalMissingPromptValidator},
				RequiresFileIO: true,
			},
			Detector: &aidetect.MissingPromptValidatorDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.prompt-version-skew",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect duplicate prompts that share substantial content under different paths.",
				SignalTypes:    []models.SignalType{signals.SignalPromptVersionSkew},
				RequiresFileIO: true,
			},
			Detector: &aidetect.PromptVersionSkewDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.retrieval-without-rerank",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect retrieval call sites that don't apply a reranker.",
				SignalTypes:    []models.SignalType{signals.SignalRetrievalWithoutRerank},
				RequiresFileIO: true,
			},
			Detector: &aidetect.RetrievalWithoutRerankDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.cold-vector-store",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect vector stores constructed without a population call.",
				SignalTypes:    []models.SignalType{signals.SignalColdVectorStore},
				RequiresFileIO: true,
			},
			Detector: &aidetect.ColdVectorStoreDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.agent-loop-risk",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect agent constructors without max_iterations / recursion_limit.",
				SignalTypes:    []models.SignalType{signals.SignalAgentLoopRisk},
				RequiresFileIO: true,
			},
			Detector: &aidetect.AgentLoopRiskDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.tool-without-budget",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect tool-calling agents without a budget / rate limit.",
				SignalTypes:    []models.SignalType{signals.SignalToolWithoutBudget},
				RequiresFileIO: true,
			},
			Detector: &aidetect.ToolWithoutBudgetDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.target-leakage",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect features derived from the target column in training code.",
				SignalTypes:    []models.SignalType{signals.SignalTargetLeakage},
				RequiresFileIO: true,
			},
			Detector: &aidetect.TargetLeakageDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:             "ai.duplicate-eval-rows",
				Domain:         signals.DomainAI,
				EvidenceType:   signals.EvidenceStructuralPattern,
				Description:    "Detect eval datasets with high row-level duplication.",
				SignalTypes:    []models.SignalType{signals.SignalDuplicateEvalRows},
				RequiresFileIO: true,
			},
			Detector: &aidetect.DuplicateEvalRowsDetector{Root: cfg.RepoRoot},
		},
		{
			Meta: signals.DetectorMeta{
				ID:           "ai.schema-drift",
				Domain:       signals.DomainAI,
				EvidenceType: signals.EvidenceRuntime,
				Description:  "Detect column-set changes between pipeline runs (dormant until 0.3.0 telemetry).",
				SignalTypes:  []models.SignalType{signals.SignalSchemaDrift},
			},
			Detector: &aidetect.SchemaDriftDetector{},
		},
		{
			Meta: signals.DetectorMeta{
				ID:           "ai.cold-start-time",
				Domain:       signals.DomainAI,
				EvidenceType: signals.EvidenceRuntime,
				Description:  "Detect first-request latency spikes vs. warm P50 (dormant until 0.3.0 telemetry).",
				SignalTypes:  []models.SignalType{signals.SignalColdStartTime},
			},
			Detector: &aidetect.ColdStartTimeDetector{},
		},
		{
			Meta: signals.DetectorMeta{
				ID:           "ai.token-cost-budget",
				Domain:       signals.DomainAI,
				EvidenceType: signals.EvidenceRuntime,
				Description:  "Detect eval-run cost crossing budget thresholds (dormant until 0.3.0 telemetry).",
				SignalTypes:  []models.SignalType{signals.SignalTokenCostBudget},
			},
			Detector: &aidetect.TokenCostBudgetDetector{},
		},
	}
	if cfg.EnablePreviewRules {
		for _, p := range previewRegs {
			reg(p)
		}
	}

	// Governance detectors (depend on signals from quality/migration detectors).
	if cfg.PolicyConfig != nil && !cfg.PolicyConfig.IsEmpty() {
		reg(signals.DetectorRegistration{
			Meta: signals.DetectorMeta{
				ID:               "governance.policy",
				Domain:           signals.DomainGovernance,
				EvidenceType:     signals.EvidencePolicy,
				Description:      "Evaluate repository state against local policy rules.",
				SignalTypes:      []models.SignalType{signals.SignalPolicyViolation, signals.SignalLegacyFrameworkUsage, signals.SignalSkippedTestsInCI, signals.SignalRuntimeBudgetExceeded},
				DependsOnSignals: true,
			},
			Detector: &GovernanceDetector{Config: cfg.PolicyConfig},
		})
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return r, nil
}

// GovernanceDetector wraps governance.Evaluate as a signals.Detector.
//
// It must run after quality and migration detectors because some policy
// checks reference their signal counts.
type GovernanceDetector struct {
	Config *policy.Config
}

// Detect implements signals.Detector.
func (d *GovernanceDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	result := governance.Evaluate(snap, d.Config)
	return result.Violations
}
