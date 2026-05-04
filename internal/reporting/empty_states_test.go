package reporting

import (
	"bytes"
	"strings"
	"testing"
)

// TestEmptyStateFor_AllKindsHaveHeader is the contract test: adding a
// new EmptyStateKind without populating its message means the helper
// will silently render the default `(no content)` placeholder. This
// test pins every defined kind so the omission surfaces in CI.
func TestEmptyStateFor_AllKindsHaveHeader(t *testing.T) {
	t.Parallel()
	kinds := []EmptyStateKind{
		EmptyZeroFindings,
		EmptyNoAISurfaces,
		EmptyNoPolicyFile,
		EmptyFirstRun,
		EmptyNoImpact,
		EmptyNoTestSelection,
		EmptyNoMigrationCandidates,
	}
	for _, k := range kinds {
		es := EmptyStateFor(k)
		if es.Header == "" || es.Header == "(no content)" {
			t.Errorf("EmptyStateKind %d has no designed header — add one in EmptyStateFor", k)
		}
	}
}

// TestEmptyStateFor_VoiceAndTone enforces the Track 10.7 voice rules
// on every shipped empty state: no exclamation marks, no emoji
// codepoints, no British spellings ("colour" / "behaviour" / etc.).
//
// Adding a friendlier-sounding string with an exclamation mark or a
// celebratory emoji breaks the design system; this test surfaces the
// drift before the string ships.
func TestEmptyStateFor_VoiceAndTone(t *testing.T) {
	t.Parallel()
	kinds := []EmptyStateKind{
		EmptyZeroFindings,
		EmptyNoAISurfaces,
		EmptyNoPolicyFile,
		EmptyFirstRun,
		EmptyNoImpact,
		EmptyNoTestSelection,
		EmptyNoMigrationCandidates,
	}
	for _, k := range kinds {
		es := EmptyStateFor(k)
		text := es.Header + " " + es.NextMove
		if strings.Contains(text, "!") {
			t.Errorf("EmptyStateKind %d uses exclamation mark — voice & tone is plain, not jarring: %q", k, text)
		}
		for _, banned := range []string{"colour", "behaviour", "favour", "centre"} {
			if strings.Contains(strings.ToLower(text), banned) {
				t.Errorf("EmptyStateKind %d uses British spelling %q: %q", k, banned, text)
			}
		}
		// Quick emoji guard — nothing in the basic-multilingual-plane
		// emoji ranges. Keeps the design surface monochrome / ASCII
		// for now (Track 10 design tokens own the symbol vocabulary).
		for _, r := range text {
			if r >= 0x1F300 && r <= 0x1FAFF {
				t.Errorf("EmptyStateKind %d uses emoji codepoint U+%X: %q", k, r, text)
			}
		}
	}
}

// TestEmptyStateFor_NextMoveIsActionable asserts every kind that
// surfaces a next-move actually names a *command* the user can run.
// Empty states without a concrete next move read as "we noticed
// nothing happened" — adopters need a verb.
func TestEmptyStateFor_NextMoveIsActionable(t *testing.T) {
	t.Parallel()
	// First-run is the only kind where the next-move can stand alone
	// without a backtick-wrapped command (it's invitational rather
	// than diagnostic). Everything else should name a command.
	commandRequired := []EmptyStateKind{
		EmptyZeroFindings,
		EmptyNoAISurfaces,
		EmptyNoPolicyFile,
		EmptyFirstRun,
		EmptyNoImpact,
		EmptyNoTestSelection,
		EmptyNoMigrationCandidates,
	}
	for _, k := range commandRequired {
		es := EmptyStateFor(k)
		if es.NextMove == "" {
			t.Errorf("EmptyStateKind %d has no next-move — every empty state should suggest a verb", k)
			continue
		}
		if !strings.Contains(es.NextMove, "`") {
			t.Errorf("EmptyStateKind %d next-move doesn't reference a command in backticks: %q", k, es.NextMove)
		}
	}
}

func TestRenderEmptyState_TerminalText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	RenderEmptyState(&buf, EmptyNoAISurfaces)
	out := buf.String()
	if !strings.Contains(out, "No AI surfaces detected") {
		t.Errorf("expected header in output, got: %q", out)
	}
	if !strings.Contains(out, "→ Skipping") {
		t.Errorf("expected next-move arrow in output, got: %q", out)
	}
	// Two lines: header + next-move. Trailing blank line is caller's
	// responsibility per the helper contract.
	if got := strings.Count(out, "\n"); got != 2 {
		t.Errorf("expected exactly 2 newlines (header + next-move), got %d in %q", got, out)
	}
}

func TestRenderEmptyState_HeaderOnly(t *testing.T) {
	t.Parallel()
	// Force the no-content branch via an out-of-range kind.
	var buf bytes.Buffer
	RenderEmptyState(&buf, EmptyStateKind(9999))
	if buf.Len() != 0 {
		t.Errorf("unknown kind should render nothing, got: %q", buf.String())
	}
}

func TestEmptyStateMarkdown_BlockquoteShape(t *testing.T) {
	t.Parallel()
	got := EmptyStateMarkdown(EmptyZeroFindings)
	if !strings.HasPrefix(got, "> ") {
		t.Errorf("markdown empty state should lead with a blockquote callout, got: %q", got)
	}
	if !strings.Contains(got, "*") {
		t.Errorf("markdown empty state should italicize the next-move, got: %q", got)
	}
}

func TestEmptyStateMarkdown_UnknownKindReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := EmptyStateMarkdown(EmptyStateKind(9999)); got != "" {
		t.Errorf("unknown kind should return empty string, got: %q", got)
	}
}
