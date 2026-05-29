// Package coverage implements the coverage/* stable rules. Each
// detector consumes the typed ImpactGraph (Tier 0) and emits Signals
// for surfaces or units with no covering test / eval.
package coverage

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectNoTestsForCodeUnit walks every CodeUnit in the snapshot and
// emits a Signal for any unit with no incoming `covered_by_test` edge
// in the ImpactGraph. Implements terrain/coverage/no-tests.
//
// Filters applied:
//   - generated / vendored / dist paths excluded by path prefix
//   - private / unexported units excluded by default (the rule fires
//     on public surface only; an adopter can change this in
//     terrain.yaml via include_private)
//   - trivial-name units (`init`, `main`) excluded
//
// Severity defaults to medium per the rule's catalog entry.
func DetectNoTestsForCodeUnit(snap *models.TestSuiteSnapshot, graph *impact.ImpactGraph) []models.Signal {
	if snap == nil || graph == nil {
		return nil
	}

	// Build the set of unit IDs that have any covering test.
	covered := make(map[string]bool, len(graph.UnitToTests))
	for unitID, tests := range graph.UnitToTests {
		if len(tests) > 0 {
			covered[unitID] = true
		}
	}

	var out []models.Signal
	for _, cu := range snap.CodeUnits {
		if shouldSkipUnit(cu) {
			continue
		}
		unitID := cu.Path + ":" + cu.Name
		if covered[unitID] {
			continue
		}
		// Also try the name-only key in case the graph stored a less
		// qualified source ID. Coverage attribution heuristics in
		// internal/impact/ sometimes emit name-only IDs for units that
		// can't be path-qualified at link time.
		if covered[cu.Name] {
			continue
		}

		out = append(out, models.Signal{
			Type:             signals.SignalNoTestsForCodeUnit,
			Category:         models.CategoryQuality,
			Severity:         models.SeverityMedium,
			Confidence:       0.9,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location: models.SignalLocation{
				File:   cu.Path,
				Symbol: cu.Name,
			},
			Explanation: fmt.Sprintf(
				"Code unit %q in %s has no test that imports or references it. Untested code reaches production undetected when changed.",
				cu.Name, cu.Path,
			),
			SuggestedAction: fmt.Sprintf(
				"Add a test that imports %q and exercises its observable behavior.",
				cu.Name,
			),
			RuleID:          "terrain/coverage/no-tests",
			RuleURI:         "docs/rules/coverage/no-tests.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"codeUnit": unitID,
				"kind":     string(cu.Kind),
				"exported": cu.Exported,
			},
		})
	}
	return out
}

// shouldSkipUnit returns true for units the rule shouldn't fire on at
// all (generated code, trivial entry points, non-exported items by
// default). Mirrors the §15 doc-page edge-case list.
func shouldSkipUnit(cu models.CodeUnit) bool {
	switch cu.Name {
	case "init", "main", "_", "__init__":
		return true
	}
	// Path-based exclusion: generated code, vendored deps, dist dirs.
	// Match either as a path prefix (vendor/...) or anywhere in the
	// path (src/__generated__/...).
	for _, prefix := range generatedPathPrefixes {
		if strings.HasPrefix(cu.Path, prefix) {
			return true
		}
	}
	for _, marker := range generatedPathMarkers {
		if strings.Contains(cu.Path, marker) {
			return true
		}
	}
	// Default-skip unexported units. Adopters can opt in via
	// terrain.yaml `rules.coverage/no-tests.include_private: true`,
	// but rule-level config wiring lives at the policy layer; the
	// detector's default stays exported-only.
	if !cu.Exported {
		return true
	}
	return false
}

// generatedPathPrefixes are vendor / build dirs typically at the
// repo root; matched as path prefix so the leading `/` isn't required.
var generatedPathPrefixes = []string{
	"vendor/",
	"node_modules/",
	"dist/",
	"build/",
	".terrain/",
}

// generatedPathMarkers are substrings that indicate generated content
// regardless of position in the path.
var generatedPathMarkers = []string{
	"/__generated__/",
	"/__pycache__/",
	".gen.go",
	".pb.go",
}
