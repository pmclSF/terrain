package main

import (
	"fmt"
	"os"
)

// legacyDeprecationNotice prints a one-line stderr hint pointing the
// user from a legacy top-level command to its canonical 0.2 namespace
// shape. The runway is:
//
//   - 0.2:    namespaces ship as aliases; both shapes work; this hint is silent
//             unless TERRAIN_LEGACY_HINT=1 is set (opt-in for now to avoid
//             noise on first ship).
//   - 0.2.x: hint enabled by default; `TERRAIN_SILENCE_DEPRECATION=1`
//             escape for CI scripts that already migrated.
//   - 0.3:   legacy commands removed.
//
// Hooks at the top of every legacy dispatch case in main.go. The
// command name passed in is the legacy form ("summary"); canonicalForm
// is the new shape ("report summary").
func legacyDeprecationNotice(legacy, canonicalForm string) {
	if os.Getenv("TERRAIN_SILENCE_DEPRECATION") != "" {
		return
	}
	// 0.2: opt-in only so the first release isn't noisy. Flip default
	// to on in 0.2.x with a tracking entry in docs/release/0.2-known-gaps.md.
	if os.Getenv("TERRAIN_LEGACY_HINT") == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"hint: `terrain %s` is deprecated; use `terrain %s` in 0.3+. "+
			"Set TERRAIN_SILENCE_DEPRECATION=1 to suppress.\n",
		legacy, canonicalForm,
	)
}
