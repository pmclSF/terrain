package reporting

import (
	"io"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// RenderPortfolioReport writes a human-readable portfolio intelligence report to w.
func RenderPortfolioReport(w io.Writer, snap *models.TestSuiteSnapshot, opts ...ReportOptions) {
	line, blank := reportHelpers(w)

	line("Terrain Portfolio Intelligence")
	line(strings.Repeat("=", 50))
	blank()

	p := snap.Portfolio
	if p == nil || p.Aggregates.TotalAssets == 0 {
		// Audit-named gap (portfolio.V3): designed empty state
		// instead of the bare two-line "No portfolio data" message.
		RenderEmptyState(w, EmptyNoPortfolio)
		blank()
		return
	}

	agg := p.Aggregates

	// Overview
	line("Overview")
	line(strings.Repeat("-", 50))
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
		agg.LowValueHighCostCount + agg.HighLeverageCount
	if totalFindings > 0 {
		line("Findings")
		line(strings.Repeat("-", 50))
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
		blank()
	}

	// Top findings detail (up to 8)
	if len(p.Findings) > 0 {
		line("Top Findings")
		line(strings.Repeat("-", 50))
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
	line(strings.Repeat("-", 50))
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
			o.LowValueHighCostCount + o.HighLeverageCount
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
	line(strings.Repeat("-", 50))
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
		agg.LowValueHighCostCount + agg.HighLeverageCount

	line("Portfolio Intelligence")
	line(strings.Repeat("-", 40))
	line("  Assets: %d    Findings: %d", agg.TotalAssets, totalFindings)
	if agg.PortfolioPostureBand != "" {
		line("  Posture: %s", strings.ToUpper(agg.PortfolioPostureBand))
	}

	if agg.HighLeverageCount > 0 {
		line("  %d high-leverage %s provide outsized protection", agg.HighLeverageCount, Plural(agg.HighLeverageCount, "test"))
	}
	problems := agg.RedundancyCandidateCount + agg.OverbroadCount + agg.LowValueHighCostCount
	if problems > 0 {
		line("  %d %s flagged for redundancy, overbreadth, or low value", problems, Plural(problems, "test"))
	}
	blank()
}
