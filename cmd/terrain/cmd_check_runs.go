package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/checkruns"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/terrainconfig"
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
func runCheckRuns(root, headSHA, outPath, failOn string, noTrustFloor bool) error {
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
	// The required check-run must see the prompt-drift detector too, or it goes
	// green while `terrain analyze` blocks the same commit. No base ref here;
	// the diff-free static prompt↔schema drift (the validated blocker) fires.
	if err := appendDriftSignals(result.Snapshot, abs, ""); err != nil {
		return fmt.Errorf("drift detection failed: %w", err)
	}
	// Consult per-repo finding-history so chronically-firing-without-
	// dismiss findings get routed to the observability check, matching
	// the PR-comment renderer's demote behavior. Without this, the two
	// surfaces would disagree: PR comment says [WATCH], required check
	// says failure on the same finding.
	hist, _ := engine.LoadFindingHistory(abs)
	// Gate the required check at the same --fail-on threshold AND the same
	// trust floor the CLI gate uses, so the required check-run and
	// `terrain analyze/test --fail-on` never disagree on the merge verdict: a
	// gate-tier finding without a closed-loop-validated remediation surfaces in
	// the observability check but does not fail the required gate.
	blockAt := models.SignalSeverity(strings.ToLower(strings.TrimSpace(failOn)))
	cfg, _ := terrainconfig.LoadForRoot(abs)
	trustFloor := resolveTrustFloor(false, noTrustFloor, cfg)
	bundle := checkruns.BuildBundleAtWithGate(result.Snapshot, headSHA, hist, blockAt, gateBlockable(abs, trustFloor))
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
