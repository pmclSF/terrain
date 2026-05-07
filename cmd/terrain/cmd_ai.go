package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/airun"
	"github.com/pmclSF/terrain/internal/comparison"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/reporting"
	"github.com/pmclSF/terrain/internal/uitokens"
)

const (
	// aiSeparatorWidth is the width of separator lines in AI text output.
	aiSeparatorWidth = 60

	// Decision actions returned by evaluateAIRunDecision.
	actionPass  = "pass"
	actionWarn  = "warn"
	actionBlock = "block"
)

// runAIList produces a comprehensive AI inventory view showing what AI systems
// exist in a repo, what capabilities they support, and what's missing validation.
func runAIList(root string, jsonOutput, verbose bool) error {
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	snap := result.Snapshot

	// --- Collect all surface types ---
	type surfaceGroup struct {
		kind  models.CodeSurfaceKind
		label string
		items []aiSurfaceEntry
	}
	groups := []surfaceGroup{
		{models.SurfacePrompt, "Prompts", nil},
		{models.SurfaceContext, "Contexts", nil},
		{models.SurfaceDataset, "Datasets", nil},
		{models.SurfaceToolDef, "Tool Definitions", nil},
		{models.SurfaceRetrieval, "Retrieval / RAG", nil},
		{models.SurfaceAgent, "Agent / Orchestration", nil},
		{models.SurfaceEvalDef, "Eval Definitions", nil},
	}
	groupIdx := map[models.CodeSurfaceKind]int{}
	for i, g := range groups {
		groupIdx[g.kind] = i
	}

	// All AI surface IDs for gap analysis.
	allAISurfaceIDs := map[string]bool{}

	for _, cs := range snap.CodeSurfaces {
		idx, ok := groupIdx[cs.Kind]
		if !ok {
			continue
		}
		entry := aiSurfaceEntry{
			SurfaceID: cs.SurfaceID, Name: cs.Name,
			Path: cs.Path, Language: cs.Language, Line: cs.Line,
			DetectionTier: cs.DetectionTier, Confidence: cs.Confidence,
			Reason: cs.Reason,
		}
		groups[idx].items = append(groups[idx].items, entry)
		allAISurfaceIDs[cs.SurfaceID] = true
	}

	// --- Scenarios with capability ---
	var scenarios []aiScenarioEntry
	capScenarios := map[string][]string{} // capability → scenario names
	for _, sc := range snap.Scenarios {
		scenarios = append(scenarios, aiScenarioEntry{
			ID:         sc.ScenarioID,
			Name:       sc.Name,
			Category:   sc.Category,
			Path:       sc.Path,
			Framework:  sc.Framework,
			Owner:      sc.Owner,
			Surfaces:   len(sc.CoveredSurfaceIDs),
			Capability: sc.Capability,
		})
		if sc.Capability != "" {
			capScenarios[sc.Capability] = append(capScenarios[sc.Capability], sc.Name)
		}
	}

	// --- Eval files ---
	var evalFiles []string
	for _, tf := range snap.TestFiles {
		if isEvalPath(tf.Path) {
			evalFiles = append(evalFiles, tf.Path)
		}
	}

	// --- Frameworks ---
	aiDet := aidetect.Detect(root)
	type fwEntry struct {
		Name       string  `json:"name"`
		Source     string  `json:"source"`
		Confidence float64 `json:"confidence"`
	}
	var frameworks []fwEntry
	for _, fw := range aiDet.Frameworks {
		frameworks = append(frameworks, fwEntry{
			Name: fw.Name, Source: fw.Source, Confidence: fw.Confidence,
		})
	}

	// --- Validation gap analysis ---
	// AI surfaces not covered by any scenario.
	coveredIDs := map[string]bool{}
	for _, sc := range snap.Scenarios {
		for _, sid := range sc.CoveredSurfaceIDs {
			coveredIDs[sid] = true
		}
	}
	var uncoveredSurfaces []aiSurfaceEntry
	for _, cs := range snap.CodeSurfaces {
		if !allAISurfaceIDs[cs.SurfaceID] {
			continue
		}
		if !coveredIDs[cs.SurfaceID] {
			uncoveredSurfaces = append(uncoveredSurfaces, aiSurfaceEntry{
				SurfaceID: cs.SurfaceID, Name: cs.Name,
				Path: cs.Path, Language: cs.Language, Line: cs.Line,
				DetectionTier: cs.DetectionTier, Confidence: cs.Confidence,
			})
		}
	}

	// --- Capabilities list ---
	var capabilities []string
	for cap := range capScenarios {
		capabilities = append(capabilities, cap)
	}
	sort.Strings(capabilities)

	// --- Surface counts for summary ---
	totalAI := 0
	for _, g := range groups {
		totalAI += len(g.items)
	}

	// --- JSON output ---
	if jsonOutput {
		type jsonResult struct {
			Frameworks        []fwEntry         `json:"frameworks"`
			Capabilities      []string          `json:"capabilities,omitempty"`
			Scenarios         []aiScenarioEntry `json:"scenarios"`
			Prompts           []aiSurfaceEntry  `json:"prompts"`
			Contexts          []aiSurfaceEntry  `json:"contexts,omitempty"`
			Datasets          []aiSurfaceEntry  `json:"datasets"`
			ToolDefs          []aiSurfaceEntry  `json:"toolDefinitions,omitempty"`
			Retrievals        []aiSurfaceEntry  `json:"retrievalSurfaces,omitempty"`
			Agents            []aiSurfaceEntry  `json:"agentSurfaces,omitempty"`
			EvalDefs          []aiSurfaceEntry  `json:"evalDefinitions,omitempty"`
			EvalFiles         []string          `json:"evalFiles"`
			ModelFiles        []string          `json:"modelFiles,omitempty"`
			UncoveredSurfaces []aiSurfaceEntry  `json:"uncoveredSurfaces,omitempty"`
			Summary           map[string]int    `json:"summary"`
		}
		summary := map[string]int{
			"scenarios":         len(scenarios),
			"capabilities":      len(capabilities),
			"totalAISurfaces":   totalAI,
			"uncoveredSurfaces": len(uncoveredSurfaces),
			"evalFiles":         len(evalFiles),
			"frameworks":        len(frameworks),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		// Ensure non-omitempty fields serialize as [] not null.
		if frameworks == nil {
			frameworks = []fwEntry{}
		}
		if scenarios == nil {
			scenarios = []aiScenarioEntry{}
		}
		if evalFiles == nil {
			evalFiles = []string{}
		}
		for i := range groups {
			if groups[i].items == nil {
				groups[i].items = []aiSurfaceEntry{}
			}
		}
		return enc.Encode(jsonResult{
			Frameworks:        frameworks,
			Capabilities:      capabilities,
			Scenarios:         scenarios,
			Prompts:           groups[0].items,
			Contexts:          groups[1].items,
			Datasets:          groups[2].items,
			ToolDefs:          groups[3].items,
			Retrievals:        groups[4].items,
			Agents:            groups[5].items,
			EvalDefs:          groups[6].items,
			EvalFiles:         evalFiles,
			ModelFiles:        aiDet.ModelFiles,
			UncoveredSurfaces: uncoveredSurfaces,
			Summary:           summary,
		})
	}

	// --- Text output ---
	fmt.Println("Terrain AI Inventory")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Println()

	// Summary table.
	fmt.Println("| Component          | Count |")
	fmt.Println("|--------------------|-------|")
	fmt.Printf("| Scenarios          | %5d |\n", len(scenarios))
	fmt.Printf("| Capabilities       | %5d |\n", len(capabilities))
	for _, g := range groups {
		if len(g.items) > 0 {
			fmt.Printf("| %-18s | %5d |\n", g.label, len(g.items))
		}
	}
	fmt.Printf("| Eval Files         | %5d |\n", len(evalFiles))
	if len(frameworks) > 0 {
		fmt.Printf("| Frameworks         | %5d |\n", len(frameworks))
	}
	if len(uncoveredSurfaces) > 0 {
		fmt.Printf("| Missing coverage   | %5d |\n", len(uncoveredSurfaces))
	}
	fmt.Println()

	// Empty state.
	if len(scenarios) == 0 && totalAI == 0 && len(evalFiles) == 0 {
		fmt.Println("No AI/eval components detected.")
		fmt.Println("Run `terrain ai doctor` to diagnose.")
		return nil
	}

	// Capabilities.
	if len(capabilities) > 0 {
		fmt.Println("Capabilities")
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, cap := range capabilities {
			names := capScenarios[cap]
			fmt.Printf("  %-30s %d scenario(s)\n", cap, len(names))
		}
		fmt.Println()
	}

	// Frameworks.
	if len(frameworks) > 0 {
		fmt.Println("Frameworks")
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, fw := range frameworks {
			fmt.Printf("  %-20s via %s (%.0f%%)\n", fw.Name, fw.Source, fw.Confidence*100)
		}
		fmt.Println()
	}

	// Scenarios grouped by capability.
	if len(scenarios) > 0 {
		fmt.Printf("Scenarios (%d)\n", len(scenarios))
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, sc := range scenarios {
			capLabel := ""
			if sc.Capability != "" {
				capLabel = " → " + sc.Capability
			}
			surfLabel := ""
			if sc.Surfaces > 0 {
				surfLabel = fmt.Sprintf(" [%d surface(s)]", sc.Surfaces)
			}
			fmt.Printf("  %-35s %s%s%s\n", sc.Name, sc.Category, capLabel, surfLabel)
		}
		fmt.Println()
	}

	// Surface sections.
	for _, g := range groups {
		if len(g.items) == 0 {
			continue
		}
		fmt.Printf("%s (%d)\n", g.label, len(g.items))
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, s := range g.items {
			if verbose {
				reporting.RenderSurfaceEvidence(os.Stdout, s.Name, s.Path, s.Line, s.DetectionTier, s.Confidence, s.Reason)
			} else {
				meta := ""
				if s.Reason != "" {
					meta = " (" + s.Reason + ")"
				} else if s.DetectionTier != "" && s.DetectionTier != models.TierPattern {
					meta = " [" + s.DetectionTier + "]"
				}
				fmt.Printf("  %-35s %s:%d%s\n", s.Name, s.Path, s.Line, meta)
			}
		}
		fmt.Println()
	}

	// Eval files.
	if len(evalFiles) > 0 {
		fmt.Printf("Eval Files (%d)\n", len(evalFiles))
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, f := range evalFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	// Validation gaps.
	if len(uncoveredSurfaces) > 0 {
		fmt.Printf("Missing Validation (%d AI surface(s) not covered by any scenario)\n", len(uncoveredSurfaces))
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		limit := 10
		if len(uncoveredSurfaces) < limit {
			limit = len(uncoveredSurfaces)
		}
		for _, s := range uncoveredSurfaces[:limit] {
			fmt.Printf("  %s  (%s)\n", s.Name, s.Path)
		}
		if len(uncoveredSurfaces) > limit {
			fmt.Printf("  ... and %d more\n", len(uncoveredSurfaces)-limit)
		}
		fmt.Println()
	}

	fmt.Println("Next steps:")
	fmt.Println("  terrain ai doctor         validate AI/eval setup")
	fmt.Println("  terrain explain <scenario> explain a scenario's coverage")
	fmt.Println("  terrain impact --base main see what a change affects")

	return nil
}

// runAIRun detects eval frameworks and executes scenarios.

// Type aliases to avoid duplicating struct definitions from airun package.
type aiRunScenario = airun.ScenarioEntry
type aiRunSignalEntry = airun.SignalEntry
type aiRunDecision = airun.Decision

func runAIRun(root string, jsonOutput bool, baseRef string, full, dryRun bool) error {
	// Step 1: Run pipeline.
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	snap := result.Snapshot

	if len(snap.Scenarios) == 0 {
		return fmt.Errorf("no eval scenarios detected.\n\nTerrain auto-derives scenarios from eval test files and AI framework imports.\nRun `terrain ai doctor` to diagnose.")
	}

	// Step 2: Detect framework for execution.
	det := aidetect.Detect(root)
	framework := "unknown"
	if len(det.Frameworks) > 0 {
		framework = det.Frameworks[0].Name
	}

	// Step 3: Select scenarios (impact-based or full).
	var selected, skipped []aiRunScenario
	mode := "full"

	if full {
		for _, sc := range snap.Scenarios {
			selected = append(selected, aiRunScenario{
				ID: sc.ScenarioID, Name: sc.Name, Capability: sc.Capability,
				Category: sc.Category, Reason: "full run (--full)", Path: sc.Path,
				Surfaces: sc.CoveredSurfaceIDs,
			})
		}
	} else {
		mode = "impacted"
		// Use impact analysis to select scenarios.
		var impactResult *impact.ImpactResult
		if baseRef != "" {
			cs, csErr := impact.ChangeSetFromGitDiff(root, baseRef)
			if csErr == nil {
				impactResult = impact.AnalyzeChangeSet(cs, snap)
			}
		}
		if impactResult == nil {
			// Fallback: try HEAD~1.
			cs, csErr := impact.ChangeSetFromGitDiff(root, "HEAD~1")
			if csErr == nil {
				impactResult = impact.AnalyzeChangeSet(cs, snap)
			}
		}

		if impactResult != nil && len(impactResult.ImpactedScenarios) > 0 {
			impactedIDs := map[string]bool{}
			for _, is := range impactResult.ImpactedScenarios {
				impactedIDs[is.ScenarioID] = true
				selected = append(selected, aiRunScenario{
					ID: is.ScenarioID, Name: is.Name, Capability: is.Capability,
					Category: is.Category, Reason: is.Relevance,
					Surfaces: is.CoversSurfaces,
				})
			}
			for _, sc := range snap.Scenarios {
				if !impactedIDs[sc.ScenarioID] {
					skipped = append(skipped, aiRunScenario{
						ID: sc.ScenarioID, Name: sc.Name, Capability: sc.Capability,
						Category: sc.Category, Reason: "not impacted by change",
					})
				}
			}
		} else {
			// No impact data or no impacted scenarios — run all.
			mode = "full"
			for _, sc := range snap.Scenarios {
				selected = append(selected, aiRunScenario{
					ID: sc.ScenarioID, Name: sc.Name, Capability: sc.Capability,
					Category: sc.Category, Reason: "no impact data; running all",
					Path: sc.Path, Surfaces: sc.CoveredSurfaceIDs,
				})
			}
		}
	}

	if dryRun {
		mode = "dry-run"
	}

	// Step 4: Build execution command.
	cmdArgs, evalOutputPath := buildEvalCommandWithOutput(framework, det, selected, snap)

	// Step 5: Execute (unless dry-run).
	var execErr error
	var stderrBuf bytes.Buffer
	if !dryRun && len(cmdArgs) > 0 {
		execCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		execCmd.Dir = root
		if jsonOutput {
			execCmd.Stderr = &stderrBuf
		} else {
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
		}
		execErr = execCmd.Run()
	}

	// Step 5b: If the framework wrote structured results, parse them
	// via the matching airun adapter and stash on the snapshot so
	// downstream detectors / reports can see the per-case data.
	// Errors are surfaced as warnings — the eval ran, we just couldn't
	// ingest its output.
	var evalRun *airun.EvalRunResult
	if !dryRun && execErr == nil && evalOutputPath != "" {
		if loaded, err := loadEvalRunByFramework(framework, evalOutputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s output: %v\n", framework, err)
		} else {
			evalRun = loaded
			if env, eerr := loaded.ToEnvelope(evalOutputPath); eerr == nil {
				snap.EvalRuns = append(snap.EvalRuns, env)
			}
			// Best-effort cleanup: framework wrote into a temp
			// path we own, so removing it after parse keeps
			// the user's tree tidy.
			_ = os.Remove(evalOutputPath)
		}
	}

	// Step 6: Collect AI signals from snapshot.
	var signalEntries []aiRunSignalEntry
	for _, sig := range snap.Signals {
		if sig.Category == models.CategoryAI {
			signalEntries = append(signalEntries, aiRunSignalEntry{
				Type: string(sig.Type), Severity: string(sig.Severity),
				Scenario: sig.Location.ScenarioID, Explanation: sig.Explanation,
			})
		}
	}

	// Step 7: Evaluate policy for CI decision.
	decision := evaluateAIRunDecision(snap, result)
	exitCode := 0
	if decision.Action == actionBlock {
		// exitAIGateBlock = 4 is the documented "AI gate blocks the run"
		// exit code per cmd/terrain/main.go's exit-code scheme. Pre-0.2.x
		// this path used exitError = 1, so CI scripts couldn't
		// distinguish AI-gate failure from any other runtime error.
		exitCode = exitAIGateBlock
	}
	if execErr != nil {
		// Eval execution failure is a runtime error, not an AI-gate
		// block — keep exit 1 so the two cases are distinguishable
		// upstream.
		decision.Action = actionBlock
		if stderr := stderrBuf.String(); stderr != "" {
			decision.Reason = fmt.Sprintf("eval execution failed: %v\n%s", execErr, stderr)
		} else {
			decision.Reason = fmt.Sprintf("eval execution failed: %v", execErr)
		}
		exitCode = exitError
	}

	// Step 7b: Compute content hashes and build persistent artifact.
	hashes := airun.ComputeHashes(root, snap.CodeSurfaces)
	persistArt := &airun.Artifact{
		Mode: mode, Framework: framework, Command: strings.Join(cmdArgs, " "),
		Decision: airun.Decision{
			Action: decision.Action, Reason: decision.Reason,
			Signals: decision.Signals, Blocked: decision.Blocked,
		},
		Hashes:   hashes,
		ExitCode: exitCode,
	}
	persistArt.Selected = selected
	persistArt.Skipped = skipped
	persistArt.Signals = signalEntries
	persistArt.EvalRun = evalRun
	if savedPath, saveErr := airun.SaveArtifact(root, persistArt); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save artifact: %v\n", saveErr)
	} else if !jsonOutput {
		fmt.Printf("Artifact saved: %s\n\n", savedPath)
	}

	// Step 8: Output.
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		// Output the full artifact (with hashes) for CI pipelines.
		if err := enc.Encode(persistArt); err != nil {
			return fmt.Errorf("encoding artifact JSON: %w", err)
		}
		if exitCode != 0 {
			return cliExitError{code: exitCode, message: decision.Reason}
		}
		return nil
	}

	// Text output.
	fmt.Println("Terrain AI Run")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Println()
	fmt.Printf("Mode:      %s\n", mode)
	fmt.Printf("Framework: %s\n", framework)
	fmt.Printf("Selected:  %d scenario(s)\n", len(selected))
	if len(skipped) > 0 {
		fmt.Printf("Skipped:   %d scenario(s) (not impacted)\n", len(skipped))
	}
	fmt.Println()

	// Show selected scenarios.
	if len(selected) > 0 {
		fmt.Println("Selected Scenarios")
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, sc := range selected {
			capLabel := ""
			if sc.Capability != "" {
				capLabel = " → " + sc.Capability
			}
			fmt.Printf("  %s%s\n", sc.Name, capLabel)
			fmt.Printf("    reason: %s\n", sc.Reason)
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Println("[dry-run] Would execute:")
		fmt.Printf("  %s\n", strings.Join(cmdArgs, " "))
		fmt.Println()
		fmt.Println("No execution performed.")
		return nil
	}

	// Hero verdict block — designed framing at the top so the
	// gating outcome carries visual weight rather than being a
	// single buried "Decision: ..." line. Reason flows below.
	verdict, headline := aiRunHeroLines(decision.Action, decision.Reason, len(signalEntries))
	fmt.Println(uitokens.HeroVerdict(verdict, headline))
	fmt.Println()

	if decision.Reason != "" && decision.Action != actionPass {
		fmt.Printf("Reason: %s\n", decision.Reason)
		fmt.Println()
	}

	if len(cmdArgs) > 0 {
		fmt.Printf("Command: %s\n", strings.Join(cmdArgs, " "))
		fmt.Println()
	}

	if len(signalEntries) > 0 {
		fmt.Printf("AI Signals (%d):\n", len(signalEntries))
		for _, s := range signalEntries {
			fmt.Printf("  [%s] %s: %s\n", s.Severity, s.Type, s.Explanation)
		}
		fmt.Println()
	}

	// Per-input ingestion diagnostics — when the gating decision
	// rests on adapter-defaulted or computed fields, surface them so
	// the adopter can audit data lineage. Audit (ai_eval_ingestion.E3
	// + ai_execution_gating.E3) called for this surface.
	if evalRun != nil && len(evalRun.Diagnostics) > 0 {
		fmt.Printf("Ingestion diagnostics (%d):\n", len(evalRun.Diagnostics))
		for _, d := range evalRun.Diagnostics {
			fmt.Printf("  [%s] %s — %s\n", d.Kind, d.Field, d.Detail)
		}
		fmt.Println()
	}

	fmt.Println("Next steps:")
	fmt.Println("  terrain ai record    save results as baseline")
	fmt.Println("  terrain explain <id> explain a scenario")

	if exitCode != 0 {
		return cliExitError{code: exitCode, message: decision.Reason}
	}
	return nil
}

// aiRunHeroLines maps the (action, reason, signalCount) triple to
// the verdict + headline pair the hero block renders. Centralized so
// the same wording flows into JSON output, terminal output, and
// downstream PR-comment surfaces consistently.
//
// Headline rules:
//   - BLOCKED — lead with the count of blocking signals; the
//     reason string fills in the why below the hero.
//   - WARN    — lead with a "review required" frame so users don't
//     mistake it for a hard fail.
//   - PASS    — confirm the gate cleared and where to go next.
func aiRunHeroLines(action, reason string, signalCount int) (verdict, headline string) {
	switch action {
	case actionBlock:
		if signalCount > 0 {
			return "BLOCKED", fmt.Sprintf(
				"%d AI eval %s — block merge",
				signalCount, reporting.Plural(signalCount, "signal"),
			)
		}
		return "BLOCKED", "AI eval gate triggered — block merge"
	case actionWarn:
		return "WARN", "AI eval gate flagged risks — review recommended"
	case actionPass:
		return "PASS", "AI eval gate clear"
	default:
		return strings.ToUpper(action), reason
	}
}

// newEvalOutputPath returns a temp-file path the eval framework can
// be asked to write its result to. Each terrain ai run owns its own
// temp file; cleanup happens after parse.
func newEvalOutputPath(framework string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("terrain-ai-run-%s-%d.json", framework, os.Getpid()))
}

