package reporting

import (
	"fmt"
	"io"
	"strings"

	"github.com/pmclSF/terrain/internal/measurement"
	"github.com/pmclSF/terrain/internal/summary"
)

// RenderExecutiveSummary writes a concise, leadership-oriented summary to w.
//
// This report is designed to be paste-ready for leadership updates,
// technical debt reviews, and migration planning discussions.
func RenderExecutiveSummary(w io.Writer, es *summary.ExecutiveSummary) {
	line, blank := reportHelpers(w)

	line("Terrain Executive Summary")
	line(strings.Repeat("=", 50))
	blank()

	// Overall posture — surface the underlying measurements alongside
	// the band. 0.2.0 polish: previously this section showed only the
	// band label ("Health: Strong"), which is a categorical
	// compression of the measurements that drove it. The reader had
	// to take the band on faith. Now the line is:
	//
	//   Health: Strong  (0.0% flaky · 3.6% skipped · 0.0% dead · 0.0% slow)
	//
	// — so the reader sees both the verdict (the band, polarity-
	// translated) and the concrete numbers. `terrain posture` retains
	// the full measurement breakdown with evidence + caveats; this
	// summary view trims to a one-line digest.
	line("Overall Posture")
	line(strings.Repeat("-", 50))
	for _, d := range es.Posture.Dimensions {
		dim := measurement.Dimension(d.Dimension)
		label := measurement.DimensionDisplayName(dim)
		band := measurement.BandDisplayForDimension(dim, measurement.PostureBand(d.Band))
		if len(d.KeyMeasurements) == 0 {
			line("  %-22s %s", label+":", band)
			continue
		}
		// Compact "value label" pairs joined by middle dot.
		parts := make([]string, 0, len(d.KeyMeasurements))
		for _, m := range d.KeyMeasurements {
			parts = append(parts, fmt.Sprintf("%s %s", m.FormattedValue, m.ShortLabel))
		}
		line("  %-22s %s  (%s)", label+":", band, strings.Join(parts, " · "))
	}
	if len(es.Posture.Dimensions) == 0 {
		line("  (no risk surfaces computed)")
	} else {
		line("  Dimension meaning:")
		line("    reliability: runtime stability and determinism")
		line("    change: test confidence for safe refactoring and delivery")
		line("    speed: execution efficiency and feedback-loop latency")
		line("    governance: policy adherence and operational control")
	}
	blank()

	// Key numbers
	line("Key Numbers")
	line(strings.Repeat("-", 50))
	line("  Test files:          %d", es.KeyNumbers.TestFiles)
	line("  Frameworks:          %d", es.KeyNumbers.Frameworks)
	line("  Total signals:       %d", es.KeyNumbers.TotalSignals)
	if es.KeyNumbers.CriticalFindings > 0 {
		line("  Critical findings:   %d", es.KeyNumbers.CriticalFindings)
	}
	if es.KeyNumbers.HighRiskAreas > 0 {
		line("  High-risk areas:     %d", es.KeyNumbers.HighRiskAreas)
	}
	blank()

	// Top risk areas
	if len(es.TopRiskAreas) > 0 {
		line("Top Risk Areas")
		line(strings.Repeat("-", 50))
		for _, a := range es.TopRiskAreas {
			// "Top Risk Areas" is unambiguously risk-shaped output —
			// translate Strong → Low, Weak → Significant, etc. so
			// "low migration risk" / "critical quality risk" both
			// read naturally. Use a synthetic risk-polarity dim to
			// reuse the helper.
			band := measurement.BandDisplayForDimension(
				measurement.DimensionStructuralRisk,
				measurement.PostureBand(a.Band),
			)
			line("  %-25s %s %s risk", a.Name, band, a.RiskType)
		}
		blank()
	}

	// Trend highlights
	if es.HasTrendData && len(es.TrendHighlights) > 0 {
		line("Trend Highlights")
		line(strings.Repeat("-", 50))
		for _, t := range es.TrendHighlights {
			icon := " "
			switch t.Direction {
			case "improved":
				icon = "↓"
			case "worsened":
				icon = "↑"
			}
			line("  %s %s", icon, t.Description)
		}
		blank()
	} else if !es.HasTrendData {
		line("Trend Highlights")
		line(strings.Repeat("-", 50))
		line("  This is the first analysis — it establishes your baseline.")
		line("  Save it and re-run later to see trends:")
		line("    terrain analyze --write-snapshot    save this as baseline")
		line("  On subsequent runs, Terrain will show changes in risk, signals, and posture.")
		blank()
	}

	// Dominant drivers
	if len(es.DominantDrivers) > 0 {
		line("Dominant Drivers")
		line(strings.Repeat("-", 50))
		for _, d := range es.DominantDrivers {
			line("  %s", d)
		}
		blank()
	}

	// Recommended focus
	if es.RecommendedFocus != "" {
		line("Recommended Focus")
		line(strings.Repeat("-", 50))
		line("  %s", es.RecommendedFocus)
		blank()
	}

	// Structured recommendations
	if len(es.Recommendations) > 0 {
		line("Prioritized Recommendations")
		line(strings.Repeat("-", 50))
		for _, r := range es.Recommendations {
			strength := string(r.EvidenceStrength)
			if strength == "" {
				strength = "unknown"
			}
			line("  %d. %s", r.Priority, r.What)
			line("     Why:      %s", r.Why)
			line("     Where:    %s", r.Where)
			line("     Evidence: %s", strength)
		}
		blank()
	}

	// Blind spots
	if len(es.BlindSpots) > 0 {
		line("Known Blind Spots")
		line(strings.Repeat("-", 50))
		for _, b := range es.BlindSpots {
			line("  %s: %s", b.Area, b.Reason)
			if b.Remediation != "" {
				line("    → %s", b.Remediation)
			}
		}
		blank()
	}

	// Benchmark readiness
	line("Benchmark Readiness")
	line(strings.Repeat("-", 50))
	if len(es.BenchmarkReadiness.ReadyDimensions) > 0 {
		line("  Ready:")
		for _, d := range es.BenchmarkReadiness.ReadyDimensions {
			line("    %s", d)
		}
	}
	if len(es.BenchmarkReadiness.LimitedDimensions) > 0 {
		line("  Limited:")
		for _, l := range es.BenchmarkReadiness.LimitedDimensions {
			line("    %s (%s)", l.Dimension, l.Reason)
		}
	}
	blank()

	// Next command hints
	line("Next steps:")
	line("  terrain posture       evidence behind each dimension")
	line("  terrain analyze       full signal-level detail")
	line("  terrain export benchmark   privacy-safe export")
	blank()
}

