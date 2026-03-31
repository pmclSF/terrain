package skipstats

import (
	"encoding/json"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// Summary captures merged skip burden across static and runtime evidence.
//
// Static skip markers come from TestFile.SkipCount. Runtime evidence is
// authoritative for files that have runtime stats or runtime skippedTest
// signals. This prevents double counting when both static and runtime data
// describe the same file.
type Summary struct {
	SkippedTests   int
	TotalTests     int
	FilesWithSkips int
	TotalFiles     int
	TestRatio      float64
	FileRatio      float64
}

type runtimeSkipCount struct {
	skipped int
	total   int
}

// Summarize merges static skip markers with runtime skippedTest evidence.
func Summarize(snap *models.TestSuiteSnapshot) Summary {
	if snap == nil {
		return Summary{}
	}

	runtimeByFile, repoRuntime := collectRuntimeSkipCounts(snap)
	summary := Summary{
		TotalFiles: len(snap.TestFiles),
	}

	seenFiles := make(map[string]bool, len(snap.TestFiles))
	for _, tf := range snap.TestFiles {
		seenFiles[tf.Path] = true
		summary.TotalTests += tf.TestCount

		runtimeCounts, hasRuntimeSignal := runtimeByFile[tf.Path]
		hasRuntimeEvidence := tf.RuntimeStats != nil || hasRuntimeSignal

		skipped := tf.SkipCount
		if hasRuntimeEvidence {
			if hasRuntimeSignal {
				skipped = runtimeCounts.skipped
			} else {
				skipped = 0
			}
		}

		if skipped > 0 {
			summary.SkippedTests += skipped
			summary.FilesWithSkips++
		}
	}

	for path, counts := range runtimeByFile {
		if seenFiles[path] {
			continue
		}
		summary.TotalFiles++
		summary.TotalTests += counts.total
		if counts.skipped > 0 {
			summary.SkippedTests += counts.skipped
			summary.FilesWithSkips++
		}
	}

	if summary.TotalTests == 0 {
		switch {
		case len(snap.TestCases) > 0:
			summary.TotalTests = len(snap.TestCases)
		case repoRuntime.total > 0:
			summary.TotalTests = repoRuntime.total
		}
	}

	if summary.SkippedTests == 0 && repoRuntime.skipped > 0 {
		summary.SkippedTests = repoRuntime.skipped
		if summary.FilesWithSkips == 0 {
			summary.FilesWithSkips = skippedFileFallbackCount(snap)
		}
	}

	if summary.TotalFiles == 0 && summary.FilesWithSkips > 0 {
		summary.TotalFiles = summary.FilesWithSkips
	}

	if summary.TotalTests > 0 {
		summary.TestRatio = float64(summary.SkippedTests) / float64(summary.TotalTests)
	}
	if summary.TotalFiles > 0 {
		summary.FileRatio = float64(summary.FilesWithSkips) / float64(summary.TotalFiles)
	}

	return summary
}

func collectRuntimeSkipCounts(snap *models.TestSuiteSnapshot) (map[string]runtimeSkipCount, runtimeSkipCount) {
	byFile := map[string]runtimeSkipCount{}
	var repo runtimeSkipCount

	for _, sig := range snap.Signals {
		if sig.Type != signals.SignalSkippedTest {
			continue
		}

		skipped := metadataInt(sig.Metadata, "skippedCount")
		if skipped == 0 {
			skipped = 1
		}
		total := metadataInt(sig.Metadata, "totalCount")

		if sig.Location.File != "" {
			current := byFile[sig.Location.File]
			if skipped > current.skipped {
				current.skipped = skipped
			}
			if total > current.total {
				current.total = total
			}
			byFile[sig.Location.File] = current
			continue
		}

		if skipped > repo.skipped {
			repo.skipped = skipped
		}
		if total > repo.total {
			repo.total = total
		}
	}

	return byFile, repo
}

func skippedFileFallbackCount(snap *models.TestSuiteSnapshot) int {
	files := map[string]bool{}
	signalCount := 0

	for _, sig := range snap.Signals {
		if sig.Type != signals.SignalSkippedTest {
			continue
		}
		signalCount++
		if sig.Location.File != "" {
			files[sig.Location.File] = true
		}
	}

	if len(files) > 0 {
		return len(files)
	}
	return signalCount
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}

	switch v := metadata[key].(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i)
		}
	}

	return 0
}
