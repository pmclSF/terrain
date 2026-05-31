package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pmclSF/terrain/internal/analysis"
	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/budget"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/promptflow"
	"github.com/pmclSF/terrain/internal/reporting"
	"github.com/pmclSF/terrain/internal/sarif"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

func runInit(root string, jsonOutput bool) error {
	result, err := engine.RunInit(root)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	sep := strings.Repeat("─", 60)

	fmt.Println("Terrain Init")
	fmt.Println(sep)
	fmt.Println()

	// Section 1: What we detected.
	fmt.Printf("Repository: %s\n", filepath.Base(result.Root))
	fmt.Printf("Test files: %d\n", result.TestFileCount)
	fmt.Println()

	if len(result.Frameworks) > 0 {
		fmt.Println("Frameworks detected:")
		for _, fw := range result.Frameworks {
			fmt.Printf("  %-20s %s (via %s, confidence %.0f%%)\n",
				fw.Name, fw.Language, fw.Source, fw.Confidence*100)
		}
		fmt.Println()
	}

	// Section 2: Artifacts.
	if result.Artifacts != nil {
		if result.Artifacts.CoveragePath != "" {
			fmt.Printf("Coverage:  %s (%s)\n", relativeToRoot(result.Artifacts.CoveragePath, result.Root), result.Artifacts.CoverageFormat)
		} else {
			fmt.Println("Coverage:  not found")
		}
		if len(result.Artifacts.RuntimePaths) > 0 {
			for i, p := range result.Artifacts.RuntimePaths {
				rtFormat := ""
				if i < len(result.Artifacts.RuntimeFormats) {
					rtFormat = result.Artifacts.RuntimeFormats[i]
				}
				fmt.Printf("Runtime:   %s (%s)\n", relativeToRoot(p, result.Root), rtFormat)
			}
		} else {
			fmt.Println("Runtime:   not found")
		}
		fmt.Println()
	}

	// Section 3: What we generated.
	fmt.Println("Configuration")
	fmt.Println(sep)
	if result.PolicyPath != "" {
		fmt.Printf("  Created: %s\n", relativeToRoot(result.PolicyPath, result.Root))
		fmt.Println("           Governance rules (all commented out — edit to enable)")
	} else if result.HasPolicyFile {
		fmt.Println("  Exists:  .terrain/policy.yaml")
	}

	if result.ConfigPath != "" {
		fmt.Printf("  Created: %s\n", relativeToRoot(result.ConfigPath, result.Root))
		fmt.Println("           Manual coverage, scenarios, and CI metadata (template)")
	} else if result.HasTerrainYAML {
		fmt.Println("  Exists:  terrain.yaml")
	}
	fmt.Println()

	// Section 4: What to do next.
	fmt.Println("Next steps")
	fmt.Println(sep)
	fmt.Println("  1. Run analysis:")
	fmt.Printf("     terrain analyze --root %q\n", root)
	fmt.Println()

	step := 2
	if result.Artifacts == nil || result.Artifacts.CoveragePath == "" {
		fmt.Printf("  %d. Generate coverage data to unlock coverage signals:\n", step)
		fmt.Println("     Example: npx jest --coverage")
		fmt.Println("     Example: go test -coverprofile=coverage.out ./...")
		fmt.Println("     Example: pytest --cov --cov-report=lcov")
		step++
		fmt.Println()
	}
	if result.Artifacts == nil || len(result.Artifacts.RuntimePaths) == 0 {
		fmt.Printf("  %d. Generate test result artifacts to unlock health signals:\n", step)
		fmt.Println("     Example: npx jest --json --outputFile=jest-results.json")
		fmt.Println("     Example: pytest --junitxml=junit.xml")
		step++
		fmt.Println()
	}
	if result.PolicyPath != "" {
		fmt.Printf("  %d. Edit .terrain/policy.yaml — three starter policies live\n", step)
		fmt.Println("     under docs/policy/examples/{minimal,balanced,strict}.yaml")
		step++
		fmt.Println()
	}

	// CI integration pointer — . Always shown so adopters
	// see the whole ladder from `terrain init` onwards. The trust-
	// ladder doc explains the four-rung adoption path; the example
	// workflow is the one canonical CI config.
	fmt.Printf("  %d. Wire Terrain into CI (warn-only by default):\n", step)
	fmt.Println("     Copy docs/examples/gate/github-action.yml to .github/workflows/")
	fmt.Println("     Start in warn-only mode; flip on blocking gates once your")
	fmt.Println("     team has cleared the existing findings.")
	fmt.Println()

	return nil
}

