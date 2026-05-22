// Package configdrift implements Terrain's detector for brittle infra-
// config patterns. See docs/PRODUCT.md §16 for product context.
//
// Empirical motivation: a meaningful share of unflagged regression-
// introducing PRs in our internal calibration corpus are "config-only"
// edits (YAML / TOML / JSON / INI / TF) that no existing detector
// covers.
//
// v1 design — analyse-time, not PR-diff-time. Walks tracked-config
// files and flags structural patterns that are forward-compat hazards:
//
//   - GitHub Actions workflows pinning third-party actions to mutable
//     refs (`@main`, `@master`, untagged branches).
//   - docker-compose / k8s / Helm using `:latest` or empty image tags.
//   - k8s manifests referencing deprecated `apiVersion`s.
//
// v2 (post-0.2.0): consume PR-diff context to flag actual schema
// changes (key renames / removals / type changes).
package configdrift

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// SchemaDriftDetector emits SignalConfigSchemaDrift for each tracked
// config file that exhibits forward-compat hazards.
type SchemaDriftDetector struct {
	Root string
}

// Threshold: a single high-confidence hazard fires; multiple stack.

// Detect walks the repo for tracked configs and emits one finding per
// file with hazards. Multiple hazards on the same file collapse into
// one finding listing all of them in Metadata.
func (d *SchemaDriftDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
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
		findings := classifyConfig(d.Root, p)
		if findings == nil {
			return nil
		}
		out = append(out, *findings)
		return nil
	})
	return out
}

// configKind classifies a path into the family of config the file
// belongs to. Empty string means "not a tracked config".
func configKind(repoRoot, p string) string {
	rel, err := filepath.Rel(repoRoot, p)
	if err != nil {
		rel = p
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)

	// GitHub Actions workflow.
	if strings.HasPrefix(rel, ".github/workflows/") &&
		(strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) {
		return "gh-actions"
	}
	// docker-compose.
	if base == "docker-compose.yml" || base == "docker-compose.yaml" ||
		strings.HasPrefix(base, "docker-compose.") &&
			(strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) {
		return "docker-compose"
	}
	// Helm values.
	if base == "values.yaml" || strings.HasPrefix(base, "values.") &&
		(strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) {
		return "helm-values"
	}
	// Kubernetes manifests under common paths. Match both "/k8s/" segments
	// and a leading "k8s/" directory at the repo root.
	if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
		segs := "/" + rel
		if strings.Contains(segs, "/k8s/") || strings.Contains(segs, "/kubernetes/") ||
			strings.Contains(segs, "/manifests/") || strings.HasPrefix(rel, "deploy/") {
			return "k8s-manifest"
		}
	}
	return ""
}

// hazardScan walks the lines of a config and returns a per-file hazard
// list. Each hazard is a short tag describing the pattern, suitable
// for the finding's Metadata.
func classifyConfig(repoRoot, p string) *models.Signal {
	kind := configKind(repoRoot, p)
	if kind == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var hazards []string
	switch kind {
	case "gh-actions":
		hazards = scanGitHubActions(data)
	case "docker-compose":
		hazards = scanDockerCompose(data)
	case "helm-values", "k8s-manifest":
		hazards = scanKubernetes(data)
	}
	if len(hazards) == 0 {
		return nil
	}
	rel, _ := filepath.Rel(repoRoot, p)
	severity := models.SeverityMedium
	if len(hazards) >= 3 {
		severity = models.SeverityHigh
	} else if len(hazards) == 1 {
		severity = models.SeverityLow
	}
	// configSchemaDrift findings on `examples/` paths are NOT demoted
	// by the live-vs-catalog classifier. Docker-compose / k8s / helm
	// files under `examples/` are typically intended to be deployable,
	// and `:latest` tags in them are forward-compat hazards regardless
	// of path.
	return &models.Signal{
		Type:             signals.SignalConfigSchemaDrift,
		Category:         models.CategoryQuality,
		Severity:         severity,
		Confidence:       0.65,
		EvidenceStrength: models.EvidenceModerate,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			File: rel,
		},
		Explanation: "Config `" + rel + "` uses " + itoa(len(hazards)) +
			" forward-compat-hazard pattern(s) (e.g. `:latest`, untagged action ref, deprecated apiVersion).",
		SuggestedAction: "Pin image tags, action refs, and apiVersions to concrete versions. Add a CI check that fails on `:latest` and floating action refs.",
		Metadata: map[string]any{
			"kind":         kind,
			"hazards":      hazards,
			"hazard_count": len(hazards),
		},
		RuleID:  "terrain/config/schema-drift",
		RuleURI: "docs/rules/config/schema-drift.md",
	}
}

