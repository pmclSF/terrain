package main

import "github.com/pmclSF/terrain/internal/uitokens"

// useASCIIOutput reports whether user-facing CLI output should use
// ASCII-only characters (e.g. "-" instead of U+2500 "─", "[OK]" instead
// of "✓"). It defers to the single source of truth in uitokens: ASCII
// when TERRAIN_ASCII=1 is set, or when none of the standard locale env
// vars (LC_ALL / LC_CTYPE / LANG) advertises UTF-8.
//
// The default is conservative: emit UTF-8 only when a locale actively
// claims UTF-8 support. Windows cmd, dumb terminals, and CI runners
// without locale config get the ASCII fallback so output renders as
// plain text instead of garbage.
func useASCIIOutput() bool {
	return !uitokens.UnicodeEnabled
}

// statusGlyph returns a status symbol appropriate for the current
// terminal. Use these for doctor / init / discover status indicators
// so the output renders cleanly on locales that can't display U+2713
// ("✓"), U+26A0 ("⚠"), etc.
//
// Returns a single token without surrounding brackets — the renderer
// supplies the bracket wrapper (e.g. `[%s]`).
//
//	ok     → "✓" or "OK"
//	warn   → "⚠" or "!"
//	miss   → "✗" or "x"
//	info   → "?" (same either way)
func statusGlyph(kind string) string {
	ascii := useASCIIOutput()
	switch kind {
	case "ok":
		if ascii {
			return "OK"
		}
		return "✓"
	case "warn":
		if ascii {
			return "!"
		}
		return "⚠"
	case "miss":
		if ascii {
			return "x"
		}
		return "✗"
	case "info":
		return "?"
	}
	return kind
}
