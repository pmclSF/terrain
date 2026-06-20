package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/analysis"
)

// InitResult holds everything detected during `terrain init`.
type InitResult struct {
	// Root is the absolute repository root path.
	Root string `json:"root"`

	// Frameworks detected at the project level.
	Frameworks []DetectedFramework `json:"frameworks"`

	// Languages detected from test files.
	Languages []string `json:"languages"`

	// Artifacts discovered at known paths.
	Artifacts *ArtifactDiscovery `json:"artifacts,omitempty"`

	// HasTerrainDir is true if .terrain/ already exists.
	HasTerrainDir bool `json:"hasTerrainDir"`

	// HasPolicyFile is true if .terrain/policy.yaml already exists.
	HasPolicyFile bool `json:"hasPolicyFile"`

	// HasTerrainYAML is true if terrain.yaml already exists.
	HasTerrainYAML bool `json:"hasTerrainYaml"`

	// TestFileCount is the number of test files discovered.
	TestFileCount int `json:"testFileCount"`

	// ConfigPath is the path to the generated config, if any.
	ConfigPath string `json:"configPath,omitempty"`

	// PolicyPath is the path to the generated policy, if any.
	PolicyPath string `json:"policyPath,omitempty"`

	// PolicyExamplePath is the path to the annotated policy.yaml.example
	// reference (always written, even when policy.yaml already exists,
	// so the latest schema is documented for adopters).
	PolicyExamplePath string `json:"policyExamplePath,omitempty"`
}

// DetectedFramework captures a framework found during init scanning.
type DetectedFramework struct {
	Name       string  `json:"name"`
	Language   string  `json:"language"`
	Source     string  `json:"source"` // "config-file", "dependency", "convention"
	Confidence float64 `json:"confidence"`
}

// ScanRepo performs a read-only scan of the repository at `root`. It
// detects frameworks, artifacts, and existing configuration WITHOUT
// writing anything to disk. Callers that want the legacy "scan + write
// config files" behavior call RunInit, which calls ScanRepo first and
// then writes config files via WriteInitConfig.
//
// `terrain` (no-args) discovery and any other read-only surface MUST
// call ScanRepo, never RunInit. Writing config files from a discovery
// surface is a UX surprise — users typing `terrain` to look around
// don't expect a new directory to appear in their working tree.
func ScanRepo(root string) (*InitResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid root path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("invalid root path %q: not a directory", root)
	}

	result := &InitResult{Root: absRoot}

	// Detect existing configuration.
	result.HasTerrainDir = dirExists(filepath.Join(absRoot, ".terrain"))
	result.HasPolicyFile = fileExists(filepath.Join(absRoot, ".terrain", "policy.yaml"))
	result.HasTerrainYAML = fileExists(filepath.Join(absRoot, ".terrain", "terrain.yaml")) ||
		fileExists(filepath.Join(absRoot, "terrain.yaml"))

	// Detect project-level frameworks.
	projectCtx := analysis.DetectProjectFrameworks(absRoot)
	for lang, fws := range projectCtx.Frameworks {
		for _, fw := range fws {
			result.Frameworks = append(result.Frameworks, DetectedFramework{
				Name:       fw.Name,
				Language:   lang,
				Source:     fw.Source,
				Confidence: fw.Confidence,
			})
		}
	}
	sort.SliceStable(result.Frameworks, func(i, j int) bool {
		if result.Frameworks[i].Confidence != result.Frameworks[j].Confidence {
			return result.Frameworks[i].Confidence > result.Frameworks[j].Confidence
		}
		return result.Frameworks[i].Name < result.Frameworks[j].Name
	})

	// Detect languages from frameworks.
	langSet := map[string]bool{}
	for _, fw := range result.Frameworks {
		langSet[fw.Language] = true
	}
	for l := range langSet {
		result.Languages = append(result.Languages, l)
	}
	sort.Strings(result.Languages)

	// Quick test file count (from project context, no full scan needed).
	// For a lightweight init, we count test files by pattern matching.
	result.TestFileCount = countTestFiles(absRoot)

	// Discover artifacts.
	result.Artifacts = DiscoverArtifacts(absRoot)

	return result, nil
}

