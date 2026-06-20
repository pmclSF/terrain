package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/aliases"
	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/logging"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/suppression"
)

// expandSuppressionAliases takes a list of suppression entries and
// expands any entry whose SignalType is registered as an alias into
// one entry per replacement rule_id. Entries without an alias hit pass
// through unchanged. Order is preserved: aliased entries are replaced
// in-place by their expansions.
//
// FindingID-only entries: a finding_id embeds the detector rule_id as
// its prefix. When that prefix is a deprecated alias, the suppression
// silently breaks after the split because findings emit under
// different (detector, hash) pairs. We can't auto-rewrite the hash
// (the symbol+line+content shaping the hash may have changed too), but
// we emit a one-time NOTE on stderr so the user knows to migrate.
//
// Without this step, a user with a `.terrain/suppressions.yaml` entry
// against `aiHardcodedAPIKey` would silently stop suppressing once
// findings start emitting under the split halves
// (`aiHardcodedAPIKey-literal-shape`, `secretScannerCoverageDegraded`).
func expandSuppressionAliases(entries []suppression.Entry, reg *aliases.Registry) []suppression.Entry {
	if reg == nil || len(entries) == 0 {
		return entries
	}
	out := make([]suppression.Entry, 0, len(entries))
	for _, e := range entries {
		if e.SignalType == "" {
			// FindingID-only path: check whether the embedded detector
			// is an alias and warn if so.
			warnIfFindingIDIsAliased(e, reg)
			out = append(out, e)
			continue
		}
		expanded := reg.ExpandOldID(e.SignalType)
		// ExpandOldID returns the original ID plus replacements when an
		// alias is hit; just the original when none is registered.
		// When only the original is returned, no alias was hit — pass
		// through.
		if len(expanded) <= 1 {
			out = append(out, e)
			continue
		}
		for _, id := range expanded {
			cp := e
			cp.SignalType = id
			out = append(out, cp)
		}
	}
	return out
}

// suppressFindingIDWarnOnce dedupes the warn-once-per-aliased-id stderr
// emission across the process lifetime. Tests can reset via
// ResetSuppressionFindingIDWarningsForTesting.
var suppressFindingIDWarnOnce sync.Map

// ResetSuppressionFindingIDWarningsForTesting clears the warn-once memo.
// Tests call this to verify the de-duplication path; production code
// never calls it.
func ResetSuppressionFindingIDWarningsForTesting() {
	suppressFindingIDWarnOnce = sync.Map{}
}

// warnIfFindingIDIsAliased parses the entry's FindingID, extracts the
// detector prefix, and emits a one-time stderr [NOTE] when the prefix
// is a deprecated alias. The NOTE tells the user the suppression has
// likely stopped working after a rule split and points them at the
// migration path.
//
// Quiet when TERRAIN_QUIET=1 or the FindingID is malformed.
func warnIfFindingIDIsAliased(e suppression.Entry, reg *aliases.Registry) {
	if e.FindingID == "" || os.Getenv("TERRAIN_QUIET") == "1" {
		return
	}
	detector, _, _, _, ok := identity.ParseFindingID(e.FindingID)
	if !ok || detector == "" {
		return
	}
	entry, isAlias := reg.Entry(detector)
	if !isAlias {
		return
	}
	if _, seen := suppressFindingIDWarnOnce.LoadOrStore(detector, true); seen {
		return
	}
	fmt.Fprintf(os.Stderr,
		"[NOTE] suppressions.yaml entry finding_id=%q references the deprecated rule_id %q.\n",
		e.FindingID, detector,
	)
	fmt.Fprintf(os.Stderr,
		"       After the split into %v, the original finding_id hash no longer matches\n",
		entry.ReplacesWith,
	)
	fmt.Fprintln(os.Stderr,
		"       any current finding; this entry has stopped suppressing.")
	fmt.Fprintln(os.Stderr,
		"       To migrate: replace the finding_id with a `signal_type` entry referencing")
	fmt.Fprintln(os.Stderr,
		"       one of the new rule_ids, OR re-capture finding_ids from the latest analyze.")
	fmt.Fprintln(os.Stderr,
		"       Inspect new rule details with `terrain show rule <id>`.")
	fmt.Fprintln(os.Stderr,
		"       Silence this notice with TERRAIN_QUIET=1.")
	fmt.Fprintln(os.Stderr)
}

// applySuppressions loads `.terrain/suppressions.yaml` (or the path
// supplied in PipelineOptions.SuppressionsPath) and removes matching
// signals from the snapshot. Expired entries don't suppress; they
// emit a `suppressionExpired` warning signal so they show up in the
// next report cycle.
//
// Missing suppressions file is normal — most users won't have one.
// A malformed file is treated as a hard failure (logs + exits the
// pipeline with the parse error) because silently ignoring would let
// CI users believe their suppressions are active when they're not.
//
// Called from RunPipelineContext after FindingID assignment.
func applySuppressions(snap *models.TestSuiteSnapshot, root, override string, now time.Time) error {
	if snap == nil {
		return nil
	}
	path := override
	if path == "" {
		path = filepath.Join(root, suppression.DefaultPath)
	}
	result, err := suppression.Load(path)
	if err != nil {
		logging.L().Warn("could not load suppressions",
			"path", path,
			"error", err.Error(),
		)
		return fmt.Errorf("load suppressions %s: %w", path, err)
	}
	if result == nil || (len(result.Entries) == 0 && len(result.Warnings) == 0) {
		return nil
	}
	for _, w := range result.Warnings {
		logging.L().Warn("suppressions: " + w)
	}
	if len(result.Entries) == 0 {
		return nil
	}

	// Expand aliases so suppressions on pre-split rule_ids continue to
	// suppress every replacement. Reads the alias registry once per
	// pipeline; failure to load the registry (rare; embedded YAML)
	// degrades gracefully to no expansion — but logs at Warn so the
	// silent breakage is at least visible in CI output. An operator
	// who renamed a rule and wrote a suppression against the OLD id
	// would otherwise have no signal that the suppression stopped
	// firing.
	if aliasReg, err := aliases.Load(); err == nil {
		result.Entries = expandSuppressionAliases(result.Entries, aliasReg)
	} else {
		logging.L().Warn("alias registry unavailable for suppression expansion; entries against pre-split rule_ids will not be expanded", "err", err)
	}

	matched, expired := suppression.Apply(snap, result.Entries, now)

	// Surface expired entries as warning signals so they don't rot.
	// Each gets its own signal so reports list them individually.
	for _, e := range expired {
		snap.Signals = append(snap.Signals, models.Signal{
			Type:             signals.SignalSuppressionExpired,
			Category:         models.CategoryGovernance,
			Severity:         models.SeverityMedium,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourcePolicy,
			Explanation: "Suppression entry has expired and is no longer in effect. " +
				"Underlying findings will fire again until you renew or remove the entry. " +
				"Reason on file: " + e.Reason,
			SuggestedAction: "Edit `.terrain/suppressions.yaml`: extend the `expires` date, or remove the entry if the underlying issue is fixed.",
			Location: models.SignalLocation{
				File: suppression.DefaultPath,
			},
			Metadata: map[string]any{
				"finding_id":  e.FindingID,
				"signal_type": e.SignalType,
				"file":        e.File,
				"reason":      e.Reason,
				"owner":       e.Owner,
				"expires":     e.Expires,
			},
		})
	}

	if len(matched) > 0 {
		logging.L().Info("suppressions applied",
			"path", path,
			"matched", len(matched),
			"expired", len(expired),
		)
	}
	return nil
}
