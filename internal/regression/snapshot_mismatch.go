package regression

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/evaladapter"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectSnapshotMismatch compares the eval cases (their output
// summaries) between baseline and current and emits a Signal for
// each case whose recorded "output snapshot" diverged. Implements
// terrain/regression/snapshot-mismatch.
//
// Snapshot vs metric distinction:
//   - eval-regression: fires on Score deltas (the metric moved)
//   - snapshot-mismatch: fires when the case's recorded *output*
//     changed even when the metric didn't (e.g., the model started
//     producing different text that still happens to score the same)
//
// At 0.2.0 the snapshot used is the case's Reason field — adapters
// populate Reason from gradingResult.reason / failure_reason /
// equivalent. This is a coarse proxy for "did the model output
// change"; richer per-case output capture is followup work that
// requires snapshot files alongside the eval results JSON.
func DetectSnapshotMismatch(baseline, current *evaladapter.EvalRun) []models.Signal {
	if baseline == nil || current == nil {
		return nil
	}

	baseByID := make(map[string]evaladapter.EvalCaseResult, len(baseline.Cases))
	for _, c := range baseline.Cases {
		baseByID[c.ID] = c
	}

	var out []models.Signal
	for _, cur := range current.Cases {
		base, ok := baseByID[cur.ID]
		if !ok {
			continue
		}
		if base.Reason == cur.Reason {
			continue
		}
		// Both empty reason → no mismatch.
		if base.Reason == "" && cur.Reason == "" {
			continue
		}

		out = append(out, models.Signal{
			Type:             signals.SignalSnapshotMismatch,
			Category:         models.CategoryAI,
			Severity:         models.SeverityMedium,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceEvalExecution,
			Location: models.SignalLocation{
				File: current.Source,
			},
			Explanation: fmt.Sprintf(
				"Eval case %q snapshot diverged. Baseline reason: %q. Current reason: %q.",
				cur.Name, base.Reason, cur.Reason,
			),
			SuggestedAction: "If the new output is correct, accept it by running `terrain ai record`. Otherwise inspect the diff for prompt / model / retrieval changes that affect this case.",
			RuleID:          "terrain/regression/snapshot-mismatch",
			RuleURI:         "docs/rules/regression/snapshot-mismatch.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"caseId":   cur.ID,
				"caseName": cur.Name,
			},
		})
	}
	return out
}
