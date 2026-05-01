package main

import (
	"strings"
	"testing"
)

func TestParseBenchmarks_SingleRun(t *testing.T) {
	t.Parallel()

	in := strings.NewReader(`
goos: linux
goarch: amd64
pkg: github.com/pmclSF/terrain/internal/scoring
BenchmarkRiskScore-12          	  120145	      9876 ns/op	     320 B/op	      4 allocs/op
PASS
`)
	out, err := parseBenchmarks(in)
	if err != nil {
		t.Fatalf("parseBenchmarks: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d entries, want 1", len(out))
	}
	if got := out["BenchmarkRiskScore"]; len(got) != 1 || got[0] != 9876 {
		t.Errorf("BenchmarkRiskScore = %v", got)
	}
}

func TestParseBenchmarks_MultipleRuns(t *testing.T) {
	t.Parallel()

	in := strings.NewReader(`
BenchmarkA-12  10000  100.0 ns/op
BenchmarkA-12  10000  120.0 ns/op
BenchmarkA-12  10000  110.0 ns/op
`)
	out, _ := parseBenchmarks(in)
	got := out["BenchmarkA"]
	if len(got) != 3 {
		t.Fatalf("got %d values, want 3", len(got))
	}
	if m := mean(got); m != 110 {
		t.Errorf("mean = %f, want 110", m)
	}
}

func TestParseBenchmarks_IgnoresNonBenchLines(t *testing.T) {
	t.Parallel()

	in := strings.NewReader(`
goos: darwin
something: random
BenchmarkOK-12   100  500 ns/op
PASS
ok    pkg/x  1.234s
`)
	out, _ := parseBenchmarks(in)
	if _, ok := out["BenchmarkOK"]; !ok {
		t.Errorf("expected BenchmarkOK to parse, got %v", out)
	}
}

func TestMean_Empty(t *testing.T) {
	t.Parallel()
	if got := mean(nil); got != 0 {
		t.Errorf("mean(nil) = %v, want 0", got)
	}
}
