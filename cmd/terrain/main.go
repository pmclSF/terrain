// terrain — test system intelligence platform.
//
// Primary commands (canonical user journeys):
//
//	terrain analyze              What is the state of our test system?
//	terrain impact               What validations matter for this change?
//	terrain insights             What should we fix in our test system?
//	terrain explain <target>     Why did Terrain make this decision?
//
// Supporting commands:
//
//	terrain init                 detect data files and print recommended analyze command
//	terrain summary              executive summary with risk, trends, benchmark readiness
//	terrain focus                prioritized next actions
//	terrain posture              detailed posture breakdown with evidence
//	terrain portfolio            portfolio intelligence (cost, breadth, leverage, redundancy)
//	terrain metrics              aggregate metrics scorecard
//	terrain compare              compare two snapshots for trend tracking
//	terrain select-tests         recommend protective test set for a change
//	terrain pr                   PR/change-scoped analysis
//	terrain show <entity> <id>   drill into test, unit, owner, or finding
//	terrain migration <sub>      readiness, blockers, or preview
//	terrain policy check         evaluate local policy rules
//	terrain export benchmark     privacy-safe JSON export for benchmarking
//	terrain serve                local HTTP server with HTML report and JSON API
//
// Advanced / debug:
//
//	terrain debug graph          dependency graph statistics
//	terrain debug coverage       structural coverage analysis
//	terrain debug fanout         high-fanout node analysis
//	terrain debug duplicates     duplicate test cluster analysis
//	terrain debug depgraph       full dependency graph analysis (all engines)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	conv "github.com/pmclSF/terrain/internal/convert"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/server"
	"github.com/pmclSF/terrain/internal/telemetry"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const defaultSlowThresholdMs = 5000.0

// Exit codes. CI scripts can distinguish failure modes from these without
// parsing stderr. The 0.1.2 contract preserves historical semantics: codes
// 0–2 keep their existing meanings, and the new code (4) is additive.
//
//	0 — success
//	1 — runtime / analysis error (file not found, parse failed, IO error)
//	2 — usage error OR policy violation (overloaded for back-compat; both
//	     meanings retained because at least one consumer pattern-matches
//	     `exit 2 == policy fail today`)
//	3 — reserved (0.2 will move policy violations here once we publish a
//	     migration guide; do not use for new codepaths)
//	4 — AI gate block (terrain ai gate; reserved for 0.2's dedicated AI
//	     gate command)
//
// Splitting code 2 cleanly into "usage" vs "policy" is a behaviour-breaking
// change that needs a migration window. It's documented in 0.2 as an
// explicit milestone in docs/release/0.2.md.
const (
	exitOK              = 0
	exitError           = 1
	exitUsageError      = 2
	exitPolicyViolation = 2 // overloaded with exitUsageError until 0.2; see comment above
	exitAIGateBlock     = 4
)

