package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/changescope"
	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/explain"
	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/metrics"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/reporting"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

// runImpactPipeline runs the analysis pipeline, computes a git diff changeset,
// performs impact analysis, and applies edge-case policy. This is the shared
// core for runImpact, runSelectTests, and runPR.
func runImpactPipeline(root, baseRef string, opts engine.PipelineOptions) (*impact.ImpactResult, *engine.PipelineResult, error) {
	result, err := runPipelineWithSignals(root, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("analysis failed: %w", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	cs, err := impact.ChangeSetFromGitDiff(absRoot, baseRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to determine changed files: %w", err)
	}

	impactResult := impact.AnalyzeChangeSet(cs, result.Snapshot)
	applyImpactPolicy(impactResult, result)

	return impactResult, result, nil
}

func runImpact(root, baseRef string, jsonOutput bool, show, ownerFilter string, explainSelection bool) error {
	impactResult, _, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		// Designed remediation for impact-pipeline failures.
		// Same shape as runPR's remediation since the
		// underlying failure modes are identical (missing
		// base ref, shallow clone, empty diff).
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "error: report impact failed: %v\n", err)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Common causes:")
			fmt.Fprintln(os.Stderr, "  - --base ref doesn't exist (default: HEAD~1; try --base main if working off a feature branch)")
			fmt.Fprintln(os.Stderr, "  - shallow clone in CI: `git fetch --unshallow` or fetch the base ref explicitly")
			fmt.Fprintln(os.Stderr, "  - diff is empty (no changed files; nothing to impact)")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "If the underlying analysis failed, run `terrain analyze` directly to see the root cause.")
		}
		return err
	}

	// Apply owner filter if specified.
	if ownerFilter != "" {
		impactResult = impact.FilterByOwner(impactResult, ownerFilter)
	}

	// `--explain-selection` surfaces the structured reason chains that
	// answer "which tests matter for this PR, and why" — the chains that
	// internal/explain produces and
	// renders them via the existing RenderSelectionExplanation. Passes
	// `verbose=true` so per-test evidence (selection reasons, code unit
	// matches, confidence) is included; that's the whole point of the
	// flag.
	if explainSelection {
		sel, err := explain.ExplainSelection(impactResult)
		if err != nil {
			return fmt.Errorf("could not build selection explanation: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(sel)
		}
		reporting.RenderSelectionExplanation(os.Stdout, sel, true)
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(impactResult)
	}

	switch show {
	case "units":
		reporting.RenderImpactUnits(os.Stdout, impactResult)
	case "gaps":
		reporting.RenderImpactGaps(os.Stdout, impactResult)
	case "tests":
		reporting.RenderImpactTests(os.Stdout, impactResult)
	case "owners":
		reporting.RenderImpactOwners(os.Stdout, impactResult)
	case "graph":
		reporting.RenderImpactGraph(os.Stdout, impactResult)
	case "selected":
		reporting.RenderProtectiveSet(os.Stdout, impactResult)
	case "":
		reporting.RenderImpactReport(os.Stdout, impactResult)
	default:
		return fmt.Errorf("unknown --show value: %q (valid: units, gaps, tests, owners, graph, selected)", show)
	}
	return nil
}

// runSelectTests performs impact analysis and outputs the protective test set.
func runSelectTests(root, baseRef string, jsonOutput bool, format string) error {
	impactResult, _, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput || format != ""))
	if err != nil {
		if !jsonOutput && format == "" {
			fmt.Fprintf(os.Stderr, "error: report select-tests failed: %v\n", err)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Common causes (same as report impact):")
			fmt.Fprintln(os.Stderr, "  - --base ref doesn't exist or shallow clone needs `git fetch --unshallow`")
			fmt.Fprintln(os.Stderr, "  - underlying analysis failed — run `terrain analyze` for the root cause")
		}
		return err
	}

	// --format paths emits one bare test-file path per line, suitable
	// for `terrain select-tests --format paths | xargs <test-runner>`.
	// Documented usage in docs/user-guides/impact-analysis-and-test-selection.md.
	if format == "paths" {
		ps := impactResult.ProtectiveSet
		if ps == nil {
			return nil
		}
		seen := map[string]bool{}
		for _, t := range ps.Tests {
			if t.Path == "" || seen[t.Path] {
				continue
			}
			seen[t.Path] = true
			fmt.Println(t.Path)
		}
		return nil
	}
	if format != "" && format != "json" && format != "text" {
		return fmt.Errorf("invalid --format %q (valid: json, text, paths)", format)
	}

	if jsonOutput || format == "json" {
		ps := impactResult.ProtectiveSet
		// Ensure Tests serializes as [] not null.
		if ps != nil && ps.Tests == nil {
			ps.Tests = []impact.SelectedTest{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ps)
	}

	reporting.RenderProtectiveSet(os.Stdout, impactResult)
	return nil
}

