package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	conv "github.com/pmclSF/terrain/internal/convert"
)

type convertCommandOptions struct {
	From              string
	To                string
	Output            string
	PreserveStructure bool
	BatchSize         int
	Concurrency       int
	Validate          bool
	DryRun            bool
	Plan              bool
	Preview           bool
	AutoDetect        bool
	StrictValidate    bool
	JSON              bool
	OnError           string
	Alias             string
}

type cliUsageError struct {
	message string
}

func (e cliUsageError) Error() string {
	return e.message
}

// cliExitError carries a specific exit code through the error return path,
// allowing functions to signal non-zero exit without calling os.Exit() directly.
type cliExitError struct {
	code    int
	message string
}

func (e cliExitError) Error() string {
	return e.message
}

func exitCodeForCLIError(err error) int {
	var exitErr cliExitError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	var usageErr cliUsageError
	if errors.As(err, &usageErr) {
		return 2
	}
	return 1
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
	fs.BoolVar(&opts.Validate, "validate", true, "validate converted output before returning or writing it")
	fs.BoolVar(&opts.PreserveStructure, "preserve-structure", false, "maintain original directory structure")
	fs.IntVar(&opts.BatchSize, "batch-size", 5, "number of files per batch")
	fs.IntVar(&opts.Concurrency, "concurrency", 4, "number of files to convert in parallel in batch mode")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "show what would be converted without making changes")
	fs.BoolVar(&opts.Plan, "plan", false, "show structured conversion plan")
	fs.BoolVar(&opts.Preview, "preview", false, "run conversion to a temp dir and print unified diffs without writing")
	fs.BoolVar(&opts.AutoDetect, "auto-detect", false, "auto-detect source framework from source content")
	fs.BoolVar(&opts.StrictValidate, "strict-validate", false, "force strict validation even when best-effort handling is requested")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.StringVar(&opts.OnError, "on-error", "skip", "error handling: skip|fail|best-effort")

	if err := fs.Parse(reorderCLIArgs(args, convertFlagsWithValue)); err != nil {
		printConvertUsage()
		return cliUsageError{message: err.Error()}
	}
	if !opts.Validate && !opts.StrictValidate {
		opts.OnError = "best-effort"
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
	fs.BoolVar(&opts.Preview, "preview", false, "run conversion to a temp dir and print unified diffs without writing")
	fs.IntVar(&opts.Concurrency, "concurrency", 4, "number of files to convert in parallel in batch mode")
	fs.StringVar(&opts.OnError, "on-error", "skip", "error handling: skip|fail|best-effort")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")

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

	validationMode, err := resolveConvertValidationMode(opts)
	if err != nil {
		return cliUsageError{message: err.Error()}
	}

	result, err := conv.RunTestMigration(source, conv.TestMigrationOptions{
		Alias:             opts.Alias,
		From:              opts.From,
		To:                opts.To,
		Output:            opts.Output,
		PreserveStructure: opts.PreserveStructure,
		BatchSize:         opts.BatchSize,
		Concurrency:       opts.Concurrency,
		AutoDetect:        opts.AutoDetect,
		ValidateSyntax:    validationMode == conv.ValidationModeStrict,
		ValidationMode:    string(validationMode),
		Plan:              opts.Plan,
		DryRun:            opts.DryRun,
		Preview:           opts.Preview,
		HistoryRoot:       resolveHistoryRoot(source),
		TerrainVersion:    version,
	})
	if err != nil {
		var inputErr conv.ConversionInputError
		if errors.As(err, &inputErr) {
			message := inputErr.Error()
			switch message {
			case "source framework is required unless auto-detect is enabled":
				message = "--from <framework> is required unless --auto-detect is set"
			case "target framework is required":
				message = "--to <framework> is required"
			}
			return cliUsageError{message: message}
		}
		return err
	}

	printExperimentalWarning(result.Direction, opts.JSON)

	if result.Plan != nil {
		return renderConvertPlan(*result.Plan, opts.JSON)
	}
	if len(result.Preview) > 0 || opts.Preview {
		return renderConvertPreview(result.Preview, result.Direction, opts.JSON)
	}
	if result.Execution != nil {
		return renderConvertExecution(*result.Execution, result.Direction, opts.JSON)
	}
	return fmt.Errorf("native test migration produced no result")
}

