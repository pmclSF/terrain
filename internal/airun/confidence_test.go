package airun

import (
	"math"
	"testing"
)

// TestWilsonInterval_Centers checks that the interval brackets the
// observed proportion for the standard cases.
func TestWilsonInterval_Centers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		successes, total int
	}{
		{45, 50},   // 90%
		{99, 100},  // 99%
		{1, 100},   // 1%
		{500, 1000}, // 50%
		{50, 50},   // 100%
		{0, 50},    // 0%
	}
	for _, c := range cases {
		lo, hi := WilsonInterval95(c.successes, c.total)
		p := float64(c.successes) / float64(c.total)
		// Wilson interval should bracket p (with rounding tolerance).
		if !(lo <= p+1e-9 && p-1e-9 <= hi) {
			t.Errorf("p=%.3f for (%d/%d) not bracketed by [%.3f, %.3f]",
				p, c.successes, c.total, lo, hi)
		}
		// Interval is in [0,1].
		if lo < 0 || hi > 1 {
			t.Errorf("interval out of [0,1]: [%.3f, %.3f]", lo, hi)
		}
	}
}

// TestWilsonInterval_NarrowsWithLargerN checks that the interval
// shrinks as n grows for the same proportion.
func TestWilsonInterval_NarrowsWithLargerN(t *testing.T) {
	t.Parallel()

	lo10, hi10 := WilsonInterval95(9, 10)
	lo100, hi100 := WilsonInterval95(90, 100)
	lo1k, hi1k := WilsonInterval95(900, 1000)

	w10 := hi10 - lo10
	w100 := hi100 - lo100
	w1k := hi1k - lo1k

	if !(w10 > w100 && w100 > w1k) {
		t.Errorf("interval widths should shrink: 10=%.3f, 100=%.3f, 1k=%.3f",
			w10, w100, w1k)
	}
}

// TestWilsonInterval_ZeroOrFullObserved confirms the edge cases that
// trip up the naive normal-approximation interval. Wilson should
// produce a non-degenerate interval at the boundaries.
func TestWilsonInterval_ZeroOrFullObserved(t *testing.T) {
	t.Parallel()

	loLow, hiLow := WilsonInterval95(0, 100)
	if loLow != 0 {
		t.Errorf("0/100 lower bound should clamp to 0, got %.3f", loLow)
	}
	if hiLow <= 0 {
		t.Errorf("0/100 upper bound should be non-zero, got %.3f", hiLow)
	}

	loHigh, hiHigh := WilsonInterval95(100, 100)
	if hiHigh != 1 {
		t.Errorf("100/100 upper bound should clamp to 1, got %.3f", hiHigh)
	}
	if loHigh >= 1 {
		t.Errorf("100/100 lower bound should be < 1, got %.3f", loHigh)
	}
}

// TestWilsonInterval_NoData returns the maximum-uncertainty interval.
func TestWilsonInterval_NoData(t *testing.T) {
	t.Parallel()
	lo, hi := WilsonInterval95(0, 0)
	if lo != 0 || hi != 1 {
		t.Errorf("no-data interval = [%.3f, %.3f], want [0, 1]", lo, hi)
	}
}

// TestWilsonInterval_BoundedInputs handles negative or out-of-range
// successes by clamping rather than producing NaN.
func TestWilsonInterval_BoundedInputs(t *testing.T) {
	t.Parallel()
	// successes > total → treated as total.
	lo, hi := WilsonInterval95(150, 100)
	if math.IsNaN(lo) || math.IsNaN(hi) {
		t.Errorf("got NaN for clamped inputs: [%.3f, %.3f]", lo, hi)
	}
	if !(lo <= 1 && hi <= 1) {
		t.Errorf("clamped interval out of bounds: [%.3f, %.3f]", lo, hi)
	}
}

// TestWilsonInterval_KnownValues checks a few hand-computed values
// against published tables (within numerical tolerance).
//
// A binomial 50% / n=100, z=1.96 is documented at roughly [0.402, 0.598].
func TestWilsonInterval_KnownValues(t *testing.T) {
	t.Parallel()

	lo, hi := WilsonInterval95(50, 100)
	if math.Abs(lo-0.402) > 0.01 {
		t.Errorf("p=50/100 lower bound = %.4f, want ~0.402", lo)
	}
	if math.Abs(hi-0.598) > 0.01 {
		t.Errorf("p=50/100 upper bound = %.4f, want ~0.598", hi)
	}
}