// applyImpactPolicy applies edge-case policy and manual coverage overlay to
// an impact result. This should be called after AnalyzeChangeSet for every
// command that surfaces impact data to users.
func applyImpactPolicy(impactResult *impact.ImpactResult, result *engine.PipelineResult) {
	snapshot := result.Snapshot
	dg := result.Graph
	dgCov := depgraph.AnalyzeCoverage(dg)
	dgDupes := depgraph.DetectDuplicates(dg)
	dgFanout := depgraph.AnalyzeFanout(dg, depgraph.DefaultFanoutThreshold)
	ms := metrics.Derive(snapshot)
	pi := depgraph.ProfileInsights{
		Coverage:   &dgCov,
		Duplicates: &dgDupes,
		Fanout:     &dgFanout,
		Snapshot:   analyze.BuildSnapshotProfileData(snapshot),
	}
	dgProfile := depgraph.AnalyzeProfile(dg, pi)
	depgraph.EnrichProfileWithHealthRatios(&dgProfile, ms.Health.SkippedTestRatio, ms.Health.FlakyTestRatio)
	dgEdgeCases := depgraph.DetectEdgeCases(dgProfile, dg, pi)
	if len(dgEdgeCases) > 0 {
		dgPolicy := depgraph.ApplyEdgeCasePolicy(dgEdgeCases, dgProfile)
		impactResult.ApplyEdgeCasePolicy(dgPolicy.ConfidenceAdjustment, dgPolicy.RiskElevated, dgPolicy.Recommendations)
	}

	if len(snapshot.ManualCoverage) > 0 {
		impactResult.ApplyManualCoverageOverlay(snapshot.ManualCoverage)
	}
}

type prRunOpts struct {
	Root            string
	BaseRef         string
	JSONOutput      bool
	Format          string
	Gate            severityGate
	BaselinePath    string
	NewFindingsOnly bool
	// NoTrustFloor opts out of the default remediation-validity gate, matching
	// `terrain analyze/test --no-trust-floor`, so all CI surfaces gate alike.
	NoTrustFloor bool
}

