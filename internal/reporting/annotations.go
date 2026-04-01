package reporting

import (
	"fmt"
	"io"

	"github.com/pmclSF/terrain/internal/analyze"
)

// RenderGitHubAnnotations writes GitHub Actions workflow commands
// (::error, ::warning, ::notice) for each KeyFinding in the analyze report.
// This enables inline PR annotations when used in CI.
func RenderGitHubAnnotations(w io.Writer, r *analyze.Report) {
	for _, kf := range r.KeyFindings {
		level := severityToAnnotationLevel(kf.Severity)
		title := fmt.Sprintf("Terrain: %s", kf.Title)
		msg := kf.Title
		if kf.Metric != "" {
			msg = fmt.Sprintf("%s (%s)", kf.Title, kf.Metric)
		}

		// If we have weak coverage areas, emit one annotation per area
		// for coverage findings.
		if kf.Category == "coverage_debt" && len(r.WeakCoverageAreas) > 0 {
			for _, wa := range r.WeakCoverageAreas {
				fmt.Fprintf(w, "::%s file=%s,title=%s::%s\n",
					level, wa.Path, title, msg)
			}
			continue
		}

		// Generic annotation without file location.
		fmt.Fprintf(w, "::%s title=%s::%s\n", level, title, msg)
	}
}

func severityToAnnotationLevel(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "notice"
	}
}
