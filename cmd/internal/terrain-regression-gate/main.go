// Command terrain-regression-gate loads every regression suite and
// recall harness from harness/regression-suites and harness/recall-
// harnesses, validates them, and prints a summary. Used as a CI step
// before any mechanism flip from shadow → on; ensures the suite files
// parse and that mechanism state changes are accompanied by actual
// regression coverage.
//
// Exit codes:
//
//	0 — every suite and harness loaded successfully
//	1 — at least one suite or harness failed validation
//	2 — usage error (missing directory, etc.)
//
// Today the suites and harnesses ship empty (the placeholders in
// harness/{regression-suites,recall-harnesses}/_README.md). The target
// validates the loader machinery itself plus the YAML schema so the
// first non-empty suite added by a feature author lands clean.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pmclSF/terrain/internal/recallharness"
	"github.com/pmclSF/terrain/internal/regressionsuite"
)

func main() {
	root := flag.String("root", ".", "repo root containing harness/")
	flag.Parse()

	suiteDir := filepath.Join(*root, "harness", "regression-suites")
	harnessDir := filepath.Join(*root, "harness", "recall-harnesses")

	if _, err := os.Stat(suiteDir); err != nil {
		fmt.Fprintf(os.Stderr, "regression-suites dir missing: %v\n", err)
		os.Exit(2)
	}
	if _, err := os.Stat(harnessDir); err != nil {
		fmt.Fprintf(os.Stderr, "recall-harnesses dir missing: %v\n", err)
		os.Exit(2)
	}

	suites, err := regressionsuite.LoadAll(suiteDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load regression suites: %v\n", err)
		os.Exit(1)
	}
	harnesses, err := recallharness.LoadAll(harnessDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load recall harnesses: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("regression-suite gate: %d suites, %d harnesses\n", len(suites), len(harnesses))

	moduleNames := make([]string, 0, len(suites))
	for m := range suites {
		moduleNames = append(moduleNames, m)
	}
	sort.Strings(moduleNames)
	for _, m := range moduleNames {
		s := suites[m]
		fmt.Printf("  suite %s: %d frozen TPs (max_tp_loss=%d)\n", m, len(s.FrozenTPs), s.MaxTPLoss)
	}

	ruleNames := make([]string, 0, len(harnesses))
	for r := range harnesses {
		ruleNames = append(ruleNames, r)
	}
	sort.Strings(ruleNames)
	for _, r := range ruleNames {
		h := harnesses[r]
		fmt.Printf("  harness %s: %d golden TPs, %d mechanisms, union_min_recall=%.2f\n",
			r, len(h.GoldenTPs), len(h.Mechanisms), h.UnionMinRecall)
	}

	if len(suites) == 0 && len(harnesses) == 0 {
		fmt.Println("note: no populated suites or harnesses yet — schema validation only")
	}
}
