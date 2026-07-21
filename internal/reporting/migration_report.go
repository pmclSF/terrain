package reporting

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/framework_migration"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// sortedBlockerTypes returns the blocker-type keys ordered by count
// descending, then name ascending, so report output is deterministic.
func sortedBlockerTypes(byType map[string]int) []string {
	keys := make([]string, 0, len(byType))
	for k := range byType {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if byType[keys[i]] != byType[keys[j]] {
			return byType[keys[i]] > byType[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

// RenderMigrationReport writes a migration readiness report to w.
func RenderMigrationReport(w io.Writer, readiness *framework_migration.ReadinessSummary) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Migration Readiness"))
	blank()

	// Frameworks
	line("Frameworks")
	line(uitokens.H2Sep)
	if len(readiness.Frameworks) == 0 {
		line("  (no frameworks detected)")
	} else {
		for _, fw := range readiness.Frameworks {
			typeBadge := ""
			if fw.Type != "" {
				typeBadge = fmt.Sprintf(" [%s]", fw.Type)
			}
			line("  %-20s %4d files%s", fw.Name, fw.FileCount, typeBadge)
		}
	}
	blank()

	// Readiness
	line("Readiness Level: %s", strings.ToUpper(readiness.ReadinessLevel))
	line("  %s", readiness.Explanation)
	blank()

	// Blockers
	line("Migration Blockers: %d", readiness.TotalBlockers)
	line(uitokens.H2Sep)
	if readiness.TotalBlockers == 0 {
		line("  (none detected)")
	} else {
		for _, bt := range sortedBlockerTypes(readiness.BlockersByType) {
			line("  %-26s %d", bt, readiness.BlockersByType[bt])
		}
	}
	blank()

	// Representative examples
	if len(readiness.RepresentativeBlockers) > 0 {
		line("Representative Blockers")
		line(uitokens.H2Sep)
		for _, ex := range readiness.RepresentativeBlockers {
			line("  [%s] %s", ex.Type, ex.Explanation)
			if ex.File != "" {
				line("    %s", ex.File)
			}
		}
		blank()
	}

	// Quality factors compounding migration risk
	if len(readiness.QualityFactors) > 0 {
		line("Quality Factors Affecting Migration")
		line(uitokens.H2Sep)
		for _, qf := range readiness.QualityFactors {
			line("  %s", qf.Explanation)
		}
		blank()
	}

	// Area assessments
	if len(readiness.AreaAssessments) > 0 {
		line("Area Assessments")
		line(uitokens.H2Sep)
		for _, area := range readiness.AreaAssessments {
			badge := strings.ToUpper(area.Classification)
			line("  [%s] %s", badge, area.Directory)
			line("    %s", area.Explanation)
		}
		blank()
	}

	// Coverage guidance
	if len(readiness.CoverageGuidance) > 0 {
		line("Where Additional Coverage Reduces Migration Risk")
		line(uitokens.H2Sep)
		for _, cg := range readiness.CoverageGuidance {
			line("  [%s] %s", strings.ToUpper(cg.Priority), cg.Directory)
			line("    %s", cg.Reason)
		}
		blank()
	}
}

// RenderMigrationBlockers writes a focused migration blockers report to w.
// This is the output for `terrain migration blockers`.
func RenderMigrationBlockers(w io.Writer, readiness *framework_migration.ReadinessSummary) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Migration Blockers"))
	blank()

	line("Total Blockers: %d", readiness.TotalBlockers)
	blank()

	if readiness.TotalBlockers == 0 {
		line("No migration blockers detected.")
		blank()
		line("This repository appears ready for framework migration or standardization.")
		return
	}

	// Blockers by type
	line("By Type")
	line(uitokens.H2Sep)
	for _, bt := range sortedBlockerTypes(readiness.BlockersByType) {
		line("  %-26s %d", bt, readiness.BlockersByType[bt])
	}
	blank()

	// Representative examples
	if len(readiness.RepresentativeBlockers) > 0 {
		line("Examples")
		line(uitokens.H2Sep)
		for _, ex := range readiness.RepresentativeBlockers {
			line("  [%s] %s", ex.Type, ex.Explanation)
			if ex.File != "" {
				line("    %s", ex.File)
			}
		}
		blank()
	}

	// Area assessments — only risky and caution
	var risky []framework_migration.AreaAssessment
	for _, area := range readiness.AreaAssessments {
		if area.Classification != "safe" {
			risky = append(risky, area)
		}
	}
	if len(risky) > 0 {
		line("Highest-Risk Areas")
		line(uitokens.H2Sep)
		for _, area := range risky {
			badge := strings.ToUpper(area.Classification)
			line("  [%s] %s", badge, area.Directory)
			line("    %s", area.Explanation)
		}
		blank()
	}

	line("Next steps:")
	line("  terrain migration readiness    full readiness assessment with quality factors")
	line("  terrain migration preview      preview migration for a specific file")
	line("  terrain policy check           check against local policy rules")
}

// RenderMigrationPreview writes a migration preview report for a single file.
func RenderMigrationPreview(w io.Writer, preview *framework_migration.PreviewResult) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Migration Preview"))
	blank()

	line("File: %s", preview.File)
	line("Source Framework: %s", preview.SourceFramework)
	if preview.SuggestedTarget != "" {
		line("Suggested Target: %s", preview.SuggestedTarget)
	}
	line("Difficulty: %s", strings.ToUpper(preview.Difficulty))
	blank()

	if !preview.PreviewAvailable {
		line("Preview Not Available")
		line(uitokens.H2Sep)
		line("  %s", preview.Explanation)
		blank()
		if len(preview.Limitations) > 0 {
			for _, l := range preview.Limitations {
				line("  - %s", l)
			}
			blank()
		}
		return
	}

	line("Assessment")
	line(uitokens.H2Sep)
	line("  %s", preview.Explanation)
	blank()

	// Blockers
	if len(preview.Blockers) > 0 {
		line("Migration Blockers (%d)", len(preview.Blockers))
		line(uitokens.H2Sep)
		for _, b := range preview.Blockers {
			line("  [%s] %s", b.Type, b.Pattern)
			line("    %s", b.Explanation)
			line("    -> %s", b.Remediation)
		}
		blank()
	} else {
		line("Migration Blockers: none")
		blank()
	}

	// Safe patterns
	if len(preview.SafePatterns) > 0 {
		line("Safe Patterns (should migrate cleanly)")
		line(uitokens.H2Sep)
		for _, p := range preview.SafePatterns {
			line("  + %s", p)
		}
		blank()
	}

	// Limitations
	if len(preview.Limitations) > 0 {
		line("Limitations")
		line(uitokens.H2Sep)
		for _, l := range preview.Limitations {
			line("  - %s", l)
		}
		blank()
	}
}

// RenderMigrationPreviewScope writes a scope-level migration preview summary.
func RenderMigrationPreviewScope(w io.Writer, previews []*framework_migration.PreviewResult) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Migration Preview (scope)"))
	blank()

	if len(previews) == 0 {
		line("No test files found in scope.")
		return
	}

	// Summary counts
	low, medium, high, unknown := 0, 0, 0, 0
	for _, p := range previews {
		switch p.Difficulty {
		case "low":
			low++
		case "medium":
			medium++
		case "high":
			high++
		default:
			unknown++
		}
	}

	line("Files analyzed: %d", len(previews))
	line("  Low difficulty:    %d", low)
	line("  Medium difficulty: %d", medium)
	line("  High difficulty:   %d", high)
	if unknown > 0 {
		line("  Unknown:           %d", unknown)
	}
	blank()

	// Show high-difficulty files first
	if high > 0 {
		line("High-Difficulty Files (need manual review)")
		line(uitokens.H2Sep)
		for _, p := range previews {
			if p.Difficulty == "high" {
				line("  %s (%s)", p.File, p.SourceFramework)
				line("    %d %s", len(p.Blockers), Plural(len(p.Blockers), "blocker"))
			}
		}
		blank()
	}

	// Medium
	if medium > 0 {
		line("Medium-Difficulty Files")
		line(uitokens.H2Sep)
		for _, p := range previews {
			if p.Difficulty == "medium" {
				line("  %s (%s)", p.File, p.SourceFramework)
				line("    %d %s", len(p.Blockers), Plural(len(p.Blockers), "blocker"))
			}
		}
		blank()
	}

	// Low
	if low > 0 {
		line("Low-Difficulty Files (good candidates to start)")
		line(uitokens.H2Sep)
		for _, p := range previews {
			if p.Difficulty == "low" {
				line("  %s (%s)", p.File, p.SourceFramework)
			}
		}
		blank()
	}
}
