package uitokens

import (
	"strings"
	"testing"
)

// runWithoutColor flips ColorEnabled off for the test scope so we can
// assert plain-text output. Tests run sequentially because they share
// the package-level ColorEnabled global.
func runWithoutColor(t *testing.T, fn func()) {
	t.Helper()
	prev := ColorEnabled
	ColorEnabled = false
	defer func() { ColorEnabled = prev }()
	fn()
}

func runWithColor(t *testing.T, fn func()) {
	t.Helper()
	prev := ColorEnabled
	ColorEnabled = true
	defer func() { ColorEnabled = prev }()
	fn()
}

// ── Color wrappers ──────────────────────────────────────────────────

func TestColorWrappers_Disabled(t *testing.T) {
	runWithoutColor(t, func() {
		cases := []struct {
			name string
			fn   func(string) string
		}{
			{"Muted", Muted},
			{"Accent", Accent},
			{"Ok", Ok},
			{"Warn", Warn},
			{"Alert", Alert},
			{"Bold", Bold},
		}
		for _, tc := range cases {
			got := tc.fn("hello")
			if got != "hello" {
				t.Errorf("%s with color disabled = %q, want plain %q", tc.name, got, "hello")
			}
		}
	})
}

func TestColorWrappers_Enabled(t *testing.T) {
	runWithColor(t, func() {
		got := Ok("hello")
		if !strings.Contains(got, "\x1b[32m") {
			t.Errorf("Ok should include green ANSI escape, got %q", got)
		}
		if !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("Ok should reset color at end, got %q", got)
		}
	})
}

func TestColorWrappers_EmptyStringNoop(t *testing.T) {
	// Empty input should never emit escape sequences — avoids
	// wrapping a stray "" with " ANSI…ANSI " noise.
	runWithColor(t, func() {
		for _, fn := range []func(string) string{Muted, Accent, Ok, Warn, Alert, Bold} {
			if got := fn(""); got != "" {
				t.Errorf("color wrapper on empty string = %q, want empty", got)
			}
		}
	})
}

// ── Severity badge ──────────────────────────────────────────────────

func TestSeverityBadge_Labels(t *testing.T) {
	runWithoutColor(t, func() {
		cases := []struct {
			sev  Severity
			want string
		}{
			{SeverityCritical, "CRITICAL"},
			{SeverityHigh, "HIGH"},
			{SeverityMedium, "MEDIUM"},
			{SeverityLow, "LOW"},
			{SeverityInfo, "INFO"},
			{SeverityNone, ""},
		}
		for _, tc := range cases {
			got := SeverityBadge(tc.sev)
			if got != tc.want {
				t.Errorf("SeverityBadge(%d) = %q, want %q", tc.sev, got, tc.want)
			}
		}
	})
}

func TestSeverityBadge_HighestSeveritiesAreBold(t *testing.T) {
	runWithColor(t, func() {
		// CRITICAL and HIGH should include bold (\x1b[1m).
		for _, sev := range []Severity{SeverityCritical, SeverityHigh} {
			got := SeverityBadge(sev)
			if !strings.Contains(got, "\x1b[1m") {
				t.Errorf("SeverityBadge(%d) should include bold escape; got %q", sev, got)
			}
		}
		// MEDIUM and below should NOT be bold.
		for _, sev := range []Severity{SeverityMedium, SeverityLow, SeverityInfo} {
			got := SeverityBadge(sev)
			if strings.Contains(got, "\x1b[1m") {
				t.Errorf("SeverityBadge(%d) should not be bold; got %q", sev, got)
			}
		}
	})
}

// ── Verdict badge ───────────────────────────────────────────────────

