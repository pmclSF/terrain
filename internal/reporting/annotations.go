package reporting

import (
	"fmt"
	"io"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
)

// escapeData escapes message data per the GitHub Actions workflow-command spec.
func escapeData(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	return r.Replace(s)
}

// escapeProperty escapes a workflow-command property value; property values
// additionally require ',' and ':' to be escaped.
func escapeProperty(s string) string {
	r := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ",", "%2C", ":", "%3A")
	return r.Replace(s)
}

// RenderGitHubAnnotations writes GitHub Actions workflow commands
// (::error, ::warning, ::notice) — one per finding in the analyze report,
// anchored to the finding's file/line where known. This enables inline PR
// annotations in CI. It iterates the canonical per-signal findings (the same
// set as findings.json and SARIF); an earlier version iterated only the top-3
// derived KeyFindings, so most findings — including the AI gate findings —
// silently never annotated.
func RenderGitHubAnnotations(w io.Writer, r *analyze.Report) {
	for _, fr := range r.Signals {
		if fr.RuleID == "" {
			continue
		}
		level := severityToAnnotationLevel(fr.Severity)
		title := fmt.Sprintf("Terrain: %s", fr.RuleID)
		msg := fr.Evidence
		if msg == "" {
			msg = fmt.Sprintf("%s finding", fr.Type)
		}
		file := escapeProperty(fr.File)
		etitle := escapeProperty(title)
		emsg := escapeData(msg)
		switch {
		case fr.File != "" && fr.Line > 0:
			fmt.Fprintf(w, "::%s file=%s,line=%d,title=%s::%s\n", level, file, fr.Line, etitle, emsg)
		case fr.File != "":
			fmt.Fprintf(w, "::%s file=%s,title=%s::%s\n", level, file, etitle, emsg)
		default:
			fmt.Fprintf(w, "::%s title=%s::%s\n", level, etitle, emsg)
		}
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
