package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/insights"
)

func skipFixtureRoot(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "fixtures", name)
	if _, err := os.Stat(root); err != nil {
		t.Skipf("fixture not found: %s", root)
	}
	return root
}

func loadFixtureSnapshot(t *testing.T, name string) *engine.PipelineResult {
	t.Helper()
	result, err := engine.RunPipeline(skipFixtureRoot(t, name), engine.PipelineOptions{EngineVersion: "test"})
	if err != nil {
		t.Fatalf("pipeline failed for %s: %v", name, err)
	}
	return result
}

func TestAnalyze_StaticSkipFixturesReportSkipBurden(t *testing.T) {
	t.Parallel()

	for _, fixture := range []string{"skipped-tests", "mobile-cross-platform"} {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			result := loadFixtureSnapshot(t, fixture)
			report := analyze.Build(&analyze.BuildInput{
				Snapshot:  result.Snapshot,
				HasPolicy: result.HasPolicy,
			})

			if report.SkippedTestBurden.SkippedCount == 0 {
				t.Fatalf("expected skipped count > 0 for %s", fixture)
			}
			if report.CIOptimization.SkippedTestsReviewable != report.SkippedTestBurden.SkippedCount {
				t.Fatalf("SkippedTestsReviewable = %d, want %d", report.CIOptimization.SkippedTestsReviewable, report.SkippedTestBurden.SkippedCount)
			}
			if report.RepoProfile.SkipBurden == "" {
				t.Fatalf("expected repoProfile.skipBurden for %s", fixture)
			}
		})
	}
}

func TestInsights_StaticSkipFixtureIncludesSkipFinding(t *testing.T) {
	t.Parallel()

	result := loadFixtureSnapshot(t, "skipped-tests")
	report := insights.Build(&insights.BuildInput{
		Snapshot:  result.Snapshot,
		HasPolicy: result.HasPolicy,
	})

	found := false
	for _, finding := range report.Findings {
		if finding.Category == insights.CategoryReliability && strings.Contains(finding.Metric, "skipped") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected skipped-test finding for skipped-tests fixture")
	}

	joined := strings.Join(report.Limitations, "\n")
	if !strings.Contains(joined, "static skip detection is available") {
		t.Fatalf("expected updated limitations text, got %q", joined)
	}
}
