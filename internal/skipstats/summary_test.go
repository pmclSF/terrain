package skipstats

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestSummarize_StaticOnly(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/a.test.ts", TestCount: 10, SkipCount: 3},
			{Path: "tests/b.test.ts", TestCount: 5},
		},
	}

	got := Summarize(snap)

	if got.SkippedTests != 3 {
		t.Fatalf("SkippedTests = %d, want 3", got.SkippedTests)
	}
	if got.TotalTests != 15 {
		t.Fatalf("TotalTests = %d, want 15", got.TotalTests)
	}
	if got.FilesWithSkips != 1 {
		t.Fatalf("FilesWithSkips = %d, want 1", got.FilesWithSkips)
	}
	if got.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", got.TotalFiles)
	}
	if got.TestRatio != 0.2 {
		t.Fatalf("TestRatio = %.2f, want 0.20", got.TestRatio)
	}
	if got.FileRatio != 0.5 {
		t.Fatalf("FileRatio = %.2f, want 0.50", got.FileRatio)
	}
}

func TestSummarize_RuntimeOnly(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/a.test.ts", TestCount: 10, RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 25}},
			{Path: "tests/b.test.ts", TestCount: 6, RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 40}},
		},
		Signals: []models.Signal{
			{
				Type:     signals.SignalSkippedTest,
				Location: models.SignalLocation{File: "tests/a.test.ts"},
				Metadata: map[string]any{"skippedCount": 2, "totalCount": 10, "scope": "file"},
			},
		},
	}

	got := Summarize(snap)

	if got.SkippedTests != 2 {
		t.Fatalf("SkippedTests = %d, want 2", got.SkippedTests)
	}
	if got.TotalTests != 16 {
		t.Fatalf("TotalTests = %d, want 16", got.TotalTests)
	}
	if got.FilesWithSkips != 1 {
		t.Fatalf("FilesWithSkips = %d, want 1", got.FilesWithSkips)
	}
	if got.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", got.TotalFiles)
	}
}

func TestSummarize_RuntimeOverridesStaticOnSameFile(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{
				Path:         "tests/a.test.ts",
				TestCount:    10,
				SkipCount:    4,
				RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 25},
			},
			{Path: "tests/b.test.ts", TestCount: 5},
		},
		Signals: []models.Signal{
			{
				Type:     signals.SignalSkippedTest,
				Location: models.SignalLocation{File: "tests/a.test.ts"},
				Metadata: map[string]any{"skippedCount": 1, "totalCount": 10, "scope": "file"},
			},
		},
	}

	got := Summarize(snap)

	if got.SkippedTests != 1 {
		t.Fatalf("SkippedTests = %d, want 1", got.SkippedTests)
	}
	if got.FilesWithSkips != 1 {
		t.Fatalf("FilesWithSkips = %d, want 1", got.FilesWithSkips)
	}
}

func TestSummarize_RuntimeEvidenceCanZeroOutStaticSkips(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{
				Path:         "tests/a.test.ts",
				TestCount:    10,
				SkipCount:    4,
				RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 25},
			},
		},
	}

	got := Summarize(snap)

	if got.SkippedTests != 0 {
		t.Fatalf("SkippedTests = %d, want 0", got.SkippedTests)
	}
	if got.FilesWithSkips != 0 {
		t.Fatalf("FilesWithSkips = %d, want 0", got.FilesWithSkips)
	}
}

func TestSummarize_MixedStaticAndRuntimeAcrossFiles(t *testing.T) {
	t.Parallel()

	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "tests/static.test.ts", TestCount: 8, SkipCount: 3},
			{Path: "tests/runtime.test.ts", TestCount: 12, RuntimeStats: &models.RuntimeStats{AvgRuntimeMs: 30}},
			{Path: "tests/clean.test.ts", TestCount: 5},
		},
		Signals: []models.Signal{
			{
				Type:     signals.SignalSkippedTest,
				Location: models.SignalLocation{File: "tests/runtime.test.ts"},
				Metadata: map[string]any{"skippedCount": 2, "totalCount": 12, "scope": "file"},
			},
		},
	}

	got := Summarize(snap)

	if got.SkippedTests != 5 {
		t.Fatalf("SkippedTests = %d, want 5", got.SkippedTests)
	}
	if got.TotalTests != 25 {
		t.Fatalf("TotalTests = %d, want 25", got.TotalTests)
	}
	if got.FilesWithSkips != 2 {
		t.Fatalf("FilesWithSkips = %d, want 2", got.FilesWithSkips)
	}
	if got.FileRatio != (2.0 / 3.0) {
		t.Fatalf("FileRatio = %.6f, want %.6f", got.FileRatio, 2.0/3.0)
	}
}
