package sarif

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/models"
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

// buildRules emits SARIF rules. Two sources combine to cover both the
// curated top-N KeyFindings (legacy category-derived IDs kept for
// continuity with existing SARIF consumers) and every per-detector
// signal in Report.Signals (canonical manifest RuleIDs like
// `terrain/hygiene/secrets-in-prompt`). Adopter SARIF tooling can
// suppress on either; new adopters should prefer the canonical
// per-detector form.
func buildRules(r *analyze.Report) []Rule {
	seen := map[string]bool{}
	var rules []Rule

	// Per-detector canonical rules from Report.Signals.
	for _, fr := range r.Signals {
		if fr.RuleID == "" || seen[fr.RuleID] {
			continue
		}
		seen[fr.RuleID] = true
		desc := fr.Evidence
		if desc == "" {
			desc = fr.Type
		}
		rules = append(rules, Rule{
			ID:               fr.RuleID,
			ShortDescription: Message{Text: desc},
			DefaultConfig:    RuleConfig{Level: severityToLevel(fr.Severity)},
			HelpURI:          canonicalRuleHelpURI(fr.RuleID),
			Properties:       pillarProperties(models.PillarFor(models.SignalCategory(fr.Category))),
		})
	}

	// Legacy category-derived rules from KeyFindings.
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
			HelpURI:          ruleHelpURI(ruleID),
			Properties:       pillarProperties(kf.Pillar),
		})
	}

	return rules
}

// canonicalRuleHelpURI builds the canonical docs URL for a manifest
// RuleID by mapping `terrain/<category>/<rule>` to the rule-doc path.
func canonicalRuleHelpURI(ruleID string) string {
	const docBase = "https://github.com/pmclSF/terrain/blob/main/docs/rules/"
	stripped := strings.TrimPrefix(ruleID, "terrain/")
	if stripped == "" || stripped == ruleID {
		return ""
	}
	return docBase + stripped + ".md"
}

// pillarProperties returns the SARIF properties bag carrying the
// "terrain:<pillar>" tag for a given pillar string. Returns nil when
// the pillar is empty so we don't emit empty properties bags.
func pillarProperties(pillar string) *Properties {
	if pillar == "" {
		return nil
	}
	return &Properties{Tags: []string{"terrain:" + pillar}}
}

// ruleHelpURI maps a legacy category-derived rule ID to its
// documentation page. Returns the canonical GitHub URL when known so
// SARIF consumers can open it directly. Empty string when the rule
// has no rendered docs page yet.
func ruleHelpURI(ruleID string) string {
	const docBase = "https://github.com/pmclSF/terrain/blob/main/docs/rules/"
	switch ruleID {
	case "terrain/duplicate-tests":
		return docBase + "quality/snapshot-heavy-test.md"
	case "terrain/high-fanout":
		return docBase + "structural/blast-radius-hotspot.md"
	case "terrain/weak-coverage":
		return docBase + "coverage/coverage-blind-spot.md"
	case "terrain/reliability":
		return docBase + "health/flaky-test.md"
	}
	return ""
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

	// Emit one SARIF result per Report.Signals entry using canonical
	// manifest RuleIDs. Adopter SARIF tooling (GitHub code-scanning,
	// security dashboards) sees the same rule IDs documented in
	// docs/rules/** and suppressed via `terrain suppress`.
	for _, fr := range r.Signals {
		if fr.RuleID == "" {
			continue
		}
		result := Result{
			RuleID:     fr.RuleID,
			Level:      severityToLevel(fr.Severity),
			Message:    Message{Text: signalMessage(fr)},
			Properties: pillarProperties(models.PillarFor(models.SignalCategory(fr.Category))),
		}
		if fr.File != "" {
			path := fr.File
			if opts.RedactPaths {
				path = redactPath(path, opts.RepoRoot)
			}
			loc := Location{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{URI: path},
				},
			}
			if fr.Line > 0 {
				loc.PhysicalLocation.Region = &Region{StartLine: fr.Line}
			}
			result.Locations = append(result.Locations, loc)
		}
		results = append(results, result)
	}

	// Legacy KeyFindings path. Kept so SARIF consumers that aggregated
	// on the category-derived IDs continue to work during the
	// transition; new consumers should index on the canonical IDs above.
	for _, kf := range r.KeyFindings {
		result := Result{
			RuleID:     ruleIDFromCategory(kf.Category),
			Level:      severityToLevel(kf.Severity),
			Message:    Message{Text: findingMessage(kf)},
			Properties: pillarProperties(kf.Pillar),
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

// signalMessage produces the SARIF result message for a per-signal
// finding. Prefers Evidence when available, falls back to the type.
func signalMessage(fr analyze.FindingRecord) string {
	if fr.Evidence != "" {
		return fr.Evidence
	}
	return fmt.Sprintf("%s finding", fr.Type)
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
