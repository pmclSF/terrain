package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	conv "github.com/pmclSF/terrain/internal/convert"
)

type migrateCommandOptions struct {
	From           string
	To             string
	Output         string
	Concurrency    int
	Continue       bool
	RetryFailed    bool
	DryRun         bool
	Plan           bool
	JSON           bool
	StrictValidate bool
}

func runMigrateCLI(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts migrateCommandOptions
	fs.StringVar(&opts.From, "from", "", "source framework")
	fs.StringVar(&opts.From, "f", "", "source framework")
	fs.StringVar(&opts.To, "to", "", "target framework")
	fs.StringVar(&opts.To, "t", "", "target framework")
	fs.StringVar(&opts.Output, "output", "", "output directory for converted files")
	fs.StringVar(&opts.Output, "o", "", "output directory for converted files")
	fs.IntVar(&opts.Concurrency, "concurrency", 4, "number of files to convert in parallel during migration")
	fs.BoolVar(&opts.Continue, "continue", false, "resume a previously started migration")
	fs.BoolVar(&opts.RetryFailed, "retry-failed", false, "retry only previously failed files")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "preview migration without writing files")
	fs.BoolVar(&opts.Plan, "plan", false, "show a structured migration plan")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.BoolVar(&opts.StrictValidate, "strict-validate", false, "validate converted output syntax before recording files as converted")

	if err := fs.Parse(reorderCLIArgs(args, workflowFlagsWithValue)); err != nil {
		printMigrateUsage()
		return cliUsageError{message: err.Error()}
	}
	positionals := fs.Args()
	if len(positionals) == 0 {
		printMigrateUsage()
		return cliUsageError{message: "migrate requires <dir>"}
	}
	return runMigrate(positionals[0], opts)
}

func runEstimateCLI(args []string) error {
	fs := flag.NewFlagSet("estimate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts migrateCommandOptions
	fs.StringVar(&opts.From, "from", "", "source framework")
	fs.StringVar(&opts.From, "f", "", "source framework")
	fs.StringVar(&opts.To, "to", "", "target framework")
	fs.StringVar(&opts.To, "t", "", "target framework")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")

	if err := fs.Parse(reorderCLIArgs(args, workflowFlagsWithValue)); err != nil {
		printEstimateUsage()
		return cliUsageError{message: err.Error()}
	}
	positionals := fs.Args()
	if len(positionals) == 0 {
		printEstimateUsage()
		return cliUsageError{message: "estimate requires <dir>"}
	}
	return runEstimate(positionals[0], opts.From, opts.To, opts.JSON)
}

func runStatusCLI(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dirFlag := fs.String("dir", ".", "project directory")
	fs.StringVar(dirFlag, "d", ".", "project directory")
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, workflowSimpleFlagsWithValue)); err != nil {
		printStatusUsage()
		return cliUsageError{message: err.Error()}
	}
	return runStatus(*dirFlag, *jsonFlag)
}

func runChecklistCLI(args []string) error {
	fs := flag.NewFlagSet("checklist", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dirFlag := fs.String("dir", ".", "project directory")
	fs.StringVar(dirFlag, "d", ".", "project directory")
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, workflowSimpleFlagsWithValue)); err != nil {
		printChecklistUsage()
		return cliUsageError{message: err.Error()}
	}
	return runChecklist(*dirFlag, *jsonFlag)
}

func runDoctorCLI(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonFlag := fs.Bool("json", false, "JSON output")
	verboseFlag := fs.Bool("verbose", false, "show extra detail for each check")
	if err := fs.Parse(reorderCLIArgs(args, nil)); err != nil {
		printDoctorUsage()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	target := "."
	if len(fs.Args()) > 0 {
		target = fs.Args()[0]
	}
	result, err := runDoctor(target, *jsonFlag, *verboseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if result.HasFail {
		return 1
	}
	return 0
}

func runResetCLI(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dirFlag := fs.String("dir", ".", "project directory")
	fs.StringVar(dirFlag, "d", ".", "project directory")
	yesFlag := fs.Bool("yes", false, "skip confirmation prompt")
	fs.BoolVar(yesFlag, "y", false, "skip confirmation prompt")
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, workflowSimpleFlagsWithValue)); err != nil {
		printResetUsage()
		return cliUsageError{message: err.Error()}
	}
	return runReset(*dirFlag, *yesFlag, *jsonFlag)
}

