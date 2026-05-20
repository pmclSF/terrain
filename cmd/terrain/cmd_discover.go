package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/engine"
)

// runDiscover prints the no-args discovery report — a friendly first-touch
// summary of what Terrain sees in the current repository. Surfaces frameworks,
// test files, AI surfaces (prompts / evals / model call sites), schema files,
// and production trace logs. Ends with three copy-pasteable next commands.
//
// Sections:
//   - Header: repo + language summary
//   - What's here: frameworks, test files, AI surfaces, schemas, traces
//   - What's missing: AI surfaces without eval coverage; absent trace logs
//   - Suggested next steps: 3 commands tailored to what was found
//
// On an empty repo or a repo with no AI surfaces, the output stays friendly
// (no flood of zeros) and points at `terrain analyze` as the next step.
func runDiscover(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	initResult, err := engine.RunInit(abs)
	if err != nil {
		return fmt.Errorf("repo scan: %w", err)
	}

	aiResult := aidetect.Detect(abs)
	schemaFiles := detectSchemaFiles(abs)
	traceLogs := detectTraceLogs(abs)

	sep := strings.Repeat("─", 60)

	fmt.Printf("Terrain — discovery report for %s\n", filepath.Base(abs))
	fmt.Println(sep)
	fmt.Println()

	// ── What's here ─────────────────────────────────────────────────
	fmt.Println("What's here")
	fmt.Println(sep)

	if len(initResult.Languages) > 0 {
		fmt.Printf("  Languages:           %s\n", strings.Join(initResult.Languages, ", "))
	}

	if len(initResult.Frameworks) > 0 {
		fwNames := uniqueFrameworkNames(initResult.Frameworks)
		fmt.Printf("  Test frameworks:     %s\n", strings.Join(fwNames, ", "))
	}

	if initResult.TestFileCount > 0 {
		fmt.Printf("  Test files:          %d\n", initResult.TestFileCount)
	}

	if len(aiResult.PromptFiles) > 0 {
		fmt.Printf("  Prompt files:        %d  %s\n", len(aiResult.PromptFiles), brief(aiResult.PromptFiles, 3))
	}
	if len(aiResult.EvalConfigs) > 0 {
		fmt.Printf("  Eval configs:        %d  %s\n", len(aiResult.EvalConfigs), brief(aiResult.EvalConfigs, 3))
	}
	if len(aiResult.ModelFiles) > 0 {
		fmt.Printf("  Model call sites:    %d files  %s\n", len(aiResult.ModelFiles), brief(aiResult.ModelFiles, 3))
	}
	if len(aiResult.DatasetFiles) > 0 {
		fmt.Printf("  Dataset files:       %d  %s\n", len(aiResult.DatasetFiles), brief(aiResult.DatasetFiles, 3))
	}
	if len(schemaFiles) > 0 {
		fmt.Printf("  Schema files:        %d  %s\n", len(schemaFiles), brief(schemaFiles, 3))
	}
	if len(traceLogs) > 0 {
		fmt.Printf("  Production traces:   %d files  %s\n", len(traceLogs), brief(traceLogs, 3))
	}

	if initResult.Artifacts != nil {
		if initResult.Artifacts.CoveragePath != "" {
			fmt.Printf("  Coverage artifact:   %s (%s)\n",
				relativeToRoot(initResult.Artifacts.CoveragePath, initResult.Root),
				initResult.Artifacts.CoverageFormat)
		}
	}

	fmt.Println()

	// ── What's missing ──────────────────────────────────────────────
	missing := []string{}
	if len(aiResult.PromptFiles) > 0 && len(aiResult.EvalConfigs) == 0 {
		missing = append(missing, "Eval configs    — found prompt files but no eval framework config (promptfoo / deepeval / ragas / Great Expectations)")
	}
	if len(aiResult.ModelFiles) > 0 && len(aiResult.EvalConfigs) == 0 && len(aiResult.PromptFiles) == 0 {
		missing = append(missing, "Eval coverage   — model call sites without prompts or evals; `terrain ai findings` will say more")
	}
	if len(traceLogs) == 0 && len(aiResult.ModelFiles) > 0 {
		missing = append(missing, "Trace logs      — point at a directory with `terrain replay <path>` to surface eval coverage gaps")
	}
	if initResult.Artifacts == nil || initResult.Artifacts.CoveragePath == "" {
		if initResult.TestFileCount > 0 {
			missing = append(missing, "Coverage data   — produce LCOV / Istanbul / cobertura output to unlock coverage signals")
		}
	}

	if len(missing) > 0 {
		fmt.Println("What's missing")
		fmt.Println(sep)
		for _, m := range missing {
			fmt.Printf("  %s\n", m)
		}
		fmt.Println()
	}

	// ── Suggested next steps ────────────────────────────────────────
	fmt.Println("Next steps")
	fmt.Println(sep)
	hasAI := len(aiResult.PromptFiles) > 0 || len(aiResult.ModelFiles) > 0 || len(aiResult.EvalConfigs) > 0
	switch {
	case initResult.TestFileCount == 0 && !hasAI:
		fmt.Println("  Nothing AI-specific detected. Run `terrain analyze` for the full posture report.")
	case hasAI:
		fmt.Println("  terrain analyze              # full posture: signals, coverage, fanout, drift")
		fmt.Println("  terrain ai findings          # calibrated AI risks (no API key required)")
		fmt.Println("  terrain explain <finding>    # drill into a specific finding")
	default:
		fmt.Println("  terrain analyze              # full posture: signals, coverage, fanout, drift")
		fmt.Println("  terrain insights             # prioritized next actions")
		fmt.Println("  terrain report pr            # what does this change put at risk?")
	}
	fmt.Println()

	return nil
}

