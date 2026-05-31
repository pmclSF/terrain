package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/checkruns"
	"github.com/pmclSF/terrain/internal/engine"
)

// runCheckRuns runs the analyze pipeline against `root`, builds the
// two-check-runs bundle from the resulting snapshot, and writes the
// JSON to stdout (or `outPath` when set).
//
// The JSON shape mirrors the GitHub Checks-API "create check run"
// request body, side-by-side for the gate + observability checks.
// Adopters' workflows post each half via `gh api`:
//
//	$ terrain report check-runs --head-sha=$GITHUB_SHA --out=/tmp/checks.json
//	$ gh api -X POST /repos/$GITHUB_REPOSITORY/check-runs \
//	    --input <(jq '.gate_check' /tmp/checks.json)
//	$ gh api -X POST /repos/$GITHUB_REPOSITORY/check-runs \
//	    --input <(jq '.observability_check' /tmp/checks.json)
func runCheckRuns(root, headSHA, outPath string) error {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	result, err := engine.RunPipeline(abs, engine.PipelineOptions{})
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	if result == nil || result.Snapshot == nil {
		return fmt.Errorf("analyze produced no snapshot")
	}
	// Consult per-repo finding-history so chronically-firing-without-
	// dismiss findings get routed to the observability check, matching
	// the PR-comment renderer's demote behavior. Without this, the two
	// surfaces would disagree: PR comment says [WATCH], required check
	// says failure on the same finding.
	hist, _ := engine.LoadFindingHistory(abs)
	bundle := checkruns.BuildBundleWithHistory(result.Snapshot, headSHA, hist)
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	if outPath != "" {
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "wrote check-runs bundle to %s\n", outPath)
		return nil
	}
	_, err = os.Stdout.Write(data)
	return err
}
