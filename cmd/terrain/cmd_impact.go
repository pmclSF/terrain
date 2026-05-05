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
	"github.com/pmclSF/terrain/internal/metrics"
	"github.com/pmclSF/terrain/internal/reporting"
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
		return err
	}

	// Apply owner filter if specified.
	if ownerFilter != "" {
		impactResult = impact.FilterByOwner(impactResult, ownerFilter)
	}

	// `--explain-selection` defends the pitch claim
	// "see which tests matter for a PR — and why" (Track 3.2). Surfaces
	// the structured reason chains that internal/explain produces and
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
func runSelectTests(root, baseRef string, jsonOutput bool) error {
	impactResult, _, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return err
	}

	if jsonOutput {
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

func runPR(root, baseRef string, jsonOutput bool, format string, gate severityGate) error {
	impactResult, result, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		// Audit-named gap (pr_change_scoped.P5): the impact
		// pipeline can fail for half a dozen different reasons —
		// missing git history, no base ref, unparseable diff,
		// analysis crash. Wrap with a hint about the most
		// adopter-actionable cause.
		if !jsonOutput {
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
	// path. Mirrors the pattern used by `runAnalyze` after the JSON-
	// stdout-purity bug fix in PR #134 — the renderer always completes
	// before the exit decision is made.
	severities := make([]string, 0, len(pr.NewFindings))
	for _, f := range pr.NewFindings {
		severities = append(severities, f.Severity)
	}
	if pr.AI != nil {
		for _, s := range pr.AI.BlockingSignals {
			severities = append(severities, s.Severity)
		}
	}
	gateBlocked, gateSummary := severityGateBlocked(gate, prSeverityBreakdown(severities))
	gateErr := func() error {
		if gateBlocked {
			return fmt.Errorf("%w: --fail-on=%s matched %s", errSeverityGateBlocked, gate, gateSummary)
		}
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(pr); err != nil {
			return err
		}
		return gateErr()
	}

	switch format {
	case "markdown", "md":
		changescope.RenderPRSummaryMarkdown(os.Stdout, pr)
	case "comment":
		changescope.RenderPRCommentConcise(os.Stdout, pr)
	case "annotation", "ci":
		changescope.RenderCIAnnotation(os.Stdout, pr)
	default:
		changescope.RenderChangeScopedReport(os.Stdout, pr)
	}
	return gateErr()
}
