// Package terrainconfig parses and validates terrain.yaml v1 — the
// adopter-facing configuration file. Schema is locked from v0.2.0;
// changes are one-cycle deprecation-cycled.
//
// The package is the canonical reader for terrain.yaml. Existing legacy
// readers in internal/policy/ continue to work for backward compat but
// don't enforce the full v1 schema — new consumers should use this
// package.
package terrainconfig

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the current terrain.yaml schema version.
const SchemaVersion = 1

// FileName is the canonical config filename.
const FileName = "terrain.yaml"

// Config is the parsed terrain.yaml v1 shape.
type Config struct {
	Version int `yaml:"version" json:"version"`

	// Rules per-rule overrides. Keys are rule IDs without the
	// `terrain/` prefix (e.g., `regression/eval-regression`).
	// Values are bare severity strings or RuleBlock tuning blocks.
	Rules map[string]RuleSpec `yaml:"rules,omitempty" json:"rules,omitempty"`

	// Ignore configures path / per-rule exclusion.
	Ignore Ignore `yaml:"ignore,omitempty" json:"ignore,omitempty"`

	// AI configures eval-framework wiring.
	AI *AISection `yaml:"ai,omitempty" json:"ai,omitempty"`

	// ML configures classical-ML / model-registry wiring.
	ML *MLSection `yaml:"ml,omitempty" json:"ml,omitempty"`

	// Surfaces declares adopter-named AI/ML surfaces.
	Surfaces map[string]Surface `yaml:"surfaces,omitempty" json:"surfaces,omitempty"`

	// OnTerrainError is "block" (default, fails closed) or "pass"
	// (fails open).
	//
	// CURRENTLY INERT: parsed but not yet consumed by the engine. The
	// pipeline always fails closed today; this flag is documented to
	// reserve the field name so adopter terrain.yaml files don't need
	// a schema migration when the consumer wiring lands. See
	// docs/LIMITATIONS.md.
	OnTerrainError string `yaml:"on_terrain_error,omitempty" json:"on_terrain_error,omitempty"`

	// RedactSource elides code excerpts from emitted artifacts when
	// adopter codebases have stringent code-confidentiality requirements.
	//
	// CURRENTLY INERT: parsed but not yet consumed by any emission
	// path. The SARIF emitter already supports RedactPaths (separate
	// flag for filesystem paths); source-content redaction lands once
	// the emission surfaces consistently carry per-finding code
	// excerpts that can be elided. See docs/LIMITATIONS.md.
	RedactSource bool `yaml:"redact_source,omitempty" json:"redact_source,omitempty"`

	// Explain configures CLI LLM enrichment (never read in CI).
	Explain *ExplainSection `yaml:"explain,omitempty" json:"explain,omitempty"`

	// Slash configures the slash-command webhook receiver's authorization
	// policy. Default zero value (deny-all on destructive verbs).
	Slash *SlashSection `yaml:"slash,omitempty" json:"slash,omitempty"`
}

// SlashSection groups slash-command webhook policies.
type SlashSection struct {
	// Dismiss configures who may invoke /dismiss via the webhook.
	Dismiss *SlashDismissSection `yaml:"dismiss,omitempty" json:"dismiss,omitempty"`
}

// SlashDismissSection mirrors slash.DismissPolicy.
type SlashDismissSection struct {
	// AllowAuthors is the explicit allowlist of GitHub logins.
	AllowAuthors []string `yaml:"allow_authors,omitempty" json:"allow_authors,omitempty"`

	// AllowAnyoneWithCommentAccess removes the allowlist gate.
	AllowAnyoneWithCommentAccess bool `yaml:"allow_anyone_with_comment_access,omitempty" json:"allow_anyone_with_comment_access,omitempty"`
}

