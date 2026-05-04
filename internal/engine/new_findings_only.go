package engine

import (
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
)

// applyNewFindingsOnly filters the snapshot to keep only signals whose
// FindingID is NOT present in the baseline snapshot. Used by Track 4.8
// (`--new-findings-only --baseline <path>`) so established repos with
// existing debt can adopt strict CI gates on day one — the gate only
// fires on findings introduced AFTER the baseline was captured.
//
// Behavior:
//   - When `snapshot.Baseline` is nil (no `--baseline` was supplied),
//     this function logs a warning and returns the snapshot unchanged.
//     The user's `--new-findings-only` flag was inert; we tell them.
//   - When the baseline is present but contains no signals (e.g.
//     a fresh first-run baseline), every current signal counts as
//     "new" — same as no filter applied.
//   - When the baseline has signals, every (top-level + per-file)
//     signal in the current snapshot is checked against the baseline
//     FindingID set; matches are removed.
//
// Idempotent. No-op when snapshot is nil.
func applyNewFindingsOnly(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Baseline == nil {
		logging.L().Warn("--new-findings-only is inert: no --baseline supplied")
		return
	}

	baselineIDs := collectBaselineFindingIDs(snapshot.Baseline)
	if len(baselineIDs) == 0 {
		// Empty baseline — nothing to subtract; every current signal
		// is "new" by definition.
		return
	}

	beforeTop := len(snapshot.Signals)
	snapshot.Signals = filterByMissingID(snapshot.Signals, baselineIDs)
	beforeFile := 0
	afterFile := 0
	for fi := range snapshot.TestFiles {
		tf := &snapshot.TestFiles[fi]
		beforeFile += len(tf.Signals)
		tf.Signals = filterByMissingID(tf.Signals, baselineIDs)
		afterFile += len(tf.Signals)
	}

	logging.L().Info("new-findings-only applied",
		"baseline_findings", len(baselineIDs),
		"top_level_dropped", beforeTop-len(snapshot.Signals),
		"per_file_dropped", beforeFile-afterFile,
	)
}

// collectBaselineFindingIDs reads every signal in the baseline (both
// top-level Signals and per-test-file Signals) and returns the set
// of populated FindingIDs. Older baselines without finding IDs return
// an empty set — those signals can't participate in the comparison.
func collectBaselineFindingIDs(baseline *models.TestSuiteSnapshot) map[string]bool {
	if baseline == nil {
		return nil
	}
	ids := make(map[string]bool)
	for _, s := range baseline.Signals {
		if s.FindingID != "" {
			ids[s.FindingID] = true
		}
	}
	for _, tf := range baseline.TestFiles {
		for _, s := range tf.Signals {
			if s.FindingID != "" {
				ids[s.FindingID] = true
			}
		}
	}
	return ids
}

// filterByMissingID keeps signals whose FindingID is NOT in the set.
// Signals with empty FindingID are kept (we can't compare them; better
// to over-report than silently drop unidentifiable findings).
func filterByMissingID(signals []models.Signal, baselineIDs map[string]bool) []models.Signal {
	if len(signals) == 0 {
		return signals
	}
	kept := signals[:0]
	for _, s := range signals {
		if s.FindingID == "" {
			kept = append(kept, s)
			continue
		}
		if baselineIDs[s.FindingID] {
			continue
		}
		kept = append(kept, s)
	}
	return kept
}
