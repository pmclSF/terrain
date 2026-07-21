package main

import (
	"bytes"
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

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/analysis"
	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/atomicfile"
	"github.com/pmclSF/terrain/internal/budget"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/promptcontract"
	"github.com/pmclSF/terrain/internal/promptflow"
	"github.com/pmclSF/terrain/internal/remediate"
	"github.com/pmclSF/terrain/internal/reporting"
	"github.com/pmclSF/terrain/internal/sarif"
	"github.com/pmclSF/terrain/internal/signals"
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
	if result.PolicyExamplePath != "" {
		fmt.Printf("  Written: %s\n", relativeToRoot(result.PolicyExamplePath, result.Root))
		fmt.Println("           Annotated reference covering every supported policy knob")
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
// flag additions stop expanding the call site.
type analyzeRunOpts struct {
	Root                   string
	JSONOutput             bool
	Format                 string
	Verbose                bool
	WriteSnapshot          bool
	CoveragePath           string
	CoverageRunLabel       string
	RuntimePaths           string
	GauntletPaths          string
	PromptfooPaths         string
	DeepEvalPaths          string
	RagasPaths             string
	GreatExpectationsPaths string
	BaselinePath           string
	SlowThreshold          float64
	RedactPaths            bool
	Gate                   severityGate
	Timeout                time.Duration
	SuppressionsPath       string
	NewFindingsOnly        bool
	EnablePreview          bool
	Diag                   bool
	// TrustFloor forces the remediation-validity gate ON (explicit
	// `--trust-floor`). It is ON by default in 0.4.0; NoTrustFloor is the
	// opt-out. When both are false the default (on) applies.
	TrustFloor bool
	// NoTrustFloor forces the remediation-validity gate OFF (`--no-trust-floor`),
	// restoring severity-only gating. Takes precedence over the default and config.
	NoTrustFloor bool
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
	greatExpectationsPaths := o.GreatExpectationsPaths
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
	cfg, cfgErr := terrainconfig.LoadForRoot(root)
	if cfgErr != nil {
		// Invalid terrain.yaml must not silently disable source redaction;
		// warn here and fail closed at the emit site below.
		logging.L().Warn("terrain analyze: could not load config; redacting source excerpts to be safe", "err", cfgErr)
	}
	// Trust floor: ON by default; opt out via --no-trust-floor or
	// trust_floor: false in terrain.yaml.
	trustFloor := resolveTrustFloor(o.TrustFloor, o.NoTrustFloor, cfg)
	if cfg != nil && cfg.AI != nil {
		if len(cfg.AI.AIMarkers) > 0 {
			analysis.SetCustomAIMarkers(cfg.AI.AIMarkers)
		}
		// ai.scenarios_dir augments the recognized eval directories so
		// evals in a non-standard location are discovered.
		if cfg.AI.ScenariosDir != "" {
			aidetect.SetCustomEvalDirs([]string{cfg.AI.ScenariosDir})
		}
		// ai.baselines_dir: when no explicit --baseline is given, auto-load
		// the canonical baseline `terrain accept-snapshot` writes there.
		baselinePath = resolveBaselinePath(root, baselinePath, cfg)
	}
	if cfg != nil && cfg.ML != nil && cfg.ML.ArtifactsDir != "" {
		analysis.SetModelArtifactDirs([]string{cfg.ML.ArtifactsDir})
	}

	parsedRuntime := parseRuntimePaths(runtimePaths)
	parsedGauntlet := parseRuntimePaths(gauntletPaths)   // same comma-split logic
	parsedPromptfoo := parseRuntimePaths(promptfooPaths) // same comma-split logic
	parsedDeepEval := parseRuntimePaths(deepevalPaths)   // same comma-split logic
	parsedRagas := parseRuntimePaths(ragasPaths)         // same comma-split logic
	parsedGreatExpectations := parseRuntimePaths(greatExpectationsPaths)
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
	if err := validateExistingPaths("--great-expectations-results", parsedGreatExpectations); err != nil {
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
	opt.GreatExpectationsPaths = parsedGreatExpectations
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
		// on_terrain_error: pass lets adopters fail open — an analysis
		// infrastructure error (crash/timeout) exits 0 instead of
		// blocking the merge. Default (block / unset) fails closed.
		if cfg != nil && cfg.OnTerrainError == "pass" {
			if !jsonOutput {
				fmt.Fprintln(os.Stderr, "terrain: on_terrain_error=pass — analysis failed; exiting 0 (fail-open).")
			}
			return nil
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

	if err := appendDriftSignals(result.Snapshot, root, o.BaseRef); err != nil {
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
	// findings. The "JSON stdout purity" property requires that the
	// renderer completes (stdout
	// stays a valid JSON document) AND the gate decision returns via
	// the error channel (so main.go writes the gate message to stderr,
	// not stdout).
	// Gate decisions use the observability-tier-excluding summary so
	// findings from detectors that explicitly ship at observability tier
	// stay informational and don't block CI.
	gateRelevant := report.GateRelevantSummary
	if trustFloor {
		// Under the trust floor, only findings with a closed-loop-validated
		// remediation may block CI; recompute the gate-relevant breakdown
		// excluding unproven/judge-only remediations. Then tell the user what
		// was held back, so a build that passes because of the trust floor is
		// never silent (a demoted High/Critical AI finding hiding behind a green
		// build is the worst failure mode).
		floored := trustFloorGateBreakdown(root, result.Snapshot.Signals)
		if gate != severityGateNone {
			if held := trustFloorHeldBack(gate, gateRelevant, floored); held > 0 {
				fmt.Fprintf(os.Stderr,
					"trust floor: %d finding(s) at or above --fail-on=%s held back (no validated auto-fix yet) — they surface in the report but do not block CI. Run --no-trust-floor to gate on severity.\n",
					held, gate)
			}
		}
		gateRelevant = floored
	}
	gateBlocked, gateSummary := severityGateBlocked(gate, gateRelevant)
	gateErr := func() error {
		if gateBlocked {
			return fmt.Errorf("%w: --fail-on=%s matched %s", errSeverityGateBlocked, gate, gateSummary)
		}
		return nil
	}

	// Persist the snapshot and .terrain/findings.json BEFORE any output-format
	// branch can return, so every format (sarif, annotations, html, json, text)
	// leaves the same on-disk artifacts. Earlier revisions placed these after
	// the sarif/annotation branches, so `--format sarif` (or `--format
	// annotations`) returned before writing findings.json — silently breaking
	// the "always write .terrain/findings.json" contract for a CI job that
	// uploads SARIF and also reads findings.json with a second tool. `--write-
	// snapshot` persists here regardless of format for the same reason. The
	// file is small and gitignored; emission failure is non-fatal so a
	// read-only filesystem doesn't break the user-facing report.
	if writeSnap {
		if err := persistSnapshot(result.Snapshot, root); err != nil {
			return err
		}
	}
	if err := writeFindingsJSON(root, result.Snapshot.Signals, cfgErr != nil || (cfg != nil && cfg.RedactSource), trustFloor); err != nil {
		logging.L().Warn("terrain analyze: writing .terrain/findings.json", "err", err)
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

	// The "Unlock more" hints used to print here, but they
	// duplicated the Recommended actions + Limitations sections the
	// report already renders. Removing the duplicate keeps the
	// post-report space clean for the gate result.
	_ = engine.MissingArtifactHints

	// --fail-on gate: text-mode renderer falls through to the same
	// gateErr() the other branches use, so the gate decision applies
	// uniformly across every output format.
	return gateErr()
}

// writeFindingsJSON serializes the canonical Finding artifact to
// .terrain/findings.json. Used by `terrain analyze` so downstream
// consumers (terrain mcp, IDE plugins, third-party SARIF uploaders)
// have a stable shape to read from after every run.
//
// Maps Signal → Finding via the manifest's signal-type → rule_id
// lookup. Signals whose type is unmapped (e.g. an experimental
// detector ahead of its manifest entry) are dropped — the canonical
// Finding shape requires a rule_id.
// resolveBaselinePath picks the baseline snapshot path: an explicit
// --baseline wins; otherwise, if ai.baselines_dir is set and contains a
// latest.json file (the canonical path `terrain accept-snapshot` writes),
// that is used. Returns empty when neither applies.
func resolveBaselinePath(root, explicit string, cfg *terrainconfig.Config) string {
	if explicit != "" {
		return explicit
	}
	if cfg == nil || cfg.AI == nil || cfg.AI.BaselinesDir == "" {
		return ""
	}
	candidate := filepath.Join(root, cfg.AI.BaselinesDir, "latest.json")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// redactSourceForRoot reports whether terrain.yaml at root requests
// source-excerpt redaction. A missing config means no redaction (the opt-in
// default); an INVALID config fails closed — redact rather than risk leaking
// source excerpts a malformed config meant to hide.
func redactSourceForRoot(root string) bool {
	cfg, err := terrainconfig.LoadForRoot(root)
	if err != nil {
		return true
	}
	return cfg != nil && cfg.RedactSource
}

func writeFindingsJSON(root string, sigs []models.Signal, redactSource, trustFloor bool) error {
	art := buildFindingsArtifact(root, sigs, redactSource, trustFloor)
	terrainDir, err := safeTerrainDir(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(terrainDir, 0o755); err != nil {
		return fmt.Errorf("create .terrain/: %w", err)
	}
	var buf bytes.Buffer
	if err := art.WriteJSON(&buf); err != nil {
		return err
	}
	path := filepath.Join(terrainDir, "findings.json")
	return atomicfile.WriteFile(path, buf.Bytes(), 0o644)
}

// buildFindingsArtifact converts signals to the canonical findings.Artifact —
// attaching validated remediations and applying the trust-floor demotion — so
// EVERY surface built from it (findings.json, the Step Summary, JUnit, SARIF)
// reflects the same gate decision and severities. Rendering a CI artifact from
// a non-demoted report is what made a passing (exit-0) build show a held-back
// drift finding as a gate-blocking "error" in JUnit and the Step Summary.
func buildFindingsArtifact(root string, sigs []models.Signal, redactSource, trustFloor bool) *findings.Artifact {
	typeToRuleID := map[models.SignalType]string{}
	for _, entry := range signals.Manifest() {
		if entry.RuleID != "" {
			typeToRuleID[entry.Type] = entry.RuleID
		}
	}
	filtered := make([]models.Signal, 0, len(sigs))
	for _, s := range sigs {
		if _, ok := typeToRuleID[s.Type]; ok {
			filtered = append(filtered, s)
		}
	}
	fxs := findings.FromSignals(filtered, func(t models.SignalType) string {
		return typeToRuleID[t]
	})
	// Attach structured, mechanically-applicable remediations where a producer
	// exists; rules without one stay judge-only (text suggestion).
	defaultFixRegistry().Attach(root, fxs)
	demoteHeldBackFindings(fxs, filtered, root, trustFloor)
	art := findings.NewArtifact(fxs)
	if redactSource {
		art.RedactSource()
	}
	return art
}

// demoteHeldBackFindings demotes (Error→Warning) and provenance-marks the
// findings the trust floor holds back, using the SAME gateBlockable predicate
// the gate uses — so it touches exactly the non-Critical AI findings without a
// validated fix, never a deterministic / Critical / user-policy finding the
// gate still blocks on. fxs must be index-parallel to filtered.
func demoteHeldBackFindings(fxs []findings.Finding, filtered []models.Signal, root string, trustFloor bool) {
	if !trustFloor {
		return
	}
	blockable := gateBlockable(root, true)
	if blockable == nil {
		return
	}
	for i := range fxs {
		if i >= len(filtered) {
			break
		}
		s := filtered[i]
		if !signals.IsGateRelevant(s.Type) || blockable(s) {
			continue
		}
		if fxs[i].Severity == findings.SeverityError {
			fxs[i].Severity = findings.SeverityWarning
		}
		if fxs[i].Metadata == nil {
			fxs[i].Metadata = map[string]any{}
		}
		fxs[i].Metadata[remediate.GateMetadataKey] = true
	}
}

// safeTerrainDir returns root/.terrain, refusing when .terrain is a symlink —
// a repo committing `.terrain` as a symlink out of the tree would otherwise
// redirect artifact writes (findings.json, snapshots) outside the repo root.
func safeTerrainDir(root string) (string, error) {
	dir := filepath.Join(root, ".terrain")
	if fi, err := os.Lstat(dir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf(".terrain is a symlink; refusing to write artifacts outside the repository")
	}
	return dir, nil
}

// appendDriftSignals runs both prompt-drift detectors and appends their
// signals to the snapshot. EVERY CI-blocking surface — analyze, test,
// report check-runs, and report pr — must call this so the validated
// prompt→schema drift detector (the flagship gate-blocker) fires everywhere,
// not only on `terrain analyze`. Grafting drift onto individual command entry
// points is what let `terrain test` and the required check-run silently pass a
// drift a plain `analyze` blocked. baseRef enables the git-diff prompt/template
// path; the diff-free static prompt↔schema path always runs.
func appendDriftSignals(snap *models.TestSuiteSnapshot, root, baseRef string) error {
	if snap == nil {
		return nil
	}
	if err := appendPromptSchemaDriftSignals(snap, root, baseRef); err != nil {
		return err
	}
	if err := appendPromptContractDriftSignals(snap, root); err != nil {
		return err
	}
	// Apply terrain.yaml config filters LAST, over the complete signal set
	// (pipeline signals plus the drift signals just appended), so `rules: off`
	// and `ignore.paths` reach drift too. Every gating surface calls this
	// helper, so the filter lands consistently on analyze/test/check-runs/pr.
	applyTerrainConfigFilters(snap, root)
	return nil
}

// applyTerrainConfigFilters drops signals a repo's terrain.yaml turns off via
// `rules.<id>: off` or excludes via `ignore.paths` / `ignore.rules` — documented
// knobs that were otherwise inert. (`rules.<id>.severity` is deliberately not
// applied: its error/warning/off vocabulary does not map onto the
// critical/high/medium/low signal severities the gate uses; only the
// unambiguous `off` drop is honored.) No-op without a terrain.yaml or config.
func applyTerrainConfigFilters(snap *models.TestSuiteSnapshot, root string) {
	if snap == nil {
		return
	}
	cfg, err := terrainconfig.LoadForRoot(root)
	if err != nil || cfg == nil {
		return
	}
	if len(cfg.Rules) == 0 && len(cfg.Ignore.Paths) == 0 && len(cfg.Ignore.Rules) == 0 {
		return
	}
	lookup := ruleIDForSignalType()
	// dropped reports whether a signal is turned off or path-ignored by
	// config. Applied to snap.Signals AND each per-file TestFiles[i].Signals
	// so every surface that reads signals (including `terrain explain`, which
	// consumes tf.Signals) sees the same config-filtered set.
	dropped := func(sig models.Signal) bool {
		ruleID := lookup(sig.Type)
		if ruleID == "" {
			return false
		}
		if cfg.SeverityFor(ruleID, "") == "off" {
			return true
		}
		return cfg.IsPathIgnored(sig.Location.File, ruleID)
	}
	kept := snap.Signals[:0]
	for _, sig := range snap.Signals {
		if !dropped(sig) {
			kept = append(kept, sig)
		}
	}
	snap.Signals = kept
	for i := range snap.TestFiles {
		fileSigs := snap.TestFiles[i].Signals
		fileKept := fileSigs[:0]
		for _, sig := range fileSigs {
			if !dropped(sig) {
				fileKept = append(fileKept, sig)
			}
		}
		snap.TestFiles[i].Signals = fileKept
	}
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

// appendPromptContractDriftSignals runs the diff-free static prompt↔schema
// consistency check (internal/promptcontract) over the whole repo and appends
// the drift as SignalAIPromptSchemaDrift — the same rule as the git-diff path,
// but reaching Python code prompts bound to in-repo schemas without needing a
// base ref. Signals already present at the same type+file+line (e.g. emitted by
// the git-diff path) are not re-added, so the two paths never double-count.
func appendPromptContractDriftSignals(snap *models.TestSuiteSnapshot, root string) error {
	if snap == nil {
		return nil
	}
	drift, err := promptcontract.AnalyzeInRepo(root)
	if err != nil {
		return fmt.Errorf("aiPromptSchemaDrift(static): %w", err)
	}
	if len(drift) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, s := range snap.Signals {
		if s.Type == signals.SignalAIPromptSchemaDrift {
			seen[driftDedupKey(string(s.Type), s.Location.File, s.Location.Line)] = true
		}
	}
	for _, s := range promptcontract.ToSignals(drift) {
		key := driftDedupKey(string(s.Type), s.Location.File, s.Location.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		snap.Signals = append(snap.Signals, s)
	}
	return nil
}

func driftDedupKey(typ, file string, line int) string {
	return fmt.Sprintf("%s\x00%s\x00%d", typ, file, line)
}

// applyFindingsBudget caps each rule's findings at terrain.yaml's
// `max_findings`. Missing terrain.yaml is fine (most adopters don't
// have one). Pruned counts surface as a one-line stderr notice when
// rendering to humans; JSON/SARIF stay quiet so machine consumers
// see the same shape as before.
func applyFindingsBudget(snap *models.TestSuiteSnapshot, root string, jsonOutput bool) {
	cfg, err := terrainconfig.LoadForRoot(root)
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