func runMigrate(root string, opts migrateCommandOptions) error {
	if strings.TrimSpace(opts.From) == "" {
		return cliUsageError{message: "--from <framework> is required"}
	}
	if strings.TrimSpace(opts.To) == "" {
		return cliUsageError{message: "--to <framework> is required"}
	}

	if opts.DryRun || opts.Plan {
		estimate, err := conv.EstimateMigration(root, opts.From, opts.To)
		if err != nil {
			return err
		}
		if opts.JSON {
			return writeJSON(estimate)
		}
		if opts.Plan {
			printEstimatePlan(root, estimate)
			return nil
		}
		printEstimateSummary(root, estimate, true)
		return nil
	}

	result, err := conv.MigrateProject(root, opts.From, opts.To, conv.MigrationRunOptions{
		Output:         opts.Output,
		Concurrency:    opts.Concurrency,
		Continue:       opts.Continue,
		RetryFailed:    opts.RetryFailed,
		StrictValidate: opts.StrictValidate,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(result)
	}

	fmt.Println("Go-native migration complete")
	fmt.Println()
	fmt.Printf("  Root: %s\n", result.Root)
	fmt.Printf("  Direction: %s -> %s\n", result.From, result.To)
	if result.Output != "" {
		fmt.Printf("  Output: %s\n", result.Output)
	}
	fmt.Printf("  Converted: %d\n", result.State.Converted)
	fmt.Printf("  Failed: %d\n", result.State.Failed)
	fmt.Printf("  Skipped: %d\n", result.State.Skipped)
	if len(result.Processed) > 0 {
		fmt.Println()
		fmt.Println("Processed:")
		for _, record := range result.Processed {
			line := fmt.Sprintf("  - %s [%s]", record.InputPath, record.Status)
			if record.Confidence > 0 {
				line += fmt.Sprintf(" (%d%%)", record.Confidence)
			}
			if record.OutputPath != "" {
				line += " -> " + record.OutputPath
			}
			if record.SkipReason != "" {
				line += " — " + record.SkipReason
			}
			if record.Error != "" {
				line += " — " + record.Error
			}
			fmt.Println(line)
		}
	}
	return nil
}

func runEstimate(root, from, to string, jsonOutput bool) error {
	if strings.TrimSpace(from) == "" {
		return cliUsageError{message: "--from <framework> is required"}
	}
	if strings.TrimSpace(to) == "" {
		return cliUsageError{message: "--to <framework> is required"}
	}
	estimate, err := conv.EstimateMigration(root, from, to)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(estimate)
	}
	printEstimateSummary(root, estimate, false)
	return nil
}

func runStatus(root string, jsonOutput bool) error {
	status, exists, err := conv.LoadMigrationStatus(root)
	if err != nil {
		return err
	}
	if !exists {
		if jsonOutput {
			return writeJSON(map[string]any{
				"exists":  false,
				"message": "No migration in progress. Run `terrain migrate` to start.",
				"status":  conv.MigrationStatus{},
			})
		}
		fmt.Println("No migration in progress. Run `terrain migrate` to start.")
		return nil
	}
	if jsonOutput {
		return writeJSON(map[string]any{
			"exists": true,
			"status": status,
		})
	}
	fmt.Println("Migration status")
	fmt.Println()
	fmt.Printf("  Source: %s\n", emptyFallback(status.Source, "unknown"))
	fmt.Printf("  Target: %s\n", emptyFallback(status.Target, "unknown"))
	fmt.Printf("  Started: %s\n", emptyFallback(status.StartedAt, "unknown"))
	fmt.Printf("  Updated: %s\n", emptyFallback(status.UpdatedAt, "unknown"))
	fmt.Printf("  Converted: %d\n", status.Converted)
	fmt.Printf("  Failed: %d\n", status.Failed)
	fmt.Printf("  Skipped: %d\n", status.Skipped)
	fmt.Printf("  Total tracked: %d\n", status.Total)
	return nil
}

func runChecklist(root string, jsonOutput bool) error {
	checklist, exists, err := conv.GenerateChecklistFromState(root)
	if err != nil {
		return err
	}
	if !exists {
		if jsonOutput {
			return writeJSON(map[string]any{
				"exists":    false,
				"message":   "No migration in progress. Run `terrain migrate` to start.",
				"checklist": "",
			})
		}
		fmt.Println("No migration in progress. Run `terrain migrate` to start.")
		return nil
	}
	if jsonOutput {
		return writeJSON(map[string]any{
			"exists":    true,
			"checklist": checklist,
		})
	}
	fmt.Print(checklist)
	return nil
}

func runDoctor(root string, jsonOutput, verbose bool) (conv.MigrationDoctorResult, error) {
	result, err := conv.RunMigrationDoctor(root)
	if err != nil {
		return conv.MigrationDoctorResult{}, err
	}
	if jsonOutput {
		return result, writeJSON(result)
	}
	fmt.Println("Terrain Doctor")
	fmt.Println()
	for _, check := range result.Checks {
		fmt.Printf("  [%s] %s: %s\n", check.Status, check.Label, check.Detail)
		if verbose && strings.TrimSpace(check.Verbose) != "" {
			fmt.Printf("         %s\n", check.Verbose)
		}
		if strings.TrimSpace(check.Remediation) != "" {
			fmt.Printf("         -> %s\n", check.Remediation)
		}
	}
	fmt.Println()
	fmt.Printf("  %d checks: %d passed, %d warnings, %d failed\n", result.Summary.Total, result.Summary.Pass, result.Summary.Warn, result.Summary.Fail)
	return result, nil
}

