package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type convertConfigResult struct {
	Command      string         `json:"command"`
	Source       string         `json:"source"`
	Output       string         `json:"output,omitempty"`
	Mode         string         `json:"mode"`
	From         string         `json:"from"`
	To           string         `json:"to"`
	AutoDetected bool           `json:"autoDetected,omitempty"`
	DryRun       bool           `json:"dryRun,omitempty"`
	Direction    conv.Direction `json:"direction"`
	Status       string         `json:"status"`
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

	to := conv.NormalizeFramework(opts.To)
	if to == "" {
		return cliUsageError{message: "--to <framework> is required"}
	}

	from := conv.NormalizeFramework(opts.From)
	autoDetected := false
	if from == "" {
		from = conv.DetectConfigFramework(source)
		autoDetected = from != ""
		if from == "" {
			return cliUsageError{message: "could not auto-detect source framework from config filename; use --from <framework>"}
		}
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

	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	converted, err := conv.ConvertConfig(string(content), from, to)
	if err != nil {
		return err
	}

	mode := "stdout"
	status := "printed"
	outputPath := ""
	if strings.TrimSpace(opts.Output) != "" {
		mode = "file"
		status = "written"
		outputPath, err = resolveConfigOutputPath(source, opts.Output)
		if err != nil {
			return err
		}
	}
	if opts.DryRun {
		mode = "dry-run"
		status = "previewed"
	}

	result := convertConfigResult{
		Command:      "convert-config",
		Source:       source,
		Output:       outputPath,
		Mode:         mode,
		From:         from,
		To:           to,
		AutoDetected: autoDetected,
		DryRun:       opts.DryRun,
		Direction:    direction,
		Status:       status,
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
		fmt.Printf("  Detected framework: %s\n", from)
		fmt.Printf("  Target framework: %s\n", to)
		if outputPath != "" {
			fmt.Printf("  Output: %s\n", outputPath)
		} else {
			fmt.Println("  Output: (stdout)")
		}
		return nil
	}

	if outputPath == "" {
		fmt.Print(converted)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return fmt.Errorf("write config output: %w", err)
	}

	fmt.Println("Go-native config conversion complete")
	fmt.Println()
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Direction: %s -> %s\n", from, to)
	fmt.Printf("  Output: %s\n", outputPath)
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

func resolveConfigOutputPath(source, output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	return output, nil
}
