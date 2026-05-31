package reporting

import (
	"fmt"
	"io"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// RenderAnalyzeReportV2 writes a human-readable analysis report from the
// structured analyze.Report. This is the "wow moment" first-run output.
func RenderAnalyzeReportV2(w io.Writer, r *analyze.Report) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Test Suite Analysis"))
	blank()

	// Headline — the single most important sentence.
	if r.Headline != "" {
		line("  %s", r.Headline)
		blank()
	}

	// Auto-discovered artifacts.
	if len(r.DiscoveredArtifacts) > 0 {
		for _, a := range r.DiscoveredArtifacts {
			line("  Auto-detected %s: %s (%s)", a.Kind, a.Path, a.Format)
		}
		blank()
	}

	// Key findings — top 3 prioritized issues, shown early for impact.
	if len(r.KeyFindings) > 0 {
		line("Key Findings")
		line(uitokens.H2Sep)
		for i, f := range r.KeyFindings {
			line("  %d. %s %s", i+1, uitokens.BracketedSeverity(f.Severity), f.Title)
		}
		remaining := r.TotalFindingCount - len(r.KeyFindings)
		if remaining > 0 {
			line("  %d more %s available — run `terrain insights` for the full report.", remaining, Plural(remaining, "finding"))
		}
		blank()
	}

	// Recommended actions — up to 3 prioritized things to do.
	if len(r.NextActions) > 0 {
		line("Recommended actions:")
		for i, a := range r.NextActions {
			line("  %d. %s", i+1, a.Title)
			line("     $ %s", a.Command)
			line("     %s", a.Explanation)
			if i < len(r.NextActions)-1 {
				blank()
			}
		}
		blank()
	}

	// Repository profile
	line("Repository Profile")
	line(uitokens.H2Sep)
	line("  Test volume:          %s", r.RepoProfile.TestVolume)
	line("  CI pressure:          %s", r.RepoProfile.CIPressure)
	line("  Coverage confidence:  %s", r.RepoProfile.CoverageConfidence)
	line("  Redundancy level:     %s", r.RepoProfile.RedundancyLevel)
	line("  Fanout burden:        %s", r.RepoProfile.FanoutBurden)
	if r.RepoProfile.SkipBurden != "" {
		line("  Skip burden:          %s", r.RepoProfile.SkipBurden)
	}
	if r.RepoProfile.FlakeBurden != "" {
		line("  Flake burden:         %s", r.RepoProfile.FlakeBurden)
	}
	if r.RepoProfile.ManualCoveragePresence != "" && r.RepoProfile.ManualCoveragePresence != "none" {
		line("  Manual coverage:      %s", r.RepoProfile.ManualCoveragePresence)
	}
	line("  (scale: tiny / small / medium / large / very-large for volume; low / moderate / high for *burden* dimensions, where lower is better)")
	blank()

	// Validation inventory
	line("Validation Inventory")
	line(uitokens.H2Sep)
	line("  Test files:     %d", r.TestsDetected.TestFileCount)
	line("  Test cases:     %d", r.TestsDetected.TestCaseCount)
	line("  Code units:     %d", r.TestsDetected.CodeUnitCount)
	if r.TestsDetected.CodeSurfaceCount > 0 {
		line("  Code surfaces:  %d", r.TestsDetected.CodeSurfaceCount)
	}
	if r.TestsDetected.ScenarioCount > 0 {
		line("  Scenarios:      %d", r.TestsDetected.ScenarioCount)
	}
	if r.TestsDetected.PromptCount > 0 {
		line("  Prompts:        %d", r.TestsDetected.PromptCount)
	}
	if r.TestsDetected.DatasetCount > 0 {
		line("  Datasets:       %d", r.TestsDetected.DatasetCount)
	}
	if len(r.TestsDetected.Frameworks) > 0 {
		line("  Frameworks:")
		for _, fw := range r.TestsDetected.Frameworks {
			typeBadge := ""
			if fw.Type != "" && fw.Type != "unknown" {
				typeBadge = fmt.Sprintf(" [%s]", fw.Type)
			}
			line("    %-20s %4d %s%s", fw.Name, fw.FileCount, Plural(fw.FileCount, "file"), typeBadge)
		}
	}
	blank()

	// Coverage confidence summary
	if r.CoverageConfidence.TotalFiles > 0 {
		line("Coverage Confidence")
		line(uitokens.H2Sep)
		total := r.CoverageConfidence.TotalFiles
		line("  High:    %d (%d%%)", r.CoverageConfidence.HighCount, pct(r.CoverageConfidence.HighCount, total))
		line("  Medium:  %d (%d%%)", r.CoverageConfidence.MediumCount, pct(r.CoverageConfidence.MediumCount, total))
		line("  Low:     %d (%d%%)", r.CoverageConfidence.LowCount, pct(r.CoverageConfidence.LowCount, total))
		blank()
	}

	// Duplicate clusters
	if r.DuplicateClusters.ClusterCount > 0 {
		line("Duplicate Clusters")
		line(uitokens.H2Sep)
		line("  Clusters:        %d", r.DuplicateClusters.ClusterCount)
		line("  Redundant tests: %d", r.DuplicateClusters.RedundantTestCount)
		if r.DuplicateClusters.HighestSimilarity > 0 {
			line("  Max similarity:  %.0f%%", r.DuplicateClusters.HighestSimilarity*100)
		}
		blank()
	}

	// High-fanout fixtures/helpers
	if r.HighFanout.FlaggedCount > 0 {
		line("High-Fanout Nodes")
		line(uitokens.H2Sep)
		line("  Flagged: %d (threshold: %d)", r.HighFanout.FlaggedCount, r.HighFanout.Threshold)
		for _, n := range r.HighFanout.TopNodes {
			line("    %s  (%s, %d dependents)", n.Path, n.NodeType, n.TransitiveFanout)
		}
		blank()
	}

	// Skipped test burden
	if r.SkippedTestBurden.SkippedCount > 0 {
		line("Skipped Test Burden")
		line(uitokens.H2Sep)
		line("  Skipped: %d / %d tests (%.0f%%)",
			r.SkippedTestBurden.SkippedCount,
			r.SkippedTestBurden.TotalTests,
			r.SkippedTestBurden.SkipRatio*100)
		blank()
	}

	// Weak coverage areas
	if len(r.WeakCoverageAreas) > 0 {
		line("Weak Coverage Areas")
		line(uitokens.H2Sep)
		for _, a := range r.WeakCoverageAreas {
			if a.TestCount == 0 {
				line("  %-40s no structural coverage", a.Path)
			} else {
				line("  %-40s %d %s", a.Path, a.TestCount, Plural(a.TestCount, "test"))
			}
		}
		blank()
	}

	// CI optimization potential
	if r.CIOptimization.Recommendation != "" {
		line("CI Optimization Potential")
		line(uitokens.H2Sep)
		if r.CIOptimization.DuplicateTestsRemovable > 0 {
			line("  Duplicate tests removable:  %d", r.CIOptimization.DuplicateTestsRemovable)
		}
		if r.CIOptimization.SkippedTestsReviewable > 0 {
			line("  Skipped tests reviewable:   %d", r.CIOptimization.SkippedTestsReviewable)
		}
		if r.CIOptimization.HighFanoutNodes > 0 {
			line("  High-fanout nodes:          %d", r.CIOptimization.HighFanoutNodes)
		}
		line("  %s", r.CIOptimization.Recommendation)
		blank()
	}

	// Fallback insight when no key findings were shown at the top.
	if len(r.KeyFindings) == 0 && r.TopInsight != "" {
		line("Top Insight")
		line(uitokens.H2Sep)
		line("  %s", r.TopInsight)
		blank()
	}

	// Risk posture + signal tally — one section so the headline
	// numbers sit next to their backing detail.
	if len(r.RiskPosture) > 0 || r.SignalSummary.Total > 0 {
		line("Risk Posture")
		line(uitokens.H2Sep)
		for _, d := range r.RiskPosture {
			line("  %-24s %s", d.Dimension+":", titleCase(d.Band))
		}
		if r.SignalSummary.Total > 0 {
			parts := []string{}
			if r.SignalSummary.Critical > 0 {
				parts = append(parts, fmt.Sprintf("%d critical", r.SignalSummary.Critical))
			}
			if r.SignalSummary.High > 0 {
				parts = append(parts, fmt.Sprintf("%d high", r.SignalSummary.High))
			}
			if r.SignalSummary.Medium > 0 {
				parts = append(parts, fmt.Sprintf("%d medium", r.SignalSummary.Medium))
			}
			if r.SignalSummary.Low > 0 {
				parts = append(parts, fmt.Sprintf("%d low", r.SignalSummary.Low))
			}
			breakdown := ""
			if len(parts) > 0 {
				breakdown = " (" + strings.Join(parts, ", ") + ")"
			}
			line("  %-24s %d%s", "Signals:", r.SignalSummary.Total, breakdown)
		}
		blank()
	}

	// Behavior redundancy — one summary line per cluster: the kind
	// reads as a header word, the metrics chain off it. Cleaner than
	// the previous `[wasteful] N tests, M shared surfaces (X% confidence) [pytest]`
	// which read as a metadata pile-on.
	if r.BehaviorRedundancy != nil && len(r.BehaviorRedundancy.Clusters) > 0 {
		br := r.BehaviorRedundancy
		line("Behavior Redundancy")
		line(uitokens.H2Sep)
		line("  Redundant tests:  %d across %d %s", br.RedundantTestCount, len(br.Clusters), Plural(len(br.Clusters), "cluster"))
		if br.CrossFrameworkOverlaps > 0 {
			line("  Cross-framework:  %d %s", br.CrossFrameworkOverlaps, Plural(br.CrossFrameworkOverlaps, "cluster"))
		}
		limit := 5
		if len(br.Clusters) < limit {
			limit = len(br.Clusters)
		}
		for _, c := range br.Clusters[:limit] {
			fwLabel := ""
			if len(c.Frameworks) > 0 {
				fwLabel = " in " + strings.Join(c.Frameworks, ", ")
			}
			line("  %s · %d %s exercise %d shared %s%s (%.0f%% confidence)",
				titleCase(string(c.OverlapKind)),
				len(c.Tests), Plural(len(c.Tests), "test"),
				len(c.SharedSurfaces), Plural(len(c.SharedSurfaces), "surface"),
				fwLabel, c.Confidence*100)
			line("    %s", c.Rationale)
		}
		if len(br.Clusters) > 5 {
			line("  … and %d more %s", len(br.Clusters)-5, Plural(len(br.Clusters)-5, "cluster"))
		}
		blank()
	}

	// Stability hints
	if r.StabilityClusters != nil && len(r.StabilityClusters.Clusters) > 0 {
		sc := r.StabilityClusters
		line("Stability")
		line(uitokens.H2Sep)
		line("  Unstable tests:  %d (%d clustered around shared dependencies)", sc.UnstableTestCount, sc.ClusteredTestCount)
		limit := 5
		if len(sc.Clusters) < limit {
			limit = len(sc.Clusters)
		}
		for _, c := range sc.Clusters[:limit] {
			line("  [%s] %s  (%d tests, %.0f%% confidence)",
				c.CauseKind, c.CauseName, len(c.Members), c.Confidence*100)
			line("         %s", c.Remediation)
		}
		if len(sc.Clusters) > 5 {
			line("  ... and %d more %s", len(sc.Clusters)-5, Plural(len(sc.Clusters)-5, "cluster"))
		}
		blank()
	} else if r.SkippedTestBurden.SkippedCount > 0 {
		// When we have skip data but no clusters, show skip-based stability hint.
		line("Stability")
		line(uitokens.H2Sep)
		line("  %d skipped %s detected. Skipped tests may mask instability.", r.SkippedTestBurden.SkippedCount, Plural(r.SkippedTestBurden.SkippedCount, "test"))
		line("  Provide --runtime artifacts to unlock flaky/slow/dead detection and root-cause clustering.")
		blank()
	} else if !hasDataSource(r.DataCompleteness, "runtime") {
		line("Stability")
		line(uitokens.H2Sep)
		line("  No runtime data provided. Static skip detection is already available.")
		line("  Provide --runtime (JUnit XML or Jest JSON) to unlock flaky/slow/dead detection")
		line("  and stability clustering.")
		blank()
	}

	// Matrix coverage
	if r.MatrixCoverage != nil && len(r.MatrixCoverage.Classes) > 0 {
		mc := r.MatrixCoverage
		line("Matrix Coverage")
		line(uitokens.H2Sep)
		line("  Classes analyzed:  %d", mc.ClassesAnalyzed)
		line("  Test files:        %d", mc.TestsAnalyzed)
		for _, cc := range mc.Classes {
			line("  [%s] %s: %d/%d members covered (%.0f%%)",
				cc.Dimension, cc.ClassName, cc.CoveredMembers, cc.TotalMembers, cc.CoverageRatio*100)
		}
		if len(mc.Gaps) > 0 {
			line("  Gaps:  %d uncovered members", len(mc.Gaps))
			limit := 3
			if len(mc.Gaps) < limit {
				limit = len(mc.Gaps)
			}
			for _, gap := range mc.Gaps[:limit] {
				line("    - %s (%s/%s)", gap.MemberName, gap.ClassName, gap.Dimension)
			}
			if len(mc.Gaps) > 3 {
				line("    ... and %d more", len(mc.Gaps)-3)
			}
		}
		if len(mc.Recommendations) > 0 {
			line("  Recommendations:  %d devices/environments to consider", len(mc.Recommendations))
			limit := 3
			if len(mc.Recommendations) < limit {
				limit = len(mc.Recommendations)
			}
			for _, rec := range mc.Recommendations[:limit] {
				line("    %d. %s — %s", rec.Priority, rec.MemberName, rec.Reason)
			}
		}
		blank()
	}

	// Manual coverage overlay
	if r.ManualCoverage != nil && r.ManualCoverage.ArtifactCount > 0 {
		line("Manual Coverage Overlay")
		line(uitokens.H2Sep)
		line("  Artifacts:  %d (not executable — supplements CI coverage)", r.ManualCoverage.ArtifactCount)
		if len(r.ManualCoverage.BySource) > 0 {
			parts := []string{}
			for src, count := range r.ManualCoverage.BySource {
				parts = append(parts, fmt.Sprintf("%s: %d", src, count))
			}
			line("  Sources:    %s", strings.Join(parts, ", "))
		}
		if len(r.ManualCoverage.ByCriticality) > 0 {
			parts := []string{}
			for crit, count := range r.ManualCoverage.ByCriticality {
				parts = append(parts, fmt.Sprintf("%s: %d", crit, count))
			}
			line("  Criticality: %s", strings.Join(parts, ", "))
		}
		if len(r.ManualCoverage.Areas) > 0 {
			line("  Areas:      %s", strings.Join(r.ManualCoverage.Areas, ", "))
		}
		if r.ManualCoverage.StaleCount > 0 {
			line("  Stale:      %d artifact(s) have no recent execution date", r.ManualCoverage.StaleCount)
		}
		blank()
	}

	// Edge cases
	if len(r.EdgeCases) > 0 {
		line("Edge Cases")
		line(uitokens.H2Sep)
		for _, ec := range r.EdgeCases {
			line("  %s %s", uitokens.BracketedSeverity(string(ec.Severity)), ec.Description)
		}
		blank()
	}

	// Policy recommendations — only emit when they're not just
	// restating what Edge Cases already covered (avoids the analyze
	// report saying "too few tests" three times in adjacent sections).
	if r.Policy != nil && len(r.Policy.Recommendations) > 0 && !policyDuplicatesEdgeCases(r.Policy.Recommendations, r.EdgeCases) {
		line("Policy Recommendations")
		line(uitokens.H2Sep)
		for _, pr := range r.Policy.Recommendations {
			line("  • %s", pr)
		}
		blank()
	}

	// Data completeness — ✓ / ✗ markers scan faster than
	// [available] / [missing  ] with padded spaces inside the brackets.
	line("Data Completeness")
	line(uitokens.H2Sep)
	for _, ds := range r.DataCompleteness {
		mark := uitokens.Muted("✗")
		if ds.Available {
			mark = uitokens.Ok("✓")
		}
		line("  %s %s", mark, ds.Name)
	}
	blank()

	// Limitations
	if len(r.Limitations) > 0 {
		line("Limitations")
		line(uitokens.H2Sep)
		for _, lim := range r.Limitations {
			line("  * %s", lim)
		}
		blank()
	}

	// Next steps
	line("Next steps:")
	if r.TotalFindingCount > len(r.KeyFindings) {
		line("  terrain insights                    prioritized actions for %d %s", r.TotalFindingCount, Plural(r.TotalFindingCount, "finding"))
	} else {
		line("  terrain insights                    prioritized actions and recommendations")
	}
	line("  terrain impact                      what tests matter for this change?")
	line("  terrain explain <test-path>         why was a test selected?")
	line("  terrain analyze --verbose           show full signal detail")
	blank()
}

// policyDuplicatesEdgeCases reports whether the Policy recommendations
// are 1:1 derived from Edge Cases (each edge case emits a parallel
// recommendation in depgraph/edgecase.go). When that's true the
// Recommendations section is just paraphrasing what Edge Cases said a
// few lines above; suppressing the duplicate keeps the report tight.
// Recommendations the adopter wrote in `.terrain/policy.yaml` still
// flow through the Policy.Recommendations field via a different code
// path and would exceed the count, so they keep rendering.
func policyDuplicatesEdgeCases(recs []string, edgeCases []depgraph.EdgeCase) bool {
	return len(edgeCases) > 0 && len(recs) <= len(edgeCases)
}

// titleCase converts a single lowercase word ("strong") to Title
// case ("Strong"). Used for posture-band rendering so reports don't
// shout in ALL CAPS.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func pct(n, total int) int {
	if total == 0 {
		return 0
	}
	return 100 * n / total
}

// hasDataSource returns true if a data source with the given name is available.
func hasDataSource(sources []analyze.DataSource, name string) bool {
	for _, ds := range sources {
		if ds.Name == name && ds.Available {
			return true
		}
	}
	return false
}
