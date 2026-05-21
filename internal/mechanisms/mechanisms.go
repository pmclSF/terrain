// Package mechanisms is the registry that gates every detector
// behavior change behind a named, three-state switch:
//
//   - off:    the mechanism is not active; the legacy detector runs.
//   - shadow: the mechanism runs alongside the legacy code path and
//             emits would-have-suppressed / would-have-added events to
//             .terrain/shadow-report.jsonl, but does NOT change the
//             user-visible findings.
//   - on:     the mechanism is live — its behavior change is observable
//             in findings.
//
// Every new mechanism ships first as state: shadow. Live activation
// requires the mechanism's regression suite (internal/regressionsuite)
// and per-mechanism recall report (internal/recallharness) both pass,
// then an explicit flip in mechanisms.yaml.
//
// Mechanism state can also be overridden per-process via the
// --mechanisms.<name>=on|off|shadow CLI flag, which calls Override.
package mechanisms

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed mechanisms.yaml
var defaultYAML []byte

// State is the per-mechanism activation state.
type State int

const (
	// StateOff means the mechanism does nothing — pre-cycle-2 behavior.
	StateOff State = iota
	// StateShadow means the mechanism runs and emits shadow events but
	// does not affect findings.
	StateShadow
	// StateOn means the mechanism is live; findings reflect its behavior.
	StateOn
)

// String renders the state as the canonical lowercase YAML form.
func (s State) String() string {
	switch s {
	case StateOn:
		return "on"
	case StateShadow:
		return "shadow"
	default:
		return "off"
	}
}

// ParseState converts the canonical YAML form (off|shadow|on) into a
// State value.
func ParseState(s string) (State, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "":
		return StateOff, nil
	case "shadow":
		return StateShadow, nil
	case "on":
		return StateOn, nil
	default:
		return StateOff, fmt.Errorf("unknown state %q (want off|shadow|on)", s)
	}
}

// Mechanism is one entry in mechanisms.yaml.
type Mechanism struct {
	// Name is the canonical identifier (e.g. "surface_literal_presence_gate").
	// Must be snake_case for CLI-flag friendliness.
	Name string `yaml:"name"`

	// State is the activation state. New mechanisms start at "shadow".
	State State `yaml:"-"`

	// Description is a one-line human-readable summary used in
	// `terrain doctor` output and CLI flag help.
	Description string `yaml:"description"`

	// Consumers lists the rule_ids whose behavior this mechanism gates.
	// Used by the doctor surface and per-mechanism recall reports.
	Consumers []string `yaml:"consumers,omitempty"`
}

// raw mirrors the YAML structure for unmarshalling — the public Mechanism
// type exposes State as the parsed enum, but YAML stores it as a string.
type raw struct {
	SchemaVersion int                 `yaml:"schema_version"`
	Mechanisms    []rawMechanism      `yaml:"mechanisms"`
}

type rawMechanism struct {
	Name        string   `yaml:"name"`
	State       string   `yaml:"state"`
	Description string   `yaml:"description"`
	Consumers   []string `yaml:"consumers,omitempty"`
}

// Registry is the loaded set of mechanisms. Override() is safe for
// concurrent use.
type Registry struct {
	SchemaVersion int

	mu         sync.RWMutex
	mechanisms map[string]*Mechanism
}

// Load parses the embedded mechanisms.yaml into a Registry.
func Load() (*Registry, error) {
	return LoadFromBytes(defaultYAML)
}

