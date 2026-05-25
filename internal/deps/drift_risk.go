// Package deps implements Terrain detectors for dependency-manifest
// drift risk.
//
// Empirical motivation: bot-authored PRs (renovate / dependabot / etc.)
// regress at multiples of the validation baseline, and a meaningful
// share of all unflagged regression-introducing PRs observed are bot
// or deps-bump shaped — a class no other detector in the roster
// targets.
//
// v1 design — analyze-time, not PR-diff-time:
//
//   - Walk repo for manifest files (package.json, requirements.txt,
//     Cargo.toml, go.mod, etc.).
//   - Parse dep specifiers per ecosystem.
//   - Compute the share of each manifest's deps that use "moving-
//     target" version specs (e.g. `*`, `latest`, unbounded `^`/`~`
//     ranges, untagged Git refs).
//   - Fire one finding per manifest whose moving-target share
//     exceeds a threshold (default 40%).
//
// v2 (post-0.2.0): consume PR-diff context so we can flag specific
// bumps in PR review. v1 catches the static "manifest is risky-by-
// construction" pattern that correlates with the bot-PR regression
// class.
package deps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// SplitMechanismName toggles the depsDriftRisk split:
// strict-pin (bare-name / unversioned) vs caret-policy (caret ranges).
// When off, the detector emits the legacy "depsDriftRisk" signal type.
const SplitMechanismName = "deps_drift_risk_split"

// DriftRiskDetector emits SignalDepsDriftRisk for each manifest file
// whose share of moving-target deps exceeds the configured threshold.
type DriftRiskDetector struct {
	Root string
}

// movingTargetShare is the threshold above which a manifest is flagged.
const movingTargetShare = 0.40

// minDepsForFinding suppresses signals on tiny manifests where the
// share metric isn't statistically meaningful.
const minDepsForFinding = 3

// trackedManifests is the canonical set of dep-manifest filenames the
// detector inspects. Filename match is intentionally exact to keep
// the detector cheap; we don't recursively scan node_modules etc.
var trackedManifests = []string{
	"package.json",
	"requirements.txt",
	"requirements-dev.txt",
	"requirements-test.txt",
	"pyproject.toml",
	"Cargo.toml",
	"go.mod",
	"Gemfile",
	"composer.json",
	"build.gradle",
	"build.gradle.kts",
	"pom.xml",
}

// Detect walks the repo for tracked manifests and emits a finding
// per manifest with high moving-target share.
func (d *DriftRiskDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || d.Root == "" {
		return nil
	}
	var out []models.Signal
	_ = filepath.Walk(d.Root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(p)
			if base == "node_modules" || base == "vendor" || base == ".git" ||
				base == "target" || base == "dist" || base == "build" ||
				base == "third_party" {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.Base(p)
		if !isTrackedManifest(name) {
			return nil
		}
		stats, ok := analyseManifest(p)
		if !ok || stats.TotalDeps < minDepsForFinding {
			return nil
		}
		share := float64(stats.MovingTargets) / float64(stats.TotalDeps)
		if share < movingTargetShare {
			return nil
		}

		rel, _ := filepath.Rel(d.Root, p)
		severity := models.SeverityMedium
		if share > 0.75 {
			severity = models.SeverityHigh
		} else if share < 0.50 {
			severity = models.SeverityLow
		}
		// Mechanism gate: deps_drift_risk_split. Route through GateAdd
		// so shadow mode emits would-add events without changing
		// user-visible types. Only state=on actually swaps the type.
		sigType := signals.SignalDepsDriftRisk
		ruleID := "terrain/deps/drift-risk"
		ruleURI := "docs/rules/deps/drift-risk.md"
		splitOn := mechanisms.GateAdd(mechanisms.Default(), SplitMechanismName,
			mechanisms.EventContext{RuleID: "depsDriftRisk", File: rel},
			func() mechanisms.PredicateResult {
				return mechanisms.PredicateResult{
					Fired:   true,
					Reasons: []string{"emit split signal type (strict-pin vs caret-policy)"},
				}
			})
		if splitOn {
			if stats.CaretIssues > stats.StrictPinIssues {
				sigType = signals.SignalDepsDriftRiskCaretPolicy
				ruleID = "terrain/deps/drift-caret-policy"
				ruleURI = "docs/rules/deps/drift-caret-policy.md"
			} else {
				sigType = signals.SignalDepsDriftRiskStrictPin
				ruleID = "terrain/deps/drift-strict-pin"
				ruleURI = "docs/rules/deps/drift-strict-pin.md"
			}
		}
		out = append(out, models.Signal{
			Type:             sigType,
			Category:         models.CategoryQuality,
			Severity:         severity,
			Confidence:       0.7,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceStructuralPattern,
			Location: models.SignalLocation{
				File: rel,
			},
			Explanation: "Dependency manifest `" + rel + "` has " +
				itoa(stats.MovingTargets) + " of " + itoa(stats.TotalDeps) +
				" deps using moving-target version specs (no version pin, range, or `latest`). " +
				"Bot-authored bumps to this manifest correlate with elevated regression rates in public-OSS data.",
			SuggestedAction: "Pin deps to specific versions (or narrow ranges with upper bounds). Configure renovate/dependabot to group + test bumps before auto-merging. Consider `--ignore-scripts` for install steps.",
			Metadata: map[string]any{
				"manifest":          name,
				"ecosystem":         stats.Ecosystem,
				"totalDeps":         stats.TotalDeps,
				"movingTargetDeps":  stats.MovingTargets,
				"movingTargetShare": share,
				"strictPinIssues":   stats.StrictPinIssues,
				"caretIssues":       stats.CaretIssues,
			},
			RuleID:  ruleID,
			RuleURI: ruleURI,
		})
		return nil
	})
	return out
}

