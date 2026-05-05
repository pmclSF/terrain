// Package cli provides the command registry that enumerates the
// CLI surface for Terrain. Track 9.6 of the 0.2.0 release plan
// calls for the registry as the source of truth for command names,
// pillar mappings, and one-line descriptions — feeding `terrain
// --help`, `terrain doctor`, and the truth-verify gate.
//
// Status in 0.2.0
//
// This is the foundation: the Command type, Pillar enum, and a
// thread-safe Register/All API. The existing dispatcher in
// cmd/terrain/main.go is NOT migrated to consume from the
// registry yet — that's 0.2.x work. The registry is additive: any
// caller (printUsage, doctor, truth-verify, docs-gen) can read
// from it today without forcing the dispatcher to become
// registry-driven.
//
// Why a separate package
//
// Putting the registry under cmd/terrain/ would couple it to the
// CLI binary's package and make it un-importable from
// internal/signals (where truth-verify will eventually want to
// cross-check command names against the manifest). internal/cli
// is the right home: importable from anywhere in the tree, no
// dependencies on cmd/.
//
// What the registry does NOT do
//
//   - Argument parsing. Each command keeps owning its own flag.FlagSet.
//   - Dispatch. The big switch in main.go stays the source of truth
//     for how arguments map to runFoo() calls, until a 0.2.x PR
//     migrates it.
//   - Help-text generation. printUsage can opt in to read from
//     here, but doesn't have to.
package cli

import (
	"fmt"
	"sort"
	"sync"
)

// Pillar names the product pillar a command belongs to. Mirrors
// the pillars enumerated in docs/release/parity/rubric.yaml so the
// parity gate, the registry, and `terrain doctor` all use the same
// vocabulary.
type Pillar string

const (
	// PillarUnderstand: see what's there ("terrain analyze",
	// "report summary", AI surface inventory).
	PillarUnderstand Pillar = "understand"

	// PillarAlign: reduce drift between code, tests, and repos
	// ("terrain migrate", "report select-tests", portfolio
	// alignment views).
	PillarAlign Pillar = "align"

	// PillarGate: gate PR changes based on the system as a whole
	// ("report pr", "report impact", "ai run", "policy check").
	PillarGate Pillar = "gate"

	// PillarMeta: cross-cutting commands that don't fit a single
	// pillar (init, doctor, version, config).
	PillarMeta Pillar = "meta"
)

// Tier names the publicly-claimable tier of a command — same axis
// the parity rubric uses for capabilities. Tier 1 is named
// publicly in 0.2.0; Tier 2 is shipping but flagged experimental;
// Tier 3 is in development.
type Tier int

const (
	TierUnknown Tier = 0
	Tier1       Tier = 1
	Tier2       Tier = 2
	Tier3       Tier = 3
)

// Command describes one CLI surface. The registry holds these by
// name; consumers (help, doctor, docs-gen) read them.
type Command struct {
	// Name is the command as the user types it (e.g. "analyze",
	// "report pr"). Subcommands are encoded as space-separated;
	// the dispatcher splits on space to route.
	Name string

	// Pillar is the product pillar this command serves.
	Pillar Pillar

	// Tier is the public-claim tier per the parity plan.
	Tier Tier

	// JourneyQuestion is the one-sentence "what does this answer"
	// the help text uses to introduce the command. Plain English,
	// no exclamation, no jargon.
	JourneyQuestion string

	// Description is the longer help-text body. May span multiple
	// lines; rendered after JourneyQuestion when help is verbose.
	Description string

	// Aliases are alternate names that route to the same command
	// (e.g. "terrain pr" → "terrain report pr"). Empty for
	// commands with no aliases.
	Aliases []string
}

// Registry holds the canonical set of commands. Thread-safe;
// tests can construct an empty registry without colliding with
// the package-level Default.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]*Command
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{
		commands: map[string]*Command{},
	}
}

// Default is the package-level registry that the CLI binary
// consults. Populate it via Register from init() functions in
// individual command files when those files migrate; until then
// it stays empty and consumers (help, doctor) skip registry-
// driven output.
var Default = New()

// Register adds a command to the registry. Returns an error if
// the name (or any alias) is already registered.
func (r *Registry) Register(cmd Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("cli.Register: command Name is required")
	}
	if cmd.Pillar == "" {
		return fmt.Errorf("cli.Register: command %q has no Pillar", cmd.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.commands[cmd.Name]; ok {
		return fmt.Errorf("cli.Register: command %q already registered", cmd.Name)
	}
	for _, alias := range cmd.Aliases {
		if _, ok := r.commands[alias]; ok {
			return fmt.Errorf("cli.Register: alias %q (for command %q) collides with existing entry",
				alias, cmd.Name)
		}
	}

	stored := cmd
	r.commands[cmd.Name] = &stored
	for _, alias := range cmd.Aliases {
		// Aliases reference the same Command; the alias key
		// resolves to the same struct so consumers can find by
		// either name.
		r.commands[alias] = &stored
	}
	return nil
}

// MustRegister panics on Register failure. Use only from package-
// level init() blocks where a duplicate name would be a
// developer-time bug, never a runtime error.
func (r *Registry) MustRegister(cmd Command) {
	if err := r.Register(cmd); err != nil {
		panic(err)
	}
}

// Get returns the Command registered under name (or any alias of
// a registered command). Returns nil + false when not found.
func (r *Registry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// All returns every registered command (deduplicated by Name —
// aliases don't produce extra entries) in deterministic order.
// Order is alphabetical by Name; consumers that want pillar
// grouping should call ByPillar.
func (r *Registry) All() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := map[string]bool{}
	out := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		if seen[cmd.Name] {
			continue
		}
		seen[cmd.Name] = true
		out = append(out, cmd)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// ByPillar groups registered commands by pillar. Order within
// each pillar is alphabetical. Pillars with no commands are
// omitted from the result.
func (r *Registry) ByPillar() map[Pillar][]*Command {
	all := r.All()
	out := map[Pillar][]*Command{}
	for _, cmd := range all {
		out[cmd.Pillar] = append(out[cmd.Pillar], cmd)
	}
	return out
}

// Names returns every registered command name (without aliases)
// in deterministic order. Used by truth-verify and docs-gen to
// cross-check the registry against external sources of truth.
func (r *Registry) Names() []string {
	all := r.All()
	out := make([]string, len(all))
	for i, cmd := range all {
		out[i] = cmd.Name
	}
	return out
}
