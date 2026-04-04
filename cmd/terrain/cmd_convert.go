package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	conv "github.com/pmclSF/terrain/internal/convert"
)

type convertCommandOptions struct {
	From              string
	To                string
	Output            string
	Config            string
	TestType          string
	Report            string
	ReportJSON        string
	PreserveStructure bool
	BatchSize         int
	Concurrency       int
	Validate          bool
	DryRun            bool
	Plan              bool
	AutoDetect        bool
	StrictValidate    bool
	Quiet             bool
	Verbose           bool
	JSON              bool
	OnError           string
	Alias             string
}

type convertPlanResult struct {
	Command         string          `json:"command"`
	Mode            string          `json:"mode"`
	Source          string          `json:"source"`
	Output          string          `json:"output,omitempty"`
	Alias           string          `json:"alias,omitempty"`
	Direction       conv.Direction  `json:"direction"`
	SourceDetection *conv.Detection `json:"sourceDetection,omitempty"`
	ExecutionStatus string          `json:"executionStatus"`
	NextStep        string          `json:"nextStep"`
}

type cliUsageError struct {
	message string
}

func (e cliUsageError) Error() string {
	return e.message
}

func exitCodeForCLIError(err error) int {
	var usageErr cliUsageError
	if errors.As(err, &usageErr) {
		return 2
	}
	return 1
}

func lookupConvertShorthand(alias string) (conv.Direction, bool) {
	return conv.LookupShorthand(alias)
}

func runConvertCLI(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts convertCommandOptions
	fs.StringVar(&opts.From, "from", "", "source framework")
	fs.StringVar(&opts.From, "f", "", "source framework")
	fs.StringVar(&opts.To, "to", "", "target framework")
	fs.StringVar(&opts.To, "t", "", "target framework")
	fs.StringVar(&opts.Output, "output", "", "output path for converted tests")
	fs.StringVar(&opts.Output, "o", "", "output path for converted tests")
	fs.StringVar(&opts.Config, "config", "", "custom configuration file path")
	fs.StringVar(&opts.TestType, "test-type", "", "test type (e2e, component, api, etc.)")
	fs.BoolVar(&opts.Validate, "validate", false, "validate converted tests")
	fs.StringVar(&opts.Report, "report", "", "generate conversion report (html, json, markdown)")
	fs.BoolVar(&opts.PreserveStructure, "preserve-structure", false, "maintain original directory structure")
	fs.IntVar(&opts.BatchSize, "batch-size", 5, "number of files per batch")
	fs.IntVar(&opts.Concurrency, "concurrency", 4, "number of files to convert in parallel in batch mode")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "show what would be converted without making changes")
	fs.BoolVar(&opts.Plan, "plan", false, "show structured conversion plan")
	fs.BoolVar(&opts.AutoDetect, "auto-detect", false, "auto-detect source framework from source content")
	fs.BoolVar(&opts.StrictValidate, "strict-validate", false, "enable parser-based syntax validation of converted output")
	fs.BoolVar(&opts.Quiet, "quiet", false, "suppress non-error output")
	fs.BoolVar(&opts.Quiet, "q", false, "suppress non-error output")
	fs.BoolVar(&opts.Verbose, "verbose", false, "detailed output")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.StringVar(&opts.OnError, "on-error", "skip", "error handling: skip|fail|best-effort")
	fs.StringVar(&opts.ReportJSON, "report-json", "", "write a structured JSON conversion report to the given file")

	if err := fs.Parse(reorderCLIArgs(args, convertFlagsWithValue)); err != nil {
		printConvertUsage()
		return cliUsageError{message: err.Error()}
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		printConvertUsage()
		return cliUsageError{message: "convert requires <source>"}
	}

	return runConvert(positionals[0], opts)
}

