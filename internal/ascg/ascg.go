// Package ascg is the Anchored Schema Configuration Gate. It classifies
// occurrences of config-shaped strings (model names, embedding dims, seeds,
// API keys, ...) as either:
//
//   - Live: the value is plausibly read at runtime to drive behavior.
//   - CatalogOrExample: the value is listed for reference (catalog of options,
//     example snippet, docstring, doc page) and never actually consumed.
//   - Unknown: no structural signal one way or the other.
//
// The classifier exists because detectors that flag config-shaped strings
// (e.g. aiModelDeprecationRisk, aiNonDeterministicEval) FP heavily on
// catalogs, docstring examples, and fixture data. ASCG is the structural
// gate that demotes those occurrences before they become findings.
//
// Phase 1 scope: structural definition + string-context classifier. The
// classifier consumes:
//
//   - The file path (for path-based catalog/fixture signals).
//   - The surrounding context (docstring, comment, list-of-options shape).
//   - An optional ReachedByLoader hint, set upstream when the cross-language
//     graph reports the file is on a known config-loader path.
//
// Phase 2 wires this into the detector pipeline; consumer detectors call
// Classify(location) and demote findings whose classification is
// CatalogOrExample.
//
// The classifier is intentionally conservative: when no signal applies the
// result is Unknown, NOT Live. Detectors decide their own policy for
// Unknown — most opt to keep firing.
package ascg

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Classification is the structural verdict for one occurrence.
type Classification int

const (
	// Unknown means no structural signal applied. Detectors decide their own
	// policy: most keep firing on Unknown.
	Unknown Classification = iota

	// Live means at least one signal indicates the value is consumed at
	// runtime (reached by a config loader, on a known config-file path).
	Live

	// CatalogOrExample means at least one signal indicates the value is
	// listed for reference (catalog file, fixture path, docstring,
	// list-of-options shape). Detectors typically demote findings with this
	// classification to NOTE severity or drop them entirely.
	CatalogOrExample
)

func (c Classification) String() string {
	switch c {
	case Live:
		return "live"
	case CatalogOrExample:
		return "catalog_or_example"
	default:
		return "unknown"
	}
}

// Location describes one occurrence of a config-shaped string. All fields
// are optional except Path; classification degrades to Unknown when fields
// are absent.
type Location struct {
	// Path is the project-relative file path. Always required.
	Path string

	// Line is the 1-indexed line number in Path. Used for tie-breaks but not
	// load-bearing for any individual signal.
	Line int

	// InDocstring is true when upstream syntactic analysis has determined
	// the occurrence sits inside a docstring (Python triple-quoted string at
	// module/function scope, JS /** */ block, Go block-comment at top of
	// file/function).
	InDocstring bool

	// InComment is true when the occurrence sits inside a line or block
	// comment.
	InComment bool

	// InCatalogList is true when upstream syntactic analysis has determined
	// the occurrence is one element of a list/dict literal of ≥3 sibling
	// values bound to a SCREAMING_SNAKE_CASE or PascalCase identifier
	// (SUPPORTED_MODELS, ALLOWED_PROVIDERS, AVAILABLE_BACKENDS, ...).
	InCatalogList bool

	// ReachedByLoader is true when the cross-language graph has determined
	// the file is reachable from a known config-loader call site
	// (yaml.safe_load, json.load, dotenv.load_dotenv, pydantic.BaseSettings,
	// ConfigParser, hydra @hydra.main, os.getenv, ...).
	ReachedByLoader bool
}

// Result is the classifier verdict + the reasons it reached that verdict.
// Reasons are useful for diagnostic emission ("demoted: catalog-list shape
// in CHANGELOG.md").
type Result struct {
	Class   Classification
	Reasons []string
}