// WriteInitConfig writes .terrain/policy.yaml and .terrain/terrain.yaml
// based on a prior ScanRepo result. Mutates the result with the paths
// of any newly-written files. Idempotent: existing files are preserved.
// Splitting this from ScanRepo lets the discovery surface remain
// read-only while `terrain init` (the explicit init command) keeps
// generating defaults.
func WriteInitConfig(result *InitResult) error {
	terrainDir := filepath.Join(result.Root, ".terrain")
	if !result.HasTerrainDir {
		if err := os.MkdirAll(terrainDir, 0o755); err != nil {
			return fmt.Errorf("create .terrain/ directory: %w", err)
		}
	}

	if !result.HasPolicyFile {
		policyPath := filepath.Join(terrainDir, "policy.yaml")
		if err := generatePolicyYAML(policyPath); err != nil {
			return fmt.Errorf("generate policy.yaml: %w", err)
		}
		result.PolicyPath = policyPath
	}
	// Always (re)write the annotated example so it stays current as
	// new config knobs land. The example is read-only reference; the
	// real policy.yaml carries the adopter's choices.
	examplePath := filepath.Join(terrainDir, "policy.yaml.example")
	if err := generatePolicyYAMLExample(examplePath); err != nil {
		return fmt.Errorf("generate policy.yaml.example: %w", err)
	}
	result.PolicyExamplePath = examplePath

	if !result.HasTerrainYAML && (len(result.Frameworks) > 0 || result.TestFileCount > 0) {
		configPath := filepath.Join(terrainDir, "terrain.yaml")
		if err := generateTerrainYAML(configPath, result); err != nil {
			return fmt.Errorf("generate terrain.yaml: %w", err)
		}
		result.ConfigPath = configPath
	}

	// Drop a .gitignore inside .terrain/ that excludes runtime
	// artifacts (shadow-report.jsonl, cache files) but keeps user-
	// authored config (policy.yaml, suppressions.yaml, terrain.yaml,
	// repos.yaml) tracked. Idempotent: existing .gitignore is
	// preserved so user customizations survive.
	gitignorePath := filepath.Join(terrainDir, ".gitignore")
	if !fileExists(gitignorePath) {
		if err := os.WriteFile(gitignorePath, []byte(defaultTerrainGitignore), 0o644); err != nil {
			return fmt.Errorf("generate .terrain/.gitignore: %w", err)
		}
	}

	return nil
}

const defaultTerrainGitignore = `# .terrain/ — runtime artifacts excluded, config tracked.
#
# Generated by ` + "`terrain init`" + `. Edit if your team prefers a different
# split (e.g. you want shadow-report.jsonl tracked for audit).

# Runtime artifacts — regenerated each analyze run.
shadow-report.jsonl
findings.json
snapshots/
*.cache
findings-cache/

# Keep config tracked (the rules below explicitly un-ignore them so
# they're committed even if a parent .gitignore excludes them).
!policy.yaml
!suppressions.yaml
!terrain.yaml
!repos.yaml
!*.example
`

// RunInit is the legacy "scan + write" entry point. ScanRepo +
// WriteInitConfig is the new contract. RunInit calls both, preserving
// the historical behavior for callers like `terrain init`. New callers
// should call ScanRepo directly when only a read-only scan is needed.
func RunInit(root string) (*InitResult, error) {
	result, err := ScanRepo(root)
	if err != nil {
		return nil, err
	}
	if err := WriteInitConfig(result); err != nil {
		return nil, err
	}
	return result, nil
}

