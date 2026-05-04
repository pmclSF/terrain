package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/analysis"
	"github.com/pmclSF/terrain/internal/coverage"
	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/gauntlet"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/measurement"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/ownership"
	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/portfolio"
	"github.com/pmclSF/terrain/internal/runtime"
	"github.com/pmclSF/terrain/internal/scoring"
)

// DefaultEngineVersion is used when PipelineOptions.EngineVersion is not set.
const DefaultEngineVersion = "dev"

// PipelineResult holds the output of a full analysis pipeline run.
type PipelineResult struct {
	Snapshot          *models.TestSuiteSnapshot
	Graph             *depgraph.Graph // sealed graph built during analysis; reusable by downstream commands
	HasPolicy         bool
	DataCompleteness  DataCompleteness
	Diagnostics       *PipelineDiagnostics // populated when CollectDiagnostics is true
	ArtifactDiscovery *ArtifactDiscovery   // populated when auto-discovery runs
	DiscoveryMessages []string             // user-facing messages about auto-detected artifacts
}

// DataCompleteness captures which analysis inputs were present and usable.
type DataCompleteness struct {
	SourceAvailable   bool
	CoverageProvided  bool
	CoverageAvailable bool
	RuntimeProvided   bool
	RuntimeAvailable  bool
	PolicyAvailable   bool
}

// ProgressFunc is called by the pipeline to report step-based progress.
// step is the 1-based step number (e.g., 1 of 5), total is the total
// number of steps, and label describes the current step.
type ProgressFunc func(step, total int, label string)

// PipelineOptions configures optional pipeline behavior.
type PipelineOptions struct {
	// CoveragePath is the path to a coverage file or directory.
	// When set, coverage data is ingested and attributed to code units.
	CoveragePath string

	// CoverageRunLabel classifies the coverage artifact source
	// (for example: unit, integration, e2e). When empty, label inference
	// falls back to artifact file names and default heuristics.
	CoverageRunLabel string

	// RuntimePaths are paths to runtime artifact files (JUnit XML, Jest JSON).
	// When set, runtime data is ingested for health signal detection.
	RuntimePaths []string

	// GauntletPaths are paths to Gauntlet AI eval result artifacts (JSON).
	// When set, Gauntlet results are ingested and applied to scenarios.
	GauntletPaths []string

	// PromptfooPaths are paths to Promptfoo `--output` JSON files.
	// When set, the Promptfoo adapter ingests them into snap.EvalRuns.
	// SignalV2 0.2 field — see internal/airun/promptfoo.go.
	PromptfooPaths []string

	// DeepEvalPaths are paths to DeepEval `--export` JSON files.
	// Same destination as PromptfooPaths: each result lands in
	// snap.EvalRuns through internal/airun/deepeval.go.
	DeepEvalPaths []string

	// RagasPaths are paths to Ragas eval result JSON files.
	// Same destination as PromptfooPaths / DeepEvalPaths.
	RagasPaths []string

	// BaselineSnapshotPath, when set, points at a previous snapshot
	// JSON file. The pipeline loads it and attaches the result to
	// snap.Baseline so regression-aware detectors (aiCostRegression,
	// aiRetrievalRegression) can compare current vs baseline.
	BaselineSnapshotPath string

	// SlowTestThresholdMs overrides the default slow test threshold.
	SlowTestThresholdMs float64

	// CollectDiagnostics enables pipeline timing and count diagnostics.
	CollectDiagnostics bool

	// EngineVersion is stamped into SnapshotMeta.EngineVersion.
	// If empty, DefaultEngineVersion is used.
	EngineVersion string

	// OnProgress is called at each pipeline step to report progress.
	// If nil, no progress is reported. Progress is always written to
	// stderr (not stdout) to avoid interfering with JSON or report output.
	OnProgress ProgressFunc
}

// RunPipeline executes the full analysis pipeline:
//  1. Static analysis (file discovery, framework detection, code units)
//  2. Policy loading
//  3. Runtime ingestion (optional)
//  4. Coverage ingestion (optional)
//  5. Signal detection via the detector registry
//  6. Ownership resolution
//  7. Risk scoring
//  8. Measurement-layer posture
//  9. Portfolio intelligence
//  10. Deterministic sorting
//
// This replaces the duplicated detector invocation across CLI commands.
func RunPipeline(root string, opts ...PipelineOptions) (*PipelineResult, error) {
	return RunPipelineContext(context.Background(), root, opts...)
}