// loadEvalRunByFramework picks the right airun adapter for the
// framework name and parses the file. Returns an error when the
// framework is unsupported or the file is malformed.
func loadEvalRunByFramework(framework, path string) (*airun.EvalRunResult, error) {
	switch framework {
	case "promptfoo":
		return airun.LoadPromptfooFile(path)
	case "deepeval":
		return airun.LoadDeepEvalFile(path)
	case "ragas":
		return airun.LoadRagasFile(path)
	}
	return nil, fmt.Errorf("framework %q has no airun adapter yet", framework)
}

// buildEvalCommand returns the argv to execute the configured eval
// framework. The 0.2 evolution adds an outputPath return: when the
// framework supports a structured output flag, we ask it to write to
// a temp file and the caller parses that via the airun adapter to
// surface aggregates + per-case data into the AI run artifact.
func buildEvalCommand(framework string, det *aidetect.DetectResult, selected []aiRunScenario, snap *models.TestSuiteSnapshot) []string {
	args, _ := buildEvalCommandWithOutput(framework, det, selected, snap)
	return args
}

// buildEvalCommandWithOutput returns argv plus the output path the
// framework will (be asked to) write to. Returns "" when the
// framework's CLI doesn't expose a structured-output flag we
// recognize.
func buildEvalCommandWithOutput(framework string, det *aidetect.DetectResult, selected []aiRunScenario, snap *models.TestSuiteSnapshot) (args []string, outputPath string) {
	switch framework {
	case "promptfoo":
		args = []string{"npx", "promptfoo", "eval"}
		if len(det.Frameworks) > 0 && det.Frameworks[0].ConfigFile != "" {
			args = append(args, "-c", det.Frameworks[0].ConfigFile)
		}
		// 0.2: write structured results to a temp file so the
		// post-execute path can ingest them via the Promptfoo adapter.
		out := newEvalOutputPath("promptfoo")
		args = append(args, "--output", out)
		return args, out
	case "deepeval":
		return []string{"deepeval", "test", "run"}, ""
	case "ragas":
		return []string{"python", "-m", "ragas", "evaluate"}, ""
	case "langsmith":
		return []string{"langsmith", "test", "run"}, ""
	}

	// Generic: run eval files with detected test runner.
	var evalFiles []string
	for _, sc := range selected {
		if sc.Path != "" {
			evalFiles = append(evalFiles, sc.Path)
		}
	}
	if len(evalFiles) == 0 {
		return nil, ""
	}

	runner := []string{"npx", "vitest", "run"}
	for _, tf := range snap.TestFiles {
		if tf.Framework == "pytest" {
			runner = []string{"pytest"}
			break
		}
		if tf.Framework == "jest" {
			runner = []string{"npx", "jest"}
			break
		}
	}
	return append(runner, evalFiles...), ""
}