func TestVerdictBadge(t *testing.T) {
	runWithoutColor(t, func() {
		cases := []struct {
			in   string
			want string
		}{
			{"PASS", SymOK + " PASS"},
			{"pass", SymOK + " PASS"},
			{"  pass  ", SymOK + " PASS"},
			{"WARN", SymWarn + " WARN"},
			{"FAIL", SymFail + " FAIL"},
			{"unknown", "unknown"},
		}
		for _, tc := range cases {
			got := VerdictBadge(tc.in)
			if got != tc.want {
				t.Errorf("VerdictBadge(%q) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})
}

// ── Bracketed badges (PR-comment / markdown surface) ───────────────

// TestBracketedSeverity locks the canonical PR-comment severity
// badge shape. The unified-PR-comment golden tests in
// internal/changescope/unified_render_test.go assert these exact
// strings; renaming a label here is a public-facing change.
func TestBracketedSeverity(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"critical", "[CRIT]"},
		{"high", "[HIGH]"},
		{"medium", "[MED]"},
		{"low", "[LOW]"},
		{"info", "[INFO]"},
		{"", "[---]"},
		{"weird-unknown-value", "[---]"},
	}
	for _, tc := range cases {
		if got := BracketedSeverity(tc.in); got != tc.want {
			t.Errorf("BracketedSeverity(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestBracketedVerdict locks the canonical PR-comment posture-band
// badge shape. Same posture as TestBracketedSeverity — these
// strings are part of the unified-PR-comment visual contract.
func TestBracketedVerdict(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"well_protected", "[PASS]"},
		{"partially_protected", "[WARN]"},
		{"weakly_protected", "[RISK]"},
		{"high_risk", "[FAIL]"},
		{"evidence_limited", "[INFO]"},
		{"", "[????]"},
		{"weird-unknown-band", "[????]"},
	}
	for _, tc := range cases {
		if got := BracketedVerdict(tc.in); got != tc.want {
			t.Errorf("BracketedVerdict(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── Bar rendering ───────────────────────────────────────────────────

func TestBarPlain_FullEmptyHalf(t *testing.T) {
	cases := []struct {
		value, max float64
		width      int
		want       string
	}{
		{0, 100, 4, "░░░░"},
		{100, 100, 4, "████"},
		{50, 100, 4, "██░░"},
		{25, 100, 4, "█░░░"},
		{75, 100, 4, "███░"},
		{1000, 100, 4, "████"}, // overflow clamps to full
		{-50, 100, 4, "░░░░"},  // negative clamps to empty
		{50, 0, 4, "░░░░"},     // zero max
		{50, -1, 4, "░░░░"},    // negative max
	}
	for _, tc := range cases {
		got := BarPlain(tc.value, tc.max, tc.width)
		if got != tc.want {
			t.Errorf("BarPlain(%v,%v,%d) = %q, want %q", tc.value, tc.max, tc.width, got, tc.want)
		}
	}
}

func TestBarPlain_ZeroWidth(t *testing.T) {
	if BarPlain(50, 100, 0) != "" {
		t.Error("BarPlain with width=0 should be empty")
	}
	if BarPlain(50, 100, -3) != "" {
		t.Error("BarPlain with negative width should be empty")
	}
}

func TestBar_ColorByProportion(t *testing.T) {
	runWithColor(t, func() {
		// ≥ 0.8 → alert (red)
		if !strings.Contains(Bar(90, 100, 5), "\x1b[31m") {
			t.Error("Bar at 90% should use alert color")
		}
		// 0.4–0.8 → warn (yellow)
		if !strings.Contains(Bar(50, 100, 5), "\x1b[33m") {
			t.Error("Bar at 50% should use warn color")
		}
		// < 0.4 → muted
		if !strings.Contains(Bar(10, 100, 5), "\x1b[90m") {
			t.Error("Bar at 10% should use muted color")
		}
	})
}

// ── Text helpers ────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hell…"},
		{"hello", 1, "…"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		// Wide-character / unicode safety.
		{"café latte", 5, "café…"},
	}
	for _, tc := range cases {
		got := Truncate(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello world"}, // no truncation
		{"", 3, "   "},
	}
	for _, tc := range cases {
		got := PadRight(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("PadRight(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

func TestPadLeft(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"hi", 5, "   hi"},
		{"hello", 5, "hello"},
	}
	for _, tc := range cases {
		got := PadLeft(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

// ── Headings & rules ────────────────────────────────────────────────

func TestRuleHasStandardWidth(t *testing.T) {
	if got := len([]rune(Rule())); got != SectionWidth {
		t.Errorf("Rule width = %d, want %d", got, SectionWidth)
	}
	if got := len([]rune(SubRule())); got != SectionWidth {
		t.Errorf("SubRule width = %d, want %d", got, SectionWidth)
	}
}

func TestHeading_TwoLines(t *testing.T) {
	runWithoutColor(t, func() {
		got := Heading("Section title")
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Errorf("Heading should be two lines, got %d", len(lines))
		}
		if lines[0] != "Section title" {
			t.Errorf("line 1 = %q, want %q", lines[0], "Section title")
		}
		if !strings.HasPrefix(lines[1], SymRule) {
			t.Errorf("line 2 should be a rule; got %q", lines[1])
		}
	})
}

// ── Composability ───────────────────────────────────────────────────

func TestColorComposition_BoldOnTopOfColor(t *testing.T) {
	runWithColor(t, func() {
		// Bold(Alert("X")) — both escape sequences present, X visible
		got := Bold(Alert("CRITICAL"))
		if !strings.Contains(got, "\x1b[1m") {
			t.Errorf("composed string missing bold; got %q", got)
		}
		if !strings.Contains(got, "\x1b[31m") {
			t.Errorf("composed string missing alert color; got %q", got)
		}
		if !strings.Contains(got, "CRITICAL") {
			t.Errorf("composed string missing payload; got %q", got)
		}
	})
}
