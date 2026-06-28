// Package outreach renders the growth-engine artifact: a GitHub comment that
// shares a repo's Terrain-validated regressions and their validated
// remediations, in Terrain's Swiss/minimal design language. It composes on
// internal/render (the design-system layer) so every outreach surface reads
// as one consistent, clean product.
package outreach

import (
	"fmt"
	"io"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/render"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// RenderRegressionComment writes the outreach comment for a repo's validated
// regressions to w.
func RenderRegressionComment(w io.Writer, regressions []findings.Finding) {
	fmt.Fprintln(w, regressionVerdict(len(regressions)))
	fmt.Fprintln(w)
	for _, f := range regressions {
		fmt.Fprintf(w, "- **`%s`** %s — %s\n", f.PrimaryLoc.Path, badgeFor(f.Severity), f.ShortMessage)
		if rem := remediationText(f); rem != "" {
			fmt.Fprintf(w, "  → **Fix** _(validated)_: %s\n", rem)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, render.ProvenanceFooter(""))
}

// remediationText returns the finding's primary suggestion text, or "" when
// it carries none.
func remediationText(f findings.Finding) string {
	if len(f.Suggestions) == 0 {
		return ""
	}
	return f.Suggestions[0].Text
}

// badgeFor maps the canonical finding severity (error/warning/notice) onto
// the design-system bracketed badge vocabulary.
func badgeFor(sev findings.Severity) string {
	switch sev {
	case findings.SeverityError:
		return uitokens.BracketedSeverity("high")
	case findings.SeverityWarning:
		return uitokens.BracketedSeverity("medium")
	default:
		return uitokens.BracketedSeverity("low")
	}
}

// regressionVerdict is the bold, count-bearing lead line, in the design
// system's verdict-first style.
func regressionVerdict(n int) string {
	noun := "regressions"
	if n == 1 {
		noun = "regression"
	}
	return fmt.Sprintf("**Terrain found %d %s in this repo.**", n, noun)
}
