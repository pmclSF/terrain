// Package render composes Terrain's human-facing surfaces (PR comment,
// CLI, step summary) in the Swiss/minimal design language. It is the
// composition layer that stacks on internal/uitokens (the primitive
// token layer: colors, symbols, severity badges) — it does not replace
// it. Where uitokens answers "what does a severity badge look like",
// render answers "what does the top of a designed PR comment look like".
//
// Design principles:
//   - The first line carries the message, so it survives a GitHub
//     notification email flattening away collapsible sections and color.
//   - Severity = disclosure level: BLOCK leads; NOTE lives in the footer.
//   - Color is offloaded to the native check status; the markdown body
//     stays monochrome and legible when flattened.
//   - Quiet by default: surface the legend only when there is something
//     to label.
//
// Contract:
//
//	R1. VerdictLine: blockingCount<=0 → "Clear…"; ==1 → singular finding;
//	    >=2 → plural findings.
//	R2. SeverityLegend names all four labels: BLOCK, GATE, WATCH, NOTE.
//	R3. ProvenanceFooter always includes "no API key"; it appends
//	    "Terrain <version>" iff a non-empty version is given.
//	R4. CommentHeader always shows the verdict; it shows the legend iff
//	    totalFindings>0 (quiet on an all-clear comment).
package render

import (
	"strconv"
	"strings"
)

// VerdictLine returns the one-line verdict that leads a PR comment. It is
// deliberately the first thing a reader — or a flattened notification
// email — sees, so it must carry the result on its own. blockingCount is
// the number of findings that fail the merge.
func VerdictLine(blockingCount int) string {
	switch {
	case blockingCount <= 0:
		return "**Clear — nothing blocks this merge.**"
	case blockingCount == 1:
		return "**1 finding blocks this merge.**"
	default:
		return "**" + strconv.Itoa(blockingCount) + " findings block this merge.**"
	}
}

// SeverityLegend returns the one-line legend explaining the PR-surface
// severity vocabulary, so a first-time reader understands the labels
// without leaving the comment. Encodes the severity = disclosure model:
// BLOCK is loudest, NOTE is quietest.
func SeverityLegend() string {
	return "`BLOCK` fails the merge · `GATE` required check · `WATCH` informational · `NOTE` quiet hint"
}

// ProvenanceFooter returns the quiet trust line shown at the foot of a
// comment. It reinforces the positioning — local, key-free, offline — on
// every surface. version is the running Terrain version (e.g. "0.4.0");
// an empty version drops the suffix.
func ProvenanceFooter(version string) string {
	base := "ran locally · no API key · no network"
	if v := strings.TrimSpace(version); v != "" {
		return base + " · Terrain " + v
	}
	return base
}

// CommentHeader assembles the top of a Terrain PR comment in the Swiss
// layout: a quiet kicker, the email-survivable verdict line, and — only
// when there is something to label — the severity legend. blockingCount
// is the merge-blocking finding count; totalFindings is every surfaced
// finding (blocking plus WATCH/NOTE).
func CommentHeader(blockingCount, totalFindings int) string {
	var b strings.Builder
	b.WriteString("**Terrain** · pre-flight\n\n")
	b.WriteString(VerdictLine(blockingCount))
	if totalFindings > 0 {
		b.WriteString("\n\n")
		b.WriteString(SeverityLegend())
	}
	b.WriteString("\n")
	return b.String()
}
