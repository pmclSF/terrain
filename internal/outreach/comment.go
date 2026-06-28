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
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, render.ProvenanceFooter(""))
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
