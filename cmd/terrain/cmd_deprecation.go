package main

import (
	"fmt"
	"os"
)

// legacyDeprecationNotice prints a one-line stderr hint pointing the
// user from a legacy top-level command to its canonical namespace
// shape. The hint is silent unless TERRAIN_LEGACY_HINT=1 is set (opt-in
// to avoid noise on first ship).
//
// Either TERRAIN_QUIET=1 (the umbrella "no stderr chatter" flag) or
// the per-feature TERRAIN_SILENCE_DEPRECATION=1 suppresses the hint.
// Users typically set TERRAIN_QUIET=1 and expect every Terrain-internal
// status line to stop.
//
// Hooks at the top of every legacy dispatch case in main.go. The
// command name passed in is the legacy form ("summary"); canonicalForm
// is the new shape ("report summary").
func legacyDeprecationNotice(legacy, canonicalForm string) {
	if isTerrainQuiet() || os.Getenv("TERRAIN_SILENCE_DEPRECATION") != "" {
		return
	}
	if os.Getenv("TERRAIN_LEGACY_HINT") == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"hint: `terrain %s` is deprecated; use `terrain %s`. "+
			"Set TERRAIN_QUIET=1 to suppress all deprecation hints.\n",
		legacy, canonicalForm,
	)
}

// isTerrainQuiet returns true when the user has opted into a quiet
// run via the umbrella TERRAIN_QUIET=1 flag. The internal engine and
// alias-notes paths use the same check, so a single env var silences
// every Terrain-internal stderr chatter source.
func isTerrainQuiet() bool {
	return os.Getenv("TERRAIN_QUIET") == "1"
}
