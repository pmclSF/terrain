package engine

import (
	"path/filepath"

	"github.com/pmclSF/terrain/internal/findinghistory"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
)

// updateFindingHistory increments the per-repo finding-history
// counter for every signal in the snapshot, then persists the file.
// Failure to load / save is logged at Debug — the analyze pipeline
// must not block on a history-file I/O hiccup.
//
// Skips when the snapshot has no signals (nothing to count).
//
// Contract: when (rule_id, file_path) fires ≥3 times without a
// dismiss, the renderer demotes that pair from inline to
// observability footer. This function provides the data the
// renderer consults via findinghistory.Store.ShouldDemote.
func updateFindingHistory(snap *models.TestSuiteSnapshot, root string) {
	if snap == nil || len(snap.Signals) == 0 {
		return
	}
	path := filepath.Join(root, findinghistory.DefaultPath)
	store, err := findinghistory.Load(path)
	if err != nil {
		logging.L().Debug("finding history: load failed", "path", path, "err", err)
		return
	}
	incremented := 0
	for _, s := range snap.Signals {
		// Skip signals without a usable (type, file) pair. The
		// counter is meaningless if we can't identify which
		// (detector, location) pair fired.
		if s.Type == "" || s.Location.File == "" {
			continue
		}
		store.Increment(string(s.Type), s.Location.File)
		incremented++
	}
	// No usable signals → no Save. Otherwise an analyze on a no-AI
	// repo (or on signals that are global-scope advisories without a
	// Location.File) would create an empty .terrain/finding-history.yaml
	// that the adopter accidentally commits, leaking "terrain ran here"
	// into the diff.
	if incremented == 0 {
		return
	}
	if err := store.Save(path); err != nil {
		logging.L().Debug("finding history: save failed", "path", path, "err", err)
	}
}

// LoadFindingHistory returns the on-disk Store for callers that
// want to consult ShouldDemote without going through the pipeline.
// Returns an empty store + nil error when the file doesn't exist.
//
// The renderer in internal/changescope uses this to make per-finding
// demote decisions at render time. The slash dispatcher's /dismiss
// path calls store.Dismiss + store.Save through this loader.
func LoadFindingHistory(root string) (*findinghistory.Store, error) {
	path := filepath.Join(root, findinghistory.DefaultPath)
	return findinghistory.Load(path)
}
