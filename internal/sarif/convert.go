package sarif

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
)

const (
	sarifSchema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
	sarifVersion = "2.1.0"
	toolName     = "terrain"
	toolURI      = "https://github.com/pmclSF/terrain"
)

// FromAnalyzeReport converts an analyze.Report into a SARIF log.
func FromAnalyzeReport(r *analyze.Report, version string) *Log {
	rules := buildRules(r)
	results := buildResults(r)

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
func buildResults(r *analyze.Report) []Result {
	// Build a location index from weak coverage areas.
	var weakPaths []string
	for _, wa := range r.WeakCoverageAreas {
		weakPaths = append(weakPaths, wa.Path)
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