func runReset(root string, yes, jsonOutput bool) error {
	if !yes {
		if jsonOutput {
			return writeJSON(map[string]any{
				"cleared": false,
				"message": "Use --yes to confirm removing conversion migration state.",
			})
		}
		fmt.Println("This will remove Terrain conversion migration state under .terrain/migration.")
		fmt.Println("Use --yes to confirm.")
		return nil
	}

	cleared, err := conv.ResetMigrationState(root)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(map[string]any{
			"cleared": cleared,
		})
	}
	if !cleared {
		fmt.Println("No migration state to reset.")
		return nil
	}
	fmt.Println("Migration state cleared.")
	return nil
}

func printEstimateSummary(root string, estimate conv.MigrationEstimate, dryRun bool) {
	if dryRun {
		fmt.Println("Dry run mode - no files will be modified")
		fmt.Println()
	}
	fmt.Printf("Estimating migration for %s...\n", root)
	fmt.Println()
	fmt.Println("Estimation summary:")
	fmt.Printf("  Total files: %d\n", estimate.Summary.TotalFiles)
	fmt.Printf("  Test files: %d\n", estimate.Summary.TestFiles)
	fmt.Printf("  Helper files: %d\n", estimate.Summary.HelperFiles)
	fmt.Printf("  Config files: %d\n", estimate.Summary.ConfigFiles)
	fmt.Printf("  High confidence: %d\n", estimate.Summary.PredictedHigh)
	fmt.Printf("  Medium confidence: %d\n", estimate.Summary.PredictedMedium)
	fmt.Printf("  Low confidence: %d\n", estimate.Summary.PredictedLow)
	if len(estimate.Blockers) > 0 {
		fmt.Println()
		fmt.Println("Top blockers:")
		for _, blocker := range estimate.Blockers {
			fmt.Printf("  - %s (%d)\n", blocker.Pattern, blocker.Count)
		}
	}
	fmt.Println()
	fmt.Println("Effort estimate:")
	fmt.Printf("  %s\n", estimate.EstimatedEffort.Description)
	if estimate.EstimatedEffort.EstimatedManualMins > 0 {
		fmt.Printf("  Estimated manual time: ~%d minutes\n", estimate.EstimatedEffort.EstimatedManualMins)
	}
}

func printEstimatePlan(root string, estimate conv.MigrationEstimate) {
	fmt.Printf("Migration plan: %s -> %s\n", estimate.From, estimate.To)
	fmt.Printf("  Directory: %s\n", root)
	fmt.Println()
	fmt.Println("  Files:")
	fmt.Printf("    Test files:   %d\n", estimate.Summary.TestFiles)
	fmt.Printf("    Config files: %d\n", estimate.Summary.ConfigFiles)
	fmt.Printf("    Helper files: %d\n", estimate.Summary.HelperFiles)
	if len(estimate.Files) > 0 {
		fmt.Println()
		fmt.Printf("  %-36s %-12s %s\n", "Input", "Type", "Confidence")
		for _, file := range estimate.Files {
			level := "low"
			switch {
			case file.Confidence >= 90:
				level = "high"
			case file.Confidence >= 70:
				level = "medium"
			}
			fmt.Printf("  %-36s %-12s %s\n", file.InputPath, file.Type, level)
		}
	}
	fmt.Println()
	fmt.Printf("  Summary: %d high, %d medium, %d low\n", estimate.Summary.PredictedHigh, estimate.Summary.PredictedMedium, estimate.Summary.PredictedLow)
	if len(estimate.Blockers) == 0 {
		fmt.Println("  Warnings: (none)")
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var workflowFlagsWithValue = map[string]bool{
	"--from":        true,
	"-f":            true,
	"--to":          true,
	"-t":            true,
	"--output":      true,
	"-o":            true,
	"--concurrency": true,
}

var workflowSimpleFlagsWithValue = map[string]bool{
	"--dir": true,
	"-d":    true,
}

func printMigrateUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain migrate <dir> --from <framework> --to <framework> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  -o, --output PATH     output directory for converted files")
	fmt.Fprintln(os.Stderr, "      --concurrency N   number of files to convert in parallel")
	fmt.Fprintln(os.Stderr, "      --continue        resume a previously started migration")
	fmt.Fprintln(os.Stderr, "      --retry-failed    retry only previously failed files")
	fmt.Fprintln(os.Stderr, "      --dry-run         preview migration without writing files")
	fmt.Fprintln(os.Stderr, "      --plan            show a structured migration plan")
	fmt.Fprintln(os.Stderr, "      --json            machine-readable output")
	fmt.Fprintln(os.Stderr, "      --strict-validate validate converted output syntax before keeping files")
}

func printEstimateUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain estimate <dir> --from <framework> --to <framework> [--json]")
}

func printStatusUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain status [--dir PATH] [--json]")
}

func printChecklistUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain checklist [--dir PATH] [--json]")
}

func printDoctorUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain doctor [path] [--json] [--verbose]")
}

func printResetUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain reset [--dir PATH] [--yes] [--json]")
}
