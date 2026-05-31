package reporting

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// RenderImpactReport writes a human-readable impact analysis report.
func RenderImpactReport(w io.Writer, result *impact.ImpactResult) {
	line, blank := reportHelpers(w)

	line("Terrain Impact Analysis")
	line(strings.Repeat("=", 60))
	blank()

	// Designed empty-state when the change has no measurable test
	// system impact — beats a wall of zeros that reads as "broken."
	if isImpactEmpty(result) {
		RenderEmptyState(w, EmptyNoImpact)
		blank()
		return
	}

	// Summary
	line("Summary: %s", result.Summary)
	blank()

	// Changed areas — dedupe by (area, path, change-kind) so a file
	// with multiple impacted code units doesn't print once per unit.
	if len(result.ChangedAreas) > 0 {
		line("Changed areas:")
		seen := map[string]bool{}
		for _, area := range result.ChangedAreas {
			for _, s := range area.Surfaces {
				key := area.Area + "\x00" + s.Path + "\x00" + string(s.ChangeKind)
				if seen[key] {
					continue
				}
				seen[key] = true
				line("  %-22s %s (%s)", area.Area, s.Path, s.ChangeKind)
			}
		}
		blank()
	}

	// Affected behaviors
	if len(result.AffectedBehaviors) > 0 {
		line("Affected behaviors:")
		for _, ab := range result.AffectedBehaviors {
			line("  %-30s %d/%d surfaces changed", ab.Label, ab.ChangedSurfaceCount, ab.TotalSurfaceCount)
		}
		blank()
	}

	// Test counts
	if len(result.ImpactedTests) > 0 || len(result.SelectedTests) > 0 {
		if result.TotalTestCount > 0 {
			line("Impacted tests:          %d of %d total", len(result.ImpactedTests), result.TotalTestCount)
		} else {
			line("Impacted tests:          %d", len(result.ImpactedTests))
		}
		if len(result.SelectedTests) > 0 && len(result.SelectedTests) != len(result.ImpactedTests) {
			line("Selected for run:        %d", len(result.SelectedTests))
		}
	}

	// Coverage confidence
	line("Coverage confidence:     %s", capitalizeFirst(result.CoverageConfidence))

	// PR risk
	line("PR risk:                 %s", strings.ToUpper(result.Posture.Band))
	blank()

	// Reason categories
	cats := result.ReasonCategories
	if cats.DirectDependency+cats.FixtureDependency+cats.DirectlyChanged+cats.DirectoryProximity > 0 {
		line("Reason categories:")
		if cats.DirectDependency > 0 {
			line("  Direct code dependency:  %d", cats.DirectDependency)
		}
		if cats.FixtureDependency > 0 {
			line("  Fixture dependency:      %d", cats.FixtureDependency)
		}
		if cats.DirectlyChanged > 0 {
			line("  Directly changed:        %d", cats.DirectlyChanged)
		}
		if cats.DirectoryProximity > 0 {
			line("  Directory proximity:     %d", cats.DirectoryProximity)
		}
		blank()
	}

	// Change-risk posture dimensions
	line("Change-Risk Posture: %s", strings.ToUpper(result.Posture.Band))
	line("  %s", result.Posture.Explanation)
	if len(result.Posture.Dimensions) > 0 {
		for _, d := range result.Posture.Dimensions {
			line("  %-20s %s", d.Name+":", d.Band)
		}
	}
	blank()

	// Selected protective tests
	if len(result.SelectedTests) > 0 {
		line("Recommended Tests (%d)", len(result.SelectedTests))
		line(strings.Repeat("-", 60))
		for _, t := range result.SelectedTests {
			conf := ""
			if t.ImpactConfidence != "" {
				conf = fmt.Sprintf(" [%s]", t.ImpactConfidence)
			}
			line("  %s%s", t.Path, conf)
			if t.Relevance != "" {
				line("    %s", t.Relevance)
			}
		}
		blank()
	}

	// Impacted scenarios (AI/eval)
	if len(result.ImpactedEvals) > 0 {
		// Collect unique capabilities.
		capSet := map[string]bool{}
		for _, sc := range result.ImpactedEvals {
			if sc.Capability != "" {
				capSet[sc.Capability] = true
			}
		}
		if len(capSet) > 0 {
			caps := make([]string, 0, len(capSet))
			for c := range capSet {
				caps = append(caps, c)
			}
			sort.Strings(caps)
			line("Impacted AI capabilities: %s", strings.Join(caps, ", "))
			blank()
		}

		line("Impacted Scenarios (%d)", len(result.ImpactedEvals))
		line(strings.Repeat("-", 60))
		for _, sc := range result.ImpactedEvals {
			conf := ""
			if sc.ImpactConfidence != "" {
				conf = fmt.Sprintf(" [%s]", sc.ImpactConfidence)
			}
			label := sc.Name
			if sc.Category != "" {
				label += " (" + sc.Category + ")"
			}
			if sc.Capability != "" {
				label += " → " + sc.Capability
			}
			line("  %s%s", label, conf)
			line("    %s", sc.Relevance)
		}
		blank()
	}

	// Fallback
	if result.Fallback.Level != "none" && result.Fallback.Level != "" {
		line("Fallback:")
		line("  Level: %s", result.Fallback.Level)
		if result.Fallback.Reason != "" {
			line("  Reason: %s", result.Fallback.Reason)
		}
		if result.Fallback.AdditionalTests > 0 {
			line("  Additional tests: %d", result.Fallback.AdditionalTests)
		}
		blank()
	}

	// Protection gaps
	if len(result.ProtectionGaps) > 0 {
		line("Protection Gaps")
		line(strings.Repeat("-", 60))
		for _, gap := range result.ProtectionGaps {
			line("  %s %s", uitokens.BracketedSeverity(gap.Severity), gap.Explanation)
			if gap.SuggestedAction != "" {
				line("    Action: %s", gap.SuggestedAction)
			}
		}
		blank()
	}

	// Impacted owners
	if len(result.ImpactedOwners) > 0 {
		line("Impacted Owners: %s", strings.Join(result.ImpactedOwners, ", "))
		blank()
	}

	// Limitations — filter to caveats that affect the *change-scoped*
	// view. "Too few tests for meaningful optimization" / "CI is
	// already fast" are full-repo edge cases that don't belong in a
	// diff-scoped report.
	if filtered := filterDiffScopedLimitations(result.Limitations); len(filtered) > 0 {
		line("Limitations")
		line(strings.Repeat("-", 60))
		for _, lim := range filtered {
			line("  * %s", lim)
		}
		blank()
	}

	// Next steps
	line("Next steps:")
	line("  terrain impact --show selected   view protective test set with reasoning")
	line("  terrain impact --show graph       see dependency graph")
	line("  terrain impact --json             machine-readable impact data")
	blank()
}

