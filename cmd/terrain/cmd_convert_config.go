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
	From   string
	To     string
	Output string
	DryRun bool
	JSON   bool
}

var convertConfigFlagsWithValue = map[string]bool{
	"--from":   true,
	"-f":       true,
	"--to":     true,
	"-t":       true,
	"--output": true,
	"-o":       true,
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
	fs.BoolVar(&opts.DryRun, "dry-run", false, "preview without writing")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")

	if err := fs.Parse(reorderCLIArgs(args, convertConfigFlagsWithValue)); err != nil {
		printConvertConfigUsage()
		return cliUsageError{message: err.Error()}
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

	result, err := conv.RunConfigMigration(source, conv.ConfigMigrationOptions{
		From:   opts.From,
		To:     opts.To,
		Output: opts.Output,
		DryRun: opts.DryRun,
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
		if result.Output != "" {
			fmt.Printf("  Output: %s\n", result.Output)
		} else {
			fmt.Println("  Output: (stdout)")
		}
		return nil
	}

	if result.Output == "" {
		fmt.Print(result.ConvertedContent)
		return nil
	}

	fmt.Println("Go-native config conversion complete")
	fmt.Println()
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Direction: %s -> %s\n", result.From, result.To)
	fmt.Printf("  Output: %s\n", result.Output)
	return nil
}

func printConvertConfigUsage() {
	fmt.Fprintln(os.Stderr, "Usage: terrain convert-config <source> --to <framework> [flags]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Key flags:")
	fmt.Fprintln(os.Stderr, "  --from, -f         source framework (auto-detected from filename if omitted)")
	fmt.Fprintln(os.Stderr, "  --to, -t           target framework")
	fmt.Fprintln(os.Stderr, "  --output, -o       write converted config to a file")
	fmt.Fprintln(os.Stderr, "  --dry-run          preview without writing")
	fmt.Fprintln(os.Stderr, "  --json             machine-readable output")
}
