package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// runTestCommand implements `terrain test --selector <pattern>`. Runs
// the analyze pipeline, filters the resulting snapshot signals to the
// selector-matched rule IDs, and emits the matching findings via the
// canonical artifacts (JSON / JUnit / Step Summary).
//
// Selector syntax: `<category>`, `<category>/<rule>`, `<category>/*`,
// or `*`. Selector matches against manifest `RuleID`s (everything
// after `terrain/`).
//
// When the analyze pipeline can't run (e.g., the root isn't a git
// checkout), the command degrades to listing matched rule IDs only —
// still useful for confirming the rule is registered.
func runTestCommand(root, selector string, jsonOut bool, junitPath, summaryPath string) error {
	matched, err := matchRulesBySelector(selector)
	if err != nil {
		return err
	}
	if len(matched) == 0 {
		return fmt.Errorf("selector %q matched no rules. Available categories: regression coverage hygiene reproducibility security performance data ai", selector)
	}

	// Map signal type → ruleID for the lookup the converter needs.
	typeToRuleID := map[models.SignalType]string{}
	for _, entry := range signals.Manifest() {
		if entry.RuleID != "" {
			typeToRuleID[entry.Type] = entry.RuleID
		}
	}
	// Set of matched rule IDs for the filter.
	wantRule := map[string]bool{}
	for _, r := range matched {
		wantRule[r] = true
	}

	// Run the analyze pipeline. Failure here is non-fatal; we fall
	// back to listing matched rules, which is still informative.
	var filtered []models.Signal
	result, perr := runPipelineWithSignals(root, defaultPipelineOptionsWithProgress(jsonOut || junitPath != "" || summaryPath != ""))
	if perr == nil && result != nil && result.Snapshot != nil {
		for _, s := range result.Snapshot.Signals {
			if ruleID := typeToRuleID[s.Type]; wantRule[ruleID] {
				filtered = append(filtered, s)
			}
		}
	}

	fxs := findings.FromSignals(filtered, func(t models.SignalType) string {
		return typeToRuleID[t]
	})
	art := findings.NewArtifact(fxs)

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

	// Human-readable output.
	fmt.Printf("Selector %q matched %d rule(s):\n", selector, len(matched))
	for _, r := range matched {
		fmt.Printf("  %s\n", r)
	}
	fmt.Println()
	if perr != nil {
		fmt.Printf("Pipeline run failed (%v); listing matched rule IDs only.\n", perr)
		fmt.Println("Fix the underlying analyze failure to see live findings here.")
		return nil
	}
	fmt.Printf("Live findings matching the selector: %d\n", len(filtered))
	if len(filtered) == 0 {
		fmt.Println("No findings from the matched rule(s) on the current snapshot.")
		return nil
	}
	for _, s := range filtered {
		loc := s.Location.File
		if s.Location.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, s.Location.Line)
		}
		fmt.Printf("  [%s] %s  %s\n", s.Severity, s.Type, loc)
	}
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
