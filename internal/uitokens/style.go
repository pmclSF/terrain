package uitokens

import "strings"

// Canonical visual constants for all CLI report renderers. Every
// reporter that emits a section header or separator should use these
// helpers so the visual scan stays uniform across `terrain analyze`,
// `terrain report pr`, `terrain report impact`, `terrain ai *`, etc.
//
// Historical context: prior to this module, separator widths varied
// across reporters (40 / 50 / 60 chars), header styles diverged
// ("Terrain — X" vs "Terrain X" vs "terrain/foo/bar"), and section
// labels mixed em-dash separators with plain ASCII. The mix made it
// hard for adopters to pattern-match the structure of one report
// against another. This module is the single source of truth.

// Width is the canonical line width for all separator rules.
// Wide enough to keep multi-word section titles legible; narrow
// enough to read at terminal default widths.
const Width = 60

// H1Sep is the heavyweight separator used directly under the report
// title. Renders as a single line of U+2500 box-drawing characters
// (lighter visually than `=`, which read as shouting).
var H1Sep = strings.Repeat("─", Width)

// H2Sep is the section-divider used under in-report section titles
// like "Key Findings", "Risk Posture", "Next steps:". Same character
// as H1Sep — uniform texture across the report.
var H2Sep = strings.Repeat("─", Width)

// Bullet is the standard list-item prefix for prose items
// ("recommendations", "limitations", etc.). Avoid `*` (markdown-y)
// and `-` (too small visually) for the human-readable surface.
const Bullet = "•"

// Indent1 / Indent2 are the canonical leading-space counts for one
// and two levels of nested item indentation. Use them rather than
// hand-typed spaces so future-you can shift the whole layout without
// editing every reporter.
const (
	Indent1 = "  "
	Indent2 = "    "
)

// Header returns the canonical top-of-report title block:
//
//	Terrain · <subtitle>
//	──────────────────────────────────────────────────────────────
//
// Uses U+00B7 middle-dot rather than em-dash to keep the gap between
// "Terrain" and the subtitle visually balanced (em-dash adds too much
// horizontal weight at the top of every report).
func Header(subtitle string) string {
	return "Terrain · " + subtitle + "\n" + H1Sep
}

// Section returns a canonical in-report section header:
//
//	<title>
//	──────────────────────────────────────────────────────────────
//
// Use for top-level sections inside a report (Key Findings, Risk
// Posture, Next steps, etc.). Avoid for nested groupings — the
// reader's eye expects one separator depth per surface.
func Section(title string) string {
	return title + "\n" + H2Sep
}
