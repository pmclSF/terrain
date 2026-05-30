// Package runtimeconfig is the RuntimeConfigRecognizer.
//
// One structural primitive:
//
//	"Any YAML / properties file with import-graph-reachable consumer
//	 that injects values into an SDK client constructor."
//
// Ships at observability tier. The capability is preserved; if the
// primitive proves too narrow on a labeled sample, demote further —
// the rule is not retired.
//
// What the recognizer does today:
//  1. RecognizeFile: parses a YAML or .properties file and reports
//     the model-config-shaped keys it carries (temperature, model,
//     seed, embedding, retry, etc.).
//  2. HasLoader: scans repo source for files that consume the config
//     file via a known loader pattern (yaml.safe_load, dotenv,
//     Config.from_yaml, etc.).
//  3. Both together → the file is plausibly a runtime config; the
//     consumer detector demotes its finding to observability tier.
//
// The recognizer is mechanism-gated by `runtime_config_recognizer`.
package runtimeconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/pmclSF/terrain/internal/mechanisms"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "runtime_config_recognizer"

// modelConfigKeys is the structural key vocabulary the recognizer
// treats as "this file plausibly configures a model at runtime." The
// list intentionally avoids vendor-specific keys; it captures the
// dimensions every model-driving config shares.
var modelConfigKeys = map[string]bool{
	"temperature":       true,
	"top_p":             true,
	"top_k":             true,
	"max_tokens":        true,
	"seed":              true,
	"model":             true,
	"model_name":        true,
	"embedding":         true,
	"embedding_model":   true,
	"retry":             true,
	"retries":           true,
	"timeout":           true,
	"frequency_penalty": true,
	"presence_penalty":  true,
	"stop":              true,
	"system_prompt":     true,
}

// Report is the recognizer's output for one config file.
type Report struct {
	Path            string   `json:"path"`
	Format          string   `json:"format"` // "yaml" | "properties" | "unknown"
	ConfigKeysHit   []string `json:"config_keys_hit"`
	HasLoaderInRepo bool     `json:"has_loader_in_repo"`
}

// IsRuntimeConfig reports the recognizer's verdict.
func (r *Report) IsRuntimeConfig() bool {
	return len(r.ConfigKeysHit) > 0 && r.HasLoaderInRepo
}

// RecognizeFile parses the file at path. Returns a Report regardless
// of whether the file is a runtime config — the verdict is in
// Report.IsRuntimeConfig().
func RecognizeFile(path, repoRoot string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := &Report{Path: path}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		report.Format = "yaml"
		report.ConfigKeysHit = configKeysInYAML(data)
	case ".properties", ".env":
		report.Format = "properties"
		report.ConfigKeysHit = configKeysInProperties(data)
	default:
		report.Format = "unknown"
	}
	if len(report.ConfigKeysHit) > 0 {
		report.HasLoaderInRepo = hasLoaderForPath(repoRoot, path)
	}
	return report, nil
}

// configKeysInYAML returns the model-config keys present anywhere in
// the YAML document (top-level or nested under any object).
func configKeysInYAML(data []byte) []string {
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	walkKeys(doc, seen)
	var out []string
	for k := range seen {
		if modelConfigKeys[k] {
			out = append(out, k)
		}
	}
	return out
}

// walkKeys recursively collects every map key in a parsed YAML
// document into `seen`.
func walkKeys(v any, seen map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			seen[k] = true
			walkKeys(val, seen)
		}
	case []any:
		for _, e := range x {
			walkKeys(e, seen)
		}
	}
}

// configKeysInProperties scans a .properties / .env file for
// key=value lines where the key matches a model-config key.
func configKeysInProperties(data []byte) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexAny(line, "=:")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		// .properties uses dot.notation — take the final segment for
		// modelConfigKeys lookup, but also try the bare key.
		base := key
		if dot := strings.LastIndex(key, "."); dot >= 0 {
			base = key[dot+1:]
		}
		base = strings.ToLower(base)
		if modelConfigKeys[base] {
			seen[base] = true
		}
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// loaderPatternRe captures the canonical config-loader call sites
// across Python, JS, and Go. Used to test "is this config file
// consumed at runtime by code in the repo?"
var loaderPatternRe = regexp.MustCompile(
	`yaml\.(?:safe_)?load\s*\(|` +
		`json\.load\s*\(|` +
		`dotenv\.load_dotenv|` +
		`os\.getenv\(|` +
		`pydantic\.BaseSettings|` +
		`ConfigParser\(|` +
		`@hydra\.main|` +
		`config\.from_(?:yaml|json|env)|` +
		`fs\.readFileSync\s*\([^)]*['"]\.(?:yaml|yml|json|env|properties)['"]|` +
		`os\.ReadFile\s*\(`,
)

// hasLoaderForPath walks the repo root and reports whether any source
// file appears to consume a config file. Best-effort: matches a
// generic loader call (yaml.safe_load, etc.), not the specific path,
// because verifying the actual path argument requires data-flow
// analysis. False positives are bounded — the consumer detector
// applies a second test (eval-config presence) before demoting.
//
// Result is cached per repoRoot: the first call walks the tree, all
// subsequent calls return the cached bool. Caller code (engine
// pipeline) calls ClearLoaderCache between runs.
func hasLoaderForPath(repoRoot, configPath string) bool {
	if v, ok := loaderCacheGet(repoRoot); ok {
		return v
	}
	found := false
	_ = filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
		if found {
			return filepath.SkipAll
		}
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == ".venv" || name == "venv" ||
				name == "dist" || name == "build" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(p)
		switch ext {
		case ".py", ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs", ".go", ".java", ".kt":
			data, _ := os.ReadFile(p)
			if loaderPatternRe.Match(data) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	loaderCacheSet(repoRoot, found)
	return found
}

// Per-repo cache for hasLoaderForPath. RuntimeConfigRecognizer is
// invoked per YAML / .properties file in the repo; without the cache
// the same whole-repo walk runs N times for N candidate config files.
var (
	loaderCacheMu sync.RWMutex
	loaderCache   = map[string]bool{}
)

func loaderCacheGet(repoRoot string) (bool, bool) {
	loaderCacheMu.RLock()
	defer loaderCacheMu.RUnlock()
	v, ok := loaderCache[repoRoot]
	return v, ok
}

func loaderCacheSet(repoRoot string, v bool) {
	loaderCacheMu.Lock()
	loaderCache[repoRoot] = v
	loaderCacheMu.Unlock()
}

// ClearLoaderCache drops the per-repo loader-presence cache. The
// engine pipeline calls this at the start of each run so a long-lived
// process picks up filesystem changes.
func ClearLoaderCache() {
	loaderCacheMu.Lock()
	loaderCache = map[string]bool{}
	loaderCacheMu.Unlock()
}

// ── Shadow-mode helper ────────────────────────────────────────────

// GateDemotion is the canonical wire-up for the aiNonDeterministicEval
// consumer. Returns true when the finding should be demoted (when the
// mechanism is on AND the file is a runtime config). Shadow → returns
// false but emits a would-demote-severity event.
//
// Routes through mechanisms.GateDemote so the off/shadow/on state
// machine is shared with every other gate helper.
func GateDemotion(reg *mechanisms.Registry, report *Report, ruleID, file string) bool {
	return mechanisms.GateDemote(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: file},
		func() mechanisms.PredicateResult {
			if !report.IsRuntimeConfig() {
				return mechanisms.PredicateResult{Fired: false}
			}
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"runtime config keys: " + strings.Join(report.ConfigKeysHit, ", "), "loader_in_repo"},
			}
		})
}