func relativeToRoot(path, root string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// analyzeRunOpts collects every input runAnalyze takes. Replaces a
// seventeen-positional-argument signature with one struct so future
// flag additions stop expanding the call site. // recovery (PR #140) introduced the struct; gate + timeout fields
// were already on the previous positional signature and are
// preserved here.
type analyzeRunOpts struct {
	Root             string
	JSONOutput       bool
	Format           string
	Verbose          bool
	WriteSnapshot    bool
	CoveragePath     string
	CoverageRunLabel string
	RuntimePaths     string
	GauntletPaths    string
	PromptfooPaths   string
	DeepEvalPaths    string
	RagasPaths       string
	BaselinePath     string
	SlowThreshold    float64
	RedactPaths      bool
	Gate             severityGate
	Timeout          time.Duration
	SuppressionsPath string
	NewFindingsOnly  bool
	EnablePreview    bool
	Diag             bool
	// BaseRef, when set, enables the prompt-template/schema drift
	// detector (aiPromptSchemaDrift). The detector compares schemas
	// at HEAD against schemas at this ref via `git show <ref>:<path>`.
	// Empty disables the detector.
	BaseRef string
}

func runAnalyze(o analyzeRunOpts) error {
	root := o.Root
	jsonOutput := o.JSONOutput
	format := o.Format
	verbose := o.Verbose
	writeSnap := o.WriteSnapshot
	coveragePath := o.CoveragePath
	coverageRunLabel := o.CoverageRunLabel
	runtimePaths := o.RuntimePaths
	gauntletPaths := o.GauntletPaths
	promptfooPaths := o.PromptfooPaths
	deepevalPaths := o.DeepEvalPaths
	ragasPaths := o.RagasPaths
	baselinePath := o.BaselinePath
	slowThreshold := o.SlowThreshold
	redactPaths := o.RedactPaths
	gate := o.Gate
	timeout := o.Timeout

	// Load terrain.yaml (optional) and register adopter-supplied
	// custom AI markers with the analysis package. Patterns extend
	// the AI-context gate so private LLM SDKs corroborate detection.
	// Missing or invalid config is non-fatal — gate falls back to
	// the canonical marker list.
	if cfg, err := terrainconfig.Load(filepath.Join(root, "terrain.yaml")); err == nil && cfg != nil && cfg.AI != nil && len(cfg.AI.AIMarkers) > 0 {
		analysis.SetCustomAIMarkers(cfg.AI.AIMarkers)
	}

	parsedRuntime := parseRuntimePaths(runtimePaths)
	parsedGauntlet := parseRuntimePaths(gauntletPaths)        // same comma-split logic
	parsedPromptfoo := parseRuntimePaths(promptfooPaths)      // same comma-split logic
	parsedDeepEval := parseRuntimePaths(deepevalPaths)        // same comma-split logic
	parsedRagas := parseRuntimePaths(ragasPaths)              // same comma-split logic
	if err := validateCommandInputs(root, coveragePath, parsedRuntime, parsedGauntlet); err != nil {
		return err
	}
	if err := validateExistingPaths("--promptfoo-results", parsedPromptfoo); err != nil {
		return err
	}
	if err := validateExistingPaths("--deepeval-results", parsedDeepEval); err != nil {
		return err
	}
	if err := validateExistingPaths("--ragas-results", parsedRagas); err != nil {
		return err
	}
	if baselinePath != "" {
		if err := validateExistingPaths("--baseline", []string{baselinePath}); err != nil {
			return err
		}
	}
	if o.SuppressionsPath != "" {
		if err := validateExistingPaths("--suppressions", []string{o.SuppressionsPath}); err != nil {
			return err
		}
	}
	var sarifOutput, annotationOutput bool
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "":
	case "json":
		jsonOutput = true
	case "text":
		jsonOutput = false
	case "sarif":
		sarifOutput = true
		jsonOutput = true // suppress progress output
	case "annotation":
		annotationOutput = true
		jsonOutput = true // suppress progress output
	case "html":
		jsonOutput = true // suppress progress output
	default:
		return fmt.Errorf("invalid --format %q (valid: json, text, sarif, annotation, html)", format)
	}

	opt := analysisPipelineOptions(coveragePath, coverageRunLabel, parsedRuntime, slowThreshold)
	opt.GauntletPaths = parsedGauntlet
	opt.PromptfooPaths = parsedPromptfoo
	opt.DeepEvalPaths = parsedDeepEval
	opt.RagasPaths = parsedRagas
	opt.BaselineSnapshotPath = baselinePath
	opt.SuppressionsPath = o.SuppressionsPath
	opt.NewFindingsOnly = o.NewFindingsOnly
	opt.EnablePreviewRules = o.EnablePreview
	opt.CollectDiagnostics = o.Diag
	opt.OnProgress = newProgressFunc(jsonOutput)
	// Honour Ctrl-C and the optional --timeout: without this, analyze
	// exits abruptly on SIGINT with no cleanup, and unbounded
	// monorepo scans can block CI indefinitely.
	// runPipelineWithSignalsAndTimeout wraps RunPipelineContext with a
	// SIGINT-aware context plus an optional deadline so in-flight
	// detectors check ctx.Err and unwind cooperatively.
	result, err := runPipelineWithSignalsAndTimeout(root, opt, timeout)
	if err != nil {
		// Render partial diagnostics if collected — useful for
		// debugging timeouts (shows which steps completed before
		// the deadline). Must happen before the early-return.
		if o.Diag && result != nil && result.Diagnostics != nil {
			result.Diagnostics.Render(os.Stderr)
		}
		// Designed remediation when analysis fails. Distinguishes
		// context cancellation (timeout / Ctrl-C) from other
		// failure modes so adopters see the right next step.
		if !jsonOutput {
			analyzeFailureRemediation(err, root, timeout)
		}
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Report auto-discovered artifacts via structured logging.
	for _, msg := range result.DiscoveryMessages {
		logging.L().Info(msg)
	}

	// Pipeline diagnostics — print to stderr so they don't pollute
	// JSON / SARIF stdout but are visible during interactive runs.
	if o.Diag && result.Diagnostics != nil {
		result.Diagnostics.Render(os.Stderr)
	}

	if err := appendPromptSchemaDriftSignals(result.Snapshot, root, o.BaseRef); err != nil {
		return err
	}

	// Per-rule findings budget. terrain.yaml `rules.<key>.max_findings`
	// caps how many findings a single rule may emit, keeping the
	// highest-priority subset. Heuristic-precision detectors that
	// otherwise fire hundreds of times per run (untestedExport on Java
	// monorepos, weakAssertion on data-science codebases) are noise
	// past a small cap.
	applyFindingsBudget(result.Snapshot, root, jsonOutput)

	// Build discovered artifacts list for the report.
	var discovered []analyze.DiscoveredArtifact
	if d := result.ArtifactDiscovery; d != nil {
		if d.CoverageAutoDetected && d.CoveragePath != "" {
			discovered = append(discovered, analyze.DiscoveredArtifact{
				Kind: "coverage", Path: engine.RelativePath(d.CoveragePath), Format: d.CoverageFormat,
			})
		}
		if d.RuntimeAutoDetected {
			for i, p := range d.RuntimePaths {
				rtFormat := ""
				if i < len(d.RuntimeFormats) {
					rtFormat = d.RuntimeFormats[i]
				}
				discovered = append(discovered, analyze.DiscoveredArtifact{
					Kind: "runtime", Path: engine.RelativePath(p), Format: rtFormat,
				})
			}
		}
	}

	// Build the structured analyze report (includes depgraph analysis).
	report := analyze.Build(&analyze.BuildInput{
		Snapshot:            result.Snapshot,
		HasPolicy:           result.HasPolicy,
		DiscoveredArtifacts: discovered,
	})

	// Compute the gate decision BEFORE rendering so it applies to every
	// output format (json, sarif, annotation, html, text). Pre-fix, the
	// gate check was at the bottom and the json/sarif/annotation
	// branches early-returned before reaching it — `terrain analyze
	// --json --fail-on=medium` silently exited 0 even with matching
	// findings. The "JSON stdout purity" property the launch-readiness
	// review asked for requires that the renderer completes (stdout
	// stays a valid JSON document) AND the gate decision returns via
	// the error channel (so main.go writes the gate message to stderr,
	// not stdout).
	// Gate decisions use the observability-tier-excluding summary so
	// findings from detectors that explicitly ship at observability tier
	// stay informational and don't block CI.
	gateBlocked, gateSummary := severityGateBlocked(gate, report.GateRelevantSummary)
	gateErr := func() error {
		if gateBlocked {
			return fmt.Errorf("%w: --fail-on=%s matched %s", errSeverityGateBlocked, gate, gateSummary)
		}
		return nil
	}

	if sarifOutput {
		sarifLog := sarif.FromAnalyzeReportWithOptions(report, version, sarif.Options{
			RedactPaths: redactPaths,
			RepoRoot:    root,
		})
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(sarifLog); err != nil {
			return err
		}
		return gateErr()
	}

	if annotationOutput {
		reporting.RenderGitHubAnnotations(os.Stdout, report)
		return gateErr()
	}

	// `--write-snapshot` runs first so it persists regardless of the
	// output format. Earlier revisions placed the persist call after
	// the rendering switch, so `--write-snapshot --json` returned from
	// the JSON branch before the snapshot was written — the canonical
	// CI shape (capture JSON to stdout, save snapshot to disk) silently
	// dropped the snapshot.
	if writeSnap {
		if err := persistSnapshot(result.Snapshot, root); err != nil {
			return err
		}
	}

	if strings.EqualFold(strings.TrimSpace(format), "html") {
		if err := reporting.RenderAnalyzeHTML(os.Stdout, report); err != nil {
			return err
		}
		return gateErr()
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		return gateErr()
	}

	if verbose {
		reporting.RenderAnalyzeReport(os.Stdout, result.Snapshot, reporting.AnalyzeReportOptions{
			Verbose: true,
		})
	} else {
		reporting.RenderAnalyzeReportV2(os.Stdout, report)
	}

	// Show hints for missing artifacts after the report.
	hints := engine.MissingArtifactHints(&opt, result.ArtifactDiscovery, result.Snapshot.Repository.Languages)
	if len(hints) > 0 {
		fmt.Println()
		fmt.Println("Unlock more:")
		for _, hint := range hints {
			fmt.Printf("  %s\n", hint)
		}
	}

	// --fail-on gate: text-mode renderer falls through to the same
	// gateErr() the other branches use, so the gate decision applies
	// uniformly across every output format.
	return gateErr()
}

// appendPromptSchemaDriftSignals runs the prompt-template / schema
// drift detector against the working tree at root, with the schemas
// at baseRef as the comparison point. Findings are appended to
// snap.Signals so every downstream surface (JSON, text, PR-comment,
// SARIF) consumes them through the same path.
//
// No-op when baseRef is empty. Returns an error (rather than logging
// a warning and continuing) when baseRef is invalid — silent failures
// look like "all clean" runs and erode trust in the gate.
func appendPromptSchemaDriftSignals(snap *models.TestSuiteSnapshot, root, baseRef string) error {
	if baseRef == "" || snap == nil {
		return nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()
	after, before, err := promptflow.DiscoverFromGit(ctx, root, baseRef)
	if err != nil {
		return fmt.Errorf("aiPromptSchemaDrift: %w", err)
	}
	findings, err := promptflow.Analyze(after, before)
	if err != nil {
		return fmt.Errorf("aiPromptSchemaDrift: %w", err)
	}
	if len(findings) > 0 {
		snap.Signals = append(snap.Signals, promptflow.ToSignals(findings)...)
	}
	return nil
}

// applyFindingsBudget caps each rule's findings at terrain.yaml's
// `max_findings`. Missing terrain.yaml is fine (most adopters don't
// have one). Pruned counts surface as a one-line stderr notice when
// rendering to humans; JSON/SARIF stay quiet so machine consumers
// see the same shape as before.
func applyFindingsBudget(snap *models.TestSuiteSnapshot, root string, jsonOutput bool) {
	cfg, err := terrainconfig.Load(filepath.Join(root, "terrain.yaml"))
	if err != nil || cfg == nil || len(cfg.Rules) == 0 {
		return
	}
	budgets := map[string]int{}
	for key, spec := range cfg.Rules {
		if spec.Block == nil || spec.Block.MaxFindings <= 0 {
			continue
		}
		// terrain.yaml uses `category/name`; engine RuleIDs are
		// `terrain/category/name`. Normalize to the engine form.
		budgets["terrain/"+key] = spec.Block.MaxFindings
	}
	if len(budgets) == 0 {
		return
	}
	pruned := budget.Apply(snap, budgets)
	if jsonOutput || len(pruned) == 0 {
		return
	}
	for rule, n := range pruned {
		fmt.Fprintf(os.Stderr,
			"budget: %s exceeded max_findings — %d additional finding(s) suppressed\n",
			rule, n)
	}
}

// runPolicyCheck evaluates the repository against its local policy.
//
// Exit codes:
//   - 0: no policy file found, or policy exists with no violations
//   - 1: policy file malformed or evaluation/runtime error
//   - 2: policy violations found
func runPolicyCheck(root string, jsonOutput bool, coveragePath, coverageRunLabel string, runtimePaths string, slowThreshold float64) int {
	parsedRuntime := parseRuntimePaths(runtimePaths)
	if err := validateCommandInputs(root, coveragePath, parsedRuntime, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	// Load policy
	policyResult, err := policy.Load(root)
	if err != nil {
		// Surface a designed remediation pointer instead of
		// dumping the bare yaml error. Adopters seeing
		// "yaml: line 5: did not find expected key" don't know
		// that's policy.yaml's fault.
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Policy file failed to load. Common causes:")
		fmt.Fprintln(os.Stderr, "  - YAML indentation: rules must nest under `rules:` (two-space indent)")
		fmt.Fprintln(os.Stderr, "  - Misspelled rule key: see docs/user-guides/writing-a-policy.md for the canonical names")
		fmt.Fprintln(os.Stderr, "  - Type mismatch: thresholds are numbers, booleans are true/false (no quotes)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To regenerate from a known-good template:")
		fmt.Fprintln(os.Stderr, "  cp docs/policy/examples/balanced.yaml .terrain/policy.yaml")
		return exitError
	}

	if !policyResult.Found {
		es := reporting.EmptyStateFor(reporting.EmptyNoPolicyFile)
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(map[string]any{
				"policyFile": nil,
				"pass":       true,
				"violations": []any{},
				"empty":      true,
				"emptyKind":  "no-policy-file",
				"header":     es.Header,
				"nextMove":   es.NextMove,
				"message":    es.Header + " " + es.NextMove,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to render policy output: %v\n", err)
				return exitError
			}
		} else {
			fmt.Println("Terrain Policy Check")
			fmt.Println()
			reporting.RenderEmptyState(os.Stdout, reporting.EmptyNoPolicyFile)
		}
		return exitOK
	}

	opt := analysisPipelineOptions(coveragePath, coverageRunLabel, parsedRuntime, slowThreshold)
	opt.OnProgress = newProgressFunc(jsonOutput)

	// Reuse the main analysis pipeline so policy evaluation can use runtime and
	// coverage artifacts when provided.
	result, err := runPipelineWithSignals(root, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: analysis failed: %v\n", err)
		return exitError
	}

	// Evaluate policy.
	govResult := governance.Evaluate(result.Snapshot, policyResult.Config)

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		violations := govResult.Violations
		if violations == nil {
			violations = []models.Signal{}
		}
		if err := enc.Encode(map[string]any{
			"policyFile": policyResult.Path,
			"pass":       govResult.Pass,
			"violations": violations,
			"message":    policyStatusMessage(govResult.Pass),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to render policy output: %v\n", err)
			return exitError
		}
	} else {
		reporting.RenderPolicyReport(os.Stdout, policyResult.Path, govResult)
	}

	if !govResult.Pass {
		return exitPolicyViolation
	}
	return exitOK
}

func parseRuntimePaths(runtimePaths string) []string {
	var paths []string
	if runtimePaths == "" {
		return paths
	}
	for _, p := range strings.Split(runtimePaths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func validateCommandInputs(root, coveragePath string, runtimePaths, gauntletPaths []string) error {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("invalid --root %q: %w", root, err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("invalid --root %q: expected a directory", root)
	}

	if coveragePath != "" {
		if _, err := os.Stat(coveragePath); err != nil {
			return fmt.Errorf("invalid --coverage %q: %w", coveragePath, err)
		}
	}

	for _, p := range runtimePaths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("invalid --runtime path %q: %w", p, err)
		}
	}

	for _, p := range gauntletPaths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("invalid --gauntlet path %q: %w", p, err)
		}
	}
	return nil
}

// validateExistingPaths is a small helper that mirrors the existing
// per-flag validation but works for any flag's path list. Used by the
// new --promptfoo-results flag and any future eval-adapter flags.
func validateExistingPaths(flagName string, paths []string) error {
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("invalid %s path %q: %w", flagName, p, err)
		}
	}
	return nil
}

