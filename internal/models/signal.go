package models

// SignalType is the canonical identifier for a Terrain signal.
// Signal type constants are defined in internal/signals for registry use,
// but the type itself lives here so the snapshot model is self-contained.
type SignalType string

// SignalCategory groups signal types into major product areas.
type SignalCategory string

const (
	CategoryStructure  SignalCategory = "structure"
	CategoryHealth     SignalCategory = "health"
	CategoryQuality    SignalCategory = "quality"
	CategoryMigration  SignalCategory = "migration"
	CategoryGovernance SignalCategory = "governance"
	CategoryAI         SignalCategory = "ai"
)

// SignalSeverity expresses how urgent or important a signal is.
type SignalSeverity string

const (
	SeverityInfo     SignalSeverity = "info"
	SeverityLow      SignalSeverity = "low"
	SeverityMedium   SignalSeverity = "medium"
	SeverityHigh     SignalSeverity = "high"
	SeverityCritical SignalSeverity = "critical"
)

// SignalLocation identifies where a signal applies.
type SignalLocation struct {
	Repository string `json:"repository,omitempty"`
	Package    string `json:"package,omitempty"`
	File       string `json:"file,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Line       int    `json:"line,omitempty"`

	// ScenarioID links the signal to a specific AI/eval scenario.
	ScenarioID string `json:"scenarioId,omitempty"`

	// Capability links the signal to a business capability.
	Capability string `json:"capability,omitempty"`
}

// EvidenceStrength describes how robust the evidence behind a signal is.
type EvidenceStrength string

const (
	EvidenceStrong   EvidenceStrength = "strong"   // AST-backed, coverage data, runtime observation
	EvidenceModerate EvidenceStrength = "moderate" // structural pattern matching with context
	EvidencePartial  EvidenceStrength = "partial"  // some evidence available, with known gaps
	EvidenceWeak     EvidenceStrength = "weak"     // path/name heuristic only
	EvidenceNone     EvidenceStrength = "none"     // no supporting evidence available
)

// EvidenceSource describes how the signal was derived.
type EvidenceSource string

const (
	SourceAST               EvidenceSource = "ast"
	SourceStructuralPattern EvidenceSource = "structural-pattern"
	SourcePathName          EvidenceSource = "path-name"
	SourceRuntime           EvidenceSource = "runtime"
	SourceCoverage          EvidenceSource = "coverage"
	SourcePolicy            EvidenceSource = "policy"
	SourceCodeowners        EvidenceSource = "codeowners"
	SourceEvalExecution     EvidenceSource = "eval-execution"
	SourceGraphTraversal    EvidenceSource = "graph-traversal"
)

// Actionability classifies how soon a signal demands attention. Distinct
// from severity: a Critical-severity signal in a deprecated module may
// still be Advisory; a Medium signal blocking a release is Immediate.
//
// SignalV2 field (0.2). Older snapshots leave the field empty.
type Actionability string

const (
	ActionabilityImmediate Actionability = "immediate" // block ship / page oncall
	ActionabilityScheduled Actionability = "scheduled" // address within sprint
	ActionabilityMonitor   Actionability = "monitor"   // track; act if it worsens
	ActionabilityAdvisory  Actionability = "advisory"  // FYI; no action expected
)

// LifecycleStage names a phase of the test/code lifecycle a signal applies
// to. A signal may attach to multiple stages; e.g. flaky-test surfaces in
// both ci-run (where it manifests) and maintenance (where it's fixed).
//
// SignalV2 field (0.2).
type LifecycleStage string

const (
	StageDesign        LifecycleStage = "design"         // architecture, planning
	StageTestAuthoring LifecycleStage = "test-authoring" // writing/editing tests
	StageCIRun         LifecycleStage = "ci-run"         // pipeline execution
	StageMaintenance   LifecycleStage = "maintenance"    // ongoing care
	StageMigration     LifecycleStage = "migration"      // framework/tooling change
	StageRetirement    LifecycleStage = "retirement"     // sunset / removal
)

// AIRelevance ranks how much a signal matters for AI-native test surfaces
// (prompts, evals, agents, RAG). Lets non-AI consumers filter cleanly.
//
// SignalV2 field (0.2).
type AIRelevance string

const (
	AIRelevanceNone   AIRelevance = "none"
	AIRelevanceLow    AIRelevance = "low"
	AIRelevanceMedium AIRelevance = "medium"
	AIRelevanceHigh   AIRelevance = "high"
)

// ConfidenceDetail is the rich form of confidence introduced in SignalV2.
// Replaces a bare 0.0–1.0 float with a Wilson/Beta-style interval, an
// origin classifier, and the evidence sources that fed the estimate.
//
// The legacy Signal.Confidence float field stays in place for existing
// consumers; detectors that opt into v2 should populate both for one or
// two releases (the float reflects ConfidenceDetail.Value).
type ConfidenceDetail struct {
	// Value is the point estimate (0.0–1.0). Mirrors the legacy
	// Signal.Confidence float so v1 consumers keep working.
	Value float64 `json:"value"`

	// IntervalLow / IntervalHigh bracket the 95% credible interval. Both
	// default to Value when no interval is computable (binary detectors).
	IntervalLow  float64 `json:"intervalLow,omitempty"`
	IntervalHigh float64 `json:"intervalHigh,omitempty"`

	// Quality classifies how the estimate was produced.
	//   "calibrated" — anchored to a labeled corpus precision/recall
	//   "heuristic"  — author-set, derived from rule structure
	//   "estimate"   — bounded but not corpus-validated
	Quality string `json:"quality,omitempty"`

	// Sources lists the EvidenceSource strings that contributed to this
	// estimate, in the order the detector consulted them.
	Sources []EvidenceSource `json:"sources,omitempty"`
}

// SignalReference points at another signal for compound-evidence
// aggregation. The minimal form is just the type; pairs of signals on
// the same location can declare a stronger combined finding.
//
// SignalV2 field (0.2).
type SignalReference struct {
	Type SignalType `json:"type"`

	// Location is optional and used when the related signal is on a
	// different file/symbol than the one referencing it. Empty means
	// "same location as the referrer".
	Location *SignalLocation `json:"location,omitempty"`

	// Relationship classifies the reference for the renderer.
	//   "corroborates" — same finding, different evidence path
	//   "contradicts"  — would invalidate this one if confirmed
	//   "supersedes"   — referrer replaces the referenced signal
	//   "depends-on"   — referrer is only meaningful if referenced fires
	Relationship string `json:"relationship,omitempty"`
}

// Signal is the canonical structured insight type in Terrain.
//
// Every meaningful user-facing finding should be representable as a Signal.
// This type lives in models because it is a core part of TestSuiteSnapshot
// and must be serializable without circular imports.
//
// Signals are designed to be:
//   - explainable
//   - serializable
//   - composable
//   - renderable in multiple surfaces (CLI, extension, CI)
//
// SignalV2 (0.2) added the multi-axis fields below the dashed line. They
// are all `omitempty`, so v1 producers and consumers continue to work
// against v2 binaries — additive changes only, no migration code needed.
// Detectors emit v2 fields opportunistically; the calibration corpus and
// severity rubric work in 0.2 fills them in across the catalog.
type Signal struct {
	Type     SignalType     `json:"type"`
	Category SignalCategory `json:"category"`
	Severity SignalSeverity `json:"severity"`

	// Confidence indicates how certain Terrain is about the signal.
	// Expected range is 0.0 to 1.0. Retained for v1 consumers; v2
	// detectors mirror this in ConfidenceDetail.Value.
	Confidence float64 `json:"confidence,omitempty"`

	// EvidenceStrength classifies the robustness of the evidence.
	// Weak-evidence signals are rendered with appropriate caveats.
	EvidenceStrength EvidenceStrength `json:"evidenceStrength,omitempty"`

	// EvidenceSource identifies how the signal was derived.
	EvidenceSource EvidenceSource `json:"evidenceSource,omitempty"`

	Location SignalLocation `json:"location"`

	Owner string `json:"owner,omitempty"`

	Explanation string `json:"explanation"`

	SuggestedAction string `json:"suggestedAction,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`

	// ── SignalV2 (0.2) fields. All optional and omitempty. ──────────

	// SeverityClauses lists the clause IDs from docs/severity-rubric.md
	// that justify the assigned Severity. Empty means the detector has
	// not yet been re-anchored to the rubric (0.2 task).
	SeverityClauses []string `json:"severityClauses,omitempty"`

	// ConfidenceDetail is the rich form of Confidence. nil for v1
	// detectors; populated by v2 detectors with Wilson/Beta intervals.
	ConfidenceDetail *ConfidenceDetail `json:"confidenceDetail,omitempty"`

	// Actionability classifies how urgently the finding demands action,
	// independent of Severity.
	Actionability Actionability `json:"actionability,omitempty"`

	// LifecycleStages names which phases of the lifecycle the signal
	// applies to. Supports filtering ("CI-run-only signals") in views.
	LifecycleStages []LifecycleStage `json:"lifecycleStages,omitempty"`

	// AIRelevance lets non-AI consumers hide AI-flavoured findings (or
	// vice versa) without a hardcoded type denylist.
	AIRelevance AIRelevance `json:"aiRelevance,omitempty"`

	// RuleID mirrors the manifest entry's RuleID at emission time. Useful
	// for SARIF emission and stable cross-references — denormalised here
	// so consumers don't need to re-resolve via the manifest.
	RuleID string `json:"ruleId,omitempty"`

	// RuleURI mirrors the manifest entry's RuleURI for the same reason.
	RuleURI string `json:"ruleUri,omitempty"`

	// DetectorVersion identifies which version of the detector emitted
	// the signal. Lets reports flag "this finding came from a detector
	// that has since been calibrated against the corpus".
	DetectorVersion string `json:"detectorVersion,omitempty"`

	// RelatedSignals lists references to other signals for compound
	// evidence aggregation. The renderer uses this to fold corroborating
	// findings into a single block instead of repeating noise.
	RelatedSignals []SignalReference `json:"relatedSignals,omitempty"`

	// FindingID is the stable identifier for this finding, used by
	// suppressions, the `terrain explain finding <id>` round-trip, and
	// `--new-findings-only` baseline gating. Format and semantics are
	// owned by `internal/identity.BuildFindingID`. Empty when emitted
	// before the engine's id-assignment pass runs (during construction
	// inside detectors); the pipeline populates this field on every
	// signal before snapshot serialization.
	//
	// Stability: same (Type, Location.File, Location.Symbol,
	// Location.Line) → same FindingID across runs. File rename or
	// symbol rename produces a new FindingID. Line drift WITHOUT a
	// symbol changes the ID; AST-anchored 0.3 work removes that
	// limitation.
	FindingID string `json:"findingId,omitempty"`
}
