package engine

import (
	"testing"

	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestDefaultRegistry_WithoutPolicy(t *testing.T) {
	t.Parallel()
	r, _ := DefaultRegistry(Config{RepoRoot: "."})

	// 7 quality (incl. deps.drift-risk + config.schema-drift) + 2
	// coverage + 4 health (assertion-free + orphaned-test + static-skip +
	// 5 runtime adapters) + 5 migration + 7 structural + 13 stable AI
	// (incl. surface-missing-eval) = 41, no governance. Preview AI
	// detectors (15) are gated behind EnablePreviewRules.
	if r.Len() != 41 {
		t.Errorf("DefaultRegistry without policy: Len() = %d, want 41", r.Len())
	}

	quality := r.ByDomain(signals.DomainQuality)
	if len(quality) != 7 {
		t.Errorf("quality detectors = %d, want 7", len(quality))
	}

	coverage := r.ByDomain(signals.DomainCoverage)
	if len(coverage) != 2 {
		t.Errorf("coverage detectors = %d, want 2", len(coverage))
	}

	migration := r.ByDomain(signals.DomainMigration)
	if len(migration) != 5 {
		t.Errorf("migration detectors = %d, want 5", len(migration))
	}

	ai := r.ByDomain(signals.DomainAI)
	if len(ai) != 13 {
		t.Errorf("ai detectors = %d, want 13 (stable; preview gated)", len(ai))
	}

	governance := r.ByDomain(signals.DomainGovernance)
	if len(governance) != 0 {
		t.Errorf("governance detectors = %d, want 0 (no policy)", len(governance))
	}
}

func TestDefaultRegistry_PreviewEnabled(t *testing.T) {
	t.Parallel()
	r, _ := DefaultRegistry(Config{RepoRoot: ".", EnablePreviewRules: true})
	if r.Len() != 56 {
		t.Errorf("DefaultRegistry with preview: Len() = %d, want 56", r.Len())
	}
	ai := r.ByDomain(signals.DomainAI)
	if len(ai) != 28 {
		t.Errorf("ai detectors = %d, want 28 (13 stable + 15 preview)", len(ai))
	}
}

func TestDefaultRegistry_WithPolicy(t *testing.T) {
	t.Parallel()
	boolTrue := true
	cfg := Config{
		RepoRoot: ".",
		PolicyConfig: &policy.Config{
			Rules: policy.Rules{
				DisallowSkippedTests: &boolTrue,
			},
		},
	}
	r, _ := DefaultRegistry(cfg)

	// Same 41 plus the policy governance detector.
	if r.Len() != 42 {
		t.Errorf("DefaultRegistry with policy: Len() = %d, want 42", r.Len())
	}

	governance := r.ByDomain(signals.DomainGovernance)
	if len(governance) != 1 {
		t.Errorf("governance detectors = %d, want 1", len(governance))
	}
}

func TestDefaultRegistry_DetectorIDs(t *testing.T) {
	t.Parallel()
	// Enable preview rules so the full ID enumeration runs end-to-end.
	r, _ := DefaultRegistry(Config{RepoRoot: ".", EnablePreviewRules: true})

	expectedIDs := []string{
		"quality.weak-assertion",
		"quality.mock-heavy",
		"quality.snapshot-heavy",
		"quality.untested-export",
		"quality.coverage-threshold",
		"coverage.blind-spot",
		"quality.static-skip",
		"health.assertion-free",
		"health.orphaned-test",
		"deps.drift-risk",
		"config.schema-drift",
		"framework_migration.deprecated-pattern",
		"framework_migration.dynamic-test-generation",
		"framework_migration.custom-matcher",
		"framework_migration.unsupported-setup",
		"framework_migration.framework-migration",
		"health.slow-test",
		"health.flaky-test",
		"health.skipped-test",
		"health.dead-test",
		"health.unstable-suite",
		"structural.assertion-free-import",
		"structural.blast-radius-hotspot",
		"structural.fixture-fragility-hotspot",
		"structural.uncovered-ai-surface",
		"structural.phantom-eval-scenario",
		"structural.untested-prompt-flow",
		"structural.capability-validation-gap",
		"ai.hardcoded-api-key",
		"ai.non-deterministic-eval",
		"ai.model-deprecation-risk",
		"ai.prompt-injection-risk",
		"ai.tool-without-sandbox",
		"ai.safety-eval-missing",
		"ai.surface-missing-eval",
		"ai.hallucination-rate",
		"ai.cost-regression",
		"ai.retrieval-regression",
		"ai.prompt-versioning",
		"ai.few-shot-contamination",
		"ai.embedding-model-change",
		"ai.orphaned-eval",
		"ai.missing-eval-categories",
		"ai.prompt-bloat",
		"ai.prompt-without-temperature",
		"ai.missing-prompt-validator",
		"ai.prompt-version-skew",
		"ai.retrieval-without-rerank",
		"ai.cold-vector-store",
		"ai.agent-loop-risk",
		"ai.tool-without-budget",
		"ai.target-leakage",
		"ai.duplicate-eval-rows",
		"ai.schema-drift",
		"ai.cold-start-time",
		"ai.token-cost-budget",
	}

	all := r.All()
	if len(all) != len(expectedIDs) {
		t.Fatalf("got %d detectors, want %d", len(all), len(expectedIDs))
	}

	for i, expected := range expectedIDs {
		if all[i].Meta.ID != expected {
			t.Errorf("detector[%d].ID = %q, want %q", i, all[i].Meta.ID, expected)
		}
	}
}

func TestDefaultRegistry_GovernanceIsLast(t *testing.T) {
	t.Parallel()
	boolTrue := true
	cfg := Config{
		RepoRoot: ".",
		PolicyConfig: &policy.Config{
			Rules: policy.Rules{
				DisallowSkippedTests: &boolTrue,
			},
		},
	}
	r, _ := DefaultRegistry(cfg)

	all := r.All()
	last := all[len(all)-1]
	if last.Meta.Domain != signals.DomainGovernance {
		t.Errorf("last detector domain = %s, want governance", last.Meta.Domain)
	}
	if !last.Meta.DependsOnSignals {
		t.Error("governance detector should have DependsOnSignals=true")
	}
}

func TestGovernanceDetector_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ signals.Detector = &GovernanceDetector{}
}

func TestDefaultRegistry_DetectorMetaFields(t *testing.T) {
	t.Parallel()
	r, _ := DefaultRegistry(Config{RepoRoot: "."})

	for _, reg := range r.All() {
		if reg.Meta.ID == "" {
			t.Error("detector has empty ID")
		}
		if reg.Meta.Domain == "" {
			t.Errorf("detector %s has empty Domain", reg.Meta.ID)
		}
		if reg.Meta.EvidenceType == "" {
			t.Errorf("detector %s has empty EvidenceType", reg.Meta.ID)
		}
		if reg.Meta.Description == "" {
			t.Errorf("detector %s has empty Description", reg.Meta.ID)
		}
		if len(reg.Meta.SignalTypes) == 0 {
			t.Errorf("detector %s has no SignalTypes", reg.Meta.ID)
		}
	}
}

func TestDefaultRegistry_EmptyPolicy(t *testing.T) {
	t.Parallel()
	// An empty policy config should NOT register governance detector.
	cfg := Config{
		RepoRoot:     ".",
		PolicyConfig: &policy.Config{},
	}
	r, _ := DefaultRegistry(cfg)

	governance := r.ByDomain(signals.DomainGovernance)
	if len(governance) != 0 {
		t.Errorf("empty policy should not register governance, got %d", len(governance))
	}
}