func evaluateAIRunDecision(snap *models.TestSuiteSnapshot, result *engine.PipelineResult) aiRunDecision {
	decision := aiRunDecision{Action: actionPass, Reason: "all checks passed"}

	// Count AI signals by severity.
	var critical, high, medium int
	for _, sig := range snap.Signals {
		if sig.Category != models.CategoryAI {
			continue
		}
		switch sig.Severity {
		case models.SeverityCritical:
			critical++
		case models.SeverityHigh:
			high++
		case models.SeverityMedium:
			medium++
		}
		decision.Signals++
	}

	// Check governance violations from AI policy.
	for _, sig := range snap.Signals {
		if sig.Category == models.CategoryGovernance {
			if md, ok := sig.Metadata["rule"]; ok {
				rule, isStr := md.(string)
				if isStr && (strings.HasPrefix(rule, "block_on_") || rule == "blocking_signal_types") {
					decision.Blocked++
				}
			}
		}
	}

	if critical > 0 || decision.Blocked > 0 {
		decision.Action = actionBlock
		parts := []string{}
		if critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical signal(s)", critical))
		}
		if decision.Blocked > 0 {
			parts = append(parts, fmt.Sprintf("%d policy violation(s)", decision.Blocked))
		}
		decision.Reason = strings.Join(parts, ", ")
	} else if high > 0 || medium > 0 {
		decision.Action = actionWarn
		decision.Reason = fmt.Sprintf("%d high + %d medium signal(s)", high, medium)
	}

	return decision
}

