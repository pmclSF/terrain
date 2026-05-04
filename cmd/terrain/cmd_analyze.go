package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/policy"
	"github.com/pmclSF/terrain/internal/reporting"
	"github.com/pmclSF/terrain/internal/sarif"
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
		fmt.Printf("  %d. Edit .terrain/policy.yaml to enable governance rules\n", step)
		fmt.Println()
	}

	return nil
}

func relativeToRoot(path, root string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func runAnalyze(root string, jsonOutput bool, format string, verbose bool, writeSnap bool, coveragePath, coverageRunLabel string, runtimePaths string, gauntletPaths string, promptfooPaths string, deepevalPaths string, ragasPaths string, baselinePath string, slowThreshold float64, redactPaths bool, gate severityGate, timeout time.Duration) error {
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
	opt.OnProgress = newProgressFunc(jsonOutput)
	// Honour Ctrl-C and the optional --timeout: pre-0.2.x analyze
	// exited abruptly on SIGINT with no cleanup, and unbounded
	// monorepo scans could block CI indefinitely.
	// runPipelineWithSignalsAndTimeout wraps RunPipelineContext with a
	// SIGINT-aware context plus an optional deadline so in-flight
	// detectors check ctx.Err and unwind cooperatively.
	result, err := runPipelineWithSignalsAndTimeout(root, opt, timeout)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Report auto-discovered artifacts via structured logging.
	for _, msg := range result.DiscoveryMessages {
		logging.L().Info(msg)
	}

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
	gateBlocked, gateSummary := severityGateBlocked(gate, report.SignalSummary)
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
	// output format. Pre-0.2.x the persist call lived after the
	// rendering switch, so `--write-snapshot --json` returned from the
	// JSON branch before the snapshot was written — the canonical CI
	// shape (capture JSON to stdout, save snapshot to disk) silently
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if !policyResult.Found {
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(map[string]any{
				"policyFile": nil,
				"pass":       true,
				"violations": []any{},
				"message":    "No policy file found. Create .terrain/policy.yaml to define policy.",
			}); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to render policy output: %v\n", err)
				return exitError
			}
		} else {
			fmt.Println("Terrain Policy Check")
			fmt.Println()
			fmt.Println("No policy file found.")
			fmt.Println("Create .terrain/policy.yaml to define policy rules.")
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
