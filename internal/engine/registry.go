// Package engine provides the analysis orchestration layer.
//
// It wires together detectors, ownership resolution, risk scoring,
// and governance evaluation into a single reusable pipeline.
package engine

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/health"
	"github.com/pmclSF/terrain/internal/migration"
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
		Detector: &quality.UntestedExportDetector{},
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
		Detector: &quality.StaticSkipDetector{},
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
		Detector: &quality.OrphanedTestDetector{},
	})

	// Migration detectors (no dependencies on other signals).
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "migration.deprecated-pattern",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect deprecated test patterns that block migration.",
			SignalTypes:    []models.SignalType{signals.SignalDeprecatedTestPattern},
			RequiresFileIO: true,
		},
		Detector: &migration.DeprecatedPatternDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "migration.dynamic-test-generation",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect dynamic test generation patterns.",
			SignalTypes:    []models.SignalType{signals.SignalDynamicTestGeneration},
			RequiresFileIO: true,
		},
		Detector: &migration.DynamicTestGenerationDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "migration.custom-matcher",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect custom matchers that complicate migration.",
			SignalTypes:    []models.SignalType{signals.SignalCustomMatcherRisk},
			RequiresFileIO: true,
		},
		Detector: &migration.CustomMatcherDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:             "migration.unsupported-setup",
			Domain:         signals.DomainMigration,
			EvidenceType:   signals.EvidenceStructuralPattern,
			Description:    "Detect framework-specific setup/fixture patterns.",
			SignalTypes:    []models.SignalType{signals.SignalUnsupportedSetup},
			RequiresFileIO: true,
		},
		Detector: &migration.UnsupportedSetupDetector{RepoRoot: cfg.RepoRoot},
	})
	reg(signals.DetectorRegistration{
		Meta: signals.DetectorMeta{
			ID:           "migration.framework-migration",
			Domain:       signals.DomainMigration,
			EvidenceType: signals.EvidenceStructuralPattern,
			Description:  "Detect multi-framework repos suitable for migration.",
			SignalTypes:  []models.SignalType{signals.SignalFrameworkMigration},
		},
		Detector: &migration.FrameworkMigrationDetector{},
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
