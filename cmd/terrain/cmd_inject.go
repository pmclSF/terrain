package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pmclSF/terrain/internal/injection"
	"github.com/pmclSF/terrain/internal/promptflow"
	"github.com/pmclSF/terrain/internal/prompttemplate"
)

// runInject implements `terrain inject` — read a prompt template
// (file path or stdin), detect which injection patterns apply, and
// emit a runnable test scaffold or a JSON dump for downstream tooling.
//
// Usage:
//
//	terrain inject --prompt prompts/system.md          # python scaffold to stdout
//	terrain inject --prompt prompts/system.md --lang ts
//	terrain inject --prompt prompts/system.md --json
//	terrain inject --prompt prompts/system.md --list   # just list matched patterns
//
// The scaffold is emitted to stdout; adopters redirect to a file
// (`> tests/test_prompt_security.py`) and drop into their test suite.
// Terrain never calls the model; the assertion is the adopter's.
func runInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ExitOnError)
	promptPath := fs.String("prompt", "", "path to the prompt template file to scan")
	lang := fs.String("lang", "python", "scaffold language: python | typescript | json")
	jsonOut := fs.Bool("json", false, "shortcut for --lang=json")
	list := fs.Bool("list", false, "only list matched patterns, don't emit a scaffold")
	_ = fs.Parse(args)

	if *promptPath == "" {
		fs.Usage()
		return cliUsageError{message: "--prompt is required"}
	}
	body, err := os.ReadFile(*promptPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", *promptPath, err)
	}

	// Sniff the kind from the file path so we render appropriately
	// if we ever support per-flavor matchers. For now, the matchers
	// are flavor-agnostic — they look at the body's substring shape.
	kind := prompttemplate.Detect(*promptPath, body)
	_ = kind
	_ = promptflow.Discoveries{} // imports retained for symmetry with future flow-aware matching

	matches := injection.DetectMatches(string(body))
	if len(matches) == 0 {
		fmt.Fprintln(os.Stderr, "No injection patterns matched. Prompt body looks structurally safe for the cataloged classes.")
		return nil
	}

	if *list {
		fmt.Printf("Matched %d injection pattern(s) in %s:\n", len(matches), *promptPath)
		for _, m := range matches {
			fmt.Printf("  - %s (%s) — matched on %q\n", m.Pattern.ID, m.Pattern.Title, m.Marker)
		}
		fmt.Println()
		fmt.Println("Re-run without --list to emit a runnable test scaffold.")
		return nil
	}

	language := *lang
	if *jsonOut {
		language = "json"
	}
	out := injection.Emit(matches, injection.EmitOptions{
		PromptPath: *promptPath,
		Language:   language,
	})
	fmt.Print(out)
	return nil
}
