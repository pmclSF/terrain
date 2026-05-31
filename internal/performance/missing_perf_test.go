// Package performance implements the performance/* stable rules.
package performance

import (
	"fmt"
	"strings"

	"github.com/pmclSF/terrain/internal/impact"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectMissingPerfTest walks AI surfaces flagged as latency-critical
// and emits a Signal for any without a benchmark / load-test reaching
// them. Implements terrain/performance/missing-perf-test.
//
// "Latency-critical" at 0.2.0 = any AI surface; future versions
// narrow to surfaces explicitly tagged in terrain.yaml. The
// conservative default is to flag every uncovered surface so adopters
// see the gap; they can ignore via path.
//
// "Benchmark / load test" = test file whose path contains /bench/,
// /benchmarks/, /perf/, /performance/, /load/, /loadtest/,
// /__benchmarks__/.
func DetectMissingPerfTest(snap *models.TestSuiteSnapshot, graph *impact.ImpactGraph) []models.Signal {
	if snap == nil || graph == nil {
		return nil
	}

	perfPaths := map[string]bool{}
	for _, tf := range snap.TestFiles {
		if isPerfTestPath(tf.Path) {
			perfPaths[tf.Path] = true
		}
	}
	if len(perfPaths) == 0 {
		// No perf tests anywhere — silent for 0.2.0, same logic as
		// coverage/no-integration-test (the gate would otherwise indict
		// the entire codebase on first run).
		return nil
	}

	var out []models.Signal
	for _, cs := range snap.CodeSurfaces {
		if !isLatencyCriticalSurface(cs.Kind) {
			continue
		}
		if surfaceHasPerfTest(cs, graph, perfPaths) {
			continue
		}
		out = append(out, models.Signal{
			Type:             signals.SignalMissingPerfTest,
			Category:         models.CategoryAI,
			Severity:         models.SeverityLow,
			Confidence:       0.75,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceGraphTraversal,
			Location: models.SignalLocation{
				File:   cs.Path,
				Symbol: cs.Name,
			},
			Explanation: fmt.Sprintf(
				"AI surface %q (kind=%s) in %s has no benchmark / load-test exercising it. Latency / throughput regressions can ship without anyone noticing.",
				cs.Name, cs.Kind, cs.Path,
			),
			SuggestedAction: fmt.Sprintf(
				"Add a benchmark in benchmarks/ or perf/ that exercises %q and records P50 / P95 latency.",
				cs.Name,
			),
			RuleID:          "terrain/performance/missing-perf-test",
			RuleURI:         "docs/rules/performance/missing-perf-test.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"surfaceId":   cs.SurfaceID,
				"surfaceKind": string(cs.Kind),
			},
		})
	}
	return out
}

func isLatencyCriticalSurface(k models.CodeSurfaceKind) bool {
	switch k {
	case models.SurfacePrompt, models.SurfaceRetrieval, models.SurfaceAgent,
		models.SurfaceModel, models.SurfaceHandler, models.SurfaceRoute:
		return true
	}
	return false
}

func surfaceHasPerfTest(cs models.CodeSurface, graph *impact.ImpactGraph, perfPaths map[string]bool) bool {
	for _, testPath := range graph.UnitToTests[cs.SurfaceID] {
		if perfPaths[testPath] {
			return true
		}
	}
	for _, testPath := range graph.UnitToTests[cs.Name] {
		if perfPaths[testPath] {
			return true
		}
	}
	return false
}

func isPerfTestPath(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range perfPathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var perfPathMarkers = []string{
	"/bench/", "/benchmarks/", "/perf/", "/performance/",
	"/load/", "/loadtest/", "/__benchmarks__/",
}
