// Command terrain-bench-gate compares two `go test -bench` output files
// and exits non-zero if any benchmark regressed more than the configured
// threshold (default 10%).
//
// Usage:
//
//	terrain-bench-gate --base bench_base.txt --head bench_head.txt
//	terrain-bench-gate --base bench_base.txt --head bench_head.txt --threshold 5
//
// Format expected: standard `go test -bench` text output. One run per
// benchmark; multiple iterations (`-count=N`) average automatically.
//
// Output is the per-benchmark delta with a clear PASS/FAIL line. The
// CI workflow also runs benchstat for richer statistics; this tool is
// the gate.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// benchLine matches a `go test -bench` result row:
//
//	BenchmarkFoo-12      12345    98765.4 ns/op    1024 B/op    8 allocs/op
//
// We only care about the name and ns/op; the rest is informational.
var benchLine = regexp.MustCompile(`^Benchmark\S+\s+\d+\s+(\d+(?:\.\d+)?)\s+ns/op`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "terrain-bench-gate:", err)
		os.Exit(2)
	}
}

func run() error {
	base := flag.String("base", "", "path to benchmark output for the base / target branch")
	head := flag.String("head", "", "path to benchmark output for the proposed change")
	threshold := flag.Float64("threshold", 10.0, "max acceptable regression percent (positive number)")
	flag.Parse()

	if *base == "" || *head == "" {
		return fmt.Errorf("--base and --head are required")
	}
	if *threshold <= 0 {
		return fmt.Errorf("--threshold must be positive")
	}

	baseRuns, err := loadBenchmarks(*base)
	if err != nil {
		return fmt.Errorf("read base: %w", err)
	}
	headRuns, err := loadBenchmarks(*head)
	if err != nil {
		return fmt.Errorf("read head: %w", err)
	}

	type row struct {
		name      string
		baseMean  float64
		headMean  float64
		deltaPct  float64
		regressed bool
	}

	var rows []row
	for name, headValues := range headRuns {
		baseValues, ok := baseRuns[name]
		if !ok {
			// New benchmark — informational, can't gate without baseline.
			rows = append(rows, row{name: name, headMean: mean(headValues)})
			continue
		}
		bMean := mean(baseValues)
		hMean := mean(headValues)
		var delta float64
		if bMean > 0 {
			delta = (hMean - bMean) / bMean * 100
		}
		rows = append(rows, row{
			name:      name,
			baseMean:  bMean,
			headMean:  hMean,
			deltaPct:  delta,
			regressed: delta > *threshold,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		// Sort regressions to top, then by absolute delta descending.
		if rows[i].regressed != rows[j].regressed {
			return rows[i].regressed
		}
		return abs(rows[i].deltaPct) > abs(rows[j].deltaPct)
	})

	regressions := 0
	fmt.Println("Benchmark regression gate")
	fmt.Printf("Threshold: +%.1f%%\n\n", *threshold)
	fmt.Printf("  %-50s  %12s  %12s  %10s\n", "name", "base ns/op", "head ns/op", "delta")
	for _, r := range rows {
		if r.baseMean == 0 {
			fmt.Printf("  %-50s  %12s  %12.0f  %10s   (new)\n", r.name, "-", r.headMean, "-")
			continue
		}
		marker := " "
		if r.regressed {
			marker = "!"
			regressions++
		}
		fmt.Printf("%s %-50s  %12.0f  %12.0f  %+9.1f%%\n",
			marker, r.name, r.baseMean, r.headMean, r.deltaPct)
	}
	fmt.Println()

	if regressions > 0 {
		fmt.Printf("FAIL: %d benchmark(s) regressed more than %.1f%%.\n", regressions, *threshold)
		os.Exit(1)
	}
	fmt.Println("PASS: no benchmark regressed beyond the threshold.")
	return nil
}

// loadBenchmarks parses a `go test -bench` output and returns a map of
// benchmark-name → recorded ns/op values (one per run iteration).
// Lines that don't match the benchmark format are silently skipped.
func loadBenchmarks(path string) (map[string][]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseBenchmarks(f)
}

func parseBenchmarks(r io.Reader) (map[string][]float64, error) {
	out := map[string][]float64{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		if !benchLine.MatchString(line) {
			continue
		}
		// fields[0] is "BenchmarkFoo-12"; strip the trailing -<GOMAXPROCS>.
		name := fields[0]
		if idx := strings.LastIndex(name, "-"); idx > 0 {
			name = name[:idx]
		}
		val, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}
		out[name] = append(out[name], val)
	}
	return out, sc.Err()
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
