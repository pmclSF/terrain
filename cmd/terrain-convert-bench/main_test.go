package main

import (
	"testing"

	conv "github.com/pmclSF/terrain/internal/convert"
)

func TestBuildBenchmarkCases_CoversSupportedDirections(t *testing.T) {
	t.Parallel()

	cases, err := buildBenchmarkCases("")
	if err != nil {
		t.Fatalf("buildBenchmarkCases returned error: %v", err)
	}
	if len(cases) != len(conv.SupportedDirections()) {
		t.Fatalf("cases = %d, want %d", len(cases), len(conv.SupportedDirections()))
	}
}

func TestSummarizeSamples(t *testing.T) {
	t.Parallel()

	stats := summarizeSamples([]int64{5, 10, 15, 20, 25})
	if stats.MinNs != 5 {
		t.Fatalf("MinNs = %d, want 5", stats.MinNs)
	}
	if stats.MedianNs != 15 {
		t.Fatalf("MedianNs = %d, want 15", stats.MedianNs)
	}
	if stats.P95Ns != 25 {
		t.Fatalf("P95Ns = %d, want 25", stats.P95Ns)
	}
	if stats.MaxNs != 25 {
		t.Fatalf("MaxNs = %d, want 25", stats.MaxNs)
	}
	if stats.MeanNs != 15 {
		t.Fatalf("MeanNs = %d, want 15", stats.MeanNs)
	}
}