// generatePolicyYAMLExample writes a comprehensive, annotated reference
// of every supported terrain.yaml + policy.yaml field. The actual
// `.terrain/policy.yaml` carries only the adopter's choices; this
// `.example` is the self-documenting "what knobs exist" reference.
//
// Regenerated on every `terrain init` so the example stays current
// as new fields land.
func generatePolicyYAMLExample(path string) error {
	const body = `# Terrain — annotated reference for terrain.yaml + .terrain/policy.yaml.
#
# This file is auto-generated by ` + "`terrain init`" + ` and is overwritten on
# every run. Edit ` + "`.terrain/policy.yaml`" + ` (or a top-level ` + "`terrain.yaml`" + `)
# directly; this example is read-only reference.
#
# Every option below is commented out by default. Uncomment what you
# need. Adopters with stringent gating land here first to see what's
# available; new options gain a one-line description here whenever
# they ship.

# ── Schema version ─────────────────────────────────────────────
# version: 1

# ── Severity gating ────────────────────────────────────────────
# rules:
#   # Two value shapes per rule:
#   #   1) A bare severity:   "error" | "warning" | "info" | "off"
#   #   2) A structured RuleBlock with extra knobs.
#
#   regression/eval-regression: warning      # bare severity form
#   regression/eval-regression:               # structured form
#     severity: error
#     threshold: 5.0                          # %-points drop to gate
#     samples_per_run: 3                      # repeated runs for noise
#     seed_strategy: pinned                   # pinned | per-run
#     confidence_alpha: 0.05                  # statistical threshold
#     base_strategy: cached                   # cached | rerun
#     extended_period_days: 14                # baseline rolling window
#
#   coverage/no-tests: warning
#   coverage/no-eval: warning
#   coverage/no-data-validation: warning
#
#   security/insecure-deserialization: error
#   security/pii-in-eval: error               # add a PII engine override
#   #   pii_engine: presidio                  # presidio | go-native
#
#   reproducibility/version-floating: warning
#   reproducibility/no-seed: warning

# ── Path / per-rule ignores ────────────────────────────────────
# ignore:
#   paths:
#     - "vendor/**"
#     - "third_party/**"
#     - "**/*.generated.go"
#   per_rule:
#     coverage/no-tests:
#       - "cmd/internal/**"   # maintainer-only binaries

# ── AI surface configuration ───────────────────────────────────
# ai:
#   # Names recognized as AI-SDK markers in source. Extends the
#   # built-in marker list (anthropic, openai, langchain, etc).
#   ai_markers:
#     - my_internal_llm_sdk
#     - acme.ai
#   framework: promptfoo        # promptfoo | deepeval | ragas (CURRENTLY INERT)
#   scenarios_dir: tests/evals  # (CURRENTLY INERT)
#   baselines_dir: .terrain/baselines  # (CURRENTLY INERT)

# ── ML surface configuration ───────────────────────────────────
# ml:
#   registry: mlflow            # mlflow | wandb (CURRENTLY INERT)
#   artifacts_dir: artifacts/    # (CURRENTLY INERT)

# ── Adopter-named surfaces ─────────────────────────────────────
# surfaces:
#   my_prompt_pipeline:
#     kind: prompt
#     files:
#       - prompts/*.md

# ── Error-handling ─────────────────────────────────────────────
# on_terrain_error: block       # block (default, fails closed) | pass
#                               # CURRENTLY INERT: pipeline always
#                               # fails closed today.

# ── Source-content redaction ───────────────────────────────────
# redact_source: false          # Reserved for future source-content
#                               # redaction. CURRENTLY INERT: parsed
#                               # but no emission path consumes it today.
#                               # See docs/LIMITATIONS.md.

# ── Slash-command authorization (webhook receiver) ─────────────
# slash:
#   dismiss:
#     # Default (zero value): deny-all. /dismiss replies with a
#     # not-authorized notice unless one of the below is set.
#     allow_authors:                              # explicit allowlist
#       - octocat
#       - dependabot[bot]
#     # OR:
#     allow_anyone_with_comment_access: false     # team-trust mode

# ── CLI LLM enrichment (` + "`terrain explain`" + ` only) ──────────────
# explain:
#   provider: ollama            # ollama | anthropic | openai (CURRENTLY INERT)
#   endpoint: http://localhost:11434
#   model: llama3.1

# End of annotated reference.
`
	return os.WriteFile(path, []byte(body), 0o644)
}