func runShorthandCLI(alias string, args []string) error {
	direction, ok := conv.LookupShorthand(alias)
	if !ok {
		return fmt.Errorf("unknown shorthand: %s", alias)
	}

	fs := flag.NewFlagSet(alias, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts convertCommandOptions
	opts.From = direction.From
	opts.To = direction.To
	opts.Alias = alias
	fs.StringVar(&opts.Output, "output", "", "output path for converted tests")
	fs.StringVar(&opts.Output, "o", "", "output path for converted tests")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "preview without writing")
	fs.BoolVar(&opts.Plan, "plan", false, "show structured conversion plan")
	fs.IntVar(&opts.Concurrency, "concurrency", 4, "number of files to convert in parallel in batch mode")
	fs.StringVar(&opts.OnError, "on-error", "skip", "error handling: skip|fail|best-effort")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.BoolVar(&opts.Verbose, "verbose", false, "detailed output")
	fs.BoolVar(&opts.Quiet, "quiet", false, "suppress non-error output")
	fs.BoolVar(&opts.Quiet, "q", false, "suppress non-error output")

	if err := fs.Parse(reorderCLIArgs(args, shorthandFlagsWithValue)); err != nil {
		printShorthandUsage(alias, direction)
		return cliUsageError{message: err.Error()}
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		printShorthandUsage(alias, direction)
		return cliUsageError{message: fmt.Sprintf("%s requires <source>", alias)}
	}

	return runConvert(positionals[0], opts)
}

func runListConversionsCLI(args []string) error {
	fs := flag.NewFlagSet("list-conversions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, nil)); err != nil {
		printListConversionsUsage()
		return cliUsageError{message: err.Error()}
	}
	return runListConversions(*jsonFlag)
}

func runShorthandsCLI(args []string) error {
	fs := flag.NewFlagSet("shorthands", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, nil)); err != nil {
		printShorthandsUsage()
		return cliUsageError{message: err.Error()}
	}
	return runShorthands(*jsonFlag)
}

func runDetectCLI(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonFlag := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(reorderCLIArgs(args, nil)); err != nil {
		printDetectUsage()
		return cliUsageError{message: err.Error()}
	}
	positionals := fs.Args()
	if len(positionals) == 0 {
		printDetectUsage()
		return cliUsageError{message: "detect requires <file-or-dir>"}
	}
	return runDetect(positionals[0], *jsonFlag)
}

func runConvert(source string, opts convertCommandOptions) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return cliUsageError{message: "convert requires <source>"}
	}

	var detection *conv.Detection
	from := conv.NormalizeFramework(opts.From)
	to := conv.NormalizeFramework(opts.To)

	if from == "" && opts.AutoDetect {
		detected, err := conv.DetectSource(source)
		if err != nil {
			return err
		}
		detection = &detected
		if detected.Framework == "" || detected.Framework == "unknown" {
			return fmt.Errorf("could not auto-detect source framework from %s", source)
		}
		from = detected.Framework
	}

	if from == "" {
		return cliUsageError{message: "--from <framework> is required unless --auto-detect is set"}
	}
	if to == "" {
		return cliUsageError{message: "--to <framework> is required"}
	}

	if _, ok := conv.LookupFramework(from); !ok {
		return cliUsageError{message: fmt.Sprintf("invalid source framework: %s. Valid options: %s", from, strings.Join(conv.FrameworkNames(), ", "))}
	}
	if _, ok := conv.LookupFramework(to); !ok {
		return cliUsageError{message: fmt.Sprintf("invalid target framework: %s. Valid options: %s", to, strings.Join(conv.FrameworkNames(), ", "))}
	}
	if from == to {
		return cliUsageError{message: "source and target frameworks must be different"}
	}

	direction, ok := conv.LookupDirection(from, to)
	if !ok {
		targets := conv.SupportedTargets(from)
		if len(targets) == 0 {
			return cliUsageError{message: fmt.Sprintf("unsupported source framework: %s", from)}
		}
		return cliUsageError{message: fmt.Sprintf("unsupported conversion: %s to %s. Supported targets for %s: %s", from, to, from, strings.Join(targets, ", "))}
	}

	if !opts.Plan && !opts.DryRun {
		if direction.GoNativeState != conv.GoNativeStateImplemented {
			return fmt.Errorf("go-native conversion execution for %s -> %s is not implemented yet; use --plan to inspect the migration path while the runtime is being ported", from, to)
		}
		return runConvertExecution(source, direction, opts)
	}

	mode := "plan"
	if opts.DryRun && !opts.Plan {
		mode = "dry-run"
	}
	result := convertPlanResult{
		Command:         "convert",
		Mode:            mode,
		Source:          source,
		Output:          opts.Output,
		Alias:           opts.Alias,
		Direction:       direction,
		SourceDetection: detection,
		ExecutionStatus: "cataloged-not-executable",
		NextStep:        "The Go CLI now owns the conversion catalog, shorthands, and detection contract. Execution for this direction will land in follow-up migration slices.",
	}
	if direction.GoNativeState == conv.GoNativeStateImplemented {
		result.ExecutionStatus = "executable"
		result.NextStep = "Run the same command without --plan to execute the Go-native converter for this direction."
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Go-native conversion plan")
	fmt.Println()
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Direction: %s -> %s\n", direction.From, direction.To)
	fmt.Printf("  Language: %s\n", direction.Language)
	fmt.Printf("  Category: %s\n", direction.Category)
	fmt.Printf("  Shorthands: %s\n", strings.Join(direction.Shorthands, ", "))
	fmt.Printf("  Legacy runtime: %s\n", direction.LegacyRuntime)
	fmt.Printf("  Go-native state: %s\n", humanizeGoNativeState(direction.GoNativeState))
	fmt.Printf("  Execution: %s\n", result.ExecutionStatus)
	if result.Output != "" {
		fmt.Printf("  Output: %s\n", result.Output)
	}
	if detection != nil {
		fmt.Printf("  Auto-detected source: %s (%.0f%% confidence via %s)\n", detection.Framework, detection.Confidence*100, detection.DetectionSource)
	}
	fmt.Println()
	fmt.Println("Next:")
	fmt.Printf("  %s\n", result.NextStep)
	return nil
}

