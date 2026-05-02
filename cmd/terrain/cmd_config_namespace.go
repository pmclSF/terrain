package main

import (
	"fmt"
	"os"

	"github.com/pmclSF/terrain/internal/telemetry"
)

// Phase A of the 0.2 CLI restructure groups workspace preferences
// under one noun: `terrain config`. The canonical shape:
//
//   terrain config feedback                       (was: feedback)
//   terrain config telemetry [--on|--off|--status]   (was: telemetry)
//
// Legacy top-level `feedback` and `telemetry` keep working unchanged
// through 0.2; deprecation note in 0.2.x; removal in 0.3.

var configVerbs = map[string]bool{
	"feedback":  true,
	"telemetry": true,
}

// runConfigNamespaceCLI dispatches `terrain config <verb> ...` against
// the canonical-verb table.
func runConfigNamespaceCLI(args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printConfigUsage()
		if len(args) == 0 {
			return fmt.Errorf("terrain config: missing verb")
		}
		return nil
	}

	verb := args[0]
	if !configVerbs[verb] {
		printConfigUsage()
		return fmt.Errorf("unknown config verb %q (valid: feedback, telemetry)", verb)
	}

	rest := args[1:]
	switch verb {
	case "feedback":
		return runConfigFeedbackCLI(rest)
	case "telemetry":
		return runConfigTelemetryCLI(rest)
	}
	return nil
}

func printConfigUsage() {
	fmt.Println("Usage: terrain config <verb> [flags]")
	fmt.Println()
	fmt.Println("Workspace preferences and feedback channels.")
	fmt.Println()
	fmt.Println("Verbs:")
	fmt.Println("  feedback                       open the feedback link")
	fmt.Println("  telemetry [--on|--off|--status] manage local telemetry config")
}

// runConfigFeedbackCLI mirrors the legacy `terrain feedback` behavior.
// Pure side-effect prints; no state change. Kept here so the legacy
// dispatch in main.go can call the same function once we collapse the
// inline implementation.
func runConfigFeedbackCLI(_ []string) error {
	url := "https://github.com/pmclSF/terrain/issues/new?template=feedback.md&title=Feedback:+&labels=feedback"
	fmt.Println("Open the following URL to share feedback:")
	fmt.Println()
	fmt.Printf("  %s\n", url)
	fmt.Println()
	fmt.Println("Or email: terrain-feedback@pmcl.dev")
	return nil
}

// runConfigTelemetryCLI mirrors the legacy `terrain telemetry` parser.
func runConfigTelemetryCLI(args []string) error {
	if len(args) == 0 {
		fmt.Println("Telemetry:", telemetry.Status())
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  terrain config telemetry --on     enable local telemetry")
		fmt.Println("  terrain config telemetry --off    disable local telemetry")
		fmt.Println("  terrain config telemetry --status show current state")
		fmt.Println()
		fmt.Println("Telemetry records command name, repo size band, languages,")
		fmt.Println("signal count, and duration to ~/.terrain/telemetry.jsonl.")
		fmt.Println("No file paths, repo URLs, or PII are recorded.")
		fmt.Println("Override with TERRAIN_TELEMETRY=on|off environment variable.")
		return nil
	}
	switch args[0] {
	case "--on", "on":
		if err := telemetry.SaveConfig(telemetry.Config{Enabled: true}); err != nil {
			return err
		}
		fmt.Println("Telemetry enabled. Events will be written to ~/.terrain/telemetry.jsonl")
	case "--off", "off":
		if err := telemetry.SaveConfig(telemetry.Config{Enabled: false}); err != nil {
			return err
		}
		fmt.Println("Telemetry disabled.")
	case "--status", "status":
		fmt.Println("Telemetry:", telemetry.Status())
	default:
		fmt.Fprintf(os.Stderr, "unknown telemetry subcommand: %q\n", args[0])
		return fmt.Errorf("unknown telemetry option")
	}
	return nil
}
