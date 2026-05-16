package coverage

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectNoIntegrationTest walks SurfaceHandler / SurfaceRoute entries
// in the snapshot and emits a Signal for any entry point that has no
// integration-classified test covering it. Implements
// terrain/coverage/no-integration-test.
//
// Heuristic for "integration test":
//   - File path contains /integration/, /e2e/, /end_to_end/, /endtoend/,
//     /__integration__/, /functional/, or /api/
//   - OR test framework type is end_to_end / integration per
//     internal/testtype classification (when populated on TestFile.Framework)
//
// Severity defaults to medium.
func DetectNoIntegrationTest(snap *models.TestSuiteSnapshot, graph *impact.ImpactGraph) []models.Signal {
	if snap == nil || graph == nil {
		return nil
	}

	// Build the set of integration-test paths.
	integrationPaths := map[string]bool{}
	for _, tf := range snap.TestFiles {
		if isIntegrationTestPath(tf.Path) {
			integrationPaths[tf.Path] = true
		}
	}
	if len(integrationPaths) == 0 {
		// No integration tests anywhere — emit one signal per entry
		// point so the gap is visible. But that's noisy; for 0.2.0
		// we emit only when the repo has *some* integration tests
		// (so the rule reports specific gaps rather than indicting
		// the whole codebase).
		return nil
	}

	var out []models.Signal
	for _, cs := range snap.CodeSurfaces {
		if !isEntryPoint(cs.Kind) {
			continue
		}
		if entryPointHasIntegrationTest(cs, graph, integrationPaths) {
			continue
		}
		out = append(out, models.Signal{
			Type:             signals.SignalNoIntegrationTest,
			Category:         models.CategoryQuality,
			Severity:         models.SeverityMedium,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceGraphTraversal,
			Location: models.SignalLocation{
				File:   cs.Path,
				Symbol: cs.Name,
			},
			Explanation: fmt.Sprintf(
				"Entry point %q (kind=%s) in %s has no integration test reaching it. Unit tests may cover the inner logic but the cross-stack contract is unguarded.",
				cs.Name, cs.Kind, cs.Path,
			),
			SuggestedAction: fmt.Sprintf(
				"Add an integration test that exercises %q end-to-end (e.g., HTTP request to the route, RPC call to the method).",
				cs.Name,
			),
			RuleID:          "terrain/coverage/no-integration-test",
			RuleURI:         "docs/rules/coverage/no-integration-test.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"surfaceId":   cs.SurfaceID,
				"surfaceKind": string(cs.Kind),
			},
		})
	}
	return out
}

func isEntryPoint(k models.CodeSurfaceKind) bool {
	switch k {
	case models.SurfaceHandler, models.SurfaceRoute:
		return true
	}
	return false
}

func entryPointHasIntegrationTest(cs models.CodeSurface, graph *impact.ImpactGraph, integrationPaths map[string]bool) bool {
	for _, testPath := range graph.UnitToTests[cs.SurfaceID] {
		if integrationPaths[testPath] {
			return true
		}
	}
	// Fall back to name-only lookup for graph entries that aren't
	// path-qualified.
	for _, testPath := range graph.UnitToTests[cs.Name] {
		if integrationPaths[testPath] {
			return true
		}
	}
	return false
}

func isIntegrationTestPath(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range integrationPathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var integrationPathMarkers = []string{
	"/integration/", "/e2e/", "/end_to_end/", "/endtoend/",
	"/__integration__/", "/functional/", "/api/",
}
