package main

import (
	"flag"
	"fmt"
	"os"
)

// Phase A of the 0.2 CLI restructure folds the conversion + migration
// universe into a single noun: `terrain migrate`. The canonical shape:
//
//   terrain migrate run <from-to>     // execute a conversion
//   terrain migrate config <from-to>  // convert config files only
//   terrain migrate list              // list supported directions
//   terrain migrate detect            // auto-detect framework
//   terrain migrate shorthands        // list aliases
//   terrain migrate estimate          // cost/time estimate
//   terrain migrate status            // migration status
//   terrain migrate checklist         // pre-migration checklist
//   terrain migrate readiness         // readiness gate
//   terrain migrate blockers          // blocker enumeration
//   terrain migrate preview           // dry-run a single file/scope
//
// `terrain convert ...` is an alias dispatched through the same entry
// point so muscle memory keeps working through 0.2. Legacy top-level
// commands (estimate, status, checklist, list, list-conversions,
// shorthands, detect, convert-config, migration <verb>) continue to
// work unchanged in 0.2 and get a deprecation note in 0.2.x. Removal
// targets 0.3.
//
// When the first arg isn't a known verb, we fall through to the legacy
// runner — preserves `terrain migrate cypress-playwright` direct
// invocation for scripts and docs.

// migrateVerbs lists the canonical verb allowlist. Anything else is
// treated as a direct framework-pair invocation (legacy shape).
var migrateVerbs = map[string]bool{
	"run":        true,
	"config":     true,
	"list":       true,
	"detect":     true,
	"shorthands": true,
	"estimate":   true,
	"status":     true,
	"checklist":  true,
	"readiness":  true,
	"blockers":   true,
	"preview":    true,
}

// runMigrateNamespaceCLI dispatches `terrain migrate ...` against the
// canonical-verb table. Unknown first args fall through to
// runMigrateCLI (the directory-mode runner) — `terrain migrate
// <directory>` was the legacy shape.
func runMigrateNamespaceCLI(args []string) error {
	return runMigrateOrConvertNamespaceCLI(args, runMigrateCLI)
}

// runConvertNamespaceCLI dispatches `terrain convert ...` against the
// same canonical-verb table. Unknown first args fall through to
// runConvertCLI (the legacy per-file converter — `terrain convert
// path/to/spec.cy.ts --to playwright`). Splitting the fall-through
// dispatchers preserves both shapes — otherwise per-file conversion
// regresses to the directory-mode runner and errors with
// "--from <framework> is required".
func runConvertNamespaceCLI(args []string) error {
	return runMigrateOrConvertNamespaceCLI(args, runConvertCLI)
}

func runMigrateOrConvertNamespaceCLI(args []string, fallthroughFn func([]string) error) error {
	if len(args) == 0 {
		// Pre-0.2.x bare `terrain migrate` / `terrain convert` fell
		// through to the legacy directory-mode runner which printed an
		// error-shaped usage block. The canonical 0.2 shape is the
		// verb listing — a plain `terrain migrate` without args means
		// "show me what I can do," not "I'm trying to migrate the cwd."
		printMigrateNamespaceUsage(noun(fallthroughFn))
		return nil
	}

	verb := args[0]
	if isHelpArg(verb) {
		// `terrain migrate --help` / `terrain convert -h` — print the
		// namespace-verb listing instead of falling through to the
		// legacy directory-mode help. Pre-0.2.x this printed
		// `Usage: terrain migrate <dir>` which actively misled users
		// away from the canonical shape.
		printMigrateNamespaceUsage(noun(fallthroughFn))
		return nil
	}
	if !migrateVerbs[verb] {
		// Legacy direct invocation or flag-prefixed call.
		return fallthroughFn(args)
	}

	rest := args[1:]
	switch verb {
	case "run":
		return runMigrateCLI(rest)
	case "config":
		return runConvertConfigCLI(rest)
	case "list":
		return runListConversionsCLI(rest)
	case "detect":
		return runDetectCLI(rest)
	case "shorthands":
		return runShorthandsCLI(rest)
	case "estimate":
		return runEstimateCLI(rest)
	case "status":
		return runStatusCLI(rest)
	case "checklist":
		return runChecklistCLI(rest)
	case "readiness", "blockers", "preview":
		return runMigrationLegacySubcommand(verb, rest)
	}

	return fallthroughFn(args)
}