func isTrackedManifest(name string) bool {
	for _, m := range trackedManifests {
		if name == m {
			return true
		}
	}
	return false
}

// manifestStats is the per-manifest deps summary.
type manifestStats struct {
	Ecosystem     string
	TotalDeps     int
	MovingTargets int
	// StrictPinIssues counts moving-target deps that have no pin at
	// all (bare name, `*`, `latest`, untagged git+/file:/workspace:).
	StrictPinIssues int
	// CaretIssues counts moving-target deps using caret-range (`^x.y.z`).
	CaretIssues int
}

func analyseManifest(path string) (manifestStats, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifestStats{}, false
	}
	base := filepath.Base(path)
	switch base {
	case "package.json":
		return analyseNPM(data), true
	case "requirements.txt", "requirements-dev.txt", "requirements-test.txt":
		return analysePipRequirements(data), true
	case "pyproject.toml":
		return analysePyProject(data), true
	case "Cargo.toml":
		return analyseCargo(data), true
	case "go.mod":
		return analyseGoMod(data), true
	case "Gemfile":
		return analyseGemfile(data), true
	}
	return manifestStats{}, false
}

// --- NPM (package.json) ---

type npmManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// npmMovingTarget matches version specs that are "moving target" —
// no upper bound, `latest`, `*`, plain Git refs, etc. Pinned exact
// specs (`1.2.3`) and tight ranges (`~1.2.3`) are NOT moving target.
var npmMovingTarget = regexp.MustCompile(
	`^(?:\*|latest|next|x|\.x|>=?|\^\d|\~\d|git\+|file:|workspace:|link:|http)`)

func analyseNPM(data []byte) manifestStats {
	var m npmManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifestStats{Ecosystem: "npm"}
	}
	stats := manifestStats{Ecosystem: "npm"}
	count := func(deps map[string]string) {
		for _, spec := range deps {
			stats.TotalDeps++
			s := strings.TrimSpace(spec)
			if isNPMMovingTarget(s) {
				stats.MovingTargets++
				if strings.HasPrefix(s, "^") {
					stats.CaretIssues++
				} else {
					stats.StrictPinIssues++
				}
			}
		}
	}
	count(m.Dependencies)
	count(m.DevDependencies)
	count(m.PeerDependencies)
	count(m.OptionalDependencies)
	return stats
}

// isNPMMovingTarget classifies an npm version specifier.
//   - `1.2.3`           → not moving (exact pin)
//   - `~1.2.3`          → not moving (tight)
//   - `^1.2.3`          → MOVING (allows minor + patch bumps)
//   - `>=1.0.0`         → MOVING
//   - `*` / `latest`    → MOVING
//   - `git+…` / `file:` → MOVING (untagged source)
func isNPMMovingTarget(s string) bool {
	if s == "" {
		return false
	}
	if npmMovingTarget.MatchString(s) {
		return true
	}
	return false
}

// --- Python requirements.txt ---

var pipMovingTarget = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\[\]]+\s*$`)
var pipLooseRange = regexp.MustCompile(`(>=|>|~=|!=|<=|<)`)
var pipExactPin = regexp.MustCompile(`==\s*\d`)

func analysePipRequirements(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "pip"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		// Strip inline comment.
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		stats.TotalDeps++
		// Exact pin (`pkg==1.2.3`) is NOT moving.
		if pipExactPin.MatchString(line) {
			continue
		}
		// No specifier at all (`pkg`) IS moving.
		if pipMovingTarget.MatchString(line) {
			stats.MovingTargets++
			continue
		}
		// Loose range without exact (`pkg>=1.0`) IS moving.
		if pipLooseRange.MatchString(line) {
			stats.MovingTargets++
		}
	}
	return stats
}

// --- Python pyproject.toml ---

// pyprojectDep matches `name = "spec"` lines inside [dependencies] /
// [tool.poetry.dependencies] / dependencies = [...] blocks. Very
// rough; full TOML parsing is overkill for the drift heuristic.
var pyprojectDep = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\-]+)\s*=\s*"([^"]*)"`)
var pyprojectArrayDep = regexp.MustCompile(`"([a-zA-Z0-9_\-\[\]]+)\s*([=<>~!]+\s*[\d.\*]+)?"`)

