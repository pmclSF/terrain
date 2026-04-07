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
	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/metrics"
	"github.com/pmclSF/terrain/internal/reporting"
)

// runImpactPipeline runs the analysis pipeline, computes a git diff changeset,
// performs impact analysis, and applies edge-case policy. This is the shared
// core for runImpact, runSelectTests, and runPR.
func runImpactPipeline(root, baseRef string, opts engine.PipelineOptions) (*impact.ImpactResult, *engine.PipelineResult, error) {
	result, err := engine.RunPipeline(root, opts)
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

func runImpact(root, baseRef string, jsonOutput bool, show, ownerFilter string) error {
	impactResult, _, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return err
	}

	// Apply owner filter if specified.
	if ownerFilter != "" {
		impactResult = impact.FilterByOwner(impactResult, ownerFilter)
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

func runPR(root, baseRef string, jsonOutput bool, format string) error {
	impactResult, result, err := runImpactPipeline(root, baseRef, defaultPipelineOptionsWithProgress(jsonOutput))
	if err != nil {
		return err
	}

	pr := changescope.AnalyzePRFromImpact(impactResult, result.Snapshot)

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pr)
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
	return nil
}
