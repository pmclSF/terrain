package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/findinghistory"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// TestFindingHistory_PipelineWritesAndRendererReads is the end-to-end
// contract test for the § P5.7 demote loop:
//
//  1. Pipeline step 10e increments per-(rule, file) counters.
//  2. After N=DefaultThreshold un-dismissed runs, ShouldDemote → true.
//  3. The renderer (via LoadFindingHistory + the changescope HistoryStore
//     interface) demotes the card from gate label to observability label.
//
// We exercise (1) and (2) directly here (the renderer side is covered by
// internal/changescope/render_history_test.go). The point of this test
// is to prove the pipeline → on-disk store round trip works for a real
// snapshot.
func TestFindingHistory_PipelineWritesAndRendererReads(t *testing.T) {
	tmp := t.TempDir()

	// Synthesize a snapshot with one repeated gate-tier signal across
	// 3 fictional analyze runs. We call updateFindingHistory directly
	// (the pipeline's own step 10e calls the same function) so the
	// test stays hermetic — it doesn't need a real repo to scan.
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:     models.SignalType(signals.SignalUntestedExport),
				Severity: models.SeverityHigh,
				Location: models.SignalLocation{File: "src/auth/login.ts", Line: 12},
			},
		},
	}

	// Three un-dismissed fires.
	for i := 0; i < findinghistory.DefaultThreshold; i++ {
		updateFindingHistory(snap, tmp)
	}

	// The on-disk file must exist and report a demote.
	loaded, err := LoadFindingHistory(tmp)
	if err != nil {
		t.Fatalf("LoadFindingHistory: %v", err)
	}
	if !loaded.ShouldDemote(string(signals.SignalUntestedExport), "src/auth/login.ts") {
		t.Errorf("after %d fires, ShouldDemote should be true; entries: %+v",
			findinghistory.DefaultThreshold, loaded.All())
	}

	// One dismiss must flip ShouldDemote back to false.
	loaded.Dismiss(string(signals.SignalUntestedExport), "src/auth/login.ts")
	path := filepath.Join(tmp, findinghistory.DefaultPath)
	if err := loaded.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := LoadFindingHistory(tmp)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ShouldDemote(string(signals.SignalUntestedExport), "src/auth/login.ts") {
		t.Error("post-dismiss reload: ShouldDemote should be false")
	}
}

// TestFindingHistory_PipelineSkipsOnEmptySnapshot guards the no-op
// branch — an analyze that produces zero signals must not create a
// history file. This matters because adopters running terrain on
// a no-AI repo would otherwise see an empty `.terrain/finding-history.yaml`
// committed by accident, which leaks "terrain ran here" to anyone
// reading the diff.
func TestFindingHistory_PipelineSkipsOnEmptySnapshot(t *testing.T) {
	tmp := t.TempDir()
	updateFindingHistory(&models.TestSuiteSnapshot{}, tmp)
	updateFindingHistory(nil, tmp)

	path := filepath.Join(tmp, findinghistory.DefaultPath)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("empty/nil snapshot must not create %q (err: %v)", path, err)
	}
}

// TestFindingHistory_PipelineSkipsEmptyTypeOrFile prevents a subtle
// corruption: snapshots can carry signals without a usable Location
// (e.g. global-scope advisories). Incrementing with an empty file
// would create entries that can never be matched by file-specific
// dismisses. The pipeline must skip these silently.
func TestFindingHistory_PipelineSkipsEmptyTypeOrFile(t *testing.T) {
	tmp := t.TempDir()

	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "", Location: models.SignalLocation{File: "src/x.ts"}},
			{Type: models.SignalType(signals.SignalUntestedExport), Location: models.SignalLocation{}},
		},
	}
	updateFindingHistory(snap, tmp)

	loaded, err := LoadFindingHistory(tmp)
	if err != nil {
		t.Fatalf("LoadFindingHistory: %v", err)
	}
	if len(loaded.All()) != 0 {
		t.Errorf("empty type or file rows must not create entries; got %+v", loaded.All())
	}
}