// RuleSpec carries either a bare severity string ("error", "warning",
// "off") or a structured RuleBlock. yaml.Unmarshal calls UnmarshalYAML
// below to pick the shape.
type RuleSpec struct {
	BareSeverity string     `yaml:"-" json:"bare_severity,omitempty"`
	Block        *RuleBlock `yaml:",inline" json:"block,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler so adopters can write
// either `regression/eval-regression: warning` (bare string) or
// the full RuleBlock form.
func (r *RuleSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		r.BareSeverity = node.Value
		return validateSeverity(r.BareSeverity)
	}
	var block RuleBlock
	if err := node.Decode(&block); err != nil {
		return err
	}
	r.Block = &block
	return block.validate()
}

// RuleBlock is the structured tuning shape for a rule.
type RuleBlock struct {
	Severity           string  `yaml:"severity,omitempty" json:"severity,omitempty"`
	Threshold          float64 `yaml:"threshold,omitempty" json:"threshold,omitempty"`
	ThresholdP95Ms     int     `yaml:"threshold_p95_ms,omitempty" json:"threshold_p95_ms,omitempty"`
	SamplesPerRun      int     `yaml:"samples_per_run,omitempty" json:"samples_per_run,omitempty"`
	SeedStrategy       string  `yaml:"seed_strategy,omitempty" json:"seed_strategy,omitempty"`
	ConfidenceAlpha    float64 `yaml:"confidence_alpha,omitempty" json:"confidence_alpha,omitempty"`
	BaseStrategy       string  `yaml:"base_strategy,omitempty" json:"base_strategy,omitempty"`
	ExtendedPeriodDays int     `yaml:"extended_period_days,omitempty" json:"extended_period_days,omitempty"`
	Scope              string  `yaml:"scope,omitempty" json:"scope,omitempty"`
	PIIEngine          string  `yaml:"pii_engine,omitempty" json:"pii_engine,omitempty"`

	// MaxFindings caps the number of findings emitted by this rule in
	// a single run. When the budget is exceeded, the highest-priority
	// findings are kept (severity DESC, confidence DESC, file ASC) and
	// the rest are dropped with a one-line notice on stderr. Zero =
	// unlimited (the default — schema is additive).
	MaxFindings int `yaml:"max_findings,omitempty" json:"max_findings,omitempty"`
}

// validate enforces the schema's enum constraints on optional fields.
func (b *RuleBlock) validate() error {
	if b.Severity != "" {
		if err := validateSeverity(b.Severity); err != nil {
			return err
		}
	}
	if b.SeedStrategy != "" {
		switch b.SeedStrategy {
		case "fixed", "rotating", "none":
		default:
			return fmt.Errorf("seed_strategy %q invalid (want fixed/rotating/none)", b.SeedStrategy)
		}
	}
	if b.BaseStrategy != "" {
		switch b.BaseStrategy {
		case "cached", "rerun", "from-ci-artifact":
		default:
			return fmt.Errorf("base_strategy %q invalid (want cached/rerun/from-ci-artifact)", b.BaseStrategy)
		}
	}
	if b.Scope != "" {
		switch b.Scope {
		case "changed_files", "impacted_tests", "all":
		default:
			return fmt.Errorf("scope %q invalid (want changed_files/impacted_tests/all)", b.Scope)
		}
	}
	if b.PIIEngine != "" {
		switch b.PIIEngine {
		case "native", "presidio":
		default:
			return fmt.Errorf("pii_engine %q invalid (want native/presidio)", b.PIIEngine)
		}
	}
	if b.ConfidenceAlpha < 0 || b.ConfidenceAlpha > 1 {
		return fmt.Errorf("confidence_alpha %v out of [0,1]", b.ConfidenceAlpha)
	}
	if b.SamplesPerRun < 0 {
		return fmt.Errorf("samples_per_run %d invalid (must be ≥ 1 or omitted)", b.SamplesPerRun)
	}
	if b.ThresholdP95Ms < 0 {
		return fmt.Errorf("threshold_p95_ms %d invalid", b.ThresholdP95Ms)
	}
	if b.ExtendedPeriodDays < 0 {
		return fmt.Errorf("extended_period_days %d invalid", b.ExtendedPeriodDays)
	}
	return nil
}

// Ignore configures path / per-rule exclusion.
type Ignore struct {
	Paths []string            `yaml:"paths,omitempty" json:"paths,omitempty"`
	Rules map[string][]string `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// AISection configures eval-framework wiring.
type AISection struct {
	Framework    string `yaml:"framework,omitempty" json:"framework,omitempty"`
	ScenariosDir string `yaml:"scenarios_dir,omitempty" json:"scenarios_dir,omitempty"`
	BaselinesDir string `yaml:"baselines_dir,omitempty" json:"baselines_dir,omitempty"`

	// AIMarkers is an opt-in list of additional regex patterns the
	// AI-context gate treats as evidence that a file is doing AI
	// work. Use this when your codebase imports a private LLM SDK
	// the canonical jsAIImports / pyAIImports lists don't cover.
	// Patterns are matched against file source as regular expressions.
	// Example:
	//
	//   ai:
	//     ai_markers:
	//       - "from internal_llm_sdk"
	//       - "@acme/llm-client"
	AIMarkers []string `yaml:"ai_markers,omitempty" json:"ai_markers,omitempty"`
}

// MLSection configures classical-ML / model-registry wiring.
type MLSection struct {
	Registry     string `yaml:"registry,omitempty" json:"registry,omitempty"`
	ArtifactsDir string `yaml:"artifacts_dir,omitempty" json:"artifacts_dir,omitempty"`
}

// Surface is an adopter-declared AI/ML surface.
type Surface struct {
	Description string `yaml:"description" json:"description"`
	Type        string `yaml:"type" json:"type"`
	FilePath    string `yaml:"file_path,omitempty" json:"file_path,omitempty"`
	Model       string `yaml:"model,omitempty" json:"model,omitempty"`
}

// ExplainSection configures CLI LLM enrichment.
type ExplainSection struct {
	Provider  string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Endpoint  string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Model     string `yaml:"model,omitempty" json:"model,omitempty"`
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
}

// Load reads terrain.yaml from path and validates it. Returns a
// usable Config or an error describing the first schema violation.
//
// When path doesn't exist, Load returns (nil, nil) — terrain.yaml is
// optional. Callers distinguish "no config" from "invalid config".
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("terrainconfig: read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes yaml bytes into a Config and validates it.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("terrainconfig: parse: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate enforces the v1 schema's hard constraints.
func (c *Config) Validate() error {
	if c.Version != SchemaVersion {
		return fmt.Errorf("terrainconfig: version=%d, want %d", c.Version, SchemaVersion)
	}
	if c.OnTerrainError != "" && c.OnTerrainError != "block" && c.OnTerrainError != "pass" {
		return fmt.Errorf("terrainconfig: on_terrain_error=%q (want block/pass)", c.OnTerrainError)
	}
	for ruleID := range c.Rules {
		if err := validateRuleKey(ruleID); err != nil {
			return fmt.Errorf("terrainconfig: rules: %w", err)
		}
	}
	for ruleID := range c.Ignore.Rules {
		if err := validateRuleKey(ruleID); err != nil {
			return fmt.Errorf("terrainconfig: ignore.rules: %w", err)
		}
	}
	if c.AI != nil && c.AI.Framework != "" {
		switch c.AI.Framework {
		case "promptfoo", "deepeval", "ragas", "gauntlet", "great-expectations", "none":
		default:
			return fmt.Errorf("terrainconfig: ai.framework=%q invalid", c.AI.Framework)
		}
	}
	if c.ML != nil && c.ML.Registry != "" {
		switch c.ML.Registry {
		case "mlflow", "wandb", "sagemaker", "vertex", "none":
		default:
			return fmt.Errorf("terrainconfig: ml.registry=%q invalid", c.ML.Registry)
		}
	}
	for name, s := range c.Surfaces {
		if !validIdentifier(name) {
			return fmt.Errorf("terrainconfig: surface key %q invalid (must be snake_case)", name)
		}
		if s.Description == "" {
			return fmt.Errorf("terrainconfig: surfaces[%q].description is required", name)
		}
		switch s.Type {
		case "llm", "classical_ml", "deep_learning", "rag_pipeline",
			"feature_pipeline", "prediction_service", "data_validator":
		default:
			return fmt.Errorf("terrainconfig: surfaces[%q].type=%q invalid", name, s.Type)
		}
	}
	if c.Explain != nil && c.Explain.Provider != "" {
		switch c.Explain.Provider {
		case "ollama", "openai", "anthropic", "custom", "none":
		default:
			return fmt.Errorf("terrainconfig: explain.provider=%q invalid", c.Explain.Provider)
		}
	}
	return nil
}

// SeverityFor returns the severity adopter wants for a rule, after
// applying terrain.yaml overrides. ruleID may carry or omit the
// `terrain/` prefix; both forms work.
func (c *Config) SeverityFor(ruleID, defaultSeverity string) string {
	if c == nil {
		return defaultSeverity
	}
	key := strings.TrimPrefix(ruleID, "terrain/")
	spec, ok := c.Rules[key]
	if !ok {
		return defaultSeverity
	}
	if spec.BareSeverity != "" {
		return spec.BareSeverity
	}
	if spec.Block != nil && spec.Block.Severity != "" {
		return spec.Block.Severity
	}
	return defaultSeverity
}

// IsPathIgnored returns true when terrain.yaml says to skip path
// (either globally via ignore.paths or for the specific rule).
// Glob matching uses doublestar-style "**" for recursive matches.
func (c *Config) IsPathIgnored(path, ruleID string) bool {
	if c == nil {
		return false
	}
	if pathMatches(c.Ignore.Paths, path) {
		return true
	}
	key := strings.TrimPrefix(ruleID, "terrain/")
	if pathMatches(c.Ignore.Rules[key], path) {
		return true
	}
	return false
}

// pathMatches walks the patterns and returns true on first match.
// Patterns use ** for "any-depth wildcard" segments.
func pathMatches(patterns []string, path string) bool {
	for _, p := range patterns {
		if matchGlob(p, path) {
			return true
		}
	}
	return false
}

// matchGlob implements a minimal glob matcher supporting:
//   - within a path segment
//     ** across path segments
//     ? single char
//
// No character classes — kept narrow on purpose.
func matchGlob(pattern, name string) bool {
	return globRecursive([]byte(pattern), []byte(name))
}

func globRecursive(pat, name []byte) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			if len(pat) > 1 && pat[1] == '*' {
				// ** — match any number of path segments
				rest := pat[2:]
				// strip a leading slash from rest if present
				if len(rest) > 0 && rest[0] == '/' {
					rest = rest[1:]
				}
				for i := 0; i <= len(name); i++ {
					if globRecursive(rest, name[i:]) {
						return true
					}
				}
				return false
			}
			// * — match within a segment (no /)
			rest := pat[1:]
			for i := 0; i <= len(name); i++ {
				if i > 0 && name[i-1] == '/' {
					break
				}
				if globRecursive(rest, name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(name) == 0 || name[0] == '/' {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		default:
			if len(name) == 0 || pat[0] != name[0] {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		}
	}
	return len(name) == 0
}

// --- validation helpers ---

func validateSeverity(s string) error {
	switch s {
	case "error", "warning", "off":
		return nil
	}
	return fmt.Errorf("severity %q invalid (want error/warning/off)", s)
}

func validateRuleKey(k string) error {
	if k == "" {
		return fmt.Errorf("rule key empty")
	}
	parts := strings.Split(k, "/")
	if len(parts) != 2 {
		return fmt.Errorf("rule key %q must be <category>/<name>", k)
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("rule key %q has empty segment", k)
		}
		for _, c := range p {
			switch {
			case c >= 'a' && c <= 'z',
				c >= '0' && c <= '9',
				c == '-':
			default:
				return fmt.Errorf("rule key %q has invalid char %q", k, c)
			}
		}
	}
	return nil
}

func validIdentifier(s string) bool {
	if s == "" {
		return false
	}
	if !(s[0] >= 'a' && s[0] <= 'z') && s[0] != '_' {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '_':
		default:
			return false
		}
	}
	return true
}