// runAIRecord saves the latest eval run results as a baseline snapshot.
func runAIRecord(root string, jsonOutput bool) error {
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	snap := result.Snapshot

	if len(snap.Scenarios) == 0 {
		return fmt.Errorf("no scenarios to record. Run `terrain ai list` to check detected scenarios.")
	}

	// Write baseline snapshot to .terrain/baselines/
	baselineDir := filepath.Join(root, ".terrain", "baselines")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		return fmt.Errorf("creating baseline dir: %w", err)
	}

	type baseline struct {
		RecordedAt string            `json:"recordedAt"`
		Scenarios  []models.Scenario `json:"scenarios"`
		Surfaces   struct {
			Prompts  int `json:"prompts"`
			Datasets int `json:"datasets"`
		} `json:"surfaces"`
	}

	bl := baseline{RecordedAt: time.Now().UTC().Format(time.RFC3339)}
	bl.Scenarios = snap.Scenarios
	for _, cs := range snap.CodeSurfaces {
		switch cs.Kind {
		case models.SurfacePrompt:
			bl.Surfaces.Prompts++
		case models.SurfaceDataset:
			bl.Surfaces.Datasets++
		}
	}

	data, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding baseline: %w", err)
	}
	blPath := filepath.Join(baselineDir, "latest.json")
	if err := os.WriteFile(blPath, data, 0o644); err != nil {
		return fmt.Errorf("writing baseline: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(bl)
	}

	fmt.Println("Terrain AI Record")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Printf("Recorded %d scenarios to %s\n", len(bl.Scenarios), blPath)
	fmt.Printf("Prompt surfaces: %d\n", bl.Surfaces.Prompts)
	fmt.Printf("Dataset surfaces: %d\n", bl.Surfaces.Datasets)
	fmt.Println()
	fmt.Println("Next: terrain ai baseline    view or compare baselines")

	return nil
}

