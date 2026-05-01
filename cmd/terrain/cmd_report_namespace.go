package main

import (
	"flag"
	"fmt"
	"strings"
)

// Phase A of the 0.2 CLI restructure folds the 11 read-side top-level
// commands into one noun: `terrain report`. The canonical shape:
//
//   terrain report summary       (was: summary)
//   terrain report insights      (was: insights)
//   terrain report metrics       (was: metrics)
//   terrain report explain <id>  (was: explain)
//   terrain report show ...      (was: show)
//   terrain report impact        (was: impact)
//   terrain report pr            (was: pr)
//   terrain report posture       (was: posture)
//   terrain report select-tests  (was: select-tests)
//
// The `focus → --focus=<path>` and `export → --output=<path>` flag
// collapses are DEFERRED to Phase B — the underlying runners
// (runFocus, runExport*) don't yet accept the path/output parameters
// these flags would set, so wiring the flags here would silently drop
// the user's value. Until Phase B lands the runner-side plumbing,
// use the legacy top-level commands (`terrain focus`, `terrain
// export`).
//
// The 9 read-side legacy top-level commands keep working unchanged
// through 0.2; they get a deprecation note in 0.2.x and removal in 0.3.

// reportVerbs is the canonical-verb allowlist. Used by the dispatcher
// and by the help text on bare `terrain report`.
var reportVerbs = []string{
	"summary",
	"insights",
	"metrics",
	"explain",
	"show",
	"impact",
	"pr",
	"posture",
	"select-tests",
}

// runReportNamespaceCLI dispatches `terrain report <verb> ...`.
func runReportNamespaceCLI(args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printReportUsage()
		if len(args) == 0 {
			return fmt.Errorf("terrain report: missing verb")
		}
		return nil
	}

	verb := args[0]
	rest := args[1:]
	switch verb {
	case "summary":
		return runReportSummaryCLI(rest)
	case "insights":
		return runReportInsightsCLI(rest)
	case "metrics":
		return runReportMetricsCLI(rest)
	case "explain":
		return runReportExplainCLI(rest)
	case "show":
		return runReportShowCLI(rest)
	case "impact":
		return runReportImpactCLI(rest)
	case "pr":
		return runReportPRCLI(rest)
	case "posture":
		return runReportPostureCLI(rest)
	case "select-tests":
		return runReportSelectTestsCLI(rest)
	default:
		printReportUsage()
		return fmt.Errorf("unknown report verb %q (valid: %s)", verb, strings.Join(reportVerbs, ", "))
	}
}

func printReportUsage() {
	fmt.Println("Usage: terrain report <verb> [flags]")
	fmt.Println()
	fmt.Println("Read-side queries over the analysis snapshot.")
	fmt.Println()
	fmt.Println("Verbs:")
	fmt.Println("  summary       high-level snapshot summary with heatmap")
	fmt.Println("  insights      derived health insights")
	fmt.Println("  metrics       metric breakdowns")
	fmt.Println("  explain <id>  explain a finding, scenario, or test selection")
	fmt.Println("  show <kind>   render a snapshot subset (test, code, surface, …)")
	fmt.Println("  impact        change-set impact analysis (--base=<ref>)")
	fmt.Println("  pr            PR-level summary (--format=markdown|comment|annotation)")
	fmt.Println("  posture       release readiness posture")
	fmt.Println("  select-tests  protective test selection for a change")
	fmt.Println()
	fmt.Println("Common flags (all verbs):")
	fmt.Println("  --root <path>       repository root (default .)")
	fmt.Println("  --json              JSON output")
	fmt.Println("  --verbose           extra detail")
}

// --- per-verb argument parsers -------------------------------------------

