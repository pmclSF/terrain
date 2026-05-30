// Package plugin defines the manifest + protocol contract for
// third-party detector plugins. The full runtime (subprocess spawn,
// JSON-over-stdin/stdout invocation, plugin signing, primitive
// whitelist enforcement) is future work; this package ships the
// manifest schema so adopters can publish plugin packages today and
// terrain can validate them.
//
// The plugin model is: each plugin declares one or more detectors via
// a manifest. Terrain reads the manifest at registration time,
// validates the declared mechanism classes are in the structural
// whitelist (no curated allowlists, no literal-string primitives),
// and refuses to load plugins that violate the binding rules.
//
// Plugin security spec — declarations that adopters must read:
//   - RequiresNetwork: the plugin will make outbound HTTP calls. Off
//     by default; adopters opt in per-plugin via
//     `terrain plugins add --allow-network <plugin>`.
//   - RequiresAPIKey: list of providers whose API keys the plugin
//     reads from the adopter's env. Empty by default.
//   - MechanismClass: every detector declares its mechanism shape so
//     terrain can refuse plugins that ship literal-string or regex
//     primitives — those violate Rule 3 (class-level rules only).
package plugin

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the manifest schema version this package
// understands. Plugins declaring a higher version are refused.
const SchemaVersion = 1

// Manifest is the parsed plugin-manifest.yaml shape.
type Manifest struct {
	// SchemaVersion identifies the manifest format version. Must be
	// SchemaVersion (=1) today; a higher value is refused at load.
	SchemaVersion int `yaml:"schema_version" json:"schema_version"`

	// ID is the plugin's unique identifier in the form
	// `<author>/<name>`. Lowercase, hyphen-separated. Stable
	// forever once published.
	ID string `yaml:"id" json:"id"`

	// Name is the human-readable plugin name.
	Name string `yaml:"name" json:"name"`

	// Version is the plugin's semver (e.g. "1.2.3").
	Version string `yaml:"version" json:"version"`

	// Author is the publisher (GitHub login, org, or company name).
	Author string `yaml:"author" json:"author"`

	// Description is a one-paragraph plugin overview.
	Description string `yaml:"description" json:"description"`

	// Detectors is the list of detectors the plugin ships.
	Detectors []DetectorSpec `yaml:"detectors" json:"detectors"`

	// RequiresNetwork is true when the plugin makes outbound network
	// calls during a scan. Adopters opt in per-plugin via
	// `terrain plugins add --allow-network <plugin>`.
	RequiresNetwork bool `yaml:"requires_network,omitempty" json:"requires_network,omitempty"`

	// RequiresAPIKey lists provider names whose API keys the plugin
	// reads from the adopter env (e.g. "anthropic", "openai", "lakera").
	// Adopters opt in per-provider via
	// `terrain plugins add --allow-keys <provider>/<plugin>`.
	RequiresAPIKey []string `yaml:"requires_api_key,omitempty" json:"requires_api_key,omitempty"`

	// Homepage is an optional URL to plugin documentation.
	Homepage string `yaml:"homepage,omitempty" json:"homepage,omitempty"`
}

// DetectorSpec describes a single detector inside a plugin manifest.
type DetectorSpec struct {
	// RuleID is the canonical rule identifier the plugin's detector
	// will emit signals under. Must be unique across all loaded
	// plugins.
	RuleID string `yaml:"rule_id" json:"rule_id"`

	// SignalType is the SignalType emitted for findings (e.g.
	// "lakeraPromptInjection"). Must be unique across all loaded
	// plugins and not collide with terrain's built-in types.
	SignalType string `yaml:"signal_type" json:"signal_type"`

	// MechanismClass is the structural primitive the detector uses.
	// Must be one of the values in AllowedMechanismClasses; literal-
	// string and regex primitives are explicitly NOT in the
	// whitelist — those violate the binding rule that every rule
	// must be class-level structural.
	MechanismClass string `yaml:"mechanism_class" json:"mechanism_class"`

	// DefaultSeverity classifies the detector's findings.
	DefaultSeverity string `yaml:"default_severity" json:"default_severity"`

	// Tier is `gate` or `observability`. Plugins ship at observability
	// by default; promotion to gate requires terrain's per-plugin
	// validation flow (cycle-3).
	Tier string `yaml:"tier,omitempty" json:"tier,omitempty"`

	// Description is the user-facing one-liner shown in
	// `terrain plugins list`.
	Description string `yaml:"description" json:"description"`
}

// AllowedMechanismClasses enumerates the structural primitives a
// plugin detector may use. The list is intentionally narrow — every
// entry produces class-level matches, not single-cell heuristics.
//
// Forbidden primitives (intentionally absent):
//   - literal-string (regex over source text)
//   - curated-allowlist (per-name list of known-bad/good values)
//
// Both fail the binding rule that every rule must clear a class, not
// a cell.
var AllowedMechanismClasses = []string{
	"structural-ast",  // AST predicate (e.g. "imports-from(<module>)")
	"import-graph",    // multi-file import-reach over the typed graph
	"receiver-type",   // method-receiver type check (assertion family)
	"manifest-schema", // JSON Schema / OpenAPI / protobuf shape check
}

// LoadManifest reads and validates a plugin manifest from path. The
// returned manifest is safe to register; Validate has run.
func LoadManifest(path string) (*Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return ParseManifest(body)
}

// ParseManifest parses + validates an in-memory manifest. Useful for
// CI tooling that emits manifests from a build pipeline.
func ParseManifest(body []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate enforces the plugin manifest schema rules. Returns the
// first violation found.
func Validate(m *Manifest) error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version=%d unsupported; this terrain understands %d only",
			m.SchemaVersion, SchemaVersion)
	}
	if !strings.Contains(m.ID, "/") {
		return fmt.Errorf("id=%q must be `<author>/<name>` (e.g. `lakera/prompt-injection`)", m.ID)
	}
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required (semver, e.g. 1.2.3)")
	}
	if m.Author == "" {
		return fmt.Errorf("author is required")
	}
	if len(m.Detectors) == 0 {
		return fmt.Errorf("at least one detector is required")
	}
	seenRule := map[string]bool{}
	seenType := map[string]bool{}
	for i, d := range m.Detectors {
		if d.RuleID == "" {
			return fmt.Errorf("detectors[%d].rule_id is required", i)
		}
		if seenRule[d.RuleID] {
			return fmt.Errorf("duplicate rule_id %q in detectors[%d]", d.RuleID, i)
		}
		seenRule[d.RuleID] = true
		if d.SignalType == "" {
			return fmt.Errorf("detectors[%d].signal_type is required", i)
		}
		if seenType[d.SignalType] {
			return fmt.Errorf("duplicate signal_type %q in detectors[%d]", d.SignalType, i)
		}
		seenType[d.SignalType] = true
		if !isAllowedMechanismClass(d.MechanismClass) {
			return fmt.Errorf("detectors[%d].mechanism_class=%q not in the allowed list %v "+
				"(literal-string and regex primitives are not permitted — every rule must "+
				"clear a class, not a cell)", i, d.MechanismClass, AllowedMechanismClasses)
		}
		if d.DefaultSeverity == "" {
			return fmt.Errorf("detectors[%d].default_severity is required (critical / high / medium / low / info)", i)
		}
		switch d.Tier {
		case "", "observability", "gate":
		default:
			return fmt.Errorf("detectors[%d].tier=%q must be `gate` or `observability`", i, d.Tier)
		}
	}
	return nil
}

func isAllowedMechanismClass(c string) bool {
	for _, allowed := range AllowedMechanismClasses {
		if c == allowed {
			return true
		}
	}
	return false
}
