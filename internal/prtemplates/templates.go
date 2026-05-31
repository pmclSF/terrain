// Package prtemplates owns the PR-comment specimens for every
// gate-tier and observability-tier detector. The shape is intentionally
// declarative: each detector has a Title, Summary, Action, and a set
// of suggested SlashHints. The render layer composes these with the
// per-finding metadata (file, line, severity badge) at runtime.
//
// Authoring convention:
//
//	Title    — 1-5 words. Headline shown above the file path. Plain
//	           English. No detector-internal vocabulary.
//	Summary  — 1-2 sentences. The "what does this mean for me?" line.
//	           Reads in <10 seconds. Plain English.
//	Action   — 1 sentence. The concrete next step the PR author can
//	           take TODAY. Imperative voice ("Wrap user input..."
//	           rather than "It is recommended that...").
//	SlashHints — Suggested slash-commands the PR author can run.
//	           The webhook receiver (cmd/terrain webhook) parses these
//	           and dispatches; without the receiver they render as
//	           plain text in the PR comment.
//
// All templates are static; no per-finding string interpolation. The
// render layer adds the file/line context outside the template. This
// keeps the templates reviewable as a single artifact.
package prtemplates

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed templates.yaml
var defaultYAML []byte

// SlashHint is one suggested slash-command for a finding.
type SlashHint struct {
	// Label is the human-readable button text (e.g. "Dismiss this
	// finding") surfaced as the user-visible action.
	Label string `yaml:"label"`
	// Command is what gets typed / parsed (e.g.
	// "/dismiss reason:<text>"). The webhook receiver's grammar
	// parses this verb and dispatches.
	Command string `yaml:"command"`
}

// Template is one detector's PR-comment specimen.
type Template struct {
	// SignalType is the detector's signal_type key (e.g.
	// "aiPromptInjectionRisk"). Used as the registry lookup key.
	SignalType string `yaml:"signal_type"`
	// Title is the headline shown above the file path.
	Title string `yaml:"title"`
	// Summary is the one-liner answering "what does this mean for me?".
	Summary string `yaml:"summary"`
	// Action is the concrete next step the PR author can take today.
	Action string `yaml:"action"`
	// SlashHints lists the suggested slash-commands for this finding.
	// Always include /dismiss + /terrain explain at minimum.
	SlashHints []SlashHint `yaml:"slash_hints"`
}

// Registry holds the loaded template table plus a lookup index.
type Registry struct {
	templates map[string]Template
}

// File is the YAML envelope. Embedded at build time from templates.yaml.
type fileSchema struct {
	Version   int        `yaml:"version"`
	Templates []Template `yaml:"templates"`
}

// Load parses the embedded default template registry.
func Load() (*Registry, error) {
	return LoadFromBytes(defaultYAML)
}

// LoadFromBytes parses a YAML template registry from in-memory bytes.
// Tests use this directly; production code uses Load() with the
// embedded default.
func LoadFromBytes(data []byte) (*Registry, error) {
	var f fileSchema
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse pr-comment templates: %w", err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("pr-comment templates: unsupported version %d (expected 1)", f.Version)
	}
	r := &Registry{templates: make(map[string]Template, len(f.Templates))}
	for i, t := range f.Templates {
		if t.SignalType == "" {
			return nil, fmt.Errorf("pr-comment templates[%d]: signal_type is required", i)
		}
		if t.Title == "" {
			return nil, fmt.Errorf("pr-comment templates[%d] (%s): title is required", i, t.SignalType)
		}
		if t.Summary == "" {
			return nil, fmt.Errorf("pr-comment templates[%d] (%s): summary is required", i, t.SignalType)
		}
		if t.Action == "" {
			return nil, fmt.Errorf("pr-comment templates[%d] (%s): action is required", i, t.SignalType)
		}
		if _, dup := r.templates[t.SignalType]; dup {
			return nil, fmt.Errorf("pr-comment templates[%d]: duplicate signal_type %q", i, t.SignalType)
		}
		r.templates[t.SignalType] = t
	}
	return r, nil
}

// MustLoad is Load() with a panic on error. Used in package init
// paths where a malformed template registry should fail the binary.
func MustLoad() *Registry {
	r, err := Load()
	if err != nil {
		panic(err)
	}
	return r
}

// Get returns the template for the given signal_type, plus a presence
// bool. A signal_type without a registered template returns the zero
// Template; callers should fall back to detector-emitted Explanation /
// SuggestedAction text for those.
func (r *Registry) Get(signalType string) (Template, bool) {
	if r == nil {
		return Template{}, false
	}
	t, ok := r.templates[signalType]
	return t, ok
}

// All returns every registered template, in stable signal-type order.
// Used by the docs-gen pipeline and the readiness-card surface to
// list every detector's user-facing copy.
func (r *Registry) All() []Template {
	if r == nil {
		return nil
	}
	out := make([]Template, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	// Stable order: by SignalType.
	sortTemplates(out)
	return out
}

// SignalTypes returns every registered signal_type, sorted.
func (r *Registry) SignalTypes() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.templates))
	for k := range r.templates {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// defaultRegistry is the package-level shared registry, loaded lazily
// on first use. Production code reads through Default(); tests can
// override via SetDefaultForTesting.
var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

// Default returns the package-level shared registry. Loaded lazily on
// first call; failure to load (rare; the YAML is embedded at build
// time) returns the nil registry and the load error.
func Default() (*Registry, error) {
	defaultOnce.Do(func() {
		defaultReg, defaultErr = Load()
	})
	return defaultReg, defaultErr
}