func generatePolicyYAML(path string) error {
	content := `# Terrain policy configuration
#
# Edit this file to enforce policy rules in CI via:
#   terrain policy check
#
# Three starter policies live under docs/policy/examples/:
#
#   minimal.yaml    safe defaults — warn on common debt, block nothing
#   balanced.yaml   gate on critical findings, leave room for catch-up
#   strict.yaml     block on any high-or-above finding (mature repos)
#
# Copy one of those over this file to get going fast, or uncomment
# the rules below one at a time.

rules:
  # ── Core test-system rules ───────────────────────────────────
  # disallow_skipped_tests: true       # block tests that .skip() in CI
  # disallow_frameworks:               # framework drift control
  #   - jest                           #   list a deprecated framework here
  # max_test_runtime_ms: 5000          # per-test runtime budget
  # minimum_coverage_percent: 80       # repository-level coverage floor
  # max_weak_assertions: 5             # density of weak-assertion findings
  # max_mock_heavy_tests: 3            # density of mock-heavy tests

  # ── AI governance rules ──────────────────────────────────────
  # Applies to repos with AI surfaces / eval scenarios.
  # ai:
  #   block_on_safety_failure: true            # gate on aiSafetyEvalMissing
  #   block_on_accuracy_regression: 5          # %-points drop allowed
  #   block_on_uncovered_context: true         # gate on uncoveredAISurface
  #   warn_on_latency_regression: true
  #   warn_on_cost_regression: true            # paired-case avg cost rising
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func generateTerrainYAML(path string, result *InitResult) error {
	var b strings.Builder

	b.WriteString("# Terrain configuration\n")
	b.WriteString("# Generated by: terrain init\n")
	b.WriteString("# Edit this file to declare manual coverage, scenarios, and CI metadata.\n")
	b.WriteString("#\n")
	b.WriteString("# Detected frameworks: ")
	if len(result.Frameworks) > 0 {
		names := make([]string, 0, len(result.Frameworks))
		for _, fw := range result.Frameworks {
			names = append(names, fw.Name)
		}
		b.WriteString(strings.Join(names, ", "))
	} else {
		b.WriteString("(none detected)")
	}
	b.WriteString("\n")

	if result.Artifacts != nil && result.Artifacts.CoveragePath != "" {
		relCov := relativeTo(result.Artifacts.CoveragePath, result.Root)
		b.WriteString("# Detected coverage: " + relCov + " (" + result.Artifacts.CoverageFormat + ")\n")
	}
	if result.Artifacts != nil && len(result.Artifacts.RuntimePaths) > 0 && len(result.Artifacts.RuntimeFormats) > 0 {
		relRT := relativeTo(result.Artifacts.RuntimePaths[0], result.Root)
		b.WriteString("# Detected runtime: " + relRT + " (" + result.Artifacts.RuntimeFormats[0] + ")\n")
	}
	b.WriteString("#\n")
	b.WriteString("# See: docs/examples/manual-coverage.md\n\n")

	// Manual coverage section (commented template).
	b.WriteString("# manual_coverage:\n")
	b.WriteString("#   - name: Regression suite\n")
	b.WriteString("#     area: src/billing\n")
	b.WriteString("#     source: testrail\n")
	b.WriteString("#     owner: qa-team\n")
	b.WriteString("#     criticality: high\n")
	b.WriteString("#     frequency: per-release\n\n")

	// Scenarios section (commented template).
	b.WriteString("# scenarios:\n")
	b.WriteString("#   - name: prompt-accuracy\n")
	b.WriteString("#     category: accuracy\n")
	b.WriteString("#     framework: custom\n")
	b.WriteString("#     surfaces:\n")
	b.WriteString("#       - surface:src/prompts.ts:systemPrompt\n\n")

	// CI duration hint.
	b.WriteString("# ci_duration_seconds: 120\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// countTestFiles does a lightweight count of test files without full analysis.
func countTestFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			// Keep in sync with analysis.skipDirs in repository_scan.go.
			switch base {
			case ".git", "node_modules", "dist", "build", "benchmarks",
				"coverage", ".next", ".turbo", ".nuxt", "vendor",
				"__pycache__", ".pytest_cache", ".mypy_cache", ".tox",
				".venv", "venv", ".idea", ".vscode", ".terrain", "target":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if isTestFileName(name) {
			count++
		}
		return nil
	})
	return count
}

func isTestFileName(name string) bool {
	// JS/TS test files.
	if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") {
		return true
	}
	// Go test files.
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	// Python test files.
	if strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") {
		return true
	}
	if strings.HasSuffix(name, "_test.py") {
		return true
	}
	// Java test files.
	if strings.HasSuffix(name, "Test.java") {
		return true
	}
	return false
}

// relativeTo returns path relative to base, falling back to the original path.
func relativeTo(path, base string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
