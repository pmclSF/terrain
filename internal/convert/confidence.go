package convert

import (
	"os"
	"regexp"
)

// annotateFileConfidence walks the Files in an ExecutionResult and
// fills in (ItemsCovered, ItemsLossy, Confidence) per file. Reads
// source + output content from disk; files that can't be read leave
// the metrics at zero (default JSON omits them). Used as a single
// post-execute pass so each per-direction Execute branch doesn't need
// to know about confidence math.
func annotateFileConfidence(result *ExecutionResult) {
	if result == nil {
		return
	}
	for i := range result.Files {
		f := &result.Files[i]
		if f.SourcePath == "" || f.OutputPath == "" {
			continue
		}
		srcBytes, err := os.ReadFile(f.SourcePath)
		if err != nil {
			continue
		}
		dstBytes, err := os.ReadFile(f.OutputPath)
		if err != nil {
			continue
		}
		f.ItemsCovered, f.ItemsLossy, f.Confidence = computeFileConfidence(string(srcBytes), string(dstBytes))
	}
	// Stdout-mode results carry the converted text in StdoutContent
	// rather than on disk. Cover that path by reading the source
	// file and pairing with StdoutContent.
	if result.Mode == "stdout" && result.StdoutContent != "" && len(result.Files) == 0 && result.Source != "" {
		srcBytes, err := os.ReadFile(result.Source)
		if err == nil {
			covered, lossy, conf := computeFileConfidence(string(srcBytes), result.StdoutContent)
			result.Files = append(result.Files, FileResult{
				SourcePath:   result.Source,
				OutputPath:   "(stdout)",
				Changed:      true,
				Status:       "converted",
				ItemsCovered: covered,
				ItemsLossy:   lossy,
				Confidence:   conf,
			})
		}
	}
}

// significantItemPatterns are the regex patterns we use to count
// test-significant items in source/output for the per-file confidence
// heuristic. Each matched substring counts once. The list is
// intentionally framework-agnostic — the same patterns work across
// Jest / Vitest / Mocha / Jasmine and Pytest, and the COUNTS are what
// matter, not the framework attribution.
var significantItemPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(?:test|it)\s*\(`),
	regexp.MustCompile(`\bdescribe\s*\(`),
	regexp.MustCompile(`\bbeforeEach\s*\(`),
	regexp.MustCompile(`\bbeforeAll\s*\(`),
	regexp.MustCompile(`\bafterEach\s*\(`),
	regexp.MustCompile(`\bafterAll\s*\(`),
	regexp.MustCompile(`\bexpect\s*\(`),
	regexp.MustCompile(`\bassert\s*[.(]`),
	// Pytest:
	regexp.MustCompile(`\bdef\s+test_\w+`),
	regexp.MustCompile(`@pytest\.fixture\b`),
	regexp.MustCompile(`@pytest\.mark\.`),
	regexp.MustCompile(`\bassert\s+\w`),
}

// countSignificantItems sums the matches of every
// significantItemPattern in s. Each pattern can match multiple times;
// we count every occurrence. This is heuristic by design — the goal
// is a stable count that converts roughly 1:1 between source and
// output for clean conversions.
func countSignificantItems(s string) int {
	if s == "" {
		return 0
	}
	total := 0
	for _, rx := range significantItemPatterns {
		matches := rx.FindAllStringIndex(s, -1)
		total += len(matches)
	}
	return total
}

// computeFileConfidence returns the (covered, lossy, confidence)
// triple for a file conversion. Covered is the count of significant
// items that appear in both src and dst (taken as min(srcCount,
// dstCount) — a heuristic). Lossy is max(0, srcCount - dstCount).
// Confidence is covered / (covered + lossy), or 1.0 when both counts
// are zero (nothing to lose).
func computeFileConfidence(src, dst string) (covered, lossy int, confidence float64) {
	srcCount := countSignificantItems(src)
	dstCount := countSignificantItems(dst)
	if srcCount == 0 && dstCount == 0 {
		// Nothing to measure; treat as a clean conversion. The
		// alternative (0 confidence) would frighten users on
		// fixtures that don't include tests.
		return 0, 0, 1.0
	}
	if srcCount <= dstCount {
		covered = srcCount
		lossy = 0
	} else {
		covered = dstCount
		lossy = srcCount - dstCount
	}
	denom := covered + lossy
	if denom == 0 {
		return covered, lossy, 1.0
	}
	confidence = float64(covered) / float64(denom)
	return covered, lossy, confidence
}
