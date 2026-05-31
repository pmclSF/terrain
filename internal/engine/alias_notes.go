package engine

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/pmclSF/terrain/internal/aliases"
)

// aliasNoteOnce de-duplicates migration NOTEs across calls within a single
// process. The first time a given old rule_id triggers an alias expansion,
// terrain emits a NOTE pointing at the new IDs; subsequent hits stay
// silent. The user has one chance per session to see each migration prompt.
var aliasNoteOnce sync.Map

// emitAliasNotes writes a one-time stderr NOTE for each old rule_id that
// expanded through the alias registry during this pipeline run. The NOTE
// surfaces the alias's `why` text (when present) and the new IDs the user
// should update their config to reference.
//
// Production callers want the default (stderr) output. Tests use
// emitAliasNotesTo so they can capture output without swapping
// os.Stderr globally (which races with parallel test code paths that
// also write to stderr).
func emitAliasNotes(reg *aliases.Registry, hits map[string]bool) {
	emitAliasNotesTo(os.Stderr, reg, hits)
}

// emitAliasNotesTo is the io.Writer-parameterized form for tests.
//
// Quiet when:
//   - reg is nil
//   - hits is empty (no aliases were exercised)
//   - terrain is running in a CI / quiet mode (TERRAIN_QUIET=1)
//
// The "once per session" gate is per-process. Tests can reset it via
// ResetAliasNotesForTesting() to verify the emit-once behavior.
func emitAliasNotesTo(w io.Writer, reg *aliases.Registry, hits map[string]bool) {
	if reg == nil || len(hits) == 0 {
		return
	}
	if os.Getenv("TERRAIN_QUIET") == "1" {
		return
	}
	keys := make([]string, 0, len(hits))
	for k := range hits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, oldID := range keys {
		if _, seen := aliasNoteOnce.LoadOrStore(oldID, true); seen {
			continue
		}
		entry, ok := reg.Entry(oldID)
		if !ok {
			continue
		}
		fmt.Fprintf(w, "[NOTE] %q is a deprecated rule_id.\n", oldID)
		fmt.Fprintf(w, "       It now expands to: %v\n", entry.ReplacesWith)
		if entry.Why != "" {
			fmt.Fprintf(w, "       Why: %s\n", entry.Why)
		}
		fmt.Fprintf(w, "       To migrate, update `.terrain/policy.yaml` (or suppressions.yaml)\n")
		fmt.Fprintf(w, "       to reference the new IDs above. Run `terrain show rule <id>`\n")
		fmt.Fprintf(w, "       for severity, status, and remediation per new ID.\n")
		fmt.Fprintf(w, "       Silence this notice with TERRAIN_QUIET=1.\n")
		fmt.Fprintln(w)
	}
}

// ResetAliasNotesForTesting clears the emit-once memo. Tests use this to
// verify the de-duplication path; production code never calls it.
func ResetAliasNotesForTesting() {
	aliasNoteOnce = sync.Map{}
}
