package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

type testRunOpts struct {
	Root            string
	Selector        string
	JSONOutput      bool
	JUnitPath       string
	SummaryPath     string
	Gate            severityGate
	BaselinePath    string
	NewFindingsOnly bool
	// NoTrustFloor opts out of the default remediation-validity gate, matching
	// `terrain analyze --no-trust-floor`, so both commands gate identically.
	NoTrustFloor bool
}

// runTestCommand implements `terrain test`. The canonical CI-mode
// wrapper around analyze: runs the analyze pipeline and emits the
// findings as CI-consumable artifacts (JUnit XML / Step Summary
// markdown / JSON).
//
// When --selector is provided, only signals whose rule_id matches the
// selector pattern are emitted. The selector syntax is `<category>`,
// `<category>/<rule>`, `<category>/*`, or `*`. Selector matches against
// manifest RuleIDs (everything after `terrain/`). Without --selector,
// every signal flows through to the artifacts.
//
// When the analyze pipeline can't run (e.g., the root isn't readable),
// the command surfaces the failure and exits non-zero — CI runs need
// an actionable failure, not a silent degradation.
func runTestCommand(o testRunOpts) error {
	root := o.Root
	selector := o.Selector
	jsonOut := o.JSONOutput
	junitPath := o.JUnitPath
	summaryPath := o.SummaryPath

	if o.BaselinePath != "" {
		if err := validateExistingPaths("--baseline", []string{o.BaselinePath}); err != nil {
			return err
		}
	}
	if o.NewFindingsOnly && o.BaselinePath == "" {
		return fmt.Errorf("--new-findings-only requires --baseline <path>")
	}

	// Map signal type → ruleID for the findings converter.
	typeToRuleID := map[models.SignalType]string{}
	for _, entry := range signals.Manifest() {
		if entry.RuleID != "" {
			typeToRuleID[entry.Type] = entry.RuleID
		}
	}

	// Build the selector-filter set when a selector is provided.
	// Empty selector → no filter (every signal flows through).
	var wantRule map[string]bool
	var matched []string
	if selector != "" {
		var err error
		matched, err = matchRulesBySelector(selector)
		if err != nil {
			return err
		}
		if len(matched) == 0 {
			return fmt.Errorf("selector %q matched no rules. Available categories: regression coverage hygiene reproducibility security performance data ai", selector)
		}
		wantRule = map[string]bool{}
		for _, r := range matched {
			wantRule[r] = true
		}
	}

	// Run the analyze pipeline.
	var filtered []models.Signal
	pipelineOpts := defaultPipelineOptionsWithProgress(jsonOut || junitPath != "" || summaryPath != "")
	pipelineOpts.BaselineSnapshotPath = o.BaselinePath
	pipelineOpts.NewFindingsOnly = o.NewFindingsOnly
	result, perr := runPipelineWithSignals(root, pipelineOpts)
	if perr != nil {
		return fmt.Errorf("analyze pipeline failed: %w", perr)
	}
	if result != nil && result.Snapshot != nil {
		// terrain test is the CI-mode wrapper around analyze, so it must see the
		// prompt-drift detector too — otherwise a drift `terrain analyze` blocks
		// silently passes here. No base ref in test mode; the diff-free static
		// prompt↔schema drift (the validated gate-blocker) still fires.
		if err := appendDriftSignals(result.Snapshot, root, ""); err != nil {
			return fmt.Errorf("drift detection failed: %w", err)
		}
	}
	// Per-rule findings budget — mirror `terrain analyze` so the CI-mode
	// wrapper writes the same capped findings.json / JUnit / Step Summary as a
	// local analyze on the same repo+config.
	if result != nil && result.Snapshot != nil {
		applyFindingsBudget(result.Snapshot, root, jsonOut)
	}
	if result != nil && result.Snapshot != nil {
		for _, s := range result.Snapshot.Signals {
			if wantRule == nil {
				filtered = append(filtered, s)
				continue
			}
			if ruleID := typeToRuleID[s.Type]; wantRule[ruleID] {
				filtered = append(filtered, s)
			}
		}
	}

	// Trust floor: terrain test is the CI-mode wrapper around analyze, so it
	// must apply the same gate semantics. Honor trust_floor from terrain.yaml
	// so a finding gated in CI reproduces with `terrain analyze` on the same
	// repo. A malformed config fails closed on source redaction.
	cfg, cfgErr := terrainconfig.LoadForRoot(root)
	if cfgErr != nil {
		logging.L().Warn("terrain test: could not load config; redacting source excerpts to be safe", "err", cfgErr)
	}
	// Trust floor is the 0.4.0 default (on); honor `--no-trust-floor` and an
	// explicit trust_floor: false opt-out from terrain.yaml so `terrain test`
	// and `terrain analyze` gate identically on the same repo.
	trustFloor := resolveTrustFloor(false, o.NoTrustFloor, cfg)
	redact := cfgErr != nil || (cfg != nil && cfg.RedactSource)

	// Build the artifact via the SHARED builder so the Step Summary and JUnit
	// reflect the same trust-floor demotion as findings.json and the exit code —
	// otherwise a held-back finding reads as a gate-blocking error on a green run.
	art := buildFindingsArtifact(root, filtered, redact, trustFloor)

	if result != nil && result.Snapshot != nil {
		if err := writeFindingsJSON(root, filtered, redact, trustFloor); err != nil {
			logging.L().Warn("terrain test: writing .terrain/findings.json", "err", err)
		}
	}

	gateBreakdown := signalSeverityBreakdown(filtered)
	if trustFloor {
		// Only heuristic findings without a validated remediation are held back
		// under the trust floor — identical to analyze. Tell the user what was
		// held back so a passing required check is never silent about it.
		raw := gateBreakdown
		gateBreakdown = trustFloorGateBreakdown(root, filtered)
		if o.Gate != severityGateNone {
			if held := trustFloorHeldBack(o.Gate, raw, gateBreakdown); held > 0 {
				fmt.Fprintf(os.Stderr,
					"trust floor: %d finding(s) at or above --fail-on=%s held back (no validated auto-fix yet) — they surface in the report but do not block CI. Run --no-trust-floor to gate on severity.\n",
					held, o.Gate)
			}
		}
	}
	gateBlocked, gateSummary := severityGateBlocked(o.Gate, gateBreakdown)
	gateErr := func() error {
		if gateBlocked {
			return fmt.Errorf("%w: --fail-on=%s matched %s", errSeverityGateBlocked, o.Gate, gateSummary)
		}
		return nil
	}

	if junitPath != "" {
		if err := writeFile(junitPath, func(f *os.File) error {
			// EmitWarnings: true so warning-severity findings (the
			// majority of medium-severity signals) appear as test
			// cases. The default JUnitOptions.EmitWarnings=false is
			// only appropriate when an adopter explicitly wants
			// errors-only — `terrain test` should surface everything
			// since CI test reporters rendering the XML are the user-
			// facing surface.
			return art.WriteJUnit(f, findings.JUnitOptions{EmitWarnings: true})
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
		if err := art.WriteJSON(os.Stdout); err != nil {
			return err
		}
		return gateErr()
	}

	// Human-readable summary. Optimized for CI logs: one line at the
	// top with what was emitted, then one line per artifact path.
	if selector == "" {
		fmt.Printf("terrain test: %d %s emitted\n", len(filtered), Plural(len(filtered), "finding"))
	} else {
		fmt.Printf("terrain test --selector %q: %d %s from %d matched %s\n",
			selector, len(filtered), Plural(len(filtered), "finding"),
			len(matched), Plural(len(matched), "rule"))
	}
	if junitPath != "" {
		fmt.Printf("  JUnit XML:       %s\n", junitPath)
	}
	if summaryPath != "" {
		fmt.Printf("  Step Summary:    %s\n", summaryPath)
	}
	if !jsonOut && junitPath == "" && summaryPath == "" {
		// No artifacts requested — show the findings inline so the
		// user can see what they would have gotten. Render from the
		// same trust-floor-demoted artifact (art.Findings) that backs
		// findings.json, JUnit, and the gate, so the terminal reports
		// one severity per finding instead of the raw signal severity.
		if len(art.Findings) == 0 {
			fmt.Println("No findings.")
			return nil
		}
		fmt.Println()
		for _, f := range art.Findings {
			loc := f.PrimaryLoc.Path
			if f.PrimaryLoc.Line > 0 {
				loc = fmt.Sprintf("%s:%d", loc, f.PrimaryLoc.Line)
			}
			fmt.Printf("  [%s] %s  %s\n", f.Severity, f.RuleID, loc)
		}
	}
	return gateErr()
}

// Plural is a tiny pluralizer for the terrain test output.
// Falls through to reporting.Plural when the count is non-1.
func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
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
