// Package budget caps the number of findings any single rule may emit
// in one analyze run. Heuristic-precision detectors that fire hundreds
// of times on real-world repos (untestedExport on Java monorepos,
// uncoveredAISurface on large prompt collections, weakAssertion on
// pandas-style codebases) are noise unless bounded.
//
// Budget is configured per-rule in terrain.yaml:
//
//	rules:
//	  coverage/untested-export:
//	    max_findings: 50
//
// The cap fires post-detector, post-suppression, so it applies to
// what the adopter would otherwise see. Top-N selection sorts by
// severity DESC (critical > high > medium > low > info) then by
// confidence DESC, then by file ASC for determinism.
//
// Pruned counts are returned so the caller can surface a notice;
// silent dropping is anti-pattern.
package budget

import (
	"sort"

	"github.com/pmclSF/terrain/internal/models"
)

// Apply caps each rule's emitted findings at its budget. budgets maps
// rule_id (with the "terrain/" prefix) to max-findings (>0). Returns
// a per-rule count of findings pruned. The snapshot is mutated in
// place — the Signals slice is replaced with the kept subset.
func Apply(snap *models.TestSuiteSnapshot, budgets map[string]int) map[string]int {
	if snap == nil || len(budgets) == 0 {
		return nil
	}

	// Bucket signals by rule_id.
	byRule := map[string][]int{}
	for i, s := range snap.Signals {
		if s.RuleID == "" {
			continue
		}
		byRule[s.RuleID] = append(byRule[s.RuleID], i)
	}

	pruned := map[string]int{}
	dropIdx := map[int]bool{}
	for rule, max := range budgets {
		if max <= 0 {
			continue
		}
		idxs, ok := byRule[rule]
		if !ok || len(idxs) <= max {
			continue
		}
		// Stable priority sort — keep the top `max`.
		sort.SliceStable(idxs, func(a, b int) bool {
			sa, sb := snap.Signals[idxs[a]], snap.Signals[idxs[b]]
			if sa.Severity != sb.Severity {
				return severityRank(sa.Severity) > severityRank(sb.Severity)
			}
			if sa.Confidence != sb.Confidence {
				return sa.Confidence > sb.Confidence
			}
			return sa.Location.File < sb.Location.File
		})
		for _, i := range idxs[max:] {
			dropIdx[i] = true
		}
		pruned[rule] = len(idxs) - max
	}

	if len(dropIdx) == 0 {
		return pruned
	}
	kept := make([]models.Signal, 0, len(snap.Signals)-len(dropIdx))
	for i, s := range snap.Signals {
		if dropIdx[i] {
			continue
		}
		kept = append(kept, s)
	}
	snap.Signals = kept
	return pruned
}

func severityRank(s models.SignalSeverity) int {
	switch s {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	}
	return 0
}