func runPR(o prRunOpts) error {
	if o.BaselinePath != "" {
		if err := validateExistingPaths("--baseline", []string{o.BaselinePath}); err != nil {
			return err
		}
	}
	if o.NewFindingsOnly && o.BaselinePath == "" {
		return fmt.Errorf("--new-findings-only requires --baseline <path>")
	}

	pipelineOpts := defaultPipelineOptionsWithProgress(o.JSONOutput)
	pipelineOpts.BaselineSnapshotPath = o.BaselinePath
	pipelineOpts.NewFindingsOnly = o.NewFindingsOnly
	impactResult, result, err := runImpactPipeline(o.Root, o.BaseRef, pipelineOpts)
	if err == nil && result != nil && result.Snapshot != nil {
		// Same drift detectors every other surface runs, via the shared helper —
		// so PR drift carries the metadata its fix producer needs and the PR
		// surface agrees with analyze/test/check-runs on what drift blocks.
		err = appendDriftSignals(result.Snapshot, o.Root, o.BaseRef)
	}
	if err != nil {
		// The impact pipeline can fail for half a dozen
		// different reasons —
		// missing git history, no base ref, unparseable diff,
		// analysis crash. Wrap with a hint about the most
		// adopter-actionable cause.
		if !o.JSONOutput {
			fmt.Fprintf(os.Stderr, "error: report pr failed: %v\n", err)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Common causes:")
			fmt.Fprintln(os.Stderr, "  - --base ref doesn't exist (default: HEAD~1; try --base main if working off a feature branch)")
			fmt.Fprintln(os.Stderr, "  - shallow clone in CI: `git fetch --unshallow` or fetch the base ref explicitly")
			fmt.Fprintln(os.Stderr, "  - diff is empty (no changed files; report pr is a no-op then)")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "If the underlying analysis failed, run `terrain analyze` directly to see the root cause.")
			// Return original error so the caller's exit code is unchanged.
		}
		return err
	}

	pr := changescope.AnalyzePRFromImpact(impactResult, result.Snapshot)

	// Compute the gate decision BEFORE rendering so the report renders
	// for every output format (json, markdown, comment, annotation,
	// default text), AND the gate error returns through the same code
	// path. The renderer always completes (stdout stays a valid document)
	// before the exit decision is made, so every output format is rendered
	// and the gate returns through the error channel.
	// Trust floor (default on): a gate-tier detector finding may fail the build
	// only when its remediation is closed-loop validated — the same rule
	// `terrain analyze/test --fail-on` applies, so the surfaces never disagree.
	// The raw snapshot signals carry the metadata the fix producers need; a
	// change-scoped finding without a proven fix still SHOWS in the comment, it
	// just doesn't count toward the gate.
	cfg, _ := terrainconfig.LoadForRoot(o.Root)
	trustFloor := resolveTrustFloor(false, o.NoTrustFloor, cfg)
	blockableTypeFile := map[string]bool{}
	if blockable := gateBlockable(o.Root, trustFloor); blockable != nil {
		for _, s := range result.Snapshot.Signals {
			if blockable(s) {
				blockableTypeFile[string(s.Type)+"\x00"+s.Location.File] = true
			}
		}
	}
	// gateCounts reports whether a change-scoped detector finding may fail the
	// build. Trust floor off → gate-relevance alone decides (unchanged). Trust
	// floor on → only when a validated-fix signal of the same (type, file) exists.
	gateCounts := func(sigType, path string) bool {
		if !trustFloor {
			return true
		}
		return blockableTypeFile[sigType+"\x00"+path]
	}

	// Collect severities for the gate decision, filtering out
	// observability-tier findings so they don't block CI.
	severities := make([]string, 0, len(pr.NewFindings))
	// Track which (signal type, file) findings the change-scoped and AI loops
	// already counted, so the raw-snapshot loop below can skip them instead of
	// counting the same finding twice toward the displayed BlockingCount.
	countedTypeFile := map[string]bool{}
	for _, f := range pr.NewFindings {
		// SignalType is empty for protection_gap entries (always gate-
		// relevant). For existing_signal entries, skip when the
		// underlying detector is observability tier.
		if f.SignalType != "" && !signals.IsGateRelevant(models.SignalType(f.SignalType)) {
			continue
		}
		// Under the trust floor, a gate-tier detector finding blocks only when
		// its remediation is validated (protection_gap entries, SignalType "",
		// are a changescope construct with no detector fix — left as-is).
		if f.SignalType != "" && !gateCounts(f.SignalType, f.Path) {
			continue
		}
		severities = append(severities, f.Severity)
		if f.SignalType != "" {
			countedTypeFile[f.SignalType+"\x00"+f.Path] = true
		}
	}
	if pr.AI != nil {
		for _, s := range pr.AI.BlockingSignals {
			if !gateCounts(s.Type, s.File) {
				continue
			}
			severities = append(severities, s.Severity)
			countedTypeFile[s.Type+"\x00"+s.File] = true
		}
	}
	// alwaysGate findings — deterministic failures, leaked secrets, safety, and
	// user policy — plus Criticals must fail the PR merge regardless of whether
	// they land on a changed file. The change-scoped AI classifier above drops
	// off-diff, repo-scope, and Medium alwaysGate findings, which would let a
	// secret in an unchanged file, a repo-level policy violation, or a test the
	// PR broke elsewhere pass the PR gate silently. Count them from the raw
	// snapshot so the PR gate agrees with analyze/test/check-runs on the
	// must-block set (still subject to --fail-on: alwaysGate bypasses the trust
	// floor, not the severity threshold). Skip findings the change-scoped and AI
	// loops above already counted, so BlockingCount reflects distinct blocking
	// findings rather than counting an on-diff must-block finding twice.
	if result != nil && result.Snapshot != nil {
		for _, s := range result.Snapshot.Signals {
			if trustFloorApplies(s) || !signals.IsGateRelevant(s.Type) {
				continue // keep only the always-block set (Critical / alwaysGate)
			}
			if countedTypeFile[string(s.Type)+"\x00"+s.Location.File] {
				continue // already counted as a change-scoped or AI blocking finding
			}
			severities = append(severities, string(s.Severity))
		}
	}
	gateBlocked, gateSummary := severityGateBlocked(o.Gate, prSeverityBreakdown(severities))
	// The PR-comment verdict must count only what actually blocks the merge, not
	// every direct-risk finding — otherwise it says "N findings block this merge"
	// on a PR that exits 0.
	if gateBlocked {
		pr.BlockingCount = countAtOrAbove(o.Gate, prSeverityBreakdown(severities))
	}
	gateErr := func() error {
		if gateBlocked {
			return fmt.Errorf("%w: --fail-on=%s matched %s", errSeverityGateBlocked, o.Gate, gateSummary)
		}
		return nil
	}

	if o.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(pr); err != nil {
			return err
		}
		return gateErr()
	}

	// Load per-repo finding-history so the PR-comment renderer can
	// demote chronically-firing-without-dismiss findings to the
	// observability footer. Missing file → empty store, the correct
	// first-run behavior. Other errors are non-fatal: log + render
	// without history rather than block the comment.
	hist, histErr := engine.LoadFindingHistory(o.Root)
	if histErr != nil {
		logging.L().Debug("finding history: load failed at render", "err", histErr)
		hist = nil
	}

	switch o.Format {
	case "markdown", "md":
		if hist != nil {
			changescope.RenderPRSummaryMarkdownWithHistory(os.Stdout, pr, hist)
		} else {
			changescope.RenderPRSummaryMarkdown(os.Stdout, pr)
		}
	case "comment":
		changescope.RenderPRCommentConcise(os.Stdout, pr)
	case "annotation", "ci":
		changescope.RenderCIAnnotation(os.Stdout, pr)
	default:
		changescope.RenderChangeScopedReport(os.Stdout, pr)
	}
	return gateErr()
}
