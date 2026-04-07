package reporting

import (
	"fmt"
	"io"
	"strings"

	"github.com/pmclSF/terrain/internal/measurement"
	"github.com/pmclSF/terrain/internal/models"
)

// RenderPostureReport writes a detailed posture breakdown with measurement
// evidence and explanations. This is the "posture explain" output.
func RenderPostureReport(w io.Writer, snap *models.TestSuiteSnapshot, opts ...ReportOptions) {
	verbose := isVerbose(opts)
	line, blank := reportHelpers(w)

	line("Terrain Posture Report")
	line(strings.Repeat("=", 60))
	blank()

	if snap.Measurements == nil || len(snap.Measurements.Posture) == 0 {
		line("  No measurement data available.")
		line("  Run `terrain analyze` to generate posture measurements.")
		return
	}

	// Compute and display overall posture band.
	overall := computeOverallPosture(snap.Measurements.Posture)
	line("  Overall: %s %s", bandMarker(overall.band), strings.ToUpper(string(overall.band)))
	if overall.explanation != "" {
		line("  %s", overall.explanation)
	}
	blank()
	line(strings.Repeat("-", 60))
	blank()

	for _, p := range snap.Measurements.Posture {
		dim := measurement.Dimension(p.Dimension)
		displayName := measurement.DimensionDisplayName(dim)
		marker := bandMarker(measurement.PostureBand(p.Band))

		line("%s %s", marker, strings.ToUpper(displayName))
		line("  Posture: %s", strings.ToUpper(p.Band))
		if verbose {
			if q := measurement.DimensionQuestion(dim); q != "" {
				line("  Question: %s", q)
			}
		}
		if p.Explanation != "" {
			line("  %s", p.Explanation)
		}
		blank()

		if len(p.DrivingMeasurements) > 0 {
			line("  Driving measurements: %s", strings.Join(p.DrivingMeasurements, ", "))
		}

		if len(p.Measurements) > 0 {
			line("  Measurements:")
			for _, m := range p.Measurements {
				bandTag := ""
				if m.Band != "" {
					bandTag = fmt.Sprintf(" [%s]", m.Band)
				}
				if m.Units == "ratio" {
					line("    %-40s %.1f%%%s", m.ID, m.Value*100, bandTag)
				} else {
					line("    %-40s %.2f%s", m.ID, m.Value, bandTag)
				}
				line("      Evidence: %s", m.Evidence)
				line("      %s", m.Explanation)
				for _, lim := range m.Limitations {
					line("      * %s", lim)
				}
				if verbose && len(m.Inputs) > 0 {
					line("      Inputs: %s", strings.Join(m.Inputs, ", "))
				}
			}
		}
		blank()
		line(strings.Repeat("-", 60))
		blank()
	}

	line("Next steps:")
	line("  terrain summary       leadership-ready overview")
	line("  terrain metrics       aggregate scorecard")
	line("  terrain posture --json   machine-readable posture data")
	if !verbose {
		line("  terrain posture --verbose   show signal inputs per measurement")
	}
	blank()
}

// overallPosture holds the computed aggregate posture.
type overallPosture struct {
	band        measurement.PostureBand
	explanation string
}

// computeOverallPosture derives an aggregate posture from dimension results.
func computeOverallPosture(dimensions []models.DimensionPostureResult) overallPosture {
	order := map[string]int{
		string(measurement.PostureCritical): 5,
		string(measurement.PostureElevated): 4,
		string(measurement.PostureWeak):     3,
		string(measurement.PostureModerate): 2,
		string(measurement.PostureStrong):   1,
	}

	worst := 0
	var worstDims []string
	resolved := 0
	for _, p := range dimensions {
		o := order[p.Band]
		if o == 0 {
			continue // skip unknown
		}
		resolved++
		if o > worst {
			worst = o
			worstDims = []string{p.Dimension}
		} else if o == worst {
			worstDims = append(worstDims, p.Dimension)
		}
	}

	if resolved == 0 {
		return overallPosture{
			band:        measurement.PostureUnknown,
			explanation: "Insufficient data to determine overall posture.",
		}
	}

	bandMap := map[int]measurement.PostureBand{
		1: measurement.PostureStrong, 2: measurement.PostureModerate,
		3: measurement.PostureWeak, 4: measurement.PostureElevated,
		5: measurement.PostureCritical,
	}

	band := bandMap[worst]
	var explanation string
	// Build a label listing all dimensions tied at the worst band.
	dimLabels := make([]string, len(worstDims))
	for i, d := range worstDims {
		dimLabels[i] = measurement.DimensionDisplayName(measurement.Dimension(d))
	}
	dimLabel := strings.Join(dimLabels, ", ")
	switch band {
	case measurement.PostureStrong:
		explanation = fmt.Sprintf("All %d dimension(s) are strong.", resolved)
	case measurement.PostureModerate:
		explanation = fmt.Sprintf("Room for improvement in %s.", dimLabel)
	case measurement.PostureWeak:
		explanation = fmt.Sprintf("Driven by %s.", dimLabel)
	case measurement.PostureElevated:
		explanation = fmt.Sprintf("Elevated concerns in %s.", dimLabel)
	case measurement.PostureCritical:
		explanation = fmt.Sprintf("Critical issues in %s require immediate attention.", dimLabel)
	}

	return overallPosture{band: band, explanation: explanation}
}

// bandMarker returns a visual indicator for posture severity.
func bandMarker(band measurement.PostureBand) string {
	switch band {
	case measurement.PostureCritical:
		return "[!!]"
	case measurement.PostureElevated:
		return "[!]"
	case measurement.PostureWeak:
		return "[~]"
	case measurement.PostureModerate:
		return "[-]"
	case measurement.PostureUnknown:
		return "[?]"
	case measurement.PostureStrong:
		return "[ok]"
	default:
		return "[-]"
	}
}