func analysePyProject(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "pyproject"}
	src := string(data)
	// Tuple-style: name = "spec"
	for _, m := range pyprojectDep.FindAllStringSubmatch(src, -1) {
		spec := strings.TrimSpace(m[2])
		// Skip non-dep keys (description, version, etc.).
		if !looksLikeDepSpec(spec) {
			continue
		}
		stats.TotalDeps++
		if !pipExactPin.MatchString(spec) {
			stats.MovingTargets++
		}
	}
	return stats
}

func looksLikeDepSpec(s string) bool {
	return strings.ContainsAny(s, "<>=~^!*") || s == "*" || s == ""
}

// --- Cargo.toml ---

var cargoDep = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\-]+)\s*=\s*"([^"]+)"`)

func analyseCargo(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "cargo"}
	src := string(data)
	// Only look inside [dependencies] / [dev-dependencies] sections.
	inDeps := false
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			inDeps = strings.HasPrefix(trim, "[dependencies") ||
				strings.HasPrefix(trim, "[dev-dependencies") ||
				strings.HasPrefix(trim, "[build-dependencies") ||
				strings.HasPrefix(trim, "[workspace.dependencies")
			continue
		}
		if !inDeps {
			continue
		}
		m := cargoDep.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		spec := strings.TrimSpace(m[2])
		stats.TotalDeps++
		// Cargo: `1.2.3` is moving (semver caret), `=1.2.3` is pinned.
		if !strings.HasPrefix(spec, "=") {
			stats.MovingTargets++
		}
	}
	return stats
}

// --- go.mod ---

var goModDep = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9\.\-_/]+)\s+v?([\d\w\.\-+]+)`)

func analyseGoMod(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "go"}
	src := string(data)
	inRequire := false
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "require ("):
			inRequire = true
			continue
		case trim == ")":
			inRequire = false
			continue
		case strings.HasPrefix(trim, "require "):
			// Single-line require: also count it.
			rest := strings.TrimPrefix(trim, "require ")
			if m := goModDep.FindStringSubmatch(rest); m != nil {
				stats.TotalDeps++
				if isGoVersionPseudoOrLatest(m[2]) {
					stats.MovingTargets++
				}
			}
			continue
		}
		if !inRequire {
			continue
		}
		if m := goModDep.FindStringSubmatch(trim); m != nil {
			stats.TotalDeps++
			if isGoVersionPseudoOrLatest(m[2]) {
				stats.MovingTargets++
			}
		}
	}
	return stats
}

// isGoVersionPseudoOrLatest returns true for pseudo-version pins
// (commit-based) or `latest` references. In go.mod, exact semver
// (`v1.2.3`) is considered pinned; pseudo-versions
// (`v0.0.0-20240101000000-abcdef`) indicate floating to trunk.
func isGoVersionPseudoOrLatest(v string) bool {
	if v == "" || v == "latest" {
		return true
	}
	return goPseudoVersion.MatchString(v)
}

// goPseudoVersion matches the standard Go pseudo-version form:
//   v<major>.<minor>.<patch>(-pre)?-<14-digit-utc>-<12-hex-hash>
// e.g. v0.0.0-20240101120000-abcdef123456
var goPseudoVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[\w.]+)?-\d{14}-[0-9a-f]{12}$`)

// --- Gemfile ---

var gemfileDep = regexp.MustCompile(`(?m)^\s*gem\s+["']([a-zA-Z0-9_\-]+)["']\s*(,\s*["']([^"']+)["'])?`)

func analyseGemfile(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "gemfile"}
	for _, m := range gemfileDep.FindAllStringSubmatch(string(data), -1) {
		stats.TotalDeps++
		spec := ""
		if len(m) > 3 {
			spec = strings.TrimSpace(m[3])
		}
		// Bare `gem 'foo'` or `gem 'foo', '~>1.0'` — the no-spec form is moving.
		if spec == "" {
			stats.MovingTargets++
			continue
		}
		// Only `=` and `==` are exact pins in Ruby Gemfile.
		if !strings.HasPrefix(spec, "=") {
			stats.MovingTargets++
		}
	}
	return stats
}

// itoa avoids strconv import in the hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := []byte{}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
