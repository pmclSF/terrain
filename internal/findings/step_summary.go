package findings

import (
	"fmt"
	"io"
	"sort"
)

// StepSummaryOptions controls Step Summary markdown rendering.
type StepSummaryOptions struct {
	// AnnotationCap is the maximum number of PR annotations to surface
	// inline in the markdown. Default 50. The remaining findings are
	// linked to findings.json.
	AnnotationCap int

	// RepoName is the optional repository name used in the header.
	RepoName string

	// Commit is the optional commit SHA shown in the header.
	Commit string
}

// WriteStepSummary renders the artifact as GitHub Step Summary markdown.
// Silence-on-green discipline:
//   - When zero findings: a single auditable line confirming the run
//     and the rule count that didn't fire. No noise.
//   - When findings exist: severity-grouped sections with primary_loc
//     links, short_message, cause path, reproduction command, docs URL.
//
// The annotation-cap behavior: the first AnnotationCap findings get
// full inline rendering; the rest are summarized as a count linked to
// findings.json.
func (a *Artifact) WriteStepSummary(w io.Writer, opts StepSummaryOptions) error {
	if opts.AnnotationCap <= 0 {
		opts.AnnotationCap = 50
	}

	bySeverity := groupBySeverity(a.Findings)
	errorCount := len(bySeverity[SeverityError])
	warningCount := len(bySeverity[SeverityWarning])
	noticeCount := len(bySeverity[SeverityNotice])
	total := errorCount + warningCount + noticeCount

	if err := writeHeader(w, opts, total, errorCount, warningCount, noticeCount); err != nil {
		return err
	}

	if total == 0 {
		return writeGreenStateLine(w)
	}

	if errorCount > 0 {
		if err := writeSeveritySection(w, "Errors (gate-blocking)", bySeverity[SeverityError], opts.AnnotationCap); err != nil {
			return err
		}
	}
	if warningCount > 0 {
		if err := writeSeveritySection(w, "Warnings", bySeverity[SeverityWarning], opts.AnnotationCap); err != nil {
			return err
		}
	}
	if noticeCount > 0 {
		if err := writeSeveritySection(w, "Notices", bySeverity[SeverityNotice], opts.AnnotationCap); err != nil {
			return err
		}
	}
	return nil
}

func writeHeader(w io.Writer, opts StepSummaryOptions, total, errs, warns, notices int) error {
	header := "# Terrain\n\n"
	if opts.RepoName != "" {
		header = fmt.Sprintf("# Terrain — %s\n\n", opts.RepoName)
	}
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if opts.Commit != "" {
		if _, err := fmt.Fprintf(w, "Commit: `%s`\n\n", opts.Commit); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "**%d** finding(s): %d error, %d warning, %d notice\n\n",
		total, errs, warns, notices); err != nil {
		return err
	}
	return nil
}

// writeGreenStateLine renders the silence-on-green confirmation. Not
// literally silent — adopters need to see that Terrain ran. But a
// single auditable line, not a noise burst.
func writeGreenStateLine(w io.Writer) error {
	_, err := io.WriteString(w, "Terrain analyzed this change. No findings — all rules passed.\n")
	return err
}

func writeSeveritySection(w io.Writer, heading string, findings []Finding, cap int) error {
	if _, err := fmt.Fprintf(w, "## %s (%d)\n\n", heading, len(findings)); err != nil {
		return err
	}
	emit := findings
	overflow := 0
	if len(findings) > cap {
		emit = findings[:cap]
		overflow = len(findings) - cap
	}
	for _, f := range emit {
		if err := writeFinding(w, f); err != nil {
			return err
		}
	}
	if overflow > 0 {
		if _, err := fmt.Fprintf(w, "_…and %d more. See `findings.json` for the full list._\n\n", overflow); err != nil {
			return err
		}
	}
	return nil
}

func writeFinding(w io.Writer, f Finding) error {
	loc := f.PrimaryLoc.Path
	if f.PrimaryLoc.Line > 0 {
		loc = fmt.Sprintf("%s:%d", loc, f.PrimaryLoc.Line)
	}
	if _, err := fmt.Fprintf(w, "### `%s`\n\n", f.RuleID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "**%s**\n\n", f.ShortMessage); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Location: `%s`\n", loc); err != nil {
		return err
	}
	if len(f.CausePath) > 0 {
		if _, err := io.WriteString(w, "- Cause path:\n"); err != nil {
			return err
		}
		for i, p := range f.CausePath {
			s := p.Path
			if p.Line > 0 {
				s = fmt.Sprintf("%s:%d", s, p.Line)
			}
			if _, err := fmt.Fprintf(w, "  %d. `%s`\n", i+1, s); err != nil {
				return err
			}
		}
	}
	if f.LongMessage != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n\n", f.LongMessage); err != nil {
			return err
		}
	}
	if f.Reproduction != "" {
		if _, err := fmt.Fprintf(w, "<details><summary>Reproduce locally</summary>\n\n```bash\n%s\n```\n\n</details>\n\n", f.Reproduction); err != nil {
			return err
		}
	}
	if f.DocsURL != "" {
		if _, err := fmt.Fprintf(w, "[Rule docs](%s)\n\n", f.DocsURL); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "---\n\n"); err != nil {
		return err
	}
	return nil
}

func groupBySeverity(findings []Finding) map[Severity][]Finding {
	out := map[Severity][]Finding{}
	for _, f := range findings {
		out[f.Severity] = append(out[f.Severity], f)
	}
	for _, group := range out {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].RuleID != group[j].RuleID {
				return group[i].RuleID < group[j].RuleID
			}
			return group[i].PrimaryLoc.Path < group[j].PrimaryLoc.Path
		})
	}
	return out
}
