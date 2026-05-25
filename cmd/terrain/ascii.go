package main

import (
	"os"
	"strings"
)

// useASCIIOutput reports whether user-facing CLI output should use
// ASCII-only characters (e.g. "-" instead of U+2500 "─", "[OK]" instead
// of "✓"). Returns true when TERRAIN_ASCII=1 is set, or when none of
// the standard locale env vars (LC_ALL / LC_CTYPE / LANG) advertises
// UTF-8.
//
// The default is conservative: emit UTF-8 only when a locale actively
// claims UTF-8 support. Windows cmd, dumb terminals, and CI runners
// without locale config get the ASCII fallback so output renders as
// plain text instead of garbage.
func useASCIIOutput() bool {
	if os.Getenv("TERRAIN_ASCII") == "1" {
		return true
	}
	for _, env := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := strings.ToUpper(os.Getenv(env))
		if strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8") {
			return false
		}
	}
	return true
}

// statusGlyph returns a status symbol appropriate for the current
// terminal. Use these for doctor / init / discover status indicators
// so the output renders cleanly on locales that can't display U+2713
// ("✓"), U+26A0 ("⚠"), etc.
//
//	ok     → "✓" or "[OK]"
//	warn   → "⚠" or "[!]"
//	miss   → "✗" or "[x]"
//	info   → "?" or "[?]"
func statusGlyph(kind string) string {
	ascii := useASCIIOutput()
	switch kind {
	case "ok":
		if ascii {
			return "[OK]"
		}
		return "✓"
	case "warn":
		if ascii {
			return "[!]"
		}
		return "⚠"
	case "miss":
		if ascii {
			return "[x]"
		}
		return "✗"
	case "info":
		if ascii {
			return "[?]"
		}
		return "?"
	}
	return kind
}