// --- GitHub Actions ---

// ghActionUse captures `uses: org/repo@ref` (with optional path).
// Accepts both list-item form (`- uses: …`) and bare-key form (`uses: …`).
var ghActionUse = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*([^\s#]+)`)

// ghMutableRef matches refs that are not pinned (branch names or
// missing version).
var ghMutableRef = regexp.MustCompile(`@(?:main|master|latest|dev|develop|HEAD)$`)

// ghPinnedSHA matches a 40-char hex SHA (the recommended pin form).
var ghPinnedSHA = regexp.MustCompile(`@[0-9a-f]{40}$`)

// ghPinnedTag matches @v<digit>... (e.g. @v3, @v4.1.7).
var ghPinnedTag = regexp.MustCompile(`@v?\d`)

func scanGitHubActions(data []byte) []string {
	var hazards []string
	mutableCount := 0
	for _, m := range ghActionUse.FindAllStringSubmatch(string(data), -1) {
		ref := strings.TrimSpace(m[1])
		// Local action refs (`./.github/actions/foo`) are not version-pinned
		// in the SHA sense; skip them.
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
			continue
		}
		if !strings.Contains(ref, "@") {
			mutableCount++
			continue
		}
		if ghMutableRef.MatchString(ref) {
			mutableCount++
			continue
		}
		if ghPinnedSHA.MatchString(ref) || ghPinnedTag.MatchString(ref) {
			continue
		}
		// Anything else (e.g. @feature-branch) is suspicious.
		mutableCount++
	}
	if mutableCount > 0 {
		hazards = append(hazards, "gh-actions:mutable-ref")
	}
	return hazards
}

// --- docker-compose ---

var imageLatest = regexp.MustCompile(`(?m)^\s*image:\s*([^\s#]+)`)

func scanDockerCompose(data []byte) []string {
	var hazards []string
	latestCount := 0
	untaggedCount := 0
	for _, m := range imageLatest.FindAllStringSubmatch(string(data), -1) {
		img := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		if img == "" {
			continue
		}
		if strings.HasSuffix(img, ":latest") || strings.HasSuffix(img, ":dev") {
			latestCount++
			continue
		}
		// No tag at all → defaults to :latest.
		// Allow registry/host with port (`localhost:5000/img`) by checking
		// the last segment for a `:`.
		lastSeg := img
		if i := strings.LastIndex(img, "/"); i >= 0 {
			lastSeg = img[i+1:]
		}
		if !strings.Contains(lastSeg, ":") {
			untaggedCount++
		}
	}
	if latestCount > 0 {
		hazards = append(hazards, "docker:latest-tag")
	}
	if untaggedCount > 0 {
		hazards = append(hazards, "docker:untagged-image")
	}
	// docker-compose version: '2' is deprecated.
	if hasLine(data, "version: '2'") || hasLine(data, `version: "2"`) {
		hazards = append(hazards, "docker:compose-v2-schema")
	}
	return hazards
}

// --- Kubernetes / Helm values ---

var apiVersionLine = regexp.MustCompile(`(?m)^\s*apiVersion:\s*([^\s#]+)`)

var deprecatedAPIs = map[string]bool{
	"extensions/v1beta1":     true,
	"apps/v1beta1":           true,
	"apps/v1beta2":           true,
	"batch/v1beta1":          true,
	"policy/v1beta1":         true,
	"networking.k8s.io/v1beta1": true,
	"rbac.authorization.k8s.io/v1beta1": true,
}

func scanKubernetes(data []byte) []string {
	var hazards []string
	deprecatedCount := 0
	for _, m := range apiVersionLine.FindAllStringSubmatch(string(data), -1) {
		api := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		if deprecatedAPIs[api] {
			deprecatedCount++
		}
	}
	if deprecatedCount > 0 {
		hazards = append(hazards, "k8s:deprecated-apiversion")
	}
	// Reuse image-tag scan — helm values & k8s manifests both use `image:`.
	hazards = append(hazards, scanDockerCompose(data)...)
	return uniqueStrings(hazards)
}

// --- helpers ---

func hasLine(data []byte, needle string) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == needle {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

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
