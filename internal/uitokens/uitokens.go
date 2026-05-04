// Package uitokens is the design-system shim for Terrain's CLI surface.
//
// One small palette + one symbol set + one severity-badge shape used
// across every renderer. Terminal output, HTML report, PR-comment
// markdown, and SARIF tags all consume from here. Ad-hoc styling
// outside this package is a parity-gate violation (V1 axis); a
// future linter (Track 10.2) will catch raw ANSI codes / inline
// styles in user-visible code paths.
//
// Design constraints:
//
//   - Stateless. No init() side effects, no globals beyond the
//     enabled-by-default ColorEnabled flag. Tests can flip
//     ColorEnabled to false and assert output is plain text.
//   - Cheap. No dependencies. Pure constants + small helpers.
//   - TTY-aware. Color suppressed when stdout is piped, when NO_COLOR
//     is set (https://no-color.org/), or when the user passes --quiet.
//   - Idiomatic. Plain English, confident, no hedging — the V3
//     "fun-to-use polish" axis lives partly in the wording choices
//     baked into this package.
package uitokens

import (
	"fmt"
	"os"
	"strings"
)

// ── Color tokens ─────────────────────────────────────────────────────
//
// One small palette. Names describe SEMANTIC roles ("Ok", "Warn"), not
// specific colors, so we can swap the underlying ANSI codes without
// rewriting callers. Each constant is a complete escape sequence (open
// + close handled separately).

const (
	colorReset = "\x1b[0m"

	colorMuted  = "\x1b[90m" // dim gray; less-important context
	colorAccent = "\x1b[36m" // cyan; the one accent color
	colorOk     = "\x1b[32m" // green; PASS / OK / Strong
	colorWarn   = "\x1b[33m" // yellow; WARN / Moderate
	colorAlert  = "\x1b[31m" // red; FAIL / Critical / High
	colorBold   = "\x1b[1m"
)

// ColorEnabled controls whether color escape sequences are emitted.
// Initialized once from the environment + TTY state; callers (mainly
// tests) may flip it to force plain-text output.
//
// Set false when:
//   - stdout is not a TTY (pipe / file redirect)
//   - the NO_COLOR environment variable is set (any non-empty value)
//   - TERM=dumb
var ColorEnabled = detectColorEnabled()

// Muted wraps text in the "muted" color (less-important context
// like timestamps or path prefixes).
func Muted(s string) string { return wrap(colorMuted, s) }

// Accent wraps text in the single accent color. Used sparingly — the
// product is mostly monochrome.
func Accent(s string) string { return wrap(colorAccent, s) }

// Ok wraps text in the success color. Use for PASS verdicts, "✓"
// markers, and Strong band labels.
func Ok(s string) string { return wrap(colorOk, s) }

// Warn wraps text in the warning color. Use for WARN verdicts and
// Moderate / Weak band labels.
func Warn(s string) string { return wrap(colorWarn, s) }

// Alert wraps text in the alert color. Reserved for FAIL verdicts,
// Critical / High severity, and "✗" markers.
func Alert(s string) string { return wrap(colorAlert, s) }

// Bold wraps text in bold. Composes with the color helpers — call
// Bold last (or innermost) for predictable rendering.
func Bold(s string) string { return wrap(colorBold, s) }

func wrap(open, s string) string {
	if !ColorEnabled || s == "" {
		return s
	}
	return open + s + colorReset
}

// ── Symbol set ──────────────────────────────────────────────────────
//
// One vocabulary of markers used across every renderer. ASCII fallbacks
// kick in when the locale doesn't support UTF-8 (rare; mostly defensive).

const (
	SymOK     = "✓"
	SymFail   = "✗"
	SymWarn   = "⚠"
	SymInfo   = "ⓘ"
	SymArrow  = "→"
	SymBullet = "•"
	SymDash   = "—"
	SymDot    = "·"

	// Box-drawing for section rules.
	SymRule    = "─"
	SymSubrule = "·"
)

// ── Severity model ──────────────────────────────────────────────────

// Severity is the canonical severity ladder used everywhere a finding
// or signal is rendered. Mirrors models.SeverityCritical etc., kept
// independent here so the uitokens package has zero dependencies on
// internal/.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityInfo
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// SeverityBadge returns a one-token rendering for the given severity:
// a colored text label, suitable for inline use ("HIGH", "CRITICAL").
// Color follows the SeverityNone → SeverityCritical ladder; bold
// applied at SeverityHigh and above so blocking findings stand out.
func SeverityBadge(s Severity) string {
	switch s {
	case SeverityCritical:
		return Bold(Alert("CRITICAL"))
	case SeverityHigh:
		return Bold(Alert("HIGH"))
	case SeverityMedium:
		return Warn("MEDIUM")
	case SeverityLow:
		return Muted("LOW")
	case SeverityInfo:
		return Muted("INFO")
	default:
		return ""
	}
}