// renderConvertPreview prints the per-file unified diffs returned by a
// preview run. JSON output emits the slice verbatim; text output prints
// a header per file followed by the diff body.
func renderConvertPreview(previews []conv.FilePreview, direction conv.Direction, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Direction conv.Direction     `json:"direction"`
			Previews  []conv.FilePreview `json:"previews"`
		}{
			Direction: direction,
			Previews:  previews,
		})
	}
	if len(previews) == 0 {
		fmt.Println("Preview: no files would be converted.")
		return nil
	}
	fmt.Printf("Preview: %s -> %s (%d file%s)\n\n", direction.From, direction.To, len(previews), pluralS(len(previews)))
	for _, p := range previews {
		fmt.Println(p.Diff)
	}
	fmt.Println("(preview only — no files were written)")
	return nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func renderConvertExecution(result conv.ExecutionResult, direction conv.Direction, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Mode == "stdout" {
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
		fmt.Print(result.StdoutContent)
		return nil
	}

	fmt.Println("Go-native conversion complete")
	fmt.Println()
	fmt.Printf("  Direction: %s -> %s\n", direction.From, direction.To)
	fmt.Printf("  Mode: %s\n", result.Mode)
	if result.ValidationMode != "" {
		fmt.Printf("  Validation: %s", result.ValidationMode)
		if result.Validated {
			fmt.Printf(" (passed)")
		} else if result.ValidationMode == string(conv.ValidationModeBestEffort) {
			fmt.Printf(" (best-effort)")
		}
		fmt.Println()
	}
	if result.Output != "" {
		fmt.Printf("  Output: %s\n", result.Output)
	}
	fmt.Printf("  Converted files: %d\n", result.ConvertedCount)
	if result.UnchangedCount > 0 {
		fmt.Printf("  Unchanged files: %d\n", result.UnchangedCount)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	return nil
}

func renderConvertPlan(plan conv.TestMigrationPlan, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	fmt.Println("Go-native conversion plan")
	fmt.Println()
	fmt.Printf("  Source: %s\n", plan.Source)
	fmt.Printf("  Direction: %s -> %s\n", plan.Direction.From, plan.Direction.To)
	fmt.Printf("  Language: %s\n", plan.Direction.Language)
	fmt.Printf("  Category: %s\n", plan.Direction.Category)
	fmt.Printf("  Shorthands: %s\n", strings.Join(plan.Direction.Shorthands, ", "))
	fmt.Printf("  Go-native state: %s\n", humanizeGoNativeState(plan.Direction.GoNativeState))
	if plan.ValidationMode != "" {
		fmt.Printf("  Validation: %s\n", plan.ValidationMode)
	}
	fmt.Printf("  Capabilities: tests=%s, config=%s, project=%s, detect=%s, validate=%s, confidence=%s\n",
		plan.Direction.Capabilities.TestMigration,
		plan.Direction.Capabilities.ConfigMigration,
		plan.Direction.Capabilities.ProjectMigration,
		plan.Direction.Capabilities.AutoDetect,
		plan.Direction.Capabilities.SyntaxValidation,
		plan.Direction.Capabilities.ConfidenceReport,
	)
	fmt.Printf("  Execution: %s\n", plan.ExecutionStatus)
	if plan.Output != "" {
		fmt.Printf("  Output: %s\n", plan.Output)
	}
	if plan.SourceDetection != nil {
		fmt.Printf(
			"  Auto-detected source: %s (%.0f%% confidence via %s)\n",
			plan.SourceDetection.Framework,
			plan.SourceDetection.Confidence*100,
			plan.SourceDetection.DetectionSource,
		)
		if plan.SourceDetection.Recommendation != "" {
			fmt.Printf("  Detection recommendation: %s\n", plan.SourceDetection.Recommendation)
		}
		if plan.SourceDetection.Mode == "directory" {
			if plan.SourceDetection.AutoDetectSafe {
				fmt.Println("  Auto-detect safe: yes")
			} else {
				fmt.Println("  Auto-detect safe: no")
			}
		}
	}
	fmt.Println()
	fmt.Println("Next:")
	fmt.Printf("  %s\n", plan.NextStep)
	return nil
}

func resolveConvertValidationMode(opts convertCommandOptions) (conv.ValidationMode, error) {
	switch strings.ToLower(strings.TrimSpace(opts.OnError)) {
	case "", "skip", "fail":
		// Both "skip" and "fail" use strict validation. The distinction is
		// handled by the execution pipeline: strict mode removes invalid
		// output and returns an error that the caller can act on.
		return conv.ValidationModeStrict, nil
	case "best-effort":
		if opts.StrictValidate {
			// --strict-validate overrides --on-error best-effort.
			return conv.ValidationModeStrict, nil
		}
		return conv.ValidationModeBestEffort, nil
	default:
		return "", fmt.Errorf("--on-error must be one of skip, fail, or best-effort")
	}
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
	fmt.Println("Use `terrain convert <source> --from <framework> --to <framework>` to run a Go-native conversion, or add `--plan` to preview.")
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
	fmt.Println("Use a shorthand directly to run the Go-native converter, or add `--plan`/`--dry-run` to preview.")
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
		if detection.Recommendation != "" {
			fmt.Printf("  Recommendation: %s\n", detection.Recommendation)
		}
		if detection.AutoDetectSafe {
			fmt.Println("  Auto-detect safe: yes")
		} else {
			fmt.Println("  Auto-detect safe: no")
		}
		if detection.Mixed {
			fmt.Println("  Mixed repo: yes")
		}
		if detection.Ambiguous {
			fmt.Println("  Ambiguous: yes")
		}
		for _, candidate := range detection.Candidates {
			label := ""
			if candidate.Primary {
				label = " [primary]"
			}
			fmt.Printf("  Candidate: %s (%.0f%% confidence across %d file(s), %.0f%% share)%s\n", candidate.Framework, candidate.Confidence*100, candidate.Files, candidate.FileShare*100, label)
		}
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
	"--batch-size":  true,
	"--concurrency": true,
	"--on-error":    true,
}

var shorthandFlagsWithValue = map[string]bool{
	"--output":      true,
	"-o":            true,
	"--concurrency": true,
	"--on-error":    true,
}

// resolveHistoryRoot returns the directory under which terrain
// should write its `.terrain/conversion-history/log.jsonl` audit
// log. Walks up from the source path looking for a go.mod /
// package.json / .git marker; falls back to the source's parent dir
// if no marker is found, so users running `terrain convert` outside
// a real repo still get history alongside their work.
//
// Returns "" only when the source path itself can't be resolved.
func resolveHistoryRoot(source string) string {
	abs, err := filepath.Abs(source)
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ""
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	for {
		for _, marker := range []string{"go.mod", "package.json", ".git"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if info.IsDir() {
		return abs
	}
	return filepath.Dir(abs)
}

// reorderCLIArgs splits a raw argv slice into a "flags first, positionals
// last" form so Go's stdlib `flag` package — which stops parsing at the
// first non-flag argument — sees every flag the user supplied, regardless
// of where in the command line they typed it.
//
// Without this helper, `terrain convert input.test.ts --to playwright`
// would parse `input.test.ts` as a positional, then refuse to consume
// `--to playwright`. Most users intuitively put flags after the file
// argument, so we accommodate that and re-order before parsing.
//
// The flagsWithValue map identifies flags that take a separate value
// argument (e.g., `--to playwright`). Flags using the `--key=value`
// inline form are detected and don't need the lookup. Positional `--`
// stops re-ordering: anything after `--` is treated as a positional
// regardless of leading dashes, matching the POSIX convention. A nil
// flagsWithValue is supported for callers that don't accept value-flags
// (e.g. `terrain detect`).
//
// Round 1 review noted this helper as undocumented; this comment is the
// canonical explanation.
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
	case conv.GoNativeStateExperimental:
		return "experimental"
	case conv.GoNativeStatePrioritized:
		return "prioritized"
	default:
		return "cataloged"
	}
}

// printExperimentalWarning emits a stderr notice when an experimental
// conversion direction is invoked. It is suppressed when JSON output is
// requested so machine consumers see clean structured output; in that case
// the same information is available via the Direction.GoNativeState field.
func printExperimentalWarning(direction conv.Direction, jsonOutput bool) {
	if direction.GoNativeState != conv.GoNativeStateExperimental || jsonOutput {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"warning: %s -> %s is marked EXPERIMENTAL. Coverage of real-world test\n"+
			"         patterns is incomplete; expect to clean up the output by hand.\n"+
			"         See docs/release/feature-status.md for promotion criteria.\n",
		direction.From, direction.To,
	)
}

func printConvertUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain convert <source> --from <framework> --to <framework> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Current status:")
	fmt.Fprintln(os.Stderr, "  Supported directions listed by `terrain list-conversions` execute with Terrain's Go-native")
	fmt.Fprintln(os.Stderr, "  conversion runtime. Use `--plan` or `--dry-run` when you want a no-write preview first.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Key flags:")
	fmt.Fprintln(os.Stderr, "  --from, -f         source framework")
	fmt.Fprintln(os.Stderr, "  --to, -t           target framework")
	fmt.Fprintln(os.Stderr, "  --output, -o       write converted output to a file or directory")
	fmt.Fprintln(os.Stderr, "  --auto-detect      detect the source framework from the file or directory")
	fmt.Fprintln(os.Stderr, "  --validate         validate converted output before returning or writing it (default: true)")
	fmt.Fprintln(os.Stderr, "  --strict-validate  force strict validation, including when paired with best-effort handling")
	fmt.Fprintln(os.Stderr, "  --on-error         fail|skip|best-effort (best-effort keeps output even if validation fails)")
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
	fmt.Fprintf(os.Stderr, "Usage: terrain %s <source> [flags]\n", alias)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Direction: %s -> %s\n", direction.From, direction.To)
}