// LoadFromBytes parses the supplied YAML. Used by tests + by external
// callers wiring a non-default config.
func LoadFromBytes(data []byte) (*Registry, error) {
	r := raw{}
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse mechanisms yaml: %w", err)
	}
	if r.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema_version %d (expected 1)", r.SchemaVersion)
	}
	reg := &Registry{
		SchemaVersion: r.SchemaVersion,
		mechanisms:    map[string]*Mechanism{},
	}
	for i, m := range r.Mechanisms {
		if m.Name == "" {
			return nil, fmt.Errorf("mechanisms[%d]: missing name", i)
		}
		if _, dup := reg.mechanisms[m.Name]; dup {
			return nil, fmt.Errorf("mechanisms[%d]: duplicate name %q", i, m.Name)
		}
		state, err := ParseState(m.State)
		if err != nil {
			return nil, fmt.Errorf("mechanisms[%d] (%s): %w", i, m.Name, err)
		}
		reg.mechanisms[m.Name] = &Mechanism{
			Name:        m.Name,
			State:       state,
			Description: m.Description,
			Consumers:   append([]string(nil), m.Consumers...),
		}
	}
	return reg, nil
}

// MustLoad is the embed-only convenience for package init paths that
// cannot recover from a malformed mechanisms.yaml.
func MustLoad() *Registry {
	reg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("mechanisms.MustLoad: %v", err))
	}
	return reg
}

// defaultRegistry is the process-wide registry consumer detectors and
// Gate helpers fall back to when no explicit registry is threaded
// through. Set once by the engine pipeline at startup; nil-safe (State
// returns StateOff when nil).
var (
	defaultRegistryMu sync.RWMutex
	defaultRegistry   *Registry
)

// SetDefault installs `reg` as the process-wide default registry.
// Returns the previous default so callers (notably the pipeline at
// shutdown) can restore it. Passing nil clears the default.
func SetDefault(reg *Registry) *Registry {
	defaultRegistryMu.Lock()
	defer defaultRegistryMu.Unlock()
	prev := defaultRegistry
	defaultRegistry = reg
	return prev
}

// Default returns the process-wide default registry. Detector
// packages that need a registry without an explicit handle call this.
// Returns nil when no default has been set; callers must be nil-safe
// (Registry methods handle nil receivers).
func Default() *Registry {
	defaultRegistryMu.RLock()
	defer defaultRegistryMu.RUnlock()
	return defaultRegistry
}

// State returns the current state of mechanism `name`. Unknown
// mechanisms return StateOff — the safe default: an unrecognized name
// can't accidentally turn something on.
func (r *Registry) State(name string) State {
	if r == nil {
		return StateOff
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.mechanisms[name]; ok {
		return m.State
	}
	return StateOff
}

// Get returns the mechanism by name, or nil if unknown.
func (r *Registry) Get(name string) *Mechanism {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.mechanisms[name]; ok {
		// Return a copy to keep internal state from leaking.
		copy := *m
		return &copy
	}
	return nil
}

// Override sets the state of mechanism `name`. Errors if the name is
// unknown — typo protection.
func (r *Registry) Override(name string, s State) error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.mechanisms[name]
	if !ok {
		return fmt.Errorf("unknown mechanism %q", name)
	}
	m.State = s
	return nil
}

// All returns every mechanism in deterministic (name-sorted) order.
// Useful for `terrain doctor` listing and tests.
func (r *Registry) All() []*Mechanism {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Mechanism, 0, len(r.mechanisms))
	for _, m := range r.mechanisms {
		copy := *m
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns just the mechanism names in sorted order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	all := r.All()
	out := make([]string, len(all))
	for i, m := range all {
		out[i] = m.Name
	}
	return out
}

// ApplyCLIOverrides accepts a slice of "name=state" strings (from
// --mechanisms.<name>=<state> CLI parsing) and applies each override.
// Errors on the first malformed entry or unknown mechanism.
func (r *Registry) ApplyCLIOverrides(overrides []string) error {
	for _, o := range overrides {
		eq := strings.IndexByte(o, '=')
		if eq <= 0 {
			return fmt.Errorf("malformed override %q (want name=state)", o)
		}
		name := strings.TrimSpace(o[:eq])
		state, err := ParseState(o[eq+1:])
		if err != nil {
			return fmt.Errorf("override %q: %w", o, err)
		}
		if err := r.Override(name, state); err != nil {
			return fmt.Errorf("override %q: %w", o, err)
		}
	}
	return nil
}
