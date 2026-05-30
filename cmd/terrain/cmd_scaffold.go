package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pmclSF/terrain/internal/scaffold"
)

// runScaffold implements `terrain scaffold` — read a JSON Schema
// describing a prompt's expected input shape and emit a runnable
// mutation-test scaffold full of boundary cases (empty, max-length,
// unicode-edge, injection-shaped, etc.) for every field.
//
// Usage:
//   terrain scaffold --schema schemas/prompt-input.json
//   terrain scaffold --schema schemas/prompt-input.json --lang typescript
//   terrain scaffold --schema schemas/prompt-input.json --json
//   terrain scaffold --schema schemas/prompt-input.json --prompt prompts/main.md
//
// The scaffold is emitted to stdout; redirect to a test file.
// Terrain never calls the model; the assertion is the adopter's.
func runScaffold(args []string) error {
	fs := flag.NewFlagSet("scaffold", flag.ExitOnError)
	schemaPath := fs.String("schema", "", "path to a JSON Schema describing the prompt's input shape (required)")
	promptPath := fs.String("prompt", "", "optional path to the prompt under test (printed as a header comment)")
	lang := fs.String("lang", "python", "scaffold language: python | typescript | json")
	jsonOut := fs.Bool("json", false, "shortcut for --lang=json")
	_ = fs.Parse(args)

	if *schemaPath == "" {
		fs.Usage()
		return fmt.Errorf("--schema is required")
	}
	body, err := os.ReadFile(*schemaPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", *schemaPath, err)
	}
	cases, err := scaffold.GenerateFromSchema(body)
	if err != nil {
		return fmt.Errorf("generate boundary cases from %s: %w", *schemaPath, err)
	}
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "Schema declared no `properties` — nothing to scaffold. Add typed properties and re-run.")
		return nil
	}

	language := *lang
	if *jsonOut {
		language = "json"
	}
	out := scaffold.Emit(cases, scaffold.EmitOptions{
		SchemaPath: *schemaPath,
		PromptPath: *promptPath,
		Language:   language,
	})
	fmt.Print(out)
	return nil
}