func runConvertExecution(source string, direction conv.Direction, opts convertCommandOptions) error {
	result, err := conv.Execute(source, direction, conv.ExecuteOptions{
		Output:            opts.Output,
		PreserveStructure: opts.PreserveStructure,
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Mode == "stdout" {
		fmt.Print(result.StdoutContent)
		return nil
	}

	fmt.Println("Go-native conversion complete")
	fmt.Println()
	fmt.Printf("  Direction: %s -> %s\n", direction.From, direction.To)
	fmt.Printf("  Mode: %s\n", result.Mode)
	if result.Output != "" {
		fmt.Printf("  Output: %s\n", result.Output)
	}
	fmt.Printf("  Converted files: %d\n", result.ConvertedCount)
	if result.UnchangedCount > 0 {
		fmt.Printf("  Unchanged files: %d\n", result.UnchangedCount)
	}
	return nil
}

func runListConversions(jsonOutput bool) error {
	categories := conv.Categories()
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Categories []conv.DirectionCategory `json:"categories"`
		}{Categories: categories})
	}

	fmt.Println("Supported conversion directions")
	fmt.Println()
	for _, category := range categories {
		fmt.Printf("  %s\n", category.Name)
		for _, direction := range category.Directions {
			fmt.Printf("    %-14s -> %-14s %-18s [%s]\n",
				direction.From,
				direction.To,
				strings.Join(direction.Shorthands, ", "),
				humanizeGoNativeState(direction.GoNativeState),
			)
		}
		fmt.Println()
	}
	fmt.Println("Use `terrain convert <source> --from <framework> --to <framework> --plan` to inspect the Go-native migration path for a direction.")
	return nil
}

func runShorthands(jsonOutput bool) error {
	entries := conv.Shorthands()
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Shorthands []conv.Shorthand `json:"shorthands"`
		}{Shorthands: entries})
	}

	fmt.Println("Shorthand command aliases")
	fmt.Println()
	fmt.Printf("  %-18s %-14s %-14s %s\n", "Alias", "From", "To", "State")
	fmt.Printf("  %-18s %-14s %-14s %s\n", strings.Repeat("-", 18), strings.Repeat("-", 14), strings.Repeat("-", 14), strings.Repeat("-", 11))
	for _, entry := range entries {
		fmt.Printf("  %-18s %-14s %-14s %s\n", entry.Alias, entry.From, entry.To, humanizeGoNativeState(entry.GoNativeState))
	}
	fmt.Println()
	fmt.Println("Use a shorthand with `--plan` or `--dry-run` while Go-native execution is being ported.")
	return nil
}