// Classify returns the structural verdict for the given location.
//
// Precedence: CatalogOrExample signals win over Live signals when both fire.
// Rationale: a string that lives in a docs/ markdown page is overwhelmingly
// likely to be illustrative even if the file is also reached by a loader on
// some unrelated code path. The cost of an FP (noisy finding) outweighs the
// cost of an FN (one fewer signal in a doc page).
func Classify(loc Location) Result {
	var (
		catalogReasons []string
		liveReasons    []string
	)

	if r, ok := catalogPathSignal(loc.Path); ok {
		catalogReasons = append(catalogReasons, r)
	}
	if r, ok := fixturePathSignal(loc.Path); ok {
		catalogReasons = append(catalogReasons, r)
	}
	if loc.InDocstring {
		catalogReasons = append(catalogReasons, "in_docstring")
	}
	if loc.InComment {
		catalogReasons = append(catalogReasons, "in_comment")
	}
	if loc.InCatalogList {
		catalogReasons = append(catalogReasons, "in_catalog_list")
	}

	if r, ok := liveConfigPathSignal(loc.Path); ok {
		liveReasons = append(liveReasons, r)
	}
	if loc.ReachedByLoader {
		liveReasons = append(liveReasons, "reached_by_loader")
	}

	switch {
	case len(catalogReasons) > 0:
		return Result{Class: CatalogOrExample, Reasons: catalogReasons}
	case len(liveReasons) > 0:
		return Result{Class: Live, Reasons: liveReasons}
	default:
		return Result{Class: Unknown}
	}
}

// catalogPathRegex matches path segments where occurrences are almost always
// illustrative: docs, examples, demos, cookbooks, notebooks.
var catalogPathRegex = regexp.MustCompile(
	`(^|/)(docs?|examples?|samples?|demos?|cookbooks?|notebooks?|tutorials?)(/|$)`,
)

// catalogFileExts are file extensions that almost always carry documentary
// content: markdown, rst, jupyter notebooks, plain-text docs.
var catalogFileExts = map[string]bool{
	".md":    true,
	".mdx":   true,
	".rst":   true,
	".txt":   true,
	".ipynb": true,
	".adoc":  true,
}

func catalogPathSignal(path string) (string, bool) {
	p := filepath.ToSlash(path)
	ext := strings.ToLower(filepath.Ext(p))
	if catalogFileExts[ext] {
		return "catalog_file_extension:" + ext, true
	}
	if catalogPathRegex.MatchString(p) {
		return "catalog_path_segment", true
	}
	return "", false
}

// fixturePathRegex matches path segments where occurrences are test fixtures
// or recorded data, not live config.
var fixturePathRegex = regexp.MustCompile(
	`(^|/)(fixtures?|testdata|test_fixtures|__fixtures__|__snapshots__|golden|recordings?|cassettes)(/|$)`,
)

func fixturePathSignal(path string) (string, bool) {
	if fixturePathRegex.MatchString(filepath.ToSlash(path)) {
		return "fixture_path_segment", true
	}
	return "", false
}

// liveConfigFileBasenames are filenames that are almost always live config
// when present at standard locations.
var liveConfigFileBasenames = map[string]bool{
	".env":            true,
	"settings.py":     true,
	"settings.yaml":   true,
	"settings.yml":    true,
	"config.yaml":     true,
	"config.yml":      true,
	"config.toml":     true,
	"config.json":     true,
	"pyproject.toml":  true,
	"setup.cfg":       true,
	"appsettings.json": true,
}

// liveConfigBasenameSuffixes catches dotenv variants and per-environment
// config files like ".env.production" or "config.prod.yaml".
var liveConfigBasenameSuffixes = []string{
	".env.",
	"settings.",
	"config.",
}

// liveConfigSegmentRegex matches path segments that indicate live config.
var liveConfigSegmentRegex = regexp.MustCompile(
	`(^|/)(conf|configs?|env|environments?)(/|$)`,
)

func liveConfigPathSignal(path string) (string, bool) {
	p := filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(p))

	if liveConfigFileBasenames[base] {
		return "live_config_basename:" + base, true
	}
	for _, prefix := range liveConfigBasenameSuffixes {
		if strings.HasPrefix(base, prefix) {
			return "live_config_basename_prefix:" + prefix, true
		}
	}
	if liveConfigSegmentRegex.MatchString(p) {
		return "live_config_path_segment", true
	}
	return "", false
}
