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

// BracketedSeverity returns the severity token wrapped in square
// brackets, the canonical shape for PR-comment markdown and any
// other surface where the bracket makes the badge scan more
// reliably than color (e.g. GitHub-flavored markdown which strips
// most ANSI color attempts). Used by internal/changescope and
// related renderers; locked by the unified-PR-comment golden tests
// (Track 3.5).
//
// Severity strings (lowercase) map to the canonical Severity ladder
// before rendering; unknown strings produce "[---]" so renderers
// don't crash on stray data.
func BracketedSeverity(severity string) string {
	switch severity {
	case "critical":
		return "[CRIT]"
	case "high":
		return "[HIGH]"
	case "medium":
		return "[MED]"
	case "low":
		return "[LOW]"
	case "info":
		return "[INFO]"
	default:
		return "[---]"
	}
}

// BracketedVerdict returns the posture-band verdict in canonical
// PR-comment shape. Mirrors the changescope renderer's previous
// inline mapping; centralized here so other renderers can consume
// the same vocabulary without duplicating the switch.
func BracketedVerdict(band string) string {
	switch band {
	case "well_protected":
		return "[PASS]"
	case "partially_protected":
		return "[WARN]"
	case "weakly_protected":
		return "[RISK]"
	case "high_risk":
		return "[FAIL]"
	case "evidence_limited":
		return "[INFO]"
	default:
		return "[????]"
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

// HeroVerdict renders a designed verdict block at the top of a
// gating output (terrain ai run, terrain analyze --fail-on,
// terrain report pr). Three lines, framed by section rules so the
// verdict carries visual weight beyond the rest of the report.
//
// Layout:
//
//	────────────────────────────────────────────────────────────
//	  [BLOCKED]  3 critical AI eval signals — block merge
//	────────────────────────────────────────────────────────────
//
// `verdict` should be one of "BLOCKED", "WARN", or "PASS". The badge
// is color-and-symbol via the same vocabulary as VerdictBadge so
// callsites stay consistent. `headline` is one short sentence
// describing the verdict in plain language.
//
// Pre-0.2 these decisions surfaced as a single buried "Decision:
// BLOCKED — reason" line; the audit (ai_execution_gating.V2 +
// pr_change_scoped.V2) called for a hero block. This is that block.
func HeroVerdict(verdict, headline string) string {
	badge := heroVerdictBadge(verdict)
	rule := Rule()
	// Indent the headline by two spaces so the badge + line breathe;
	// the rule rows give the block its frame.
	return fmt.Sprintf("%s\n  %s  %s\n%s", rule, badge, headline, rule)
}

// HeroVerdictMarkdown renders the same hero verdict for a markdown
// surface (PR comment). Uses a blockquote callout for the badge +
// headline so GitHub renders it as a tinted box, then a horizontal
// rule below to reinforce the visual frame. Layout:
//
//	> ### [BLOCKED] 3 critical AI eval signals — block merge
//	>
//	> Reason text, optional, italic.
//
//	---
//
// The blockquote tints the entire block on GitHub, giving the same
// "this is the verdict; everything below explains it" framing as
// the terminal block.
func HeroVerdictMarkdown(verdict, headline, reason string) string {
	bracket := bracketVerdict(verdict)
	var b strings.Builder
	fmt.Fprintf(&b, "> ### %s %s\n", bracket, headline)
	if strings.TrimSpace(reason) != "" {
		fmt.Fprintln(&b, ">")
		fmt.Fprintf(&b, "> *%s*\n", strings.TrimSpace(reason))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "---")
	return b.String()
}

// heroVerdictBadge returns the colorized badge for a hero block.
// Distinct from VerdictBadge so the hero presentation can use a
// slightly heavier shape ("[BLOCKED]" instead of "PASS") without
// changing VerdictBadge's contract.
func heroVerdictBadge(verdict string) string {
	switch strings.ToUpper(strings.TrimSpace(verdict)) {
	case "BLOCKED", "BLOCK", "FAIL":
		return Alert("[" + SymFail + " BLOCKED]")
	case "WARN", "WARNING":
		return Warn("[" + SymWarn + " WARN]")
	case "PASS", "OK":
		return Ok("[" + SymOK + " PASS]")
	default:
		return "[" + strings.ToUpper(verdict) + "]"
	}
}

// bracketVerdict is the markdown variant — no color escapes (GitHub
// markdown doesn't render ANSI), but keeps the same vocabulary.
func bracketVerdict(verdict string) string {
	switch strings.ToUpper(strings.TrimSpace(verdict)) {
	case "BLOCKED", "BLOCK", "FAIL":
		return "[BLOCKED]"
	case "WARN", "WARNING":
		return "[WARN]"
	case "PASS", "OK":
		return "[PASS]"
	default:
		return "[" + strings.ToUpper(verdict) + "]"
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