func policyStatusMessage(pass bool) string {
	if pass {
		return "Policy checks passed."
	}
	return "Policy violations detected."
}

// analyzeFailureRemediation prints a designed remediation block to
// stderr when the analyze pipeline fails. Distinguishes the three
// most common failure modes so adopters see a relevant next step:
//
//   - context cancelled (--timeout fired or Ctrl-C)
//   - filesystem / parse error
//   - everything else (generic remediation)
func analyzeFailureRemediation(err error, root string, timeout time.Duration) {
	fmt.Fprintln(os.Stderr)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		if timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "Analysis exceeded the --timeout=%s budget. Common next steps:\n", timeout)
			fmt.Fprintln(os.Stderr, "  - Increase --timeout (large monorepos may need 5–10 minutes)")
			fmt.Fprintln(os.Stderr, "  - Run on a subdirectory: `terrain analyze <path>` to scope down")
			fmt.Fprintln(os.Stderr, "  - Use `--verbose` to see per-stage timing and identify the slow detector")
		} else {
			fmt.Fprintln(os.Stderr, "Analysis was canceled. Re-run when ready.")
		}
		return
	}
	fmt.Fprintln(os.Stderr, "Common causes of analysis failure:")
	fmt.Fprintln(os.Stderr, "  - --root path is not a git repository (some detectors need git history)")
	fmt.Fprintf(os.Stderr, "  - Permission errors walking %s — check file permissions\n", root)
	fmt.Fprintln(os.Stderr, "  - Malformed coverage / runtime artifact at the path passed via --coverage / --runtime")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Run with `--verbose` for per-stage timing or `--json` for a machine-readable error report.")
}
