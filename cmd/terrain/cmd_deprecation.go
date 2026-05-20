package main

import (
	"fmt"
	"os"
)

// legacyDeprecationNotice prints a one-line stderr hint pointing the
// user from a legacy top-level command to its canonical namespace shape.
// The hint is silent unless TERRAIN_LEGACY_HINT=1 is set (opt-in to
// avoid noise on first ship); TERRAIN_SILENCE_DEPRECATION=1 suppresses
// even when the hint is enabled.
//
// Hooks at the top of every legacy dispatch case in main.go. The
// command name passed in is the legacy form ("summary"); canonicalForm
// is the new shape ("report summary").
func legacyDeprecationNotice(legacy, canonicalForm string) {
	if os.Getenv("TERRAIN_SILENCE_DEPRECATION") != "" {
		return
	}
	if os.Getenv("TERRAIN_LEGACY_HINT") == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"hint: `terrain %s` is deprecated; use `terrain %s`. "+
			"Set TERRAIN_SILENCE_DEPRECATION=1 to suppress.\n",
		legacy, canonicalForm,
	)
}