// runAIBaseline manages eval baselines (show, compare).
func runAIBaseline(root string, jsonOutput bool) error {
	blPath := filepath.Join(root, ".terrain", "baselines", "latest.json")
	data, err := os.ReadFile(blPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no baseline found. Run `terrain ai record` to create one.")
		}
		return fmt.Errorf("reading baseline: %w", err)
	}

	if jsonOutput {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("writing baseline: %w", err)
		}
		fmt.Println()
		return nil
	}

	var bl struct {
		RecordedAt string `json:"recordedAt"`
		Scenarios  []struct {
			ScenarioID string `json:"scenarioId"`
			Name       string `json:"name"`
			Category   string `json:"category"`
		} `json:"scenarios"`
		Surfaces struct {
			Prompts  int `json:"prompts"`
			Datasets int `json:"datasets"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(data, &bl); err != nil {
		return fmt.Errorf("parsing baseline: %w", err)
	}

	fmt.Println("Terrain AI Baseline")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Printf("Recorded: %s\n", bl.RecordedAt)
	fmt.Printf("Scenarios: %d\n", len(bl.Scenarios))
	fmt.Printf("Prompt surfaces: %d\n", bl.Surfaces.Prompts)
	fmt.Printf("Dataset surfaces: %d\n", bl.Surfaces.Datasets)
	fmt.Println()

	if len(bl.Scenarios) > 0 {
		fmt.Println("Scenarios:")
		for _, sc := range bl.Scenarios {
			fmt.Printf("  %-40s %s\n", sc.Name, sc.Category)
		}
	}

	// Compare with current state.
	fmt.Println()
	fmt.Println("To compare with current state: terrain ai list --json")

	return nil
}

func runAIBaselineCompare(root string, jsonOutput bool) error {
	blPath := filepath.Join(root, ".terrain", "baselines", "latest.json")
	data, err := os.ReadFile(blPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no baseline found. Run `terrain ai record` to create one.")
		}
		return fmt.Errorf("reading baseline: %w", err)
	}

	// Parse baseline for scenario list and any stored metrics.
	var baseline struct {
		RecordedAt string `json:"recordedAt"`
		Scenarios  []struct {
			ScenarioID string             `json:"scenarioId"`
			Name       string             `json:"name"`
			Metrics    map[string]float64 `json:"metrics,omitempty"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(data, &baseline); err != nil {
		return fmt.Errorf("parsing baseline: %w", err)
	}

	// Run current analysis to get current scenario state.
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Build metric maps: scenarioID → metricName → value.
	baselineMetrics := make(map[string]map[string]float64)
	for _, s := range baseline.Scenarios {
		if len(s.Metrics) > 0 {
			baselineMetrics[s.ScenarioID] = s.Metrics
		}
	}

	// Extract current metrics from Gauntlet signals in snapshot.
	currentMetrics := make(map[string]map[string]float64)
	for _, sig := range result.Snapshot.Signals {
		if sig.Location.ScenarioID == "" || sig.Metadata == nil {
			continue
		}
		sid := sig.Location.ScenarioID
		if currentMetrics[sid] == nil {
			currentMetrics[sid] = make(map[string]float64)
		}
		// Extract numeric metrics from signal metadata.
		for k, v := range sig.Metadata {
			if f, ok := v.(float64); ok {
				currentMetrics[sid][k] = f
			}
		}
	}

	// Compare scenarios: added/removed.
	baselineIDs := make(map[string]string) // id → name
	for _, s := range baseline.Scenarios {
		baselineIDs[s.ScenarioID] = s.Name
	}
	currentIDs := make(map[string]string)
	for _, s := range result.Snapshot.Scenarios {
		currentIDs[s.ScenarioID] = s.Name
	}

	var added, removed []string
	for id, name := range currentIDs {
		if _, ok := baselineIDs[id]; !ok {
			added = append(added, name)
		}
	}
	for id, name := range baselineIDs {
		if _, ok := currentIDs[id]; !ok {
			removed = append(removed, name)
		}
	}

	// Compare metrics.
	deltas := comparison.CompareEvalMetrics(baselineMetrics, currentMetrics)

	if jsonOutput {
		out := map[string]any{
			"baselineRecordedAt": baseline.RecordedAt,
			"baselineScenarios":  len(baseline.Scenarios),
			"currentScenarios":   len(result.Snapshot.Scenarios),
			"addedScenarios":     added,
			"removedScenarios":   removed,
			"metricDeltas":       deltas,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Human-readable output.
	fmt.Println("Terrain AI Baseline Comparison")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Printf("Baseline recorded: %s\n", baseline.RecordedAt)
	fmt.Printf("Scenarios: %d → %d", len(baseline.Scenarios), len(result.Snapshot.Scenarios))
	if diff := len(result.Snapshot.Scenarios) - len(baseline.Scenarios); diff > 0 {
		fmt.Printf(" (+%d)\n", diff)
	} else if diff < 0 {
		fmt.Printf(" (%d)\n", diff)
	} else {
		fmt.Println(" (unchanged)")
	}

	if len(added) > 0 {
		fmt.Println("\nAdded scenarios:")
		for _, name := range added {
			fmt.Printf("  + %s\n", name)
		}
	}
	if len(removed) > 0 {
		fmt.Println("\nRemoved scenarios:")
		for _, name := range removed {
			fmt.Printf("  - %s\n", name)
		}
	}

	if len(deltas) > 0 {
		fmt.Println("\nMetric changes:")
		regressionCount := 0
		for _, d := range deltas {
			marker := "  "
			if d.IsRegression {
				marker = "▼ "
				regressionCount++
			} else {
				marker = "▲ "
			}
			fmt.Printf("  %s%-30s %-20s %.4f → %.4f (%+.4f)\n",
				marker, d.ScenarioID, d.MetricName, d.FromValue, d.ToValue, d.Delta)
		}
		if regressionCount > 0 {
			fmt.Printf("\n⚠ %d regression(s) detected\n", regressionCount)
		}
	} else if len(baselineMetrics) == 0 {
		fmt.Println("\nNo baseline metrics recorded. Re-run `terrain ai record` with --gauntlet to capture metrics.")
	} else {
		fmt.Println("\nNo metric changes detected.")
	}

	return nil
}

func runAIReplay(root string, jsonOutput bool, artifactPath string) error {
	// Run pipeline for current state.
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	snap := result.Snapshot

	replayResult, err := airun.Replay(artifactPath, root, snap.CodeSurfaces, len(snap.Scenarios))
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(replayResult)
	}

	fmt.Println("Terrain AI Replay")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Println()
	fmt.Printf("Artifact:    %s\n", artifactPath)
	fmt.Printf("Scenarios:   %d original → %d current\n", replayResult.OriginalScenarios, replayResult.CurrentScenarios)
	fmt.Printf("Hashes:      %d surface(s) tracked\n", replayResult.CurrentHashes.TotalHashCount())
	fmt.Println()

	if replayResult.Match {
		fmt.Println("Result: MATCH — current repo state matches the original run.")
		fmt.Println("All content hashes identical. Scenario count unchanged.")
	} else {
		fmt.Printf("Result: MISMATCH — %d difference(s) found\n", len(replayResult.Mismatches))
		fmt.Println()
		fmt.Println("Differences")
		fmt.Println(strings.Repeat("-", aiSeparatorWidth))
		for _, m := range replayResult.Mismatches {
			fmt.Printf("  [%s] %s\n", m.Kind, m.Detail)
			if m.Surface != "" {
				fmt.Printf("    surface: %s\n", m.Surface)
			}
			if m.Original != "" && m.Current != "" {
				fmt.Printf("    original: %s → current: %s\n", m.Original, m.Current)
			}
		}
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  terrain ai run --full    re-run all scenarios")
	fmt.Println("  terrain ai list          view current inventory")

	return nil
}

func runAIDoctor(root string, jsonOutput bool) error {
	result, err := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	snap := result.Snapshot

	type doctorCheck struct {
		Name    string `json:"name"`
		Status  string `json:"status"` // "pass", "warn", "fail"
		Message string `json:"message"`
	}

	var checks []doctorCheck

	// Check 1: Are there any scenarios?
	if len(snap.Scenarios) > 0 {
		checks = append(checks, doctorCheck{
			Name:    "scenarios",
			Status:  "pass",
			Message: fmt.Sprintf("%d scenario(s) detected", len(snap.Scenarios)),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "scenarios",
			Status:  "warn",
			Message: "No scenarios detected. Add scenarios via .terrain/terrain.yaml or use an eval framework.",
		})
	}

	// Check 2: Are there prompt surfaces?
	promptCount := 0
	contextCount := 0
	datasetCount := 0
	for _, cs := range snap.CodeSurfaces {
		switch cs.Kind {
		case models.SurfacePrompt:
			promptCount++
		case models.SurfaceContext:
			contextCount++
		case models.SurfaceDataset:
			datasetCount++
		}
	}
	if promptCount > 0 {
		checks = append(checks, doctorCheck{
			Name:    "prompts",
			Status:  "pass",
			Message: fmt.Sprintf("%d prompt surface(s) detected", promptCount),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "prompts",
			Status:  "warn",
			Message: "No prompt surfaces detected. Export functions with 'prompt' or 'template' in the name to enable prompt tracking.",
		})
	}

	// Check 3: Are there dataset surfaces?
	if datasetCount > 0 {
		checks = append(checks, doctorCheck{
			Name:    "datasets",
			Status:  "pass",
			Message: fmt.Sprintf("%d dataset surface(s) detected", datasetCount),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "datasets",
			Status:  "warn",
			Message: "No dataset surfaces detected. Export functions with 'dataset' or 'dataloader' in the name to enable dataset tracking.",
		})
	}

	// Check 3b: Are there context surfaces?
	if contextCount > 0 {
		checks = append(checks, doctorCheck{
			Name:    "contexts",
			Status:  "pass",
			Message: fmt.Sprintf("%d context surface(s) detected (system messages, policies, few-shot, etc.)", contextCount),
		})
	}
	// No warning for missing contexts — they're optional.

	// Check 4: Are there eval-related test files?
	evalFileCount := 0
	for _, tf := range snap.TestFiles {
		if isEvalPath(tf.Path) {
			evalFileCount++
		}
	}
	if evalFileCount > 0 {
		checks = append(checks, doctorCheck{
			Name:    "eval_files",
			Status:  "pass",
			Message: fmt.Sprintf("%d eval-related test file(s) found", evalFileCount),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "eval_files",
			Status:  "warn",
			Message: "No eval-related test files found. Files in eval/, evals/, or __evals__/ directories are detected automatically.",
		})
	}

	// Check 5: AI framework detection.
	aiDet := aidetect.Detect(root)
	if len(aiDet.Frameworks) > 0 {
		names := make([]string, len(aiDet.Frameworks))
		for i, fw := range aiDet.Frameworks {
			names[i] = fw.Name
		}
		checks = append(checks, doctorCheck{
			Name:    "frameworks",
			Status:  "pass",
			Message: fmt.Sprintf("%d framework(s) detected: %s", len(aiDet.Frameworks), strings.Join(names, ", ")),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "frameworks",
			Status:  "warn",
			Message: "No AI/eval frameworks detected. Install deepeval, promptfoo, langchain, etc.",
		})
	}

	// Check 6: Graph wiring — do scenarios connect to surfaces?
	if len(snap.Scenarios) > 0 {
		wired := 0
		for _, sc := range snap.Scenarios {
			if len(sc.CoveredSurfaceIDs) > 0 {
				wired++
			}
		}
		if wired == len(snap.Scenarios) {
			checks = append(checks, doctorCheck{
				Name:    "graph_wiring",
				Status:  "pass",
				Message: fmt.Sprintf("All %d scenario(s) linked to code surfaces", wired),
			})
		} else {
			checks = append(checks, doctorCheck{
				Name:    "graph_wiring",
				Status:  "warn",
				Message: fmt.Sprintf("%d of %d scenario(s) have no linked code surfaces", len(snap.Scenarios)-wired, len(snap.Scenarios)),
			})
		}
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(checks)
	}

	// Text output.
	fmt.Println("Terrain AI Doctor")
	fmt.Println(strings.Repeat("=", aiSeparatorWidth))
	fmt.Println()

	passCount := 0
	warnCount := 0
	for _, c := range checks {
		icon := "  "
		switch c.Status {
		case "pass":
			icon = "  [pass]"
			passCount++
		case "warn":
			icon = "  [warn]"
			warnCount++
		case "fail":
			icon = "  [FAIL]"
		}
		fmt.Printf("%s %-16s %s\n", icon, c.Name, c.Message)
	}

	fmt.Println()
	if warnCount == 0 {
		fmt.Println("All checks passed. AI/eval setup looks good.")
	} else {
		fmt.Printf("%d check(s) passed, %d warning(s).\n", passCount, warnCount)
	}

	return nil
}

// aiScenarioEntry is the JSON representation of a detected scenario.
type aiScenarioEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category,omitempty"`
	Path       string `json:"path,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Surfaces   int    `json:"surfaces"`
	Capability string `json:"capability,omitempty"`
}

// aiSurfaceEntry is the JSON representation of a prompt/dataset surface.
type aiSurfaceEntry struct {
	SurfaceID     string  `json:"surfaceId"`
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	Language      string  `json:"language"`
	Line          int     `json:"line"`
	DetectionTier string  `json:"detectionTier,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
	Reason        string  `json:"reason,omitempty"`
}

// isEvalPath returns true if a file path looks like an eval/benchmark file.
func isEvalPath(path string) bool {
	return aidetect.IsEvalTestPath(path)
}
