package analyze

import "fmt"

// deriveHeadline produces a single opinionated sentence from the Report.
// It evaluates conditions in priority order and returns the first match.
// All data is already computed in the Report — no new analysis.
func deriveHeadline(r *Report) string {
	if r.SignalSummary.Critical > 0 {
		return fmt.Sprintf(
			"%d high-priority signal(s) detected — review recommended.",
			r.SignalSummary.Critical,
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

	// Healthy default.
	fwCount := len(r.TestsDetected.Frameworks)
	return fmt.Sprintf(
		"Your test suite looks healthy: %d test files across %d frameworks.",
		r.TestsDetected.TestFileCount,
		fwCount,
	)
}
