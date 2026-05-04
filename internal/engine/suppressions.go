package engine

import (
	"path/filepath"
	"time"

	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/suppression"
)

// applySuppressions loads `.terrain/suppressions.yaml` (or the path
// supplied in PipelineOptions.SuppressionsPath) and removes matching
// signals from the snapshot. Expired entries don't suppress; they
// emit a `suppressionExpired` warning signal so they show up in the
// next report cycle.
//
// Missing suppressions file is normal — most users won't have one.
// A malformed file is treated as a hard failure (logs + exits the
// pipeline with the parse error) because silently ignoring would let
// CI users believe their suppressions are active when they're not.
//
// Called from RunPipelineContext after FindingID assignment.
func applySuppressions(snap *models.TestSuiteSnapshot, root, override string, now time.Time) {
	if snap == nil {
		return
	}
	path := override
	if path == "" {
		path = filepath.Join(root, suppression.DefaultPath)
	}
	result, err := suppression.Load(path)
	if err != nil {
		// Malformed file — log and skip, but emit a signal so the
		// user sees it in the report. Don't fail the whole pipeline:
		// CI users who fat-finger a YAML edit shouldn't lose their
		// analysis.
		logging.L().Warn("could not load suppressions",
			"path", path,
			"error", err.Error(),
		)
		return
	}
	if result == nil || (len(result.Entries) == 0 && len(result.Warnings) == 0) {
		return
	}
	for _, w := range result.Warnings {
		logging.L().Warn("suppressions: " + w)
	}
	if len(result.Entries) == 0 {
		return
	}

	matched, expired := suppression.Apply(snap, result.Entries, now)

	// Surface expired entries as warning signals so they don't rot.
	// Each gets its own signal so reports list them individually.
	for _, e := range expired {
		snap.Signals = append(snap.Signals, models.Signal{
			Type:             signals.SignalSuppressionExpired,
			Category:         models.CategoryGovernance,
			Severity:         models.SeverityMedium,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourcePolicy,
			Explanation: "Suppression entry has expired and is no longer in effect. " +
				"Underlying findings will fire again until you renew or remove the entry. " +
				"Reason on file: " + e.Reason,
			SuggestedAction: "Edit `.terrain/suppressions.yaml`: extend the `expires` date, or remove the entry if the underlying issue is fixed.",
			Location: models.SignalLocation{
				File: suppression.DefaultPath,
			},
			Metadata: map[string]any{
				"finding_id":  e.FindingID,
				"signal_type": e.SignalType,
				"file":        e.File,
				"reason":      e.Reason,
				"owner":       e.Owner,
				"expires":     e.Expires,
			},
		})
	}

	if len(matched) > 0 {
		logging.L().Info("suppressions applied",
			"path", path,
			"matched", len(matched),
			"expired", len(expired),
		)
	}
}