func runReportSummaryCLI(args []string) error {
	fs := flag.NewFlagSet("report summary", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	jsonOut := fs.Bool("json", false, "output JSON summary with heatmap")
	verbose := fs.Bool("verbose", false, "show detailed heatmap breakdown")
	_ = fs.Parse(args)
	return runSummary(*root, *jsonOut, *verbose)
}

func runReportInsightsCLI(args []string) error {
	fs := flag.NewFlagSet("report insights", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	jsonOut := fs.Bool("json", false, "output JSON insights")
	verbose := fs.Bool("verbose", false, "show per-finding evidence and file details")
	_ = fs.Parse(args)
	return runInsights(*root, *jsonOut, *verbose)
}

func runReportMetricsCLI(args []string) error {
	fs := flag.NewFlagSet("report metrics", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	jsonOut := fs.Bool("json", false, "output JSON metrics snapshot")
	verbose := fs.Bool("verbose", false, "show detailed metric breakdowns")
	_ = fs.Parse(args)
	return runMetrics(*root, *jsonOut, *verbose)
}

func runReportExplainCLI(args []string) error {
	fs := flag.NewFlagSet("report explain", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	baseRef := fs.String("base", "", "git base ref for diff (default: HEAD~1)")
	jsonOut := fs.Bool("json", false, "output JSON")
	verbose := fs.Bool("verbose", false, "show detection evidence, tiers, and confidence details")
	flagsWithValue := map[string]bool{"--root": true, "--base": true}
	_ = fs.Parse(reorderCLIArgs(args, flagsWithValue))
	pos := fs.Args()
	if len(pos) == 0 {
		return fmt.Errorf("terrain report explain: target required (test path, code unit, scenario id, owner, or 'selection')")
	}
	return runExplain(pos[0], *root, *baseRef, *jsonOut, *verbose)
}

func runReportShowCLI(args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printShowUsage()
		if len(args) == 0 {
			return fmt.Errorf("terrain report show: missing kind")
		}
		return nil
	}
	// Same flexible-position parser as the legacy `show` entry point.
	var positional []string
	jsonOut := false
	root := "."
	for _, arg := range args {
		switch {
		case arg == "--json" || arg == "-json":
			jsonOut = true
		case strings.HasPrefix(arg, "--root="):
			root = strings.TrimPrefix(arg, "--root=")
		case strings.HasPrefix(arg, "-root="):
			root = strings.TrimPrefix(arg, "-root=")
		case arg == "--root" || arg == "-root":
			root = ""
		default:
			if root == "" {
				root = arg
			} else {
				positional = append(positional, arg)
			}
		}
	}
	if root == "" {
		root = "."
	}
	if len(positional) == 0 {
		return fmt.Errorf("terrain report show: missing kind")
	}
	id := ""
	if len(positional) > 1 {
		id = positional[1]
	}
	return runShow(positional[0], id, root, jsonOut)
}

func runReportImpactCLI(args []string) error {
	fs := flag.NewFlagSet("report impact", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	baseRef := fs.String("base", "", "git base ref for diff (default: HEAD~1)")
	jsonOut := fs.Bool("json", false, "output JSON impact result")
	show := fs.String("show", "", "drill-down view: units, gaps, tests, owners, graph, selected")
	owner := fs.String("owner", "", "filter results by owner")
	_ = fs.Parse(args)
	return runImpact(*root, *baseRef, *jsonOut, *show, *owner)
}

func runReportPRCLI(args []string) error {
	fs := flag.NewFlagSet("report pr", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	baseRef := fs.String("base", "", "git base ref for diff (default: HEAD~1)")
	jsonOut := fs.Bool("json", false, "output JSON PR analysis")
	format := fs.String("format", "", "output format: markdown, comment, annotation")
	_ = fs.Parse(args)
	return runPR(*root, *baseRef, *jsonOut, *format)
}

func runReportPostureCLI(args []string) error {
	fs := flag.NewFlagSet("report posture", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	jsonOut := fs.Bool("json", false, "output JSON posture snapshot")
	verbose := fs.Bool("verbose", false, "show measurement values and thresholds")
	_ = fs.Parse(args)
	return runPosture(*root, *jsonOut, *verbose)
}

func runReportSelectTestsCLI(args []string) error {
	fs := flag.NewFlagSet("report select-tests", flag.ExitOnError)
	root := fs.String("root", ".", "repository root to analyze")
	baseRef := fs.String("base", "", "git base ref for diff (default: HEAD~1)")
	jsonOut := fs.Bool("json", false, "output JSON protective test set")
	_ = fs.Parse(args)
	return runSelectTests(*root, *baseRef, *jsonOut)
}
