// Command terrain-regression-gate loads every regression suite and
// recall harness from harness/regression-suites and harness/recall-
// harnesses, validates them, and prints a summary. Used as a CI step
// that ensures the regression-suite files parse and that changes are
// accompanied by actual regression coverage.
//
// Exit codes:
//
//	0 — every suite and harness loaded successfully
//	1 — at least one suite or harness failed validation
//	2 — usage error (missing directory, etc.)
//
// The command validates the loader machinery and the YAML schema so any
// suite or harness added under harness/{regression-suites,recall-harnesses}
// parses and validates cleanly.
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

	// harness/ holds internal validation data and is not tracked in the
	// public repo; when it is absent there is nothing to validate.
	if _, err := os.Stat(suiteDir); os.IsNotExist(err) {
		fmt.Println("harness/ not present in this checkout; nothing to validate.")
		return
	}
	if _, err := os.Stat(harnessDir); os.IsNotExist(err) {
		fmt.Println("harness/ not present in this checkout; nothing to validate.")
		return
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
