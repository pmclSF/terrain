package preview

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// LatencyObservation is one runtime sample.
type LatencyObservation struct {
	Surface  string
	IsFirst  bool // first request of a fresh process
	Millis   float64
}

// DetectColdStartTime fires when the first-request latency exceeds
// (multiplier × warm-P50 latency). Implements
// terrain/performance/cold-start-time.
//
// 0.2.0 takes pre-computed observations; runtime telemetry collection
// is followup work.
func DetectColdStartTime(samples []LatencyObservation, multiplier float64) []models.Signal {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	bySurface := map[string][]LatencyObservation{}
	for _, s := range samples {
		bySurface[s.Surface] = append(bySurface[s.Surface], s)
	}

	var out []models.Signal
	for surface, obs := range bySurface {
		var coldMs float64
		var warmMs []float64
		for _, o := range obs {
			if o.IsFirst {
				coldMs = o.Millis
			} else {
				warmMs = append(warmMs, o.Millis)
			}
		}
		if coldMs == 0 || len(warmMs) == 0 {
			continue
		}
		p50 := percentile(warmMs, 0.5)
		if p50 == 0 {
			continue
		}
		if coldMs <= p50*multiplier {
			continue
		}
		out = append(out, signal(
			signals.SignalColdStartTime, models.SeverityLow,
			"terrain/performance/cold-start-time",
			"docs/rules/performance/cold-start-time.md",
			models.SignalLocation{File: surface},
			fmt.Sprintf("Cold-start latency %.0fms is %.1f× the warm P50 (%.0fms).", coldMs, coldMs/p50, p50),
			"Preload model weights, vector indexes, and heavy imports at process startup so first requests don't pay the load cost.",
			map[string]any{"cold_ms": coldMs, "warm_p50_ms": p50, "ratio": coldMs / p50},
		))
	}
	return out
}

// percentile returns the value at the p-quantile of xs (0 ≤ p ≤ 1).
// Uses linear interpolation between adjacent samples after sorting.
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := append([]float64(nil), xs...)
	insertionSort(sorted)
	idx := p * float64(len(sorted)-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func insertionSort(xs []float64) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j-1] > xs[j]; j-- {
			xs[j-1], xs[j] = xs[j], xs[j-1]
		}
	}
}

// CostObservation is one eval-run cost sample.
type CostObservation struct {
	RunID       string
	TotalTokens int
	CostUSD     float64
}

// DetectTokenCostBudget fires when an eval run's cost exceeds either:
//   - the absolute ceiling (USD), or
//   - the ratio threshold relative to baseline.
//
// Implements terrain/performance/token-cost-budget.
func DetectTokenCostBudget(baseline, current *CostObservation, absoluteCeilingUSD float64, ratioThreshold float64) []models.Signal {
	if current == nil {
		return nil
	}
	if absoluteCeilingUSD > 0 && current.CostUSD > absoluteCeilingUSD {
		return []models.Signal{signal(
			signals.SignalTokenCostBudget, models.SeverityMedium,
			"terrain/performance/token-cost-budget",
			"docs/rules/performance/token-cost-budget.md",
			models.SignalLocation{File: current.RunID},
			fmt.Sprintf("Eval run cost $%.2f exceeds budget $%.2f.", current.CostUSD, absoluteCeilingUSD),
			"Inspect prompt / context size growth. Consider switching to a smaller model for non-critical paths.",
			map[string]any{"cost_usd": current.CostUSD, "ceiling_usd": absoluteCeilingUSD},
		)}
	}
	if baseline != nil && ratioThreshold > 0 && baseline.CostUSD > 0 {
		ratio := current.CostUSD / baseline.CostUSD
		if ratio >= 1+ratioThreshold {
			return []models.Signal{signal(
				signals.SignalTokenCostBudget, models.SeverityMedium,
				"terrain/performance/token-cost-budget",
				"docs/rules/performance/token-cost-budget.md",
				models.SignalLocation{File: current.RunID},
				fmt.Sprintf("Eval run cost grew %.1fx vs baseline ($%.2f → $%.2f).", ratio, baseline.CostUSD, current.CostUSD),
				"Inspect the diff for prompt growth, longer contexts, or model upgrades. Update the budget if the increase is intentional.",
				map[string]any{"cost_usd": current.CostUSD, "baseline_usd": baseline.CostUSD, "ratio": ratio},
			)}
		}
	}
	return nil
}