// VerdictBadge renders one of the canonical CLI verdicts (PASS / WARN
// / FAIL) consistently. Used by the parity-gate matrix, the AI risk
// review hero block, and the policy-check summary.
func VerdictBadge(verdict string) string {
	switch strings.ToUpper(strings.TrimSpace(verdict)) {
	case "PASS":
		return Ok(SymOK + " PASS")
	case "WARN":
		return Warn(SymWarn + " WARN")
	case "FAIL":
		return Alert(SymFail + " FAIL")
	default:
		return verdict
	}
}

// ── Spacing & rules ─────────────────────────────────────────────────

// SectionWidth is the width budget for section rules and table layouts.
// Chosen to fit comfortably in a 100-column terminal with margins for
// indentation. All renderers should use this so headings line up
// visually across commands.
const SectionWidth = 60

// Rule renders a section separator at the standard width.
func Rule() string { return strings.Repeat(SymRule, SectionWidth) }

// SubRule renders a softer separator for sub-sections within a
// section. Visually quieter than Rule.
func SubRule() string { return strings.Repeat(SymSubrule, SectionWidth) }

// Heading formats a top-level section heading: title + rule on the
// next line. Returns a two-line string ready to print.
func Heading(title string) string {
	return fmt.Sprintf("%s\n%s", Bold(title), Rule())
}

// Subheading formats a sub-section heading: title + subrule.
func Subheading(title string) string {
	return fmt.Sprintf("%s\n%s", title, SubRule())
}

// ── ASCII bar rendering ─────────────────────────────────────────────

// BarChar is the canonical filled-cell character used by every ASCII
// bar visualization. One character so every bar has the same visual
// weight regardless of which renderer emitted it.
const BarChar = "█"

// BarEmpty is the canonical empty-cell character for ASCII bars.
const BarEmpty = "░"

// Bar renders an ASCII progress / proportion bar of the given width.
// `value` is normalized against `max`; when max <= 0 or value <= 0
// the bar renders as fully empty. When value >= max the bar renders
// as fully filled. The output is always exactly `width` runes wide.
//
// Color is applied based on the proportion: ≥ 0.8 alerts (red),
// 0.4–0.8 warns (yellow), < 0.4 muted (default). Callers that want
// inverse-polarity coloring (e.g. coverage where high is good) should
// use BarPlain and color-wrap themselves.
func Bar(value, max float64, width int) string {
	plain := BarPlain(value, max, width)
	if max <= 0 {
		return Muted(plain)
	}
	ratio := value / max
	switch {
	case ratio >= 0.8:
		return Alert(plain)
	case ratio >= 0.4:
		return Warn(plain)
	default:
		return Muted(plain)
	}
}

// BarPlain renders the bar without color. Useful when the caller
// applies its own color rule.
func BarPlain(value, max float64, width int) string {
	if width <= 0 {
		return ""
	}
	if max <= 0 || value <= 0 {
		return strings.Repeat(BarEmpty, width)
	}
	ratio := value / max
	if ratio >= 1 {
		return strings.Repeat(BarChar, width)
	}
	filled := int(ratio*float64(width) + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat(BarChar, filled) + strings.Repeat(BarEmpty, width-filled)
}

// ── Text helpers ────────────────────────────────────────────────────
//
// String-width helpers operate on rune count, not byte count. ANSI
// escape sequences (added by the color wrappers) confuse `len()` —
// callers that need to align colored strings should call PadRight
// AFTER coloring with care, or color AFTER padding (the safer path).

// Truncate cuts s to at most width visible characters, appending an
// ellipsis if truncation occurred. A width of 0 or negative returns "".
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// PadRight pads s with spaces on the right to reach width visible
// characters. If s is already wider than width, returns s unchanged
// (does not truncate; callers that want truncation should call
// Truncate first).
func PadRight(s string, width int) string {
	gap := width - runeCount(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// PadLeft pads s with spaces on the left.
func PadLeft(s string, width int) string {
	gap := width - runeCount(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// ── TTY / environment detection ─────────────────────────────────────

func detectColorEnabled() bool {
	// NO_COLOR per https://no-color.org/ — any non-empty value disables.
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	// Stdout TTY check. We don't pull in a terminal lib for one bool —
	// Stat() the file and look at the device-character bit.
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
