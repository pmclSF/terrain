package changescope

import (
	"bytes"
	"fmt"
	"testing"
)

// BenchmarkRenderPRSummaryMarkdown_Small benchmarks the PR markdown
// renderer against a small, realistic PR (5 findings, 3 selections,
// 2 owners). Audit-named gap (pr_change_scoped.E5): published
// performance evidence for the PR pipeline's render stage.
//
// Run with: go test -bench=BenchmarkRenderPR -benchmem ./internal/changescope/
//
// Reference baseline (Intel i7-8850H @ 2.60GHz, captured 2026-05):
//   small  (5 findings)    ≈ 19 µs/op,  9 KB/op,  93 allocs/op
//   medium (50 findings)   ≈ 51 µs/op, 44 KB/op, 241 allocs/op
//   large  (200 findings)  ≈ 155 µs/op, 164 KB/op, 553 allocs/op
//
// Linear in finding count; no quadratic blow-up in the dedup or
// classification paths. These numbers are environment-sensitive;
// use them as order-of-magnitude anchors, not strict CI gates.
func BenchmarkRenderPRSummaryMarkdown_Small(b *testing.B) {
	pr := newBenchPR(5, 3, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		RenderPRSummaryMarkdown(&buf, pr)
	}
}

// BenchmarkRenderPRSummaryMarkdown_Medium benchmarks a typical
// service-repo PR (50 findings, 20 selections, 5 owners).
func BenchmarkRenderPRSummaryMarkdown_Medium(b *testing.B) {
	pr := newBenchPR(50, 20, 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		RenderPRSummaryMarkdown(&buf, pr)
	}
}

// BenchmarkRenderPRSummaryMarkdown_Large benchmarks a large-PR
// stress shape (200 findings, 100 selections, 20 owners). Catches
// quadratic regressions in the dedup / classify / render pipeline.
func BenchmarkRenderPRSummaryMarkdown_Large(b *testing.B) {
	pr := newBenchPR(200, 100, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		RenderPRSummaryMarkdown(&buf, pr)
	}
}

// newBenchPR constructs a PRAnalysis fixture for benchmarking.
// Distributes findings across direct / indirect / existing scopes
// to exercise the real classification path.
func newBenchPR(findingCount, selectionCount, ownerCount int) *PRAnalysis {
	findings := make([]ChangeScopedFinding, findingCount)
	for i := 0; i < findingCount; i++ {
		scope := "direct"
		switch i % 3 {
		case 1:
			scope = "indirect"
		case 2:
			scope = "existing"
		}
		findings[i] = ChangeScopedFinding{
			Type:        "protection_gap",
			Scope:       scope,
			Path:        fmt.Sprintf("src/pkg%d/file_%d.go", i%10, i),
			Severity:    severityRotation[i%4],
			Explanation: fmt.Sprintf("Finding %d explanation goes here.", i),
		}
	}

	selections := make([]TestSelection, selectionCount)
	for i := 0; i < selectionCount; i++ {
		selections[i] = TestSelection{
			Path:        fmt.Sprintf("tests/pkg%d_test.go", i%10),
			Confidence:  "exact",
			CoversUnits: []string{fmt.Sprintf("src/pkg%d/file_%d.go:Func%d", i%10, i, i)},
		}
	}

	recommended := make([]string, selectionCount)
	for i := range recommended {
		recommended[i] = selections[i].Path
	}

	owners := make([]string, ownerCount)
	for i := 0; i < ownerCount; i++ {
		owners[i] = fmt.Sprintf("@team-%d", i)
	}

	return &PRAnalysis{
		PostureBand:        "partially_protected",
		ChangedFileCount:   findingCount / 2,
		ChangedSourceCount: findingCount / 3,
		ChangedTestCount:   findingCount / 6,
		ImpactedUnitCount:  findingCount,
		ProtectionGapCount: findingCount / 2,
		TotalTestCount:     500,
		NewFindings:        findings,
		AffectedOwners:     owners,
		RecommendedTests:   recommended,
		TestSelections:     selections,
	}
}

var severityRotation = []string{"critical", "high", "medium", "low"}
