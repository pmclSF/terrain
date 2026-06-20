package reporting

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// RenderPortfolioReport writes a human-readable portfolio intelligence report to w.
func RenderPortfolioReport(w io.Writer, snap *models.TestSuiteSnapshot, opts ...ReportOptions) {
	line, blank := reportHelpers(w)

	line(uitokens.Header("Portfolio Intelligence"))
	blank()

	p := snap.Portfolio
	if p == nil || p.Aggregates.TotalAssets == 0 {
		// Designed empty state instead of the bare two-line
		// "No portfolio data" message.
		RenderEmptyState(w, EmptyNoPortfolio)
		blank()
		return
	}

	agg := p.Aggregates

	// Overview
	line("Overview")
	line(uitokens.H2Sep)
	line("  Test assets:          %d", agg.TotalAssets)
	if agg.HasRuntimeData {
		line("  Total runtime:        %.0fms", agg.TotalRuntimeMs)
		if agg.RuntimeConcentration > 0 {
			line("  Runtime concentration: %.0f%% in top 20%%", agg.RuntimeConcentration*100)
		}
	}
	if agg.PortfolioPostureBand != "" {
		line("  Portfolio posture:    %s", strings.ToUpper(agg.PortfolioPostureBand))
	}
	blank()

	// Findings summary
	totalFindings := agg.RedundancyCandidateCount + agg.OverbroadCount +
		agg.LowValueHighCostCount + agg.HighLeverageCount + agg.FrameworkDriftCount
	if totalFindings > 0 {
		line("Findings")
		line(uitokens.H2Sep)
		if agg.HighLeverageCount > 0 {
			line("  High-leverage tests:      %d", agg.HighLeverageCount)
		}
		if agg.RedundancyCandidateCount > 0 {
			line("  Redundancy candidates:    %d", agg.RedundancyCandidateCount)
		}
		if agg.OverbroadCount > 0 {
			line("  Overbroad tests:          %d", agg.OverbroadCount)
		}
		if agg.LowValueHighCostCount > 0 {
			line("  Low-value high-cost:      %d", agg.LowValueHighCostCount)
		}
		if agg.FrameworkDriftCount > 0 {
			line("  Framework drift:          %d", agg.FrameworkDriftCount)
		}
		blank()
	}

	// Top findings detail (up to 8)
	if len(p.Findings) > 0 {
		line("Top Findings")
		line(uitokens.H2Sep)
		limit := 8
		if len(p.Findings) < limit {
			limit = len(p.Findings)
		}
		for _, f := range p.Findings[:limit] {
			badge := findingBadge(f.Type)
			line("  %s %s", badge, f.Path)
			line("    %s", f.Explanation)
			if f.SuggestedAction != "" {
				line("    Action: %s", f.SuggestedAction)
			}
		}
		if len(p.Findings) > limit {
			line("  ... and %d more findings", len(p.Findings)-limit)
		}
		blank()
	}

	// Per-owner summary (top 5)
	renderOwnerSummary(w, p)

	// Evidence notes
	line("Evidence")
	line(uitokens.H2Sep)
	if agg.HasRuntimeData && agg.HasCoverageData {
		line("  Runtime and coverage data available. Findings are high-confidence.")
	} else if agg.HasRuntimeData {
		line("  Runtime data available. Coverage linkage would improve finding precision.")
	} else if agg.HasCoverageData {
		line("  Coverage linkage available. Runtime data would improve cost estimates.")
	} else {
		line("  Limited data. Cost and breadth estimates are based on test type heuristics.")
	}
	blank()

	// Next steps
	line("Next steps:")
	line("  terrain portfolio --json     full portfolio data as JSON")
	line("  terrain posture              see measurement-level evidence")
	line("  terrain summary              leadership-ready overview")
	blank()
}

func findingBadge(findingType string) string {
	switch findingType {
	case "high_leverage":
		return "[LEVERAGE]"
	case "redundancy_candidate":
		return "[REDUNDANCY]"
	case "overbroad":
		return "[OVERBROAD]"
	case "low_value_high_cost":
		return "[LOW-VALUE]"
	case "framework_drift":
		return "[DRIFT]"
	default:
		return "[FINDING]"
	}
}

func renderOwnerSummary(w io.Writer, p *models.PortfolioSnapshot) {
	line, blank := reportHelpers(w)

	if len(p.Aggregates.ByOwner) == 0 {
		return
	}

	// Only show owners with findings.
	type ownerEntry struct {
		owner    string
		findings int
	}
	var entries []ownerEntry
	for _, o := range p.Aggregates.ByOwner {
		total := o.RedundancyCandidateCount + o.OverbroadCount +
			o.LowValueHighCostCount + o.HighLeverageCount + o.FrameworkDriftCount
		if total > 0 {
			entries = append(entries, ownerEntry{owner: o.Owner, findings: total})
		}
	}
	if len(entries) == 0 {
		return
	}

	// Sort by findings descending so the top-N are the most impactful.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].findings > entries[j].findings
	})

	line("By Owner")
	line(uitokens.H2Sep)
	limit := 5
	if len(entries) < limit {
		limit = len(entries)
	}
	for _, e := range entries[:limit] {
		line("  %-24s %d %s", e.owner, e.findings, Plural(e.findings, "finding"))
	}
	blank()
}