// brief returns a short string summarizing the first `max` entries.
// Returns "[paths]" or "[3 paths; …and N more]" depending on length.
func brief(items []string, max int) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) <= max {
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	}
	return fmt.Sprintf("[%s, …and %d more]", strings.Join(items[:max], ", "), len(items)-max)
}

// uniqueFrameworkNames deduplicates the framework list by name. Multiple
// detection sources (config file + dependency + convention) can produce
// repeated entries — the report just wants the names.
func uniqueFrameworkNames(frameworks []engine.DetectedFramework) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, fw := range frameworks {
		if !seen[fw.Name] {
			seen[fw.Name] = true
			out = append(out, fw.Name)
		}
	}
	sort.Strings(out)
	return out
}

// detectSchemaFiles surfaces likely schema/contract files: JSON Schema
// (.schema.json), Protocol Buffers (.proto), GraphQL SDL (.graphql /
// .gql), Pydantic model files (heuristic), and TypeScript type files
// (interface / type aliases in a dedicated types/ or schemas/ dir).
// Skips paths inside test-fixture / benchmark / vendor trees so the
// report doesn't flood on borrowed fixture content. Heuristic, not
// exhaustive — calibration happens in `terrain analyze`.
func detectSchemaFiles(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if shouldSkipDir(base) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if isFixturePath(rel) {
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)

		// File-extension matches.
		if strings.HasSuffix(lower, ".proto") ||
			strings.HasSuffix(lower, ".graphql") ||
			strings.HasSuffix(lower, ".gql") ||
			strings.HasSuffix(lower, ".schema.json") {
			out = append(out, rel)
			return nil
		}

		// Path-based matches: files inside `schemas/` or `models/`
		// directories named with the conventional suffixes.
		relLower := strings.ToLower(rel)
		if (strings.Contains(relLower, "/schemas/") || strings.HasPrefix(relLower, "schemas/")) &&
			(strings.HasSuffix(lower, ".py") ||
				strings.HasSuffix(lower, ".ts") ||
				strings.HasSuffix(lower, ".json") ||
				strings.HasSuffix(lower, ".yaml") ||
				strings.HasSuffix(lower, ".yml")) {
			out = append(out, rel)
			return nil
		}

		return nil
	})
	sort.Strings(out)
	return out
}

// isFixturePath returns true when the path lives inside a directory
// commonly used to hold borrowed test fixtures, benchmark inputs, or
// vendored examples. The discovery report should not surface those as
// part of the adopter's actual codebase.
func isFixturePath(rel string) bool {
	lower := strings.ToLower(rel)
	for _, seg := range []string{
		"/testdata/", "testdata/",
		"/fixtures/", "fixtures/",
		"/__fixtures__/", "__fixtures__/",
		"/benchmarks/", "benchmarks/",
		"/examples/", "examples/",
		"/vendor/", "vendor/",
		"/third_party/", "third_party/",
	} {
		if strings.HasPrefix(lower, seg) || strings.Contains(lower, seg) {
			return true
		}
	}
	return false
}

// detectTraceLogs scans for production LLM-call trace logs: common
// directory conventions (langfuse / helicone / arize-phoenix /
// langsmith / llm-logs) and well-known file extensions (.langsmith.jsonl,
// *.trace.json). Conservative — only surfaces directories that look
// intentional, and skips anything inside test-fixture / benchmark trees.
//
// `traces/` alone is too ambiguous (Prettier, flow, and many other
// projects have a `traces/` test-fixture dir) so it's only surfaced
// when it's at the repo root, not buried inside a tree.
func detectTraceLogs(root string) []string {
	// Directory names that are unambiguous LLM-trace signals.
	strongDirs := map[string]bool{
		"langfuse": true, "helicone": true, "phoenix": true, "langsmith": true,
		"llm-logs": true, "llm_logs": true, "llm-traces": true,
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if shouldSkipDir(base) {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if isFixturePath(rel) {
				return nil
			}
			if strongDirs[strings.ToLower(base)] {
				// Found a strong-signal trace directory.
				out = append(out, rel+"/")
				return filepath.SkipDir
			}
			// `traces/` at the repo root only.
			if strings.ToLower(base) == "traces" && rel == "traces" {
				out = append(out, rel+"/")
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if isFixturePath(rel) {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".langsmith.jsonl") || strings.HasSuffix(name, ".trace.json") {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// shouldSkipDir returns true for common no-scan directories. Mirrors the
// skip set used by other walkers in the codebase.
func shouldSkipDir(base string) bool {
	switch base {
	case ".git", "node_modules", "vendor", ".venv", "venv", "dist", "build",
		".terrain", ".next", ".cache", "target", "__pycache__", ".pytest_cache":
		return true
	}
	return false
}

// _ ensures os import is used in code generated tests; safe to remove.
var _ = os.Stat