// noun returns the user-facing verb namespace ("migrate" or "convert")
// for the dispatcher whose fallthrough is fn. Used so the namespace
// help block reads correctly under either entry point.
func noun(fn func([]string) error) string {
	// Compare by function pointer identity. We only have two
	// fallthroughs in this dispatcher: runMigrateCLI (directory-mode)
	// and runConvertCLI (per-file). Anything else falls back to the
	// migrate noun since that's where the namespace originated.
	if fmt.Sprintf("%p", fn) == fmt.Sprintf("%p", runConvertCLI) {
		return "convert"
	}
	return "migrate"
}

// printMigrateNamespaceUsage prints the canonical 0.2 verb listing for
// `terrain migrate ...` / `terrain convert ...`. Goes to stderr so it
// composes with shell pipes the same way that `--help` traditionally
// does (informational, doesn't pollute stdout).
func printMigrateNamespaceUsage(name string) {
	w := os.Stderr
	fmt.Fprintf(w, "Usage: terrain %s <verb> [args...]\n\n", name)
	fmt.Fprintln(w, "Verbs:")
	fmt.Fprintln(w, "  run         execute a conversion (terrain migrate run cypress-playwright src/)")
	fmt.Fprintln(w, "  config      convert config files only (terrain migrate config eslint-biome)")
	fmt.Fprintln(w, "  list        list supported framework directions")
	fmt.Fprintln(w, "  detect      auto-detect framework in the current repo")
	fmt.Fprintln(w, "  shorthands  list framework-pair aliases")
	fmt.Fprintln(w, "  estimate    estimate cost / time for a conversion")
	fmt.Fprintln(w, "  status      report migration status")
	fmt.Fprintln(w, "  checklist   pre-migration checklist")
	fmt.Fprintln(w, "  readiness   migration readiness gate")
	fmt.Fprintln(w, "  blockers    enumerate migration blockers")
	fmt.Fprintln(w, "  preview     dry-run a single file or scope")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Legacy shapes still work in 0.2:")
	if name == "convert" {
		fmt.Fprintln(w, "  terrain convert <file> --from <fw> --to <fw>")
	} else {
		fmt.Fprintln(w, "  terrain migrate <dir> --from <fw> --to <fw>")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "See: docs/cli/migrate.md  (or `terrain migrate run --help` for run-specific flags)")
}

// runMigrationLegacySubcommand wraps the historical `terrain migration
// <verb>` subcommand parsing so the same options reach `terrain migrate
// <verb>`. Mirrors the inline parsing in main.go (kept there for the
// legacy entry point).
func runMigrationLegacySubcommand(subCmd string, args []string) error {
	switch subCmd {
	case "readiness", "blockers":
		fs := flag.NewFlagSet("migrate "+subCmd, flag.ExitOnError)
		rootFlag := fs.String("root", ".", "repository root to analyze")
		jsonFlag := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(args)
		return runMigration(subCmd, *rootFlag, *jsonFlag, "", "")
	case "preview":
		fs := flag.NewFlagSet("migrate preview", flag.ExitOnError)
		rootFlag := fs.String("root", ".", "repository root to analyze")
		jsonFlag := fs.Bool("json", false, "output JSON")
		fileFlag := fs.String("file", "", "file path for preview (relative to root)")
		scopeFlag := fs.String("scope", "", "directory scope for preview")
		_ = fs.Parse(args)
		return runMigration(subCmd, *rootFlag, *jsonFlag, *fileFlag, *scopeFlag)
	}
	return fmt.Errorf("unknown migrate subcommand: %q", subCmd)
}

