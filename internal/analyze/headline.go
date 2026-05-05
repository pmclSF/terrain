package analyze

import "fmt"

// plural returns the singular when n == 1, otherwise singular + "s".
// Local helper used to avoid `n thing(s)` notation in headline text.
func plural(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}

// deriveHeadline produces a single opinionated sentence from the Report.
// It evaluates conditions in priority order and returns the first match.
// All data is already computed in the Report — no new analysis.
func deriveHeadline(r *Report) string {
	if r.SignalSummary.Critical > 0 {
		// Use "critical" rather than "high-priority" so the
		// headline severity vocabulary matches the body. Pre-fix
		// the headline said "N high-priority signals" while the
		// body listed them as `[HIGH] N critical signals` — same
		// number, two different labels, confusing.
		return fmt.Sprintf(
			"%d critical %s detected — review recommended.",
			r.SignalSummary.Critical,
			plural(r.SignalSummary.Critical, "signal"),
		)
	}

	// Duplicate clusters — even small counts are surprising and actionable.
	if r.DuplicateClusters.ClusterCount > 0 {
		return fmt.Sprintf(
			"%d tests across %d clusters are structurally similar — consolidation would reduce CI load.",
			r.DuplicateClusters.RedundantTestCount,
			r.DuplicateClusters.ClusterCount,
		)
	}

	if r.HighFanout.FlaggedCount > 0 && len(r.HighFanout.TopNodes) > 0 {
		top := r.HighFanout.TopNodes[0]
		return fmt.Sprintf(
			"%s has %d dependents — any change ripples across many tests.",
			top.Path, top.TransitiveFanout,
		)
	}

	if r.HighFanout.FlaggedCount > 0 {
		return fmt.Sprintf(
			"%d shared fixtures have high fan-out — any change to them ripples across many tests.",
			r.HighFanout.FlaggedCount,
		)
	}

	// Skip burden is surprising and actionable.
	if r.SkippedTestBurden.SkipRatio > 0.1 {
		return fmt.Sprintf(
			"%.0f%% of tests are skipped — %d tests may be masking instability.",
			r.SkippedTestBurden.SkipRatio*100,
			r.SkippedTestBurden.SkippedCount,
		)
	}

	weakCount := len(r.WeakCoverageAreas)
	if weakCount > 0 {
		return fmt.Sprintf(
			"%d source areas have weak or no structural test coverage.",
			weakCount,
		)
	}

	if r.StabilityClusters != nil && r.StabilityClusters.UnstableTestCount > 0 {
		return fmt.Sprintf(
			"%d tests are unstable, clustering around %d shared root causes.",
			r.StabilityClusters.UnstableTestCount,
			len(r.StabilityClusters.Clusters),
		)
	}

	// Empty repo or repo with no detected tests — say so honestly
	// rather than calling zero tests "healthy".
	tfCount := r.TestsDetected.TestFileCount
	fwCount := len(r.TestsDetected.Frameworks)
	if tfCount == 0 {
		return "No test files detected. Add tests with your framework of choice, then re-run `terrain analyze`."
	}

	// Healthy default.
	return fmt.Sprintf(
		"Your test suite looks healthy: %d test %s across %d %s.",
		tfCount, plural(tfCount, "file"),
		fwCount, plural(fwCount, "framework"),
	)
}