func runDetect(path string, jsonOutput bool) error {
	detection, err := conv.DetectSource(path)
	if err != nil {
		return err
	}
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(detection)
	}

	fmt.Println("Framework detection")
	fmt.Println()
	fmt.Printf("  Path: %s\n", path)
	fmt.Printf("  Mode: %s\n", detection.Mode)
	fmt.Printf("  Detected framework: %s\n", detection.Framework)
	fmt.Printf("  Confidence: %.0f%%\n", detection.Confidence*100)
	if detection.Language != "" {
		fmt.Printf("  Language: %s\n", detection.Language)
	}
	if detection.Category != "" {
		fmt.Printf("  Category: %s\n", detection.Category)
	}
	if detection.DetectionSource != "" {
		fmt.Printf("  Detection source: %s\n", detection.DetectionSource)
	}
	if detection.Mode == "directory" {
		fmt.Printf("  Files scanned: %d\n", detection.FilesScanned)
	}
	return nil
}

var convertFlagsWithValue = map[string]bool{
	"--from":        true,
	"-f":            true,
	"--to":          true,
	"-t":            true,
	"--output":      true,
	"-o":            true,
	"--config":      true,
	"--test-type":   true,
	"--report":      true,
	"--batch-size":  true,
	"--concurrency": true,
	"--on-error":    true,
	"--report-json": true,
}

var shorthandFlagsWithValue = map[string]bool{
	"--output":      true,
	"-o":            true,
	"--concurrency": true,
	"--on-error":    true,
}

func reorderCLIArgs(args []string, flagsWithValue map[string]bool) []string {
	if len(args) == 0 {
		return nil
	}

	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			continue
		}
		if flagsWithValue != nil && flagsWithValue[arg] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	return append(flags, positionals...)
}

func humanizeGoNativeState(state conv.GoNativeState) string {
	switch state {
	case conv.GoNativeStateImplemented:
		return "implemented"
	case conv.GoNativeStatePrioritized:
		return "prioritized"
	default:
		return "cataloged"
	}
}

func printConvertUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain convert <source> --from <framework> --to <framework> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Current status:")
	fmt.Fprintln(os.Stderr, "  This Go-native foundation supports direction cataloging, shorthands, framework detection,")
	fmt.Fprintln(os.Stderr, "  and executable `jest -> vitest`, `cypress -> playwright`, `cypress -> webdriverio`,")
	fmt.Fprintln(os.Stderr, "  `playwright -> cypress`, `playwright -> puppeteer`, `playwright -> webdriverio`,")
	fmt.Fprintln(os.Stderr, "  `puppeteer -> playwright`, `webdriverio -> cypress`, plus `webdriverio -> playwright` conversion.")
	fmt.Fprintln(os.Stderr, "  Other directions remain plan-only for now.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Key flags:")
	fmt.Fprintln(os.Stderr, "  --from, -f         source framework")
	fmt.Fprintln(os.Stderr, "  --to, -t           target framework")
	fmt.Fprintln(os.Stderr, "  --output, -o       write converted output to a file or directory")
	fmt.Fprintln(os.Stderr, "  --auto-detect      detect the source framework from the file or directory")
	fmt.Fprintln(os.Stderr, "  --plan             show the Go-native migration plan for this direction")
	fmt.Fprintln(os.Stderr, "  --dry-run          same as plan, but framed as a no-write preview")
	fmt.Fprintln(os.Stderr, "  --json             machine-readable output")
}

func printListConversionsUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain list-conversions [--json]")
}

func printShorthandsUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain shorthands [--json]")
}

func printDetectUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain detect <file-or-dir> [--json]")
}

func printShorthandUsage(alias string, direction conv.Direction) {
	fmt.Fprintf(os.Stderr, "Usage: terrain %s <source> [--plan|--dry-run] [--json]\n", alias)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Direction: %s -> %s\n", direction.From, direction.To)
}
