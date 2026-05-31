package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestDetectorDrift_ReferenceFixture runs the full analyze pipeline
// against a checked-in reference fixture and asserts that the per-
// detector signal counts stay within tolerance of a committed baseline.
//
// Purpose: catch the kind of regression the 80-repo OSS audit
// surfaced — detector behavior drifting silently because no test
// in the suite covered the per-detector signal volume. Today,
// modifying a detector heuristic might inadvertently double the FP
// rate on real repos with no test failure to warn us.
//
// How it works:
//  1. Run RunPipelineWithSignals on tests/fixtures/terrain-world
//  2. Count signals per detector type
//  3. Compare with baseline at tests/fixtures/terrain-world/detector-counts.baseline.json
//  4. Fail when any count drifts >tolerance from the baseline
//
// Updating the baseline (when a fix intentionally changes behavior):
//  TERRAIN_UPDATE_DRIFT_BASELINE=1 go test ./internal/engine/ \
//      -run TestDetectorDrift_ReferenceFixture
//
// Tolerance: ±15% per detector OR ±2 absolute, whichever is larger.
// Wider than ideal but accommodates the natural variance from small-N
// detectors (a single new finding flips ±50% on a count-of-2). Tighten
// to ±5% once the reference fixture grows.
func TestDetectorDrift_ReferenceFixture(t *testing.T) {
	const fixtureDir = "../../tests/fixtures/terrain-world"
	const baselinePath = fixtureDir + "/detector-counts.baseline.json"

	abs, _ := filepath.Abs(fixtureDir)
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("reference fixture missing at %s: %v", abs, err)
	}

	result, err := RunPipeline(abs, PipelineOptions{
		EngineVersion: "test-detector-drift",
	})
	if err != nil {
		t.Fatalf("analyze pipeline: %v", err)
	}

	counts := countByDetector(result.Snapshot.Signals)

	if os.Getenv("TERRAIN_UPDATE_DRIFT_BASELINE") == "1" {
		writeBaseline(t, baselinePath, counts)
		t.Logf("baseline written to %s; re-run without TERRAIN_UPDATE_DRIFT_BASELINE=1 to verify", baselinePath)
		return
	}

	baseline := readBaseline(t, baselinePath)
	checkDrift(t, counts, baseline)
}

// countByDetector groups signals by type and returns counts.
func countByDetector(sigs []models.Signal) map[string]int {
	out := map[string]int{}
	for _, s := range sigs {
		out[string(s.Type)]++
	}
	return out
}

// writeBaseline serialises detector counts in deterministic order.
func writeBaseline(t *testing.T, path string, counts map[string]int) {
	t.Helper()
	type row struct {
		Detector string `json:"detector"`
		Count    int    `json:"count"`
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]row, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, row{Detector: k, Count: counts[k]})
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
}

func readBaseline(t *testing.T, path string) map[string]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline %s: %v (run with TERRAIN_UPDATE_DRIFT_BASELINE=1 to create)", path, err)
	}
	type row struct {
		Detector string `json:"detector"`
		Count    int    `json:"count"`
	}
	var rows []row
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	out := map[string]int{}
	for _, r := range rows {
		out[r.Detector] = r.Count
	}
	return out
}

// checkDrift compares actual counts against baseline; per-detector
// tolerance = max(15% of baseline, 2 absolute). Reports the full list
// of drifts (not just the first) so a single test run shows the whole
// regression picture.
func checkDrift(t *testing.T, actual, baseline map[string]int) {
	t.Helper()
	type drift struct {
		detector string
		actual   int
		baseline int
		delta    int
		pct      float64
	}
	var drifts []drift

	all := map[string]bool{}
	for k := range actual {
		all[k] = true
	}
	for k := range baseline {
		all[k] = true
	}
	for det := range all {
		a := actual[det]
		b := baseline[det]
		if a == b {
			continue
		}
		tolerance := int(float64(b) * 0.15)
		if tolerance < 2 {
			tolerance = 2
		}
		delta := a - b
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		if abs <= tolerance {
			continue
		}
		pct := 0.0
		if b > 0 {
			pct = float64(delta) / float64(b) * 100
		}
		drifts = append(drifts, drift{detector: det, actual: a, baseline: b, delta: delta, pct: pct})
	}

	if len(drifts) == 0 {
		return
	}
	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].pct == drifts[j].pct {
			return drifts[i].detector < drifts[j].detector
		}
		return drifts[i].pct < drifts[j].pct
	})

	var msg string
	msg += fmt.Sprintf("detector counts drifted %d of %d detectors\n", len(drifts), len(all))
	msg += "(update baseline with TERRAIN_UPDATE_DRIFT_BASELINE=1 if change is intentional)\n\n"
	msg += fmt.Sprintf("%-40s %8s %8s %8s %8s\n", "detector", "baseline", "actual", "delta", "pct")
	msg += "-----------------------------------------------------------------------------\n"
	for _, d := range drifts {
		msg += fmt.Sprintf("%-40s %8d %8d %+8d %+8.1f%%\n", d.detector, d.baseline, d.actual, d.delta, d.pct)
	}
	t.Error(msg)
}
