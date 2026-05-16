package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/signals"
)

// runTestCommand implements `terrain test --selector <pattern>`.
// 0.2.0 ships the wiring: parse the selector, list which rules match,
// emit the canonical artifacts (findings.json / JUnit / Step Summary).
//
// Detector execution against the live snapshot is the responsibility
// of the analyze pipeline; this command's purpose is to filter the
// rule set and route artifacts. The selector syntax is `<category>`,
// `<category>/<rule>`, `<category>/*`, or `*`.
func runTestCommand(root, selector string, jsonOut bool, junitPath, summaryPath string) error {
	matched, err := matchRulesBySelector(selector)
	if err != nil {
		return err
	}
	if len(matched) == 0 {
		return fmt.Errorf("selector %q matched no rules. Available categories: regression coverage hygiene reproducibility security performance data ai", selector)
	}

	// 0.2.0 stub: we don't re-execute the full analyze pipeline here.
	// Instead, render an empty Artifact filtered to the matched rules
	// so the artifact-emission contract is exercisable. The full
	// integration with the analyze pipeline lands once the engine
	// surfaces selector-aware execution (Tier 3 followup).
	art := findings.NewArtifact(nil)

	if junitPath != "" {
		if err := writeFile(junitPath, func(f *os.File) error {
			return art.WriteJUnit(f, findings.JUnitOptions{})
		}); err != nil {
			return fmt.Errorf("write junit: %w", err)
		}
	}
	if summaryPath != "" {
		if err := writeFile(summaryPath, func(f *os.File) error {
			return art.WriteStepSummary(f, findings.StepSummaryOptions{})
		}); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
	}
	if jsonOut {
		return art.WriteJSON(os.Stdout)
	}

	// Human-readable output: one line per matched rule.
	fmt.Printf("Selector %q matched %d rule(s):\n", selector, len(matched))
	for _, r := range matched {
		fmt.Printf("  %s\n", r)
	}
	fmt.Println()
	fmt.Println("Detector execution against the live snapshot integrates with the analyze pipeline.")
	fmt.Println("Use --junit / --summary to write empty artifacts for CI plumbing testing.")
	fmt.Println("Run `terrain analyze` to execute the full pipeline.")
	return nil
}

// matchRulesBySelector returns rule IDs whose URI suffix matches the
// selector pattern. Selector forms:
//
//	"*"                 → every rule
//	"regression"        → every rule in the regression/ category
//	"regression/*"      → same as above
//	"regression/test-failed" → exact match
func matchRulesBySelector(selector string) ([]string, error) {
	if selector == "" {
		return nil, fmt.Errorf("selector is required (e.g., --selector regression/test-failed)")
	}
	all := allRuleIDs()
	if selector == "*" {
		return all, nil
	}
	// Normalize: strip leading "terrain/" if present.
	sel := strings.TrimPrefix(selector, "terrain/")
	var category, rule string
	if i := strings.Index(sel, "/"); i >= 0 {
		category = sel[:i]
		rule = sel[i+1:]
	} else {
		category = sel
		rule = "*"
	}
	if category == "" {
		return nil, fmt.Errorf("invalid selector %q", selector)
	}

	var out []string
	for _, id := range all {
		// id is "terrain/<cat>/<rule>"
		parts := strings.SplitN(strings.TrimPrefix(id, "terrain/"), "/", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] != category {
			continue
		}
		if rule == "*" || parts[1] == rule {
			out = append(out, id)
		}
	}
	return out, nil
}

func allRuleIDs() []string {
	var out []string
	for _, entry := range signals.Manifest() {
		if entry.RuleID == "" {
			continue
		}
		out = append(out, entry.RuleID)
	}
	return out
}

// writeFile opens path for write, calls fn with the file, ensures
// close + error propagation. Caller's fn does the actual emission.
func writeFile(path string, fn func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}
