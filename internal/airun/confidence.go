package airun

import "math"

// WilsonInterval returns the Wilson score confidence interval for a
// proportion p = successes/total at the given z-score. Returns
// (lowerBound, upperBound), each in [0.0, 1.0].
//
// The Wilson interval is the standard go-to for binomial proportion
// CIs because it handles small samples and edge cases (p=0, p=1)
// correctly, unlike the naive normal-approximation interval. We use
// it to convert per-detector precision/recall numbers from the
// calibration corpus into ConfidenceDetail.IntervalLow / IntervalHigh
// instead of hardcoded heuristic values.
//
// z=1.96 corresponds to a 95% confidence level. WilsonInterval95() is
// the convenience wrapper.
//
// Reference:
//
//	Wilson, E. B. (1927). "Probable inference, the law of succession,
//	and statistical inference". JASA 22 (158): 209-212.
//
// total == 0 returns (0, 1) — maximally uncertain.
func WilsonInterval(successes, total int, z float64) (float64, float64) {
	if total <= 0 {
		return 0, 1
	}
	if successes < 0 {
		successes = 0
	}
	if successes > total {
		successes = total
	}
	n := float64(total)
	pHat := float64(successes) / n
	z2 := z * z

	denom := 1 + z2/n
	center := (pHat + z2/(2*n)) / denom
	margin := z * math.Sqrt(pHat*(1-pHat)/n+z2/(4*n*n)) / denom

	lo := center - margin
	hi := center + margin
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// WilsonInterval95 is WilsonInterval(successes, total, 1.959964...) —
// the standard 95% confidence level. Used by the calibration runner
// when producing per-detector ConfidenceDetail intervals.
func WilsonInterval95(successes, total int) (float64, float64) {
	return WilsonInterval(successes, total, 1.959964)
}