// capitalizeFirst uppercases the first letter of a string.
// Replaces deprecated strings.Title for simple single-word capitalization.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// filterDiffScopedLimitations drops full-repo edge-case messaging
// (e.g. "Too few tests for meaningful optimization", "CI is already
// fast") from the limitations list rendered under change-scoped
// reports like `terrain report impact` and `terrain report pr`.
// Those advisories belong in the analyze report's Edge Cases section;
// they confuse adopters when they appear under a PR's diff view.
func filterDiffScopedLimitations(lims []string) []string {
	if len(lims) == 0 {
		return nil
	}
	dropPrefixes := []string{
		"too few tests",
		"ci is already fast",
		"ci is fast",
		"high test duplication",
		"high proportion of skipped",
		"high proportion of flaky",
		"low graph visibility",
	}
	out := make([]string, 0, len(lims))
	for _, lim := range lims {
		lower := strings.ToLower(strings.TrimLeft(lim, " *•"))
		drop := false
		for _, p := range dropPrefixes {
			if strings.HasPrefix(lower, p) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, lim)
		}
	}
	return out
}

// isImpactEmpty reports whether an ImpactResult has nothing
// substantive to render — no changed areas, no impacted tests, no
// affected behaviors. The change-risk posture is computed even on
// empty input, so we inspect the substantive fields instead.
func isImpactEmpty(r *impact.ImpactResult) bool {
	if r == nil {
		return true
	}
	return len(r.ChangedAreas) == 0 &&
		len(r.AffectedBehaviors) == 0 &&
		len(r.ImpactedTests) == 0 &&
		len(r.SelectedTests) == 0
}