// RunPipelineContext executes the full analysis pipeline with cancellation
// support via context.
func RunPipelineContext(ctx context.Context, root string, opts ...PipelineOptions) (*PipelineResult, error) {
	var opt PipelineOptions
	if len(opts) > 1 {
		return nil, fmt.Errorf("expected at most one PipelineOptions value, got %d", len(opts))
	}
	if len(opts) == 1 {
		opt = opts[0]
	}
	if err := validatePipelineOptions(root, opt); err != nil {
		return nil, err
	}

	// Auto-discover coverage and runtime artifacts when not explicitly provided.
	discovery := DiscoverArtifacts(root)
	discoveryMessages := ApplyDiscovery(&opt, discovery)

	engineVersion := strings.TrimSpace(opt.EngineVersion)
	if engineVersion == "" {
		engineVersion = DefaultEngineVersion
	}

	var diag *PipelineDiagnostics
	if opt.CollectDiagnostics {
		diag = &PipelineDiagnostics{}
	}
	pipelineStart := time.Now()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Progress reporting. totalSteps is fixed at 5 user-visible stages.
	const totalSteps = 5
	progress := func(step int, label string) {
		if opt.OnProgress != nil {
			opt.OnProgress(step, totalSteps, label)
		}
	}

	logging.L().Debug("pipeline starting", "root", root, "coveragePath", opt.CoveragePath, "runtimePaths", len(opt.RuntimePaths))

	progress(1, "Scanning repository")

	// Group 1: perform independent preparation work concurrently.
	// - static analysis (required for all downstream stages)
	// - policy loading
	// - terrain.yaml config loading (manual coverage, CI duration)
	// - runtime artifact ingestion (if provided)
	// - coverage artifact ingestion (if provided)
	var (
		snapshot          *models.TestSuiteSnapshot
		policyResult      *policy.LoadResult
		policyErr         error
		terrainCfg        *policy.TerrainConfig
		terrainCfgErr     error
		ownerResolver     *ownership.Resolver
		runtimeResults    []runtime.TestResult
		runtimeIngestErr     error
		coverageArtifacts    []coverage.CoverageArtifact
		coverageIngestErr    error
		gauntletArtifacts    []*gauntlet.Artifact
		gauntletIngestErr    error
		promptfooEnvelopes   []models.EvalRunEnvelope
		promptfooIngestErr   error
		deepevalEnvelopes    []models.EvalRunEnvelope
		deepevalIngestErr    error
		ragasEnvelopes       []models.EvalRunEnvelope
		ragasIngestErr       error

		staticAnalysisDuration  time.Duration
		policyLoadDuration      time.Duration
		ownershipLoadDuration   time.Duration
		runtimeIngestDuration   time.Duration
		coverageIngestDuration  time.Duration
		staticAnalysisTestFiles int
	)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	fatalErrCh := make(chan error, 1)
	recordFatal := func(err error) {
		if err == nil {
			return
		}
		select {
		case fatalErrCh <- err:
		default:
		}
		cancel()
	}

	startTask := func(wg *sync.WaitGroup, task func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := task(runCtx); err != nil {
				recordFatal(err)
			}
		}()
	}

	var prepWG sync.WaitGroup
	startTask(&prepWG, func(taskCtx context.Context) error {
		if err := taskCtx.Err(); err != nil {
			return err
		}
		stepStart := time.Now()
		analyzer := analysis.New(root)
		analyzedSnapshot, err := analyzer.AnalyzeContext(taskCtx)
		staticAnalysisDuration = time.Since(stepStart)
		if err != nil {
			return err
		}
		snapshot = analyzedSnapshot
		staticAnalysisTestFiles = len(analyzedSnapshot.TestFiles)
		return nil
	})
	startTask(&prepWG, func(taskCtx context.Context) error {
		if err := taskCtx.Err(); err != nil {
			return err
		}
		stepStart := time.Now()
		policyResult, policyErr = policy.Load(root)
		policyLoadDuration = time.Since(stepStart)
		return nil
	})
	startTask(&prepWG, func(taskCtx context.Context) error {
		if err := taskCtx.Err(); err != nil {
			return err
		}
		terrainCfg, terrainCfgErr = policy.LoadTerrainConfig(root)
		return nil
	})
	startTask(&prepWG, func(taskCtx context.Context) error {
		if err := taskCtx.Err(); err != nil {
			return err
		}
		stepStart := time.Now()
		ownerResolver = ownership.NewResolver(root)
		ownershipLoadDuration = time.Since(stepStart)
		return nil
	})
	if len(opt.RuntimePaths) > 0 {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			stepStart := time.Now()
			runtimeResults, runtimeIngestErr = ingestRuntimeArtifacts(taskCtx, opt.RuntimePaths)
			runtimeIngestDuration = time.Since(stepStart)
			return nil
		})
	}
	if opt.CoveragePath != "" {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			stepStart := time.Now()
			coverageArtifacts, coverageIngestErr = ingestCoverageArtifacts(taskCtx, opt.CoveragePath, opt.CoverageRunLabel)
			coverageIngestDuration = time.Since(stepStart)
			return nil
		})
	}
	if len(opt.GauntletPaths) > 0 {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			gauntletArtifacts, gauntletIngestErr = ingestGauntletArtifacts(opt.GauntletPaths)
			return nil
		})
	}
	if len(opt.PromptfooPaths) > 0 {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			promptfooEnvelopes, promptfooIngestErr = ingestPromptfooArtifacts(root, opt.PromptfooPaths)
			return nil
		})
	}
	if len(opt.DeepEvalPaths) > 0 {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			deepevalEnvelopes, deepevalIngestErr = ingestDeepEvalArtifacts(root, opt.DeepEvalPaths)
			return nil
		})
	}
	if len(opt.RagasPaths) > 0 {
		startTask(&prepWG, func(taskCtx context.Context) error {
			if err := taskCtx.Err(); err != nil {
				return err
			}
			ragasEnvelopes, ragasIngestErr = ingestRagasArtifacts(root, opt.RagasPaths)
			return nil
		})
	}
	prepWG.Wait()
	select {
	case err := <-fatalErrCh:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("static analysis produced nil snapshot")
	}
	if diag != nil {
		diag.add("static-analysis", staticAnalysisDuration, staticAnalysisTestFiles)
		diag.add("policy-load", policyLoadDuration, 1)
		diag.add("ownership-load", ownershipLoadDuration, 1)
		if len(opt.RuntimePaths) > 0 {
			diag.add("runtime-ingestion", runtimeIngestDuration, len(opt.RuntimePaths))
		}
		if opt.CoveragePath != "" {
			diag.add("coverage-ingestion", coverageIngestDuration, len(coverageArtifacts))
		}
	}

	logging.L().Debug("static analysis complete",
		"testFiles", len(snapshot.TestFiles),
		"codeUnits", len(snapshot.CodeUnits),
		"codeSurfaces", len(snapshot.CodeSurfaces),
		"duration", staticAnalysisDuration,
	)

	// Step 2: Load policy config result and attach data-source metadata.
	hasPolicy := policyResult != nil && policyResult.Found
	var stepStart time.Time

	var policyCfg *policy.Config
	if hasPolicy {
		policyCfg = policyResult.Config
		snapshot.Policies = policyConfigMap(policyCfg)
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "policy",
			Status: models.DataSourceAvailable,
			Detail: ".terrain/policy.yaml",
		})
	} else if policyErr != nil {
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "policy",
			Status: models.DataSourceError,
			Detail: policyErr.Error(),
			Impact: "Governance checks will not run. Policy violations cannot be detected.",
		})
	} else {
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "policy",
			Status: models.DataSourceUnavailable,
			Impact: "No .terrain/policy.yaml found. Governance checks will not run.",
		})
	}

	// Step 2b: Load manual coverage and scenarios from terrain.yaml (if present).
	if terrainCfgErr != nil {
		logging.L().Warn("failed to load terrain.yaml", "error", terrainCfgErr)
	} else if terrainCfg != nil {
		mcArtifacts := terrainCfg.ToManualCoverageArtifacts()
		if len(mcArtifacts) > 0 {
			snapshot.ManualCoverage = append(snapshot.ManualCoverage, mcArtifacts...)
		}
		scenarios := terrainCfg.ToScenarios()
		if len(scenarios) > 0 {
			snapshot.Scenarios = append(snapshot.Scenarios, scenarios...)
		}
	}

	// Step 2c: Auto-derive AI scenarios from code (no YAML required).
	// Detects eval frameworks (promptfoo, deepeval, langchain, etc.) and
	// derives scenarios from eval test files and AI import patterns.
	aiDetection := aidetect.Detect(root)
	derivedScenarios := aidetect.DeriveScenarios(root, aiDetection, snapshot.CodeSurfaces, snapshot.TestFiles)
	if len(derivedScenarios) > 0 {
		// Merge with manual scenarios, avoiding duplicates by ID or by
		// name+path (manual YAML and auto-derived may have different IDs
		// but represent the same logical scenario).
		existingIDs := map[string]bool{}
		existingKeys := map[string]bool{}
		for _, s := range snapshot.Scenarios {
			existingIDs[s.ScenarioID] = true
			existingKeys[s.Name+"|"+s.Path] = true
		}
		for _, ds := range derivedScenarios {
			if existingIDs[ds.ScenarioID] || existingKeys[ds.Name+"|"+ds.Path] {
				continue
			}
			snapshot.Scenarios = append(snapshot.Scenarios, ds)
		}
	}

	// Step 2d: Infer capabilities on scenarios from naming, paths, and surfaces.
	analysis.InferCapabilities(snapshot.Scenarios, snapshot.CodeSurfaces)

	// Step 2e: Infer AI capabilities from surface kinds (framework-agnostic).
	snapshot.InferredCapabilities = analysis.InferAICapabilities(snapshot.CodeSurfaces, snapshot.Scenarios)

	// Step 3: Runtime ingestion and health detection (optional).
	if len(opt.RuntimePaths) > 0 {
		stepStart = time.Now()
		if runtimeIngestErr != nil {
			logging.L().Warn("runtime ingestion failed", "error", runtimeIngestErr)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "runtime",
				Status: models.DataSourceError,
				Detail: runtimeIngestErr.Error(),
				Impact: "Health measurements (flaky_share, slow_test_share) will report unknown. Risk scoring lacks runtime context.",
			})
		} else {
			applyRuntimeResults(snapshot, runtimeResults, opt.SlowTestThresholdMs)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "runtime",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("%d artifact(s) ingested", len(opt.RuntimePaths)),
			})
		}
		if diag != nil {
			diag.add("runtime-apply", time.Since(stepStart), len(runtimeResults))
		}
	} else {
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "runtime",
			Status: models.DataSourceUnavailable,
			Impact: "Health measurements (flaky_share, slow_test_share) will report unknown. Portfolio cost estimates use type heuristics instead of observed runtime.",
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Step 4: Coverage ingestion (optional).
	// Moved before risk scoring so risk surfaces can account for coverage context.
	if opt.CoveragePath != "" {
		stepStart = time.Now()
		if coverageIngestErr != nil {
			logging.L().Warn("coverage ingestion failed", "error", coverageIngestErr, "path", opt.CoveragePath)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "coverage",
				Status: models.DataSourceError,
				Detail: fmt.Sprintf("path: %s, error: %v", opt.CoveragePath, coverageIngestErr),
				Impact: "Coverage measurements will report unknown. Portfolio breadth estimates use module heuristics. untestedExport signals rely on import graph only.",
			})
		} else {
			applyCoverageArtifacts(snapshot, coverageArtifacts)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "coverage",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("path: %s", opt.CoveragePath),
			})
		}
		if diag != nil {
			diag.add("coverage-apply", time.Since(stepStart), len(snapshot.CoverageInsights))
		}
	} else {
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "coverage",
			Status: models.DataSourceUnavailable,
			Impact: "Coverage measurements will report unknown. Portfolio breadth estimates use module heuristics. No coverage-based insights will be generated.",
		})
	}
	// Step 4c: Apply Gauntlet artifacts.
	if len(opt.GauntletPaths) > 0 {
		if gauntletIngestErr != nil {
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "gauntlet",
				Status: models.DataSourceError,
				Detail: gauntletIngestErr.Error(),
				Impact: "Gauntlet eval results are unavailable. Scenario execution status will not be reflected.",
			})
		} else {
			for _, art := range gauntletArtifacts {
				gauntlet.ApplyToSnapshot(snapshot, art)
			}
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "gauntlet",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("%d artifact(s) ingested", len(opt.GauntletPaths)),
			})
		}
	}

	// Step 4d: Apply Promptfoo eval-run envelopes (the 0.2 adapter
	// path; the runtime-aware AI detectors will consume these).
	if len(opt.PromptfooPaths) > 0 {
		if promptfooIngestErr != nil {
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "promptfoo",
				Status: models.DataSourceError,
				Detail: promptfooIngestErr.Error(),
				Impact: "Promptfoo results are unavailable. Per-case scoring + token usage will not feed cost/hallucination/retrieval detectors.",
			})
		} else {
			snapshot.EvalRuns = append(snapshot.EvalRuns, promptfooEnvelopes...)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "promptfoo",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("%d artifact(s) ingested", len(opt.PromptfooPaths)),
			})
		}
	}

	// Step 4d-bis: Apply DeepEval eval-run envelopes (same destination
	// as Promptfoo; both adapters write into snap.EvalRuns).
	if len(opt.DeepEvalPaths) > 0 {
		if deepevalIngestErr != nil {
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "deepeval",
				Status: models.DataSourceError,
				Detail: deepevalIngestErr.Error(),
				Impact: "DeepEval results are unavailable. Per-case scoring + token usage will not feed cost/hallucination/retrieval detectors.",
			})
		} else {
			snapshot.EvalRuns = append(snapshot.EvalRuns, deepevalEnvelopes...)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "deepeval",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("%d artifact(s) ingested", len(opt.DeepEvalPaths)),
			})
		}
	}

	// Step 4d-tris: Apply Ragas eval-run envelopes.
	if len(opt.RagasPaths) > 0 {
		if ragasIngestErr != nil {
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "ragas",
				Status: models.DataSourceError,
				Detail: ragasIngestErr.Error(),
				Impact: "Ragas results are unavailable. Per-case retrieval/faithfulness scores will not feed retrieval/hallucination detectors.",
			})
		} else {
			snapshot.EvalRuns = append(snapshot.EvalRuns, ragasEnvelopes...)
			snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
				Name:   "ragas",
				Status: models.DataSourceAvailable,
				Detail: fmt.Sprintf("%d artifact(s) ingested", len(opt.RagasPaths)),
			})
		}
	}

	// Step 4e: Load baseline snapshot when --baseline was provided.
	// Attaches the parsed result to snap.Baseline so regression-aware
	// detectors can compare current vs baseline. Failure is loud
	// rather than degraded: the user explicitly asked for the
	// comparison, so a malformed baseline should fail the run.
	if opt.BaselineSnapshotPath != "" {
		baseline, err := loadBaselineSnapshot(opt.BaselineSnapshotPath)
		if err != nil {
			return nil, fmt.Errorf("load --baseline %s: %w", opt.BaselineSnapshotPath, err)
		}
		snapshot.Baseline = baseline
		snapshot.DataSources = append(snapshot.DataSources, models.DataSource{
			Name:   "baseline-snapshot",
			Status: models.DataSourceAvailable,
			Detail: fmt.Sprintf("loaded from %s (eval runs: %d)", opt.BaselineSnapshotPath, len(baseline.EvalRuns)),
		})
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Step 5: Build dependency graph, then run detectors (flat → graph → dependent).
	progress(2, "Building graph")
	stepStart = time.Now()
	dg := depgraph.Build(snapshot)
	logging.L().Debug("dependency graph built", "nodes", dg.Stats().NodeCount, "edges", dg.Stats().EdgeCount, "duration", time.Since(stepStart))
	if diag != nil {
		diag.add("graph-build", time.Since(stepStart), dg.Stats().NodeCount)
	}

	stepStart = time.Now()
	var runtimeResultsPtr *[]runtime.TestResult
	if len(runtimeResults) > 0 {
		runtimeResultsPtr = &runtimeResults
	}
	registry, regErr := DefaultRegistry(Config{
		RepoRoot:            root,
		PolicyConfig:        policyCfg,
		RuntimeResults:      runtimeResultsPtr,
		SlowTestThresholdMs: opt.SlowTestThresholdMs,
	})
	if regErr != nil {
		return nil, fmt.Errorf("initialize detector registry: %w", regErr)
	}
	signalsBefore := len(snapshot.Signals)
	registry.RunWithGraph(snapshot, dg)
	signalsProduced := len(snapshot.Signals) - signalsBefore
	logging.L().Debug("signal detection complete", "detectors", registry.Len(), "signals", signalsProduced, "duration", time.Since(stepStart))
	if diag != nil {
		diag.add("signal-detection", time.Since(stepStart), signalsProduced)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Populate snapshot provenance metadata.
	detectorIDs := make([]string, 0, registry.Len())
	for _, reg := range registry.All() {
		detectorIDs = append(detectorIDs, reg.Meta.ID)
	}
	snapshot.SnapshotMeta = models.SnapshotMeta{
		SchemaVersion: models.SnapshotSchemaVersion,
		EngineVersion: engineVersion,
		DetectorCount: registry.Len(),
		Detectors:     detectorIDs,
	}

	// Step 6: Propagate ownership after all signal-producing stages so
	// runtime and coverage-derived signals receive owners consistently.
	progress(3, "Inferring validations")
	stepStart = time.Now()
	if ownerResolver == nil {
		ownerResolver = ownership.NewResolver(root)
	}
	ownership.Propagate(ownerResolver, snapshot)
	if diag != nil {
		diag.add("ownership-resolution", time.Since(stepStart), len(snapshot.Signals))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizeSignalMetadata(snapshot)

	// Attach file-scoped signals to their corresponding test files.
	attachSignalsToTestFiles(snapshot)

	// Step 7: Compute risk surfaces from signals.
	progress(4, "Computing insights")
	stepStart = time.Now()
	snapshot.Risk = scoring.ComputeRisk(snapshot)
	if diag != nil {
		diag.add("risk-scoring", time.Since(stepStart), len(snapshot.Risk))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Step 8: Compute measurement-layer posture.
	stepStart = time.Now()
	measRegistry, measRegErr := measurement.DefaultRegistry()
	if measRegErr != nil {
		return nil, fmt.Errorf("initialize measurement registry: %w", measRegErr)
	}
	measSnap := measRegistry.ComputeSnapshot(snapshot)
	snapshot.Measurements = measSnap.ToModel()
	snapshot.SnapshotMeta.MethodologyFingerprint = methodologyFingerprint(
		snapshot.SnapshotMeta.Detectors,
		measurementDefinitionIDs(measRegistry.All()),
		scoring.RiskModelVersion,
	)
	if diag != nil {
		diag.add("measurement", time.Since(stepStart), len(snapshot.Measurements.Posture))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Step 9: Compute portfolio intelligence.
	stepStart = time.Now()
	portfolioSummary := portfolio.Analyze(snapshot)
	snapshot.Portfolio = portfolioSummary.ToModel()
	compactRiskContributingSignals(snapshot)
	if diag != nil {
		diag.add("portfolio", time.Since(stepStart), len(portfolioSummary.Findings))
	}

	populateSnapshotMetadata(snapshot, opt, hasPolicy)

	// Step 10: Sort all snapshot slices into canonical order for determinism.
	progress(5, "Writing report")
	models.SortSnapshot(snapshot)

	// Step 10b: assign stable FindingIDs to every signal. Runs after
	// the sort so IDs land in canonical order; uses Type + Location
	// (file/symbol/line) so the IDs survive everything except a
	// rename/move of the signal's underlying location. See
	// `internal/identity.BuildFindingID` for the format.
	assignFindingIDs(snapshot)

	if err := models.ValidateSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("invalid snapshot produced by pipeline: %w", err)
	}

	totalDuration := time.Since(pipelineStart)
	logging.L().Debug("pipeline complete", "duration", totalDuration, "signals", len(snapshot.Signals), "risk", len(snapshot.Risk))
	if diag != nil {
		diag.Total = totalDuration
	}

	return &PipelineResult{
		Snapshot:          snapshot,
		Graph:             dg,
		HasPolicy:         hasPolicy,
		DataCompleteness:  deriveDataCompleteness(snapshot, opt, hasPolicy),
		Diagnostics:       diag,
		ArtifactDiscovery: discovery,
		DiscoveryMessages: discoveryMessages,
	}, nil
}

func validatePipelineOptions(root string, opt PipelineOptions) error {
	if root == "" {
		return fmt.Errorf("repository root cannot be empty")
	}
	if opt.SlowTestThresholdMs < 0 {
		return fmt.Errorf("slow test threshold must be >= 0, got %f", opt.SlowTestThresholdMs)
	}
	if label := strings.ToLower(strings.TrimSpace(opt.CoverageRunLabel)); label != "" {
		switch label {
		case "unit", "integration", "e2e":
		default:
			return fmt.Errorf("coverage run label must be one of unit, integration, e2e; got %q", opt.CoverageRunLabel)
		}
	}
	return nil
}

func attachSignalsToTestFiles(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil || len(snapshot.TestFiles) == 0 || len(snapshot.Signals) == 0 {
		return
	}

	byFile := map[string][]models.Signal{}
	for _, s := range snapshot.Signals {
		if s.Location.File == "" {
			continue
		}
		byFile[s.Location.File] = append(byFile[s.Location.File], s)
	}

	for i := range snapshot.TestFiles {
		path := snapshot.TestFiles[i].Path
		if path == "" {
			continue
		}
		signals := byFile[path]
		if len(signals) == 0 {
			snapshot.TestFiles[i].Signals = []models.Signal{}
			continue
		}
		snapshot.TestFiles[i].Signals = append([]models.Signal(nil), signals...)
	}
}

func populateSnapshotMetadata(snapshot *models.TestSuiteSnapshot, opt PipelineOptions, hasPolicy bool) {
	if snapshot == nil {
		return
	}

	available := 0
	unavailable := 0
	errors := 0
	for _, ds := range snapshot.DataSources {
		switch ds.Status {
		case models.DataSourceAvailable:
			available++
		case models.DataSourceUnavailable:
			unavailable++
		case models.DataSourceError:
			errors++
		}
	}

	testFilesWithLinkedUnits := 0
	testFilesWithSignals := 0
	for _, tf := range snapshot.TestFiles {
		if len(tf.LinkedCodeUnits) > 0 {
			testFilesWithLinkedUnits++
		}
		if len(tf.Signals) > 0 {
			testFilesWithSignals++
		}
	}

	snapshot.Metadata = map[string]any{
		"runtimeArtifactsProvided": len(opt.RuntimePaths),
		"coverageInputProvided":    opt.CoveragePath != "",
		"coverageRunLabel":         strings.TrimSpace(opt.CoverageRunLabel),
		"policyConfigLoaded":       hasPolicy,
		"dataSourcesAvailable":     available,
		"dataSourcesUnavailable":   unavailable,
		"dataSourcesError":         errors,
		"testFilesWithLinkedUnits": testFilesWithLinkedUnits,
		"testFilesWithFileSignals": testFilesWithSignals,
		"totalSignals":             len(snapshot.Signals),
	}
}

func normalizeSignalMetadata(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil || len(snapshot.Signals) == 0 {
		return
	}
	for i := range snapshot.Signals {
		s := &snapshot.Signals[i]
		if s.Metadata == nil {
			s.Metadata = map[string]any{}
		}
		if _, ok := s.Metadata["signalType"]; !ok {
			s.Metadata["signalType"] = string(s.Type)
		}
		if _, ok := s.Metadata["category"]; !ok {
			s.Metadata["category"] = string(s.Category)
		}
		if _, ok := s.Metadata["severity"]; !ok {
			s.Metadata["severity"] = string(s.Severity)
		}
		if s.EvidenceSource != "" {
			if _, ok := s.Metadata["evidenceSource"]; !ok {
				s.Metadata["evidenceSource"] = string(s.EvidenceSource)
			}
		}
		if s.Confidence > 0 {
			if _, ok := s.Metadata["confidence"]; !ok {
				s.Metadata["confidence"] = s.Confidence
			}
		}
		if _, ok := s.Metadata["scope"]; !ok {
			switch {
			case s.Location.File != "":
				s.Metadata["scope"] = "file"
			case s.Location.Package != "":
				s.Metadata["scope"] = "package"
			case s.Location.Repository != "":
				s.Metadata["scope"] = "repository"
			default:
				s.Metadata["scope"] = "unknown"
			}
		}
	}
}

func compactRiskContributingSignals(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil || len(snapshot.Risk) == 0 {
		return
	}
	const maxSignalsPerRiskSurface = 25
	for i := range snapshot.Risk {
		rs := &snapshot.Risk[i]
		if len(rs.ContributingSignals) == 0 {
			continue
		}
		seen := map[string]bool{}
		compact := make([]models.Signal, 0, len(rs.ContributingSignals))
		for _, s := range rs.ContributingSignals {
			key := string(s.Type) + "|" + s.Location.File + "|" + s.Location.Repository + "|" + s.Explanation
			if seen[key] {
				continue
			}
			seen[key] = true
			compact = append(compact, models.Signal{
				Type:        s.Type,
				Category:    s.Category,
				Severity:    s.Severity,
				Location:    s.Location,
				Owner:       s.Owner,
				Explanation: s.Explanation,
			})
			if len(compact) >= maxSignalsPerRiskSurface {
				break
			}
		}
		rs.ContributingSignals = compact
	}
}

func policyConfigMap(cfg *policy.Config) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	rules := map[string]any{}
	if cfg.Rules.DisallowSkippedTests != nil {
		rules["disallow_skipped_tests"] = *cfg.Rules.DisallowSkippedTests
	}
	if len(cfg.Rules.DisallowFrameworks) > 0 {
		frameworks := append([]string(nil), cfg.Rules.DisallowFrameworks...)
		sort.Strings(frameworks)
		rules["disallow_frameworks"] = frameworks
	}
	if cfg.Rules.MaxTestRuntimeMs != nil {
		rules["max_test_runtime_ms"] = *cfg.Rules.MaxTestRuntimeMs
	}
	if cfg.Rules.MinimumCoveragePercent != nil {
		rules["minimum_coverage_percent"] = *cfg.Rules.MinimumCoveragePercent
	}
	if cfg.Rules.MaxWeakAssertions != nil {
		rules["max_weak_assertions"] = *cfg.Rules.MaxWeakAssertions
	}
	if cfg.Rules.MaxMockHeavyTests != nil {
		rules["max_mock_heavy_tests"] = *cfg.Rules.MaxMockHeavyTests
	}
	if len(rules) == 0 {
		return nil
	}
	out["rules"] = rules
	return out
}

func measurementDefinitionIDs(defs []measurement.Definition) []string {
	if len(defs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(defs))
	for _, d := range defs {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func methodologyFingerprint(detectors, measurementIDs []string, riskModelVersion string) string {
	parts := []string{
		"schema=" + models.SnapshotSchemaVersion,
		"riskModel=" + riskModelVersion,
		"detectors=" + strings.Join(detectors, ","),
		"measurements=" + strings.Join(measurementIDs, ","),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func deriveDataCompleteness(snapshot *models.TestSuiteSnapshot, opt PipelineOptions, hasPolicy bool) DataCompleteness {
	dc := DataCompleteness{
		SourceAvailable:   snapshot != nil,
		CoverageProvided:  strings.TrimSpace(opt.CoveragePath) != "",
		RuntimeProvided:   len(opt.RuntimePaths) > 0,
		PolicyAvailable:   hasPolicy,
		CoverageAvailable: false,
		RuntimeAvailable:  false,
	}
	if snapshot == nil {
		return dc
	}
	if len(snapshot.TestFiles) == 0 && len(snapshot.CodeUnits) == 0 {
		dc.SourceAvailable = false
	}
	for _, ds := range snapshot.DataSources {
		switch ds.Name {
		case "coverage":
			dc.CoverageAvailable = ds.Status == models.DataSourceAvailable
		case "runtime":
			dc.RuntimeAvailable = ds.Status == models.DataSourceAvailable
		case "policy":
			dc.PolicyAvailable = ds.Status == models.DataSourceAvailable
		}
	}
	return dc
}

func ingestRuntimeArtifacts(ctx context.Context, paths []string) ([]runtime.TestResult, error) {
	if len(paths) <= 1 {
		// Fast path: single artifact, no parallelism overhead.
		if len(paths) == 0 {
			return nil, nil
		}
		result, err := runtime.Ingest(paths[0])
		if err != nil {
			return nil, err
		}
		return result.Results, nil
	}

	// Parallel ingestion: each artifact is independent.
	// Results collected per-index to preserve deterministic ordering.
	type indexedResult struct {
		results []runtime.TestResult
		err     error
	}
	perFile := make([]indexedResult, len(paths))

	workers := goruntime.GOMAXPROCS(0)
	if workers > len(paths) {
		workers = len(paths)
	}
	indexCh := make(chan int, len(paths))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				if ctx.Err() != nil {
					return
				}
				r, err := runtime.Ingest(paths[idx])
				if err != nil {
					perFile[idx] = indexedResult{err: err}
				} else {
					perFile[idx] = indexedResult{results: r.Results}
				}
			}
		}()
	}
	for i := range paths {
		indexCh <- i
	}
	close(indexCh)
	wg.Wait()

	// Merge in deterministic order.
	var allResults []runtime.TestResult
	for _, ir := range perFile {
		if ir.err != nil {
			return nil, ir.err
		}
		allResults = append(allResults, ir.results...)
	}
	return allResults, nil
}

func applyRuntimeResults(snapshot *models.TestSuiteSnapshot, allResults []runtime.TestResult, slowThreshold float64) {
	if snapshot == nil || len(allResults) == 0 {
		return
	}

	// Resolve stable test IDs by joining runtime results to extracted test cases.
	runtime.ResolveTestIDs(allResults, snapshot.TestCases)

	// Apply runtime stats to matching test files.
	updates := make([]runtime.TestFileUpdate, len(snapshot.TestFiles))
	for i, tf := range snapshot.TestFiles {
		updates[i] = runtime.TestFileUpdate{Path: tf.Path}
	}
	runtime.ApplyToTestFiles(allResults, updates)
	for i, u := range updates {
		if u.AvgRuntimeMs > 0 {
			snapshot.TestFiles[i].RuntimeStats = &models.RuntimeStats{
				AvgRuntimeMs:    u.AvgRuntimeMs,
				P95RuntimeMs:    u.P95RuntimeMs,
				PassRate:        u.PassRate,
				RetryRate:       u.RetryRate,
				RuntimeVariance: u.RuntimeVariance,
			}
		}
	}

	// Health detector signal emission is now handled by the registry via
	// RuntimeDetectorAdapter. The runtime results are passed to the registry
	// Config so adapted health detectors run during the normal signal phase.
}

func ingestCoverageArtifacts(ctx context.Context, coveragePath string, runLabel string) ([]coverage.CoverageArtifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(coveragePath)
	if err != nil {
		return nil, fmt.Errorf("coverage path %s: %w", coveragePath, err)
	}

	var artifacts []coverage.CoverageArtifact
	if info.IsDir() {
		artifacts, err = coverage.IngestDirectory(coveragePath, runLabel)
		if err != nil {
			var warn *coverage.IngestWarning
			if errors.As(err, &warn) {
				logging.L().Warn("partial coverage ingest", "error", warn, "path", coveragePath)
				err = nil
			}
		}
	} else {
		var art *coverage.CoverageArtifact
		art, err = coverage.IngestFile(coveragePath, runLabel)
		if art != nil {
			artifacts = append(artifacts, *art)
		}
	}
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

func ingestGauntletArtifacts(paths []string) ([]*gauntlet.Artifact, error) {
	var artifacts []*gauntlet.Artifact
	for _, p := range paths {
		art, err := gauntlet.Ingest(p)
		if err != nil {
			return nil, fmt.Errorf("gauntlet artifact %s: %w", p, err)
		}
		artifacts = append(artifacts, art)
	}
	return artifacts, nil
}

// loadBaselineSnapshot reads a previous snapshot from disk and returns
// it as a fully-decoded TestSuiteSnapshot. The result is attached to
// snap.Baseline by the pipeline so regression-aware detectors can
// compare current vs baseline state.
//
// Returns an error rather than nil when the file is missing or
// malformed — the user explicitly asked for the comparison via
// --baseline, so a silent fallback would mask intent.
func loadBaselineSnapshot(path string) (*models.TestSuiteSnapshot, error) {
	// 0.2.0 final-polish: stream-decode via json.NewDecoder rather
	// than loading the whole file into memory. A 100MB historical
	// snapshot is tractable; multi-repo / multi-month historical
	// snapshots can run several hundred MB and used to spike RSS by
	// the same amount under os.ReadFile + json.Unmarshal.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	defer f.Close()
	// Empty-file check via stat to avoid pulling the file content
	// into memory just to count length.
	if fi, statErr := f.Stat(); statErr == nil && fi.Size() == 0 {
		return nil, fmt.Errorf("baseline file is empty")
	}
	var snap models.TestSuiteSnapshot
	dec := json.NewDecoder(f)
	if err := dec.Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// A null JSON value decodes to a zero TestSuiteSnapshot — non-nil
	// but empty. Detectors that check `snap.Baseline == nil` (cost,
	// retrieval) would silently disable themselves with no diagnostic.
	// Reject explicitly.
	if snap.SnapshotMeta.SchemaVersion == "" && len(snap.Signals) == 0 && len(snap.TestFiles) == 0 && len(snap.EvalRuns) == 0 {
		return nil, fmt.Errorf("baseline appears empty (no schemaVersion, signals, testFiles, or evalRuns)")
	}
	// Reject snapshots from a future major version we don't understand.
	// Pre-0.2.x this check was missing, so a 2.0.0 baseline would
	// silently decode into the v1 struct, losing fields.
	if err := models.ValidateSchemaVersion(snap.SnapshotMeta.SchemaVersion); err != nil {
		return nil, fmt.Errorf("baseline schema: %w", err)
	}
	// Migrate older snapshots forward in place (idempotent for current).
	// Pre-0.2.x this call was missing, so 0.1.x baselines decoded
	// raw and were silently compared as-if same-schema. Migration runs
	// the same code path as cmd_compare.go uses; returned notes are
	// discarded here (the warn is structural, not actionable for the
	// regression detectors).
	_ = models.MigrateSnapshotInPlace(&snap)
	return &snap, nil
}

// relativeArtifactPath converts a CLI-provided path into a repo-
// relative form when possible. 0.2.0 final-polish: pre-fix the
// SourcePath stamped into EvalRunEnvelope was whatever the user
// passed on the CLI — `--promptfoo-results /Users/alice/proj/...`
// produced absolute paths in SARIF output, leaking developer home
// directories. Now `filepath.Rel(root, p)` is attempted; on failure
// (different volume, error) we fall back to the original path.
//
// Result is always slash-separated. `filepath.Rel` returns native
// separators (backslash on Windows); snapshot JSON, calibration
// labels, and SARIF all expect forward slashes, so we normalize to
// `/` as the final step. Without this, Windows builds produced
// backslash-separated SourcePaths that mismatched forward-slash
// labels in the calibration corpus.
func relativeArtifactPath(root, p string) string {
	if root == "" || p == "" {
		return filepath.ToSlash(p)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(p)
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	rel, err := filepath.Rel(absRoot, absP)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(rel)
}

// ingestPromptfooArtifacts parses each Promptfoo `--output` JSON file
// and returns the resulting envelope per file. Errors abort early so a
// malformed file fails the run loudly.
//
// The actual parsing lives in internal/airun.ParsePromptfooJSON; this
// helper is the thin pipeline-side wrapper that translates each
// EvalRunResult into the snapshot envelope.
func ingestPromptfooArtifacts(root string, paths []string) ([]models.EvalRunEnvelope, error) {
	out := make([]models.EvalRunEnvelope, 0, len(paths))
	for _, p := range paths {
		result, err := airun.LoadPromptfooFile(p)
		if err != nil {
			return nil, fmt.Errorf("promptfoo artifact %s: %w", p, err)
		}
		env, err := result.ToEnvelope(relativeArtifactPath(root, p))
		if err != nil {
			return nil, fmt.Errorf("promptfoo envelope for %s: %w", p, err)
		}
		out = append(out, env)
	}
	return out, nil
}

// ingestDeepEvalArtifacts mirrors ingestPromptfooArtifacts for the
// DeepEval adapter. Both adapters target the same EvalRunEnvelope
// shape; the runtime-aware AI detectors don't care which framework
// produced the data.
func ingestDeepEvalArtifacts(root string, paths []string) ([]models.EvalRunEnvelope, error) {
	out := make([]models.EvalRunEnvelope, 0, len(paths))
	for _, p := range paths {
		result, err := airun.LoadDeepEvalFile(p)
		if err != nil {
			return nil, fmt.Errorf("deepeval artifact %s: %w", p, err)
		}
		env, err := result.ToEnvelope(relativeArtifactPath(root, p))
		if err != nil {
			return nil, fmt.Errorf("deepeval envelope for %s: %w", p, err)
		}
		out = append(out, env)
	}
	return out, nil
}

// ingestRagasArtifacts mirrors the Promptfoo / DeepEval helpers for
// the Ragas adapter. Ragas's named-score axes (faithfulness,
// context_relevance, answer_relevancy) feed aiRetrievalRegression
// directly via the same EvalRunEnvelope plumbing.
func ingestRagasArtifacts(root string, paths []string) ([]models.EvalRunEnvelope, error) {
	out := make([]models.EvalRunEnvelope, 0, len(paths))
	for _, p := range paths {
		result, err := airun.LoadRagasFile(p)
		if err != nil {
			return nil, fmt.Errorf("ragas artifact %s: %w", p, err)
		}
		env, err := result.ToEnvelope(relativeArtifactPath(root, p))
		if err != nil {
			return nil, fmt.Errorf("ragas envelope for %s: %w", p, err)
		}
		out = append(out, env)
	}
	return out, nil
}

func applyCoverageArtifacts(snapshot *models.TestSuiteSnapshot, artifacts []coverage.CoverageArtifact) {
	if snapshot == nil {
		return
	}
	if len(artifacts) == 0 {
		return
	}

	// Merge all artifacts.
	merged := coverage.Merge(artifacts)

	// Attribute to code units.
	unitCov := coverage.AttributeToCodeUnits(merged, snapshot.CodeUnits)

	// Compute type-based coverage.
	typeCov := coverage.ComputeByType(artifacts, snapshot.CodeUnits)

	// Build repo summary.
	repoSummary := coverage.BuildRepoSummary(typeCov, snapshot.CodeUnits)

	// Populate snapshot coverage summary.
	snapshot.CoverageSummary = &models.CoverageSummary{
		TotalCodeUnits:       repoSummary.TotalCodeUnits,
		CoveredByUnitTests:   repoSummary.CoveredByUnitTests,
		CoveredByIntegration: repoSummary.CoveredByIntegration,
		CoveredByE2E:         repoSummary.CoveredByE2E,
		CoveredOnlyByE2E:     repoSummary.CoveredOnlyByE2E,
		UncoveredExported:    repoSummary.UncoveredExported,
		Uncovered:            repoSummary.TotalCodeUnits - repoSummary.CoveredByUnitTests,
	}

	// Compute overall line/branch coverage from merged data.
	var totalLines, coveredLines, totalBranches, coveredBranches int
	for _, rec := range merged.ByFile {
		totalLines += rec.LineTotalCount
		coveredLines += rec.LineCoveredCount
		totalBranches += rec.BranchTotalCount
		coveredBranches += rec.BranchCoveredCount
	}
	if totalLines > 0 {
		snapshot.CoverageSummary.LineCoveragePct = float64(coveredLines) / float64(totalLines) * 100
	}
	if totalBranches > 0 {
		snapshot.CoverageSummary.BranchCoveragePct = float64(coveredBranches) / float64(totalBranches) * 100
	}

	// Derive insights.
	covInsights := coverage.DeriveInsights(typeCov, snapshot.CodeUnits)
	unitInsights := coverage.DeriveUnitInsights(unitCov)
	for _, ci := range append(covInsights, unitInsights...) {
		snapshot.CoverageInsights = append(snapshot.CoverageInsights, models.CoverageInsight{
			Type:            ci.Type,
			Severity:        ci.Severity,
			Description:     ci.Description,
			Path:            ci.Path,
			UnitID:          ci.UnitID,
			SuggestedAction: ci.SuggestedAction,
		})
	}
}
