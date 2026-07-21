// Package deps implements Terrain's dependency-manifest drift-risk
// detector.
//
// It walks the repository for manifest files (package.json,
// requirements.txt, Cargo.toml, go.mod, etc.), parses each ecosystem's
// version specifiers, and computes the share of a manifest's
// dependencies that use "moving-target" specs (e.g. `*`, `latest`,
// unbounded `^`/`~` ranges, untagged Git refs). It fires one finding
// per manifest whose moving-target share exceeds a threshold
// (default 40%).
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
//
// Contract:
//
//	D1.   Fires when totalDeps>=3 AND movingTargetShare>=0.40.
//	D2.   Severity: share>0.75 High, 0.50–0.75 Medium, 0.40–0.50 Low.
//	D3-8. Per-ecosystem classification — npm (^/~/*/latest/ranges/VCS moving;
//	      only exact `1.2.3` pinned), pip, pyproject (Poetry tuple + PEP 621
//	      array), cargo
//	      (bare-semver moving), go.mod (pseudo-versions moving), gemfile.
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
				"Unpinned specs let an install resolve to a different version than was tested, so a dependency bump can change behavior with no manifest edit or review.",
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

// maxManifestSize caps the bytes read per manifest (256 KB). A real
// package.json / requirements.txt / pyproject.toml is a few KB; a larger one
// is generated or hostile. The cap plus the regular-file check stop a manifest
// symlinked to /dev/zero or a huge file from growing memory unbounded.
const maxManifestSize = 256 * 1024

func analyseManifest(path string) (manifestStats, bool) {
	// Lstat (not Stat) rejects a symlink on its own type without following it,
	// so a manifest symlinked to a device or huge file never reaches ReadFile.
	if fi, statErr := os.Lstat(path); statErr != nil || !fi.Mode().IsRegular() || fi.Size() > maxManifestSize {
		return manifestStats{}, false
	}
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
// no upper bound, `latest`, `*`, plain Git refs, etc. Only an exact
// pin (`1.2.3`) is NOT a moving target; caret (`^1.2.3`) and tilde
// (`~1.2.3`) ranges both float and are treated as moving.
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
//   - `~1.2.3`          → MOVING (allows patch bumps)
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
		// Strip the environment marker / URL clause so classification
		// runs on the requirement portion only. Otherwise a bare
		// unpinned package like `requests; sys_platform == "linux"`
		// would be misread (the marker's operators leak into the
		// range check, or its absence of `<`/`>` hides a bare name).
		line = stripPipRequirementSuffix(line)
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

// pyprojectDep matches Poetry-style `name = "spec"` tuple lines inside
// [tool.poetry.dependencies] blocks. Very rough; full TOML parsing is
// overkill for the drift heuristic.
var pyprojectDep = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\-]+)\s*=\s*"([^"]*)"`)

// pyprojectArrayHeader matches the opening of a PEP 621
// `dependencies = [` list (both the top-level `[project]` dependencies
// and each `[project.optional-dependencies]` group whose value is an
// array of requirement strings), so the quoted requirements inside can
// be scanned. Non-dependency arrays (classifiers, keywords, authors)
// are excluded by entry-level filtering below.
var pyprojectArrayHeader = regexp.MustCompile(`(?m)^\s*(?:dependencies|[a-zA-Z0-9_\-]+)\s*=\s*\[`)

// pyprojectArrayEntry matches a single quoted requirement string.
var pyprojectArrayEntry = regexp.MustCompile(`"([^"]+)"`)

func analysePyProject(data []byte) manifestStats {
	stats := manifestStats{Ecosystem: "pyproject"}
	src := string(data)
	// Poetry tuple-style: name = "spec".
	for _, m := range pyprojectDep.FindAllStringSubmatch(src, -1) {
		name := strings.TrimSpace(m[1])
		// The Python interpreter constraint is not a package and cannot be
		// pinned like one; excluding it prevents inflating the moving-target
		// share and the reported movingTargetDeps count.
		if name == "python" || name == "requires-python" {
			continue
		}
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
	// PEP 621 array-style: dependencies = ["requests>=2.0", "numpy", ...]
	// and [project.optional-dependencies] groups. Scan each `key = [ ... ]`
	// span and classify every quoted requirement string with the pip rules.
	for _, span := range pyprojectArraySpans(src) {
		for _, m := range pyprojectArrayEntry.FindAllStringSubmatch(span, -1) {
			req := strings.TrimSpace(m[1])
			if !looksLikePyProjectRequirement(req) {
				continue
			}
			// Ignore the interpreter constraint if it appears here.
			if strings.HasPrefix(req, "python") && looksLikeDepSpec(strings.TrimPrefix(req, "python")) {
				continue
			}
			stats.TotalDeps++
			if classifyPipRequirementMoving(req) {
				stats.MovingTargets++
			}
		}
	}
	return stats
}

// pyprojectArraySpans returns the text inside each `key = [ ... ]` block
// found in src, so array-style dependency lists can be scanned for
// quoted requirement strings.
func pyprojectArraySpans(src string) []string {
	var spans []string
	for _, loc := range pyprojectArrayHeader.FindAllStringIndex(src, -1) {
		// loc[1] points just past the opening `[`; find the matching `]`.
		rest := src[loc[1]:]
		if end := strings.IndexByte(rest, ']'); end >= 0 {
			spans = append(spans, rest[:end])
		}
	}
	return spans
}

// classifyPipRequirementMoving reports whether a pip-style requirement
// string (e.g. `requests>=2.0`, `numpy`, `flask==2.0.1`) resolves to a
// moving target, using the same rules as analysePipRequirements.
func classifyPipRequirementMoving(req string) bool {
	req = stripPipRequirementSuffix(req)
	// Exact pin (`pkg==1.2.3`) is NOT moving.
	if pipExactPin.MatchString(req) {
		return false
	}
	// No specifier at all (`pkg`) IS moving.
	if pipMovingTarget.MatchString(req) {
		return true
	}
	// Loose range without exact (`pkg>=1.0`) IS moving.
	if pipLooseRange.MatchString(req) {
		return true
	}
	return false
}

// stripPipRequirementSuffix removes a PEP 508 environment marker
// (`; python_version < "3.8"`) and a direct-URL reference
// (`pkg @ https://…`) from a requirement, returning just the
// name+specifier portion so drift classification isn't confused by
// operators inside the marker or URL.
func stripPipRequirementSuffix(req string) string {
	if i := strings.Index(req, ";"); i >= 0 {
		req = req[:i]
	}
	if i := strings.Index(req, "@"); i >= 0 {
		req = req[:i]
	}
	return strings.TrimSpace(req)
}

func looksLikeDepSpec(s string) bool {
	return strings.ContainsAny(s, "<>=~^!*") || s == "*" || s == ""
}

// pyProjectReqName matches the leading package-name token of a PEP 621
// requirement string (letters/digits, then name characters).
var pyProjectReqName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-]*`)

// looksLikePyProjectRequirement reports whether a quoted array entry is
// plausibly a PEP 621 dependency requirement rather than a classifier
// trove string, keyword, or author entry. Requirements begin with a
// package name and never contain the `::` trove separator.
func looksLikePyProjectRequirement(s string) bool {
	if s == "" || strings.Contains(s, "::") {
		return false
	}
	return pyProjectReqName.MatchString(s)
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
//
//	v<major>.<minor>.<patch>(-pre)?-<14-digit-utc>-<12-hex-hash>
//
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