func main() {
	// Parse global --log-level flag before subcommand dispatch.
	// Accepted values: quiet, debug (default: info-level).
	initLogging(os.Args[1:])

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "analyze":
		analyzeCmd := flag.NewFlagSet("analyze", flag.ExitOnError)
		rootFlag := analyzeCmd.String("root", ".", "repository root to analyze")
		jsonFlag := analyzeCmd.Bool("json", false, "output JSON snapshot")
		formatFlag := analyzeCmd.String("format", "", "output format: json or text")
		verboseFlag := analyzeCmd.Bool("verbose", false, "show all findings in analyze output")
		writeSnapshot := analyzeCmd.Bool("write-snapshot", false, "persist snapshot to .terrain/snapshots/latest.json")
		coverageFlag := analyzeCmd.String("coverage", "", "path to coverage file or directory (LCOV, Istanbul JSON)")
		coverageRunLabelFlag := analyzeCmd.String("coverage-run-label", "", "coverage run label: unit, integration, or e2e")
		runtimeFlag := analyzeCmd.String("runtime", "", "path to runtime artifact (JUnit XML, Jest JSON); comma-separated for multiple")
		gauntletFlag := analyzeCmd.String("gauntlet", "", "path to Gauntlet eval result artifact (JSON); comma-separated for multiple")
		promptfooFlag := analyzeCmd.String("promptfoo-results", "", "path to Promptfoo --output result file(s); comma-separated for multiple")
		slowThreshold := analyzeCmd.Float64("slow-threshold", defaultSlowThresholdMs, "slow test threshold in ms")
		redactPathsFlag := analyzeCmd.Bool("redact-paths", false, "rewrite absolute paths in --format=sarif output to repo-relative form (or basename if outside repo)")
		_ = analyzeCmd.Parse(os.Args[2:])
		if err := runAnalyze(*rootFlag, *jsonFlag, *formatFlag, *verboseFlag, *writeSnapshot, *coverageFlag, *coverageRunLabelFlag, *runtimeFlag, *gauntletFlag, *promptfooFlag, *slowThreshold, *redactPathsFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		rootFlag := initCmd.String("root", ".", "repository root to inspect")
		jsonFlag := initCmd.Bool("json", false, "output JSON init result")
		_ = initCmd.Parse(os.Args[2:])
		if err := runInit(*rootFlag, *jsonFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "impact":
		impactCmd := flag.NewFlagSet("impact", flag.ExitOnError)
		rootFlag := impactCmd.String("root", ".", "repository root to analyze")
		baseRef := impactCmd.String("base", "", "git base ref for diff (default: HEAD~1)")
		jsonFlag := impactCmd.Bool("json", false, "output JSON impact result")
		showFlag := impactCmd.String("show", "", "drill-down view: units, gaps, tests, owners, graph, selected")
		ownerFlag := impactCmd.String("owner", "", "filter results by owner")
		_ = impactCmd.Parse(os.Args[2:])
		if err := runImpact(*rootFlag, *baseRef, *jsonFlag, *showFlag, *ownerFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "convert":
		if err := runConvertCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "convert-config":
		if err := runConvertConfigCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "list", "list-conversions":
		if err := runListConversionsCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "shorthands":
		if err := runShorthandsCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "detect":
		if err := runDetectCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "migrate":
		if err := runMigrateCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "estimate":
		if err := runEstimateCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "status":
		if err := runStatusCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "checklist":
		if err := runChecklistCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "doctor":
		os.Exit(runDoctorCLI(os.Args[2:]))

	case "reset":
		if err := runResetCLI(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(exitCodeForCLIError(err))
		}

	case "policy":
		if len(os.Args) < 3 || os.Args[2] != "check" {
			fmt.Fprintln(os.Stderr, "Usage: terrain policy check [flags]")
			os.Exit(2)
		}
		policyCmd := flag.NewFlagSet("policy check", flag.ExitOnError)
		rootFlag := policyCmd.String("root", ".", "repository root to analyze")
		jsonFlag := policyCmd.Bool("json", false, "output JSON policy check result")
		coverageFlag := policyCmd.String("coverage", "", "path to coverage file or directory (LCOV, Istanbul JSON)")
		coverageRunLabelFlag := policyCmd.String("coverage-run-label", "", "coverage run label: unit, integration, or e2e")
		runtimeFlag := policyCmd.String("runtime", "", "path to runtime artifact (JUnit XML, Jest JSON); comma-separated for multiple")
		slowThreshold := policyCmd.Float64("slow-threshold", defaultSlowThresholdMs, "slow test threshold in ms")
		_ = policyCmd.Parse(os.Args[3:])
		exitCode := runPolicyCheck(*rootFlag, *jsonFlag, *coverageFlag, *coverageRunLabelFlag, *runtimeFlag, *slowThreshold)
		os.Exit(exitCode)

	case "metrics":
		metricsCmd := flag.NewFlagSet("metrics", flag.ExitOnError)
		rootFlag := metricsCmd.String("root", ".", "repository root to analyze")
		jsonFlag := metricsCmd.Bool("json", false, "output JSON metrics snapshot")
		verboseFlag := metricsCmd.Bool("verbose", false, "show detailed metric breakdowns")
		_ = metricsCmd.Parse(os.Args[2:])
		if err := runMetrics(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "posture":
		postureCmd := flag.NewFlagSet("posture", flag.ExitOnError)
		rootFlag := postureCmd.String("root", ".", "repository root to analyze")
		jsonFlag := postureCmd.Bool("json", false, "output JSON posture snapshot")
		verboseFlag := postureCmd.Bool("verbose", false, "show measurement values and thresholds")
		_ = postureCmd.Parse(os.Args[2:])
		if err := runPosture(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "portfolio":
		portfolioCmd := flag.NewFlagSet("portfolio", flag.ExitOnError)
		rootFlag := portfolioCmd.String("root", ".", "repository root to analyze")
		jsonFlag := portfolioCmd.Bool("json", false, "output JSON portfolio snapshot")
		verboseFlag := portfolioCmd.Bool("verbose", false, "show per-asset details")
		_ = portfolioCmd.Parse(os.Args[2:])
		if err := runPortfolio(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "insights":
		insightsCmd := flag.NewFlagSet("insights", flag.ExitOnError)
		rootFlag := insightsCmd.String("root", ".", "repository root to analyze")
		jsonFlag := insightsCmd.Bool("json", false, "output JSON insights")
		verboseFlag := insightsCmd.Bool("verbose", false, "show per-finding evidence and file details")
		_ = insightsCmd.Parse(os.Args[2:])
		if err := runInsights(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "explain":
		explainCmd := flag.NewFlagSet("explain", flag.ExitOnError)
		rootFlag := explainCmd.String("root", ".", "repository root to analyze")
		baseRef := explainCmd.String("base", "", "git base ref for diff (default: HEAD~1)")
		jsonFlag := explainCmd.Bool("json", false, "output JSON")
		verboseFlag := explainCmd.Bool("verbose", false, "show detection evidence, tiers, and confidence details")
		explainFlagsWithValue := map[string]bool{
			"--root": true, "--base": true,
		}
		_ = explainCmd.Parse(reorderCLIArgs(os.Args[2:], explainFlagsWithValue))
		explainArgs := explainCmd.Args()
		if len(explainArgs) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: terrain explain <target> [flags]")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Explain why Terrain made a decision. Target is auto-detected:")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  terrain explain <test-path>     explain a test file")
			fmt.Fprintln(os.Stderr, "  terrain explain <test-id>       explain a test case by ID")
			fmt.Fprintln(os.Stderr, "  terrain explain <code-unit>     explain a code unit (path:name)")
			fmt.Fprintln(os.Stderr, "  terrain explain <owner>         explain an owner's scope")
			fmt.Fprintln(os.Stderr, "  terrain explain <scenario-id>   explain an AI/eval scenario")
			fmt.Fprintln(os.Stderr, "  terrain explain selection       explain overall test selection")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Flags:")
			fmt.Fprintln(os.Stderr, "  --verbose    show detection evidence, tiers, and confidence details")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "See: docs/examples/explain-report.md")
			os.Exit(2)
		}
		if err := runExplain(explainArgs[0], *rootFlag, *baseRef, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "summary":
		summaryCmd := flag.NewFlagSet("summary", flag.ExitOnError)
		rootFlag := summaryCmd.String("root", ".", "repository root to analyze")
		jsonFlag := summaryCmd.Bool("json", false, "output JSON summary with heatmap")
		verboseFlag := summaryCmd.Bool("verbose", false, "show detailed heatmap breakdown")
		_ = summaryCmd.Parse(os.Args[2:])
		if err := runSummary(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "focus":
		focusCmd := flag.NewFlagSet("focus", flag.ExitOnError)
		rootFlag := focusCmd.String("root", ".", "repository root to analyze")
		jsonFlag := focusCmd.Bool("json", false, "output JSON focus summary")
		verboseFlag := focusCmd.Bool("verbose", false, "show full rationale and dependency chains")
		_ = focusCmd.Parse(os.Args[2:])
		if err := runFocus(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "compare":
		compareCmd := flag.NewFlagSet("compare", flag.ExitOnError)
		fromFlag := compareCmd.String("from", "", "path to baseline snapshot JSON")
		toFlag := compareCmd.String("to", "", "path to current snapshot JSON")
		rootFlag := compareCmd.String("root", ".", "repository root (used to find .terrain/snapshots/)")
		jsonFlag := compareCmd.Bool("json", false, "output JSON comparison")
		_ = compareCmd.Parse(os.Args[2:])
		if err := runCompare(*fromFlag, *toFlag, *rootFlag, *jsonFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "migration":
		if len(os.Args) < 3 {
			printMigrationUsage()
			os.Exit(2)
		}
		if isHelpArg(os.Args[2]) {
			printMigrationUsage()
			return
		}
		subCmd := os.Args[2]
		var err error
		switch subCmd {
		case "readiness", "blockers":
			migCmd := flag.NewFlagSet("migration "+subCmd, flag.ExitOnError)
			rootFlag := migCmd.String("root", ".", "repository root to analyze")
			jsonFlag := migCmd.Bool("json", false, "output JSON")
			_ = migCmd.Parse(os.Args[3:])
			err = runMigration(subCmd, *rootFlag, *jsonFlag, "", "")
		case "preview":
			migCmd := flag.NewFlagSet("migration preview", flag.ExitOnError)
			rootFlag := migCmd.String("root", ".", "repository root to analyze")
			jsonFlag := migCmd.Bool("json", false, "output JSON")
			fileFlag := migCmd.String("file", "", "file path for preview (relative to root)")
			scopeFlag := migCmd.String("scope", "", "directory scope for preview")
			_ = migCmd.Parse(os.Args[3:])
			err = runMigration(subCmd, *rootFlag, *jsonFlag, *fileFlag, *scopeFlag)
		default:
			fmt.Fprintf(os.Stderr, "unknown migration subcommand: %q (valid: readiness, blockers, preview)\n", subCmd)
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "select-tests":
		stCmd := flag.NewFlagSet("select-tests", flag.ExitOnError)
		rootFlag := stCmd.String("root", ".", "repository root to analyze")
		baseRef := stCmd.String("base", "", "git base ref for diff (default: HEAD~1)")
		jsonFlag := stCmd.Bool("json", false, "output JSON protective test set")
		_ = stCmd.Parse(os.Args[2:])
		if err := runSelectTests(*rootFlag, *baseRef, *jsonFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "pr":
		prCmd := flag.NewFlagSet("pr", flag.ExitOnError)
		rootFlag := prCmd.String("root", ".", "repository root to analyze")
		baseRef := prCmd.String("base", "", "git base ref for diff (default: HEAD~1)")
		jsonFlag := prCmd.Bool("json", false, "output JSON PR analysis")
		formatFlag := prCmd.String("format", "", "output format: markdown, comment, annotation")
		_ = prCmd.Parse(os.Args[2:])
		if err := runPR(*rootFlag, *baseRef, *jsonFlag, *formatFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "show":
		if len(os.Args) >= 3 {
			switch os.Args[2] {
			case "--help", "-h", "help":
				printShowUsage()
				return
			}
		}
		if len(os.Args) < 3 {
			printShowUsage()
			os.Exit(2)
		}
		// Separate flags from positional args manually so flags can appear
		// in any position (e.g., "terrain show --json test foo",
		// "terrain show test foo --json", or "terrain show test --json foo").
		var showPositional []string
		showJSON := false
		showRoot := "."
		for _, arg := range os.Args[2:] {
			switch {
			case arg == "--json" || arg == "-json":
				showJSON = true
			case strings.HasPrefix(arg, "--root="):
				showRoot = strings.TrimPrefix(arg, "--root=")
			case strings.HasPrefix(arg, "-root="):
				showRoot = strings.TrimPrefix(arg, "-root=")
			case arg == "--root" || arg == "-root":
				// Next arg would be the root value — handled below.
				showRoot = ""
			default:
				if showRoot == "" {
					showRoot = arg // consume the value after --root
				} else {
					showPositional = append(showPositional, arg)
				}
			}
		}
		if showRoot == "" {
			showRoot = "." // --root provided without value; fall back to cwd
		}
		showSubCmd := ""
		showID := ""
		if len(showPositional) > 0 {
			showSubCmd = showPositional[0]
		}
		if len(showPositional) > 1 {
			showID = showPositional[1]
		}
		if showSubCmd == "" {
			printShowUsage()
			os.Exit(2)
		}
		rootFlag := &showRoot
		jsonFlag := &showJSON
		if err := runShow(showSubCmd, showID, *rootFlag, *jsonFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "export":
		if len(os.Args) < 3 {
			printExportUsage()
			os.Exit(2)
		}
		if isHelpArg(os.Args[2]) {
			printExportUsage()
			return
		}
		if os.Args[2] != "benchmark" {
			fmt.Fprintf(os.Stderr, "unknown export subcommand: %q\n\n", os.Args[2])
			printExportUsage()
			os.Exit(2)
		}
		exportCmd := flag.NewFlagSet("export benchmark", flag.ExitOnError)
		rootFlag := exportCmd.String("root", ".", "repository root to analyze")
		_ = exportCmd.Parse(os.Args[3:])
		if err := runExportBenchmark(*rootFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "debug":
		if len(os.Args) < 3 {
			printDebugUsage()
			os.Exit(2)
		}
		if isHelpArg(os.Args[2]) {
			printDebugUsage()
			return
		}
		debugSub := os.Args[2]
		if debugSub == "depgraph" {
			// Full depgraph analysis under debug namespace.
			dgCmd := flag.NewFlagSet("debug depgraph", flag.ExitOnError)
			rootFlag := dgCmd.String("root", ".", "repository root to analyze")
			jsonFlag := dgCmd.Bool("json", false, "output JSON")
			showFlag := dgCmd.String("show", "", "sub-view: stats, coverage, duplicates, fanout, impact, profile")
			changedFlag := dgCmd.String("changed", "", "comma-separated changed files for impact analysis")
			_ = dgCmd.Parse(os.Args[3:])
			if err := runDepgraph(*rootFlag, *jsonFlag, *showFlag, *changedFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		debugCmd := flag.NewFlagSet("debug "+debugSub, flag.ExitOnError)
		rootFlag := debugCmd.String("root", ".", "repository root to analyze")
		jsonFlag := debugCmd.Bool("json", false, "output JSON")
		changedFlag := debugCmd.String("changed", "", "comma-separated changed files for impact analysis")
		_ = debugCmd.Parse(os.Args[3:])
		showView := ""
		switch debugSub {
		case "graph":
			showView = "stats"
		case "coverage":
			showView = "coverage"
		case "fanout":
			showView = "fanout"
		case "duplicates":
			showView = "duplicates"
		default:
			fmt.Fprintf(os.Stderr, "unknown debug command: %s\n", debugSub)
			fmt.Fprintln(os.Stderr, "Available: graph, coverage, fanout, duplicates, depgraph")
			os.Exit(2)
		}
		if err := runDepgraph(*rootFlag, *jsonFlag, showView, *changedFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "depgraph":
		// Backward-compat alias for "debug depgraph".
		dgCmd := flag.NewFlagSet("depgraph", flag.ExitOnError)
		rootFlag := dgCmd.String("root", ".", "repository root to analyze")
		jsonFlag := dgCmd.Bool("json", false, "output JSON")
		showFlag := dgCmd.String("show", "", "sub-view: stats, coverage, duplicates, fanout, impact, profile")
		changedFlag := dgCmd.String("changed", "", "comma-separated changed files for impact analysis")
		_ = dgCmd.Parse(os.Args[2:])
		if err := runDepgraph(*rootFlag, *jsonFlag, *showFlag, *changedFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "ai":
		if len(os.Args) < 3 {
			printAIUsage()
			os.Exit(2)
		}
		if isHelpArg(os.Args[2]) {
			printAIUsage()
			return
		}
		aiSub := os.Args[2]
		switch aiSub {
		case "list":
			aiCmd := flag.NewFlagSet("ai list", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			verboseFlag := aiCmd.Bool("verbose", false, "show detection evidence per surface")
			_ = aiCmd.Parse(os.Args[3:])
			if err := runAIList(*rootFlag, *jsonFlag, *verboseFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "run":
			aiCmd := flag.NewFlagSet("ai run", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			baseRef := aiCmd.String("base", "", "git base ref for impact-based scenario selection")
			fullFlag := aiCmd.Bool("full", false, "run all scenarios (skip impact selection)")
			dryRunFlag := aiCmd.Bool("dry-run", false, "show what would run without executing")
			_ = aiCmd.Parse(os.Args[3:])
			if err := runAIRun(*rootFlag, *jsonFlag, *baseRef, *fullFlag, *dryRunFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "replay":
			aiCmd := flag.NewFlagSet("ai replay", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			_ = aiCmd.Parse(os.Args[3:])
			args := aiCmd.Args()
			if len(args) == 0 {
				// Default to latest artifact.
				args = []string{filepath.Join(*rootFlag, ".terrain", "artifacts", "ai-run-latest.json")}
			}
			if err := runAIReplay(*rootFlag, *jsonFlag, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "record":
			aiCmd := flag.NewFlagSet("ai record", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			_ = aiCmd.Parse(os.Args[3:])
			if err := runAIRecord(*rootFlag, *jsonFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "baseline":
			aiCmd := flag.NewFlagSet("ai baseline", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			_ = aiCmd.Parse(os.Args[3:])
			// Check for sub-subcommand: terrain ai baseline compare
			baselineArgs := aiCmd.Args()
			if len(baselineArgs) > 0 && baselineArgs[0] == "compare" {
				if err := runAIBaselineCompare(*rootFlag, *jsonFlag); err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					os.Exit(1)
				}
				return
			}
			if err := runAIBaseline(*rootFlag, *jsonFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		case "doctor":
			aiCmd := flag.NewFlagSet("ai doctor", flag.ExitOnError)
			rootFlag := aiCmd.String("root", ".", "repository root to analyze")
			jsonFlag := aiCmd.Bool("json", false, "output JSON")
			_ = aiCmd.Parse(os.Args[3:])
			if err := runAIDoctor(*rootFlag, *jsonFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "error: unknown ai subcommand: %q\n\n", aiSub)
			printAIUsage()
			os.Exit(1)
		}

	case "feedback":
		url := "https://github.com/pmclSF/terrain/issues/new?template=feedback.md&title=Feedback:+&labels=feedback"
		fmt.Println("Open the following URL to share feedback:")
		fmt.Println()
		fmt.Printf("  %s\n", url)
		fmt.Println()
		fmt.Println("Or email: terrain-feedback@pmcl.dev")

	case "telemetry":
		if len(os.Args) < 3 {
			fmt.Println("Telemetry:", telemetry.Status())
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  terrain telemetry --on     enable local telemetry")
			fmt.Println("  terrain telemetry --off    disable local telemetry")
			fmt.Println("  terrain telemetry --status show current state")
			fmt.Println()
			fmt.Println("Telemetry records command name, repo size band, languages,")
			fmt.Println("signal count, and duration to ~/.terrain/telemetry.jsonl.")
			fmt.Println("No file paths, repo URLs, or PII are recorded.")
			fmt.Println("Override with TERRAIN_TELEMETRY=on|off environment variable.")
			return
		}
		switch os.Args[2] {
		case "--on", "on":
			if err := telemetry.SaveConfig(telemetry.Config{Enabled: true}); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Telemetry enabled. Events will be written to ~/.terrain/telemetry.jsonl")
		case "--off", "off":
			if err := telemetry.SaveConfig(telemetry.Config{Enabled: false}); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Telemetry disabled.")
		case "--status", "status":
			fmt.Println("Telemetry:", telemetry.Status())
		default:
			fmt.Fprintf(os.Stderr, "unknown telemetry subcommand: %q\n", os.Args[2])
			os.Exit(1)
		}

	case "version", "--version", "-v":
		versionCmd := flag.NewFlagSet("version", flag.ExitOnError)
		jsonFlag := versionCmd.Bool("json", false, "output JSON version info")
		_ = versionCmd.Parse(os.Args[2:])
		if *jsonFlag {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]string{
				"version": version,
				"commit":  commit,
				"date":    date,
			})
			return
		}
		fmt.Printf("terrain %s (commit %s, built %s)\n", version, commit, date)

	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		rootFlag := serveCmd.String("root", ".", "repository root to analyze")
		portFlag := serveCmd.Int("port", server.DefaultPort, "port to listen on")
		hostFlag := serveCmd.String("host", server.DefaultHost, "bind host (default 127.0.0.1; non-localhost values are unauthenticated and warned about)")
		readOnlyFlag := serveCmd.Bool("read-only", false, "forbid state-changing API endpoints (no-op in 0.1.2; reserved for 0.2)")
		_ = serveCmd.Parse(os.Args[2:])
		if err := runServe(*rootFlag, *portFlag, *hostFlag, *readOnlyFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

	case "--help", "-h", "help":
		printUsage()
	default:
		if _, ok := conv.LookupShorthand(os.Args[1]); ok {
			if err := runShorthandCLI(os.Args[1], os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(exitCodeForCLIError(err))
			}
			return
		}
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		if suggestions := didYouMean(os.Args[1], 3); len(suggestions) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Did you mean one of these?")
			for _, s := range suggestions {
				fmt.Fprintf(os.Stderr, "  terrain %s\n", s)
			}
		}
		fmt.Fprintln(os.Stderr)
		printUsage()
		os.Exit(exitUsageError)
	}
}

// knownCommands lists every top-level command the dispatcher accepts. Kept
// in sync with the switch statement in main(); when you add a new case,
// add it here so `terrain mistyped-name` can suggest it.
var knownCommands = []string{
	"analyze", "init", "impact", "explain", "insights", "summary",
	"focus", "posture", "portfolio", "metrics", "compare",
	"select-tests", "pr", "show", "policy", "export",
	"convert", "convert-config", "list", "list-conversions",
	"shorthands", "detect",
	"migration", "migrate", "estimate", "status", "checklist",
	"doctor", "reset",
	"ai", "feedback", "telemetry",
	"debug", "depgraph",
	"version", "serve", "help", "--help", "-h",
}

// didYouMean returns up to maxResults command names from knownCommands
// closest to candidate by Levenshtein distance, sorted nearest-first.
// Suggestions are emitted only for distance <= 2 — any further away is
// noisy more often than helpful.
func didYouMean(candidate string, maxResults int) []string {
	candidate = strings.ToLower(candidate)
	type scored struct {
		name string
		dist int
	}
	var ranked []scored
	for _, cmd := range knownCommands {
		d := levenshtein(candidate, strings.ToLower(cmd))
		if d <= 2 {
			ranked = append(ranked, scored{name: cmd, dist: d})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].dist != ranked[j].dist {
			return ranked[i].dist < ranked[j].dist
		}
		return ranked[i].name < ranked[j].name
	})
	if len(ranked) > maxResults {
		ranked = ranked[:maxResults]
	}
	out := make([]string, 0, len(ranked))
	for _, s := range ranked {
		out = append(out, s.name)
	}
	return out
}

// levenshtein returns the Levenshtein edit distance between a and b.
// Standard dynamic-programming implementation; O(len(a) * len(b)) time and
// O(min(len(a), len(b))) space.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			best := del
			if ins < best {
				best = ins
			}
			if sub < best {
				best = sub
			}
			curr[j] = best
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// initLogging scans args for --log-level=<level> or --log-level <level>
// and configures the global structured logger. This runs before subcommand
// parsing so all commands inherit the configured level.
func initLogging(args []string) {
	for i, arg := range args {
		var level string
		if strings.HasPrefix(arg, "--log-level=") {
			level = strings.TrimPrefix(arg, "--log-level=")
		} else if arg == "--log-level" && i+1 < len(args) {
			level = args[i+1]
		}
		if level != "" {
			logging.Init(logging.ParseLevel(level))
			return
		}
	}
	// Default: info level (already set by logging.init()).
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Terrain — test system intelligence platform")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Primary commands:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  analyze  [flags]         What is the state of our test system?")
	fmt.Fprintln(os.Stderr, "                           Example: terrain analyze --root ./myproject")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  impact   [flags]         What validations matter for this change?")
	fmt.Fprintln(os.Stderr, "                           Example: terrain impact --base main")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  insights [flags]         What should we fix in our test system?")
	fmt.Fprintln(os.Stderr, "                           Example: terrain insights --json")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  explain  <target>        Why did Terrain make this decision?")
	fmt.Fprintln(os.Stderr, "                           Example: terrain explain src/auth/login.test.ts")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Supporting commands:")
	fmt.Fprintln(os.Stderr, "  init [flags]             detect data paths and print recommended analyze command")
	fmt.Fprintln(os.Stderr, "  convert <source> [flags] inspect or execute Go-native conversion directions")
	fmt.Fprintln(os.Stderr, "  convert-config [flags]   convert framework config files with the Go-native runtime")
	fmt.Fprintln(os.Stderr, "  migrate <dir> [flags]    run project-wide Go-native conversion workflow")
	fmt.Fprintln(os.Stderr, "  estimate <dir> [flags]   estimate migration complexity without writing files")
	fmt.Fprintln(os.Stderr, "  status [flags]           show current migration progress")
	fmt.Fprintln(os.Stderr, "  checklist [flags]        generate the current migration checklist")
	fmt.Fprintln(os.Stderr, "  doctor [path] [flags]    run migration diagnostics for a directory")
	fmt.Fprintln(os.Stderr, "  reset [flags]            clear conversion migration state")
	fmt.Fprintln(os.Stderr, "  list-conversions [flags] list supported conversion directions")
	fmt.Fprintln(os.Stderr, "  shorthands [flags]       list shorthand conversion aliases")
	fmt.Fprintln(os.Stderr, "  detect <file-or-dir>     detect the dominant framework for a file or directory")
	fmt.Fprintln(os.Stderr, "  summary [flags]          executive summary with risk, trends, benchmark readiness")
	fmt.Fprintln(os.Stderr, "  focus [flags]            prioritized next actions")
	fmt.Fprintln(os.Stderr, "  posture [flags]          detailed posture breakdown with measurement evidence")
	fmt.Fprintln(os.Stderr, "  portfolio [flags]        portfolio intelligence: cost, breadth, leverage, redundancy")
	fmt.Fprintln(os.Stderr, "  select-tests [flags]     recommend protective test set for a change")
	fmt.Fprintln(os.Stderr, "  pr [flags]               PR/change-scoped analysis")
	fmt.Fprintln(os.Stderr, "  show <entity> <id>       drill into test, unit/codeunit, owner, or finding")
	fmt.Fprintln(os.Stderr, "  metrics [flags]          aggregate metrics scorecard")
	fmt.Fprintln(os.Stderr, "  compare [flags]          compare two snapshots for trend tracking")
	fmt.Fprintln(os.Stderr, "  migration <sub> [flags]  readiness, blockers, or preview")
	fmt.Fprintln(os.Stderr, "  policy check [flags]     evaluate local policy rules")
	fmt.Fprintln(os.Stderr, "  export benchmark [flags] privacy-safe JSON export for benchmarking")
	fmt.Fprintln(os.Stderr, "  serve [flags]            local HTTP server with HTML report and JSON API")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "AI / eval:")
	fmt.Fprintln(os.Stderr, "  ai list [flags]          list detected AI/eval scenarios and surfaces")
	fmt.Fprintln(os.Stderr, "  ai run [flags]           execute eval scenarios and collect results")
	fmt.Fprintln(os.Stderr, "  ai replay [flags]        replay and verify a previous eval run artifact")
	fmt.Fprintln(os.Stderr, "  ai record [flags]        record eval run results as a baseline snapshot")
	fmt.Fprintln(os.Stderr, "  ai baseline [flags]      manage eval baselines (show, compare, promote)")
	fmt.Fprintln(os.Stderr, "  ai doctor [flags]        validate AI/eval setup and configuration")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Advanced / debug:")
	fmt.Fprintln(os.Stderr, "  debug graph [flags]      dependency graph statistics")
	fmt.Fprintln(os.Stderr, "  debug coverage [flags]   structural coverage analysis")
	fmt.Fprintln(os.Stderr, "  debug fanout [flags]     high-fanout node analysis")
	fmt.Fprintln(os.Stderr, "  debug duplicates [flags] duplicate test cluster analysis")
	fmt.Fprintln(os.Stderr, "  debug depgraph [flags]   full dependency graph analysis (all engines)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Benchmark / validation (separate binaries):")
	fmt.Fprintln(os.Stderr, "  terrain-bench            run benchmark suite across repos (go run ./cmd/terrain-bench)")
	fmt.Fprintln(os.Stderr, "  terrain-convert-bench    compare Go converters against the legacy JS performance floor")
	fmt.Fprintln(os.Stderr, "  terrain-truthcheck       validate output against ground truth (go run ./cmd/terrain-truthcheck)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Common repo-scoped flags:")
	fmt.Fprintln(os.Stderr, "  --root PATH              repository root (default: current directory)")
	fmt.Fprintln(os.Stderr, "  --json                   machine-readable output where supported")
	fmt.Fprintln(os.Stderr, "  --base REF               git base ref for diff (impact, pr, select-tests)")
	fmt.Fprintln(os.Stderr, "  --log-level LEVEL        diagnostic verbosity: quiet, debug (default: info)")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Typical flow:")
	fmt.Fprintln(os.Stderr, "  1. terrain analyze                    understand your test system")
	fmt.Fprintln(os.Stderr, "  2. terrain insights                   find what to improve")
	fmt.Fprintln(os.Stderr, "  3. terrain impact                     see what a change affects")
	fmt.Fprintln(os.Stderr, "  4. terrain explain <target>           understand why")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Docs: docs/examples/{analyze,summary,insights,explain,focus,impact}-report.md")
}

func printMigrationUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain migration <readiness|blockers|preview> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Subcommands:")
	fmt.Fprintln(os.Stderr, "  readiness   assess migration readiness and risk")
	fmt.Fprintln(os.Stderr, "  blockers    list migration blockers and highest-risk areas")
	fmt.Fprintln(os.Stderr, "  preview     preview migration for a file or directory scope")
}

func printExportUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain export benchmark [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Exports a privacy-safe benchmark artifact as JSON.")
}

func printDebugUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain debug <graph|coverage|fanout|duplicates|depgraph> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Subcommands:")
	fmt.Fprintln(os.Stderr, "  graph       dependency graph statistics")
	fmt.Fprintln(os.Stderr, "  coverage    structural coverage analysis")
	fmt.Fprintln(os.Stderr, "  fanout      high-fanout node analysis")
	fmt.Fprintln(os.Stderr, "  duplicates  duplicate test cluster analysis")
	fmt.Fprintln(os.Stderr, "  depgraph    full dependency graph analysis (supports --show: stats, coverage, duplicates, fanout, impact, profile)")
}

func printAIUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain ai <list|run|replay|record|baseline|doctor> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  list       list detected AI/eval scenarios and surfaces")
	fmt.Fprintln(os.Stderr, "  run        execute eval scenarios and collect results")
	fmt.Fprintln(os.Stderr, "  replay     replay and verify a previous run artifact")
	fmt.Fprintln(os.Stderr, "  record     record eval run results as a baseline snapshot")
	fmt.Fprintln(os.Stderr, "  baseline   manage eval baselines (show, compare, promote)")
	fmt.Fprintln(os.Stderr, "  doctor     validate AI/eval setup and surface configuration issues")
}

func defaultPipelineOptions() engine.PipelineOptions {
	return engine.PipelineOptions{
		EngineVersion: version,
	}
}

// defaultPipelineOptionsWithProgress returns pipeline options with progress
// reporting enabled for interactive terminals. Pass jsonOutput=true to
// suppress progress (keeps stdout clean for JSON).
func defaultPipelineOptionsWithProgress(jsonOutput bool) engine.PipelineOptions {
	return engine.PipelineOptions{
		EngineVersion: version,
		OnProgress:    newProgressFunc(jsonOutput),
	}
}

func analysisPipelineOptions(coveragePath, coverageRunLabel string, runtimePaths []string, slowThreshold float64) engine.PipelineOptions {
	opt := defaultPipelineOptions()
	opt.CoveragePath = coveragePath
	opt.CoverageRunLabel = strings.TrimSpace(coverageRunLabel)
	opt.RuntimePaths = runtimePaths
	opt.SlowTestThresholdMs = slowThreshold
	return opt
}
