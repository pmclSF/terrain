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

type convertConfigCommandOptions struct {
	From           string
	To             string
	Output         string
	Validate       bool
	StrictValidate bool
	DryRun         bool
	JSON           bool
	OnError        string
}

var convertConfigFlagsWithValue = map[string]bool{
	"--from":     true,
	"-f":         true,
	"--to":       true,
	"-t":         true,
	"--output":   true,
	"-o":         true,
	"--on-error": true,
}

func runConvertConfigCLI(args []string) error {
	fs := flag.NewFlagSet("convert-config", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var opts convertConfigCommandOptions
	fs.StringVar(&opts.From, "from", "", "source framework (auto-detected from filename if omitted)")
	fs.StringVar(&opts.From, "f", "", "source framework (auto-detected from filename if omitted)")
	fs.StringVar(&opts.To, "to", "", "target framework")
	fs.StringVar(&opts.To, "t", "", "target framework")
	fs.StringVar(&opts.Output, "output", "", "output config path")
	fs.StringVar(&opts.Output, "o", "", "output config path")
	fs.BoolVar(&opts.Validate, "validate", true, "validate converted config before returning or writing it")
	fs.BoolVar(&opts.StrictValidate, "strict-validate", false, "force strict validation even when paired with best-effort handling")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "preview without writing")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.StringVar(&opts.OnError, "on-error", "skip", "error handling: skip|fail|best-effort")

	if err := fs.Parse(reorderCLIArgs(args, convertConfigFlagsWithValue)); err != nil {
		printConvertConfigUsage()
		return cliUsageError{message: err.Error()}
	}
	if !opts.Validate && !opts.StrictValidate {
		opts.OnError = "best-effort"
	}

	positionals := fs.Args()
	if len(positionals) == 0 {
		printConvertConfigUsage()
		return cliUsageError{message: "convert-config requires <source>"}
	}

	return runConvertConfig(positionals[0], opts)
}

func runConvertConfig(source string, opts convertConfigCommandOptions) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return cliUsageError{message: "convert-config requires <source>"}
	}

	validationMode, err := resolveConfigValidationMode(opts)
	if err != nil {
		return cliUsageError{message: err.Error()}
	}

	result, err := conv.RunConfigMigration(source, conv.ConfigMigrationOptions{
		From:           opts.From,
		To:             opts.To,
		Output:         opts.Output,
		DryRun:         opts.DryRun,
		ValidateSyntax: validationMode == conv.ValidationModeStrict,
		ValidationMode: string(validationMode),
	})
	if err != nil {
		var inputErr conv.ConversionInputError
		if errors.As(err, &inputErr) {
			message := inputErr.Error()
			switch message {
			case "target framework is required":
				message = "--to <framework> is required"
			case "source is required":
				message = "convert-config requires <source>"
			}
			return cliUsageError{message: message}
		}
		return err
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if opts.DryRun {
		fmt.Println("Dry run")
		fmt.Println()
		fmt.Printf("  Source: %s\n", source)
		fmt.Printf("  Detected framework: %s\n", result.From)
		fmt.Printf("  Target framework: %s\n", result.To)
		if result.ValidationMode != "" {
			fmt.Printf("  Validation: %s\n", result.ValidationMode)
		}
		if result.Output != "" {
			fmt.Printf("  Output: %s\n", result.Output)
		} else {
			fmt.Println("  Output: (stdout)")
		}
		return nil
	}

	if result.Output == "" {
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
		fmt.Print(result.ConvertedContent)
		return nil
	}

	fmt.Println("Go-native config conversion complete")
	fmt.Println()
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Direction: %s -> %s\n", result.From, result.To)
	if result.ValidationMode != "" {
		fmt.Printf("  Validation: %s", result.ValidationMode)
		if result.Validated {
			fmt.Printf(" (passed)")
		}
		fmt.Println()
	}
	fmt.Printf("  Output: %s\n", result.Output)
	for _, warning := range result.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	return nil
}

func printConvertConfigUsage() {
	// Lead with the canonical 0.2 shape (`terrain migrate config ...`).
	// The legacy `terrain convert-config ...` form continues to work
	// in 0.2 — both shapes route through the same runner — but the
	// help output should point new users at the canonical path so
	// they don't memorize a name we plan to remove in 0.3. The
	// migrate-namespace dispatch transparently strips the verb before
	// reaching this function, so the same usage block serves both
	// legacy and canonical entry points.
	fmt.Fprintln(os.Stderr, "Usage: terrain migrate config <source> --to <framework> [flags]")
	fmt.Fprintln(os.Stderr, "       (legacy alias: terrain convert-config <source> --to <framework> [flags])")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Key flags:")
	fmt.Fprintln(os.Stderr, "  --from, -f         source framework (auto-detected from filename if omitted)")
	fmt.Fprintln(os.Stderr, "  --to, -t           target framework")
	fmt.Fprintln(os.Stderr, "  --output, -o       write converted config to a file")
	fmt.Fprintln(os.Stderr, "  --validate         validate converted config before returning or writing it (default: true)")
	fmt.Fprintln(os.Stderr, "  --strict-validate  force strict validation, including when paired with best-effort handling")
	fmt.Fprintln(os.Stderr, "  --on-error         fail|skip|best-effort (best-effort keeps output even if validation fails)")
	fmt.Fprintln(os.Stderr, "  --dry-run          preview without writing")
	fmt.Fprintln(os.Stderr, "  --json             machine-readable output")
}

func resolveConfigValidationMode(opts convertConfigCommandOptions) (conv.ValidationMode, error) {
	switch strings.ToLower(strings.TrimSpace(opts.OnError)) {
	case "", "skip", "fail":
		return conv.ValidationModeStrict, nil
	case "best-effort":
		if opts.StrictValidate {
			return conv.ValidationModeStrict, nil
		}
		return conv.ValidationModeBestEffort, nil
	default:
		return "", fmt.Errorf("--on-error must be one of skip, fail, or best-effort")
	}
}
