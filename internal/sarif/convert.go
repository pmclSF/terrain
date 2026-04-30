package sarif

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
)

const (
	sarifSchema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
	sarifVersion = "2.1.0"
	toolName     = "terrain"
	toolURI      = "https://github.com/pmclSF/terrain"
)

// Options controls SARIF emission.
type Options struct {
	// RedactPaths rewrites every artifact-location URI to be relative to
	// RepoRoot. Absolute paths that don't sit under RepoRoot, plus the
	// usual home-directory tilde expansions, are rewritten to bare
	// basenames so the SARIF can be safely posted to a public PR comment
	// or shared issue without leaking internal directory structure.
	RedactPaths bool

	// RepoRoot anchors path redaction. When empty and RedactPaths is set,
	// the current working directory is used.
	RepoRoot string
}

// FromAnalyzeReport converts an analyze.Report into a SARIF log using
// default emission options (no redaction).
func FromAnalyzeReport(r *analyze.Report, version string) *Log {
	return FromAnalyzeReportWithOptions(r, version, Options{})
}

// FromAnalyzeReportWithOptions converts an analyze.Report into a SARIF log,
// applying the supplied emission options.
func FromAnalyzeReportWithOptions(r *analyze.Report, version string, opts Options) *Log {
	rules := buildRules(r)
	results := buildResults(r, opts)

	return &Log{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []Run{{
			Tool: Tool{
				Driver: ToolComponent{
					Name:           toolName,
					Version:        version,
					InformationURI: toolURI,
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}
}

// redactPath converts an absolute path to a repo-relative path. Paths that
// don't sit beneath repoRoot are reduced to their basename so the SARIF
// artifact can be shared without leaking internal directory layout. The
// returned path always uses forward slashes for cross-OS portability and
// SARIF spec compliance.
func redactPath(p, repoRoot string) string {
	if p == "" {
		return p
	}
	clean := filepath.Clean(p)
	root := repoRoot
	if root == "" {
		root = "."
	}
	root = filepath.Clean(root)

	// If repoRoot is absolute, try to make the path relative to it.
	if filepath.IsAbs(clean) {
		if filepath.IsAbs(root) {
			if rel, err := filepath.Rel(root, clean); err == nil &&
				!strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel)
			}
		}
		// Outside the repo (or no repo root anchor): drop directory info
		// entirely, keep only the filename.
		return filepath.Base(clean)
	}
	// Already relative (or relative-ish). Normalise slashes.
	return filepath.ToSlash(clean)
}

// buildRules derives SARIF rules from the KeyFindings categories.
func buildRules(r *analyze.Report) []Rule {
	seen := map[string]bool{}
	var rules []Rule

	for _, kf := range r.KeyFindings {
		ruleID := ruleIDFromCategory(kf.Category)
		if seen[ruleID] {
			continue
		}
		seen[ruleID] = true
		rules = append(rules, Rule{
			ID:               ruleID,
			ShortDescription: Message{Text: ruleDescription(kf.Category)},
			DefaultConfig:    RuleConfig{Level: severityToLevel(kf.Severity)},
		})
	}

	return rules
}

// buildResults converts each KeyFinding into a SARIF Result, attaching
// file locations from WeakCoverageAreas where applicable.
func buildResults(r *analyze.Report, opts Options) []Result {
	// Build a location index from weak coverage areas, honouring redaction.
	var weakPaths []string
	for _, wa := range r.WeakCoverageAreas {
		p := wa.Path
		if opts.RedactPaths {
			p = redactPath(p, opts.RepoRoot)
		}
		weakPaths = append(weakPaths, p)
	}

	var results []Result

	for _, kf := range r.KeyFindings {
		result := Result{
			RuleID:  ruleIDFromCategory(kf.Category),
			Level:   severityToLevel(kf.Severity),
			Message: Message{Text: findingMessage(kf)},
		}

		// Attach file locations where we have them.
		switch kf.Category {
		case "coverage_debt":
			for _, p := range weakPaths {
				result.Locations = append(result.Locations, Location{
					PhysicalLocation: PhysicalLocation{
						ArtifactLocation: ArtifactLocation{URI: p},
					},
				})
			}
		}

		results = append(results, result)
	}

	return results
}

func ruleIDFromCategory(category string) string {
	switch category {
	case "optimization":
		return "terrain/duplicate-tests"
	case "architecture_debt":
		return "terrain/high-fanout"
	case "coverage_debt":
		return "terrain/weak-coverage"
	case "reliability":
		return "terrain/reliability"
	default:
		return "terrain/" + strings.ReplaceAll(category, "_", "-")
	}
}

func ruleDescription(category string) string {
	switch category {
	case "optimization":
		return "Duplicate or redundant tests detected"
	case "architecture_debt":
		return "High fan-out fixtures creating fragile dependencies"
	case "coverage_debt":
		return "Source areas with weak or missing test coverage"
	case "reliability":
		return "Test reliability issues detected"
	default:
		return "Test system finding"
	}
}

func severityToLevel(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	case "low":
		return "note"
	default:
		return "note"
	}
}

func findingMessage(kf analyze.KeyFinding) string {
	if kf.Metric != "" {
		return fmt.Sprintf("%s (%s)", kf.Title, kf.Metric)
	}
	return kf.Title
}