// RenderPortfolioSection writes a compact portfolio summary suitable for
// inclusion in the analyze report.
func RenderPortfolioSection(w io.Writer, p *models.PortfolioSnapshot) {
	if p == nil || p.Aggregates.TotalAssets == 0 {
		return
	}

	line, blank := reportHelpers(w)

	agg := p.Aggregates
	totalFindings := agg.RedundancyCandidateCount + agg.OverbroadCount +
		agg.LowValueHighCostCount + agg.HighLeverageCount + agg.FrameworkDriftCount

	line("Portfolio Intelligence")
	line(uitokens.H2Sep)
	line("  Assets: %d    Findings: %d", agg.TotalAssets, totalFindings)
	if agg.PortfolioPostureBand != "" {
		line("  Posture: %s", strings.ToUpper(agg.PortfolioPostureBand))
	}

	if agg.HighLeverageCount > 0 {
		line("  %d high-leverage %s provide outsized protection", agg.HighLeverageCount, Plural(agg.HighLeverageCount, "test"))
	}
	problems := agg.RedundancyCandidateCount + agg.OverbroadCount + agg.LowValueHighCostCount + agg.FrameworkDriftCount
	if problems > 0 {
		line("  %d %s flagged for redundancy, overbreadth, low value, or framework drift", problems, Plural(problems, "item"))
	}
	blank()
}

// RenderMultiRepoPortfolioReport writes a human-readable report for
// `terrain portfolio --from <manifest>`.
func RenderMultiRepoPortfolioReport(w io.Writer, p *models.PortfolioSnapshot, opts ...ReportOptions) {
	line, blank := reportHelpers(w)

	title := "Terrain Portfolio"
	if p != nil && p.Description != "" {
		title = title + " - " + p.Description
	}
	line(uitokens.Header(title))
	blank()

	if p == nil || p.Aggregates.TotalRepos == 0 {
		RenderEmptyState(w, EmptyNoPortfolio)
		blank()
		return
	}

	agg := p.Aggregates
	totalFindings := agg.RedundancyCandidateCount + agg.OverbroadCount +
		agg.LowValueHighCostCount + agg.HighLeverageCount + agg.FrameworkDriftCount

	line("Cross-repo summary")
	line(uitokens.H2Sep)
	line("  Repos: %d    Test assets: %d    Findings: %d", agg.TotalRepos, agg.TotalAssets, totalFindings)
	if agg.PortfolioPostureBand != "" {
		line("  Portfolio posture: %s", strings.ToUpper(agg.PortfolioPostureBand))
	}
	if agg.HasRuntimeData {
		line("  Total runtime: %.0fms", agg.TotalRuntimeMs)
	}
	blank()

	line("Repositories")
	line(uitokens.H2Sep)
	for _, repo := range p.Repositories {
		line("  %-22s %-12s %4d %s    %s",
			repo.Name,
			strings.ToUpper(repo.Status),
			repo.AssetCount,
			Plural(repo.AssetCount, "asset"),
			frameworkCountsText(repo.ObservedFrameworks),
		)
		if isVerbose(opts) {
			if len(repo.FrameworksOfRecord) > 0 {
				line("    frameworksOfRecord: %s", strings.Join(repo.FrameworksOfRecord, ", "))
			}
			if len(repo.Tags) > 0 {
				line("    tags: %s", strings.Join(repo.Tags, ", "))
			}
		}
	}
	blank()

	if agg.FrameworkDriftCount > 0 {
		line("Framework drift")
		line(uitokens.H2Sep)
		for _, repo := range p.Repositories {
			if len(repo.DriftFrameworks) == 0 {
				continue
			}
			line("  %s drifts from frameworksOfRecord (%s):",
				repo.Name,
				strings.Join(repo.FrameworksOfRecord, ", "),
			)
			for _, fw := range repo.DriftFrameworks {
				line("    %s: %d %s", fw.Name, fw.TestFiles, Plural(fw.TestFiles, "test file"))
			}
		}
		blank()
	}

	if len(p.Findings) > 0 {
		line("Top findings")
		line(uitokens.H2Sep)
		limit := 8
		if len(p.Findings) < limit {
			limit = len(p.Findings)
		}
		for _, f := range p.Findings[:limit] {
			line("  %s %s", findingBadge(f.Type), f.Path)
			line("    %s", f.Explanation)
			if f.SuggestedAction != "" {
				line("    Action: %s", f.SuggestedAction)
			}
		}
		if len(p.Findings) > limit {
			line("  ... and %d more findings", len(p.Findings)-limit)
		}
		blank()
	}

	if len(p.Aggregates.ByOwner) > 0 {
		line("By owner")
		line(uitokens.H2Sep)
		for _, owner := range p.Aggregates.ByOwner {
			findings := owner.RedundancyCandidateCount + owner.OverbroadCount +
				owner.LowValueHighCostCount + owner.HighLeverageCount + owner.FrameworkDriftCount
			line("  %-24s %4d %s    %d %s",
				owner.Owner,
				owner.AssetCount,
				Plural(owner.AssetCount, "asset"),
				findings,
				Plural(findings, "finding"),
			)
		}
		blank()
	}

	line("Next steps:")
	line("  terrain portfolio --from <manifest> --json    machine-readable aggregate")
	line("  terrain portfolio --root <repo>                inspect one repo")
	line("  terrain migrate list                           see supported conversion directions")
	blank()
}

func frameworkCountsText(counts []models.PortfolioFrameworkCount) string {
	if len(counts) == 0 {
		return "no frameworks observed"
	}
	parts := make([]string, 0, len(counts))
	for _, fw := range counts {
		if fw.TestFiles > 0 {
			parts = append(parts, fw.Name+" "+strconv.Itoa(fw.TestFiles))
		} else {
			parts = append(parts, fw.Name)
		}
	}
	return strings.Join(parts, ", ")
}
