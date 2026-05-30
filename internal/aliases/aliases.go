// Package aliases is the signal-type alias registry. When a rule is split or
// renamed, the old rule_id is registered here with the list of new IDs it now
// maps to. User policies, suppressions, and CLI flags that reference the old
// ID continue to work — they expand to all the new IDs.
//
// The canonical YAML lives at internal/aliases/signal_type_aliases.yaml and is
// embedded at build time. Tests can load a custom registry via LoadFromBytes
// for hermeticity.
//
// The alias machinery must ship before any rule split lands so existing
// suppressions don't silently break across the rename.
package aliases

import (
	_ "embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed signal_type_aliases.yaml
var defaultYAML []byte

// AliasEntry describes the deprecation runway for one renamed/split rule_id.
type AliasEntry struct {
	// ReplacesWith is the list of new rule_ids that the old ID now maps to.
	// A single old ID can expand to multiple new IDs (the rule-split case).
	ReplacesWith []string `yaml:"replaces_with"`

	// DeprecatedIn is the semver release when this alias was added (the
	// release that shipped the split / rename). Documentary only.
	DeprecatedIn string `yaml:"deprecated_in,omitempty"`

	// RemovalTarget is the semver release after which the alias may stop
	// expanding. Allows a future release to drop the alias cleanly.
	// Documentary only — the registry honors all aliases regardless until
	// the entry is deleted from the YAML.
	RemovalTarget string `yaml:"removal_target,omitempty"`

	// Why is a short prose explanation surfaced to users in the migration
	// NOTE when this alias is hit.
	Why string `yaml:"why,omitempty"`
}

// Registry holds the loaded alias table plus reverse-lookup indexes.
type Registry struct {
	// Version is the schema version of the loaded YAML.
	Version int `yaml:"version"`

	// Aliases maps old rule_id -> AliasEntry.
	Aliases map[string]AliasEntry `yaml:"aliases"`

	// reverse maps new rule_id -> old rule_id (for NOTE emission when
	// terrain wants to tell a user "this signal you see used to be called X").
	// Populated by buildReverseIndex during loading.
	reverse map[string]string
}

// Load parses the embedded default alias registry.
func Load() (*Registry, error) {
	return LoadFromBytes(defaultYAML)
}

// LoadFromBytes parses a YAML alias registry from in-memory bytes. Tests use
// this directly; production code uses Load() with the embedded default.
func LoadFromBytes(data []byte) (*Registry, error) {
	r := &Registry{Aliases: map[string]AliasEntry{}}
	if err := yaml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse signal-type aliases: %w", err)
	}
	if r.Version != 1 {
		return nil, fmt.Errorf("signal-type alias registry: unsupported schema version %d (expected 1)", r.Version)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	r.buildReverseIndex()
	return r, nil
}

// MustLoad is Load() with a panic on error. Used in package init paths where
// a malformed alias registry should fail the binary.
func MustLoad() *Registry {
	r, err := Load()
	if err != nil {
		panic(err)
	}
	return r
}

// ExpandOldID returns the list of new rule_ids that the given (possibly old)
// rule_id maps to. Behavior:
//   - If oldID is registered as an alias, returns the alias's ReplacesWith list
//     PLUS the oldID itself (so suppressions on the old ID continue to suppress
//     anything emitted under the old name during the deprecation window).
//   - If oldID is not aliased, returns []string{oldID} unchanged.
func (r *Registry) ExpandOldID(oldID string) []string {
	if r == nil {
		return []string{oldID}
	}
	entry, ok := r.Aliases[oldID]
	if !ok {
		return []string{oldID}
	}
	out := make([]string, 0, len(entry.ReplacesWith)+1)
	out = append(out, oldID)
	out = append(out, entry.ReplacesWith...)
	return out
}

// OldIDFor returns the deprecated (old) rule_id that a given new ID was split
// from, plus true if such a mapping exists. Used for emitting migration
// NOTEs that point at the new ID's predecessor.
func (r *Registry) OldIDFor(newID string) (string, bool) {
	if r == nil {
		return "", false
	}
	old, ok := r.reverse[newID]
	return old, ok
}

// Entry returns the AliasEntry for an old rule_id, plus a presence bool.
// Used by the NOTE emitter to surface the `why` text.
func (r *Registry) Entry(oldID string) (AliasEntry, bool) {
	if r == nil {
		return AliasEntry{}, false
	}
	entry, ok := r.Aliases[oldID]
	return entry, ok
}

// AllOldIDs returns every registered (deprecated) rule_id, sorted for
// determinism. Used by `terrain doctor` and CI surfaces that audit the
// active deprecation runway.
func (r *Registry) AllOldIDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.Aliases))
	for k := range r.Aliases {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validate checks the loaded registry for internal consistency.
func (r *Registry) validate() error {
	for oldID, entry := range r.Aliases {
		if oldID == "" {
			return fmt.Errorf("signal-type alias registry: empty old rule_id")
		}
		if len(entry.ReplacesWith) == 0 {
			return fmt.Errorf("signal-type alias registry: %q has no replaces_with entries", oldID)
		}
		seen := map[string]bool{}
		for _, newID := range entry.ReplacesWith {
			if newID == "" {
				return fmt.Errorf("signal-type alias registry: %q has empty replaces_with entry", oldID)
			}
			if newID == oldID {
				return fmt.Errorf("signal-type alias registry: %q replaces itself", oldID)
			}
			if seen[newID] {
				return fmt.Errorf("signal-type alias registry: %q has duplicate replaces_with entry %q", oldID, newID)
			}
			seen[newID] = true
		}
	}
	return nil
}

// buildReverseIndex populates Registry.reverse from Registry.Aliases. Called
// once during loading. If two old IDs map to the same new ID (unusual but
// allowed), the reverse lookup returns the first one registered — for the
// canonical migration NOTE we want the most recent rename, but that's a
// future concern; today the registry is empty.
func (r *Registry) buildReverseIndex() {
	r.reverse = map[string]string{}
	for oldID, entry := range r.Aliases {
		for _, newID := range entry.ReplacesWith {
			if _, exists := r.reverse[newID]; !exists {
				r.reverse[newID] = oldID
			}
		}
	}
}
