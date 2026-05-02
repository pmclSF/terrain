package convert

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/analysis"
)

type Detection struct {
	Path            string               `json:"path"`
	Mode            string               `json:"mode"`
	Framework       string               `json:"framework"`
	Confidence      float64              `json:"confidence"`
	Recommendation  string               `json:"recommendation,omitempty"`
	AutoDetectSafe  bool                 `json:"autoDetectSafe,omitempty"`
	DetectionSource string               `json:"detectionSource,omitempty"`
	Language        string               `json:"language,omitempty"`
	Category        string               `json:"category,omitempty"`
	FilesScanned    int                  `json:"filesScanned,omitempty"`
	Mixed           bool                 `json:"mixed,omitempty"`
	Ambiguous       bool                 `json:"ambiguous,omitempty"`
	Candidates      []DetectionCandidate `json:"candidates,omitempty"`
}

type DetectionCandidate struct {
	Framework  string  `json:"framework"`
	Confidence float64 `json:"confidence"`
	Files      int     `json:"files"`
	FileShare  float64 `json:"fileShare,omitempty"`
	Primary    bool    `json:"primary,omitempty"`
	Source     string  `json:"source,omitempty"`
}

type detectionCandidateSummary struct {
	framework  string
	confidence float64
	files      int
	source     string
}

func DetectSource(path string) (Detection, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Detection{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return detectDirectory(path)
	}
	return detectFile(path)
}

func detectFile(path string) (Detection, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	result := analysis.DetectFrameworkForFile(absPath)
	candidateCount := 0
	if result.Framework != "" && result.Framework != "unknown" {
		candidateCount = 1
	}
	detection := Detection{
		Path:            path,
		Mode:            "file",
		Framework:       result.Framework,
		Confidence:      result.Confidence,
		Recommendation:  detectionRecommendation(false, false, result.Framework != "" && result.Framework != "unknown", candidateCount),
		AutoDetectSafe:  result.Framework != "" && result.Framework != "unknown",
		DetectionSource: result.Source,
		FilesScanned:    1,
	}
	if result.Framework != "" && result.Framework != "unknown" {
		detection.Candidates = []DetectionCandidate{{
			Framework:  result.Framework,
			Confidence: result.Confidence,
			Files:      1,
			FileShare:  1,
			Primary:    true,
			Source:     result.Source,
		}}
	}
	if framework, ok := LookupFramework(result.Framework); ok {
		detection.Language = framework.Language
		detection.Category = framework.Category
	}
	return detection, nil
}

func detectDirectory(root string) (Detection, error) {
	best := Detection{
		Path:      root,
		Mode:      "directory",
		Framework: "unknown",
	}
	summaries := map[string]*detectionCandidateSummary{}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isLikelyTestPath(path) {
			return nil
		}

		detection, err := detectFile(path)
		if err != nil {
			return nil
		}
		best.FilesScanned++
		if detection.Framework == "unknown" {
			return nil
		}

		summary := summaries[detection.Framework]
		if summary == nil {
			summary = &detectionCandidateSummary{framework: detection.Framework}
			summaries[detection.Framework] = summary
		}
		summary.files++
		if detection.Confidence > summary.confidence {
			summary.confidence = detection.Confidence
			summary.source = detection.DetectionSource
		}

		return nil
	})
	if walkErr != nil {
		return Detection{}, fmt.Errorf("scan directory: %w", walkErr)
	}
	best.Candidates = buildDetectionCandidates(summaries)
	best.Mixed = len(best.Candidates) > 1
	best.Ambiguous = detectionIsAmbiguous(best.Candidates)
	best.AutoDetectSafe = detectionAutoDetectSafe(best.Candidates, best.Ambiguous)
	best.Recommendation = detectionRecommendation(best.Mixed, best.Ambiguous, best.AutoDetectSafe, len(best.Candidates))
	if len(best.Candidates) > 0 {
		top := best.Candidates[0]
		best.Framework = top.Framework
		best.Confidence = top.Confidence
		best.DetectionSource = top.Source
		if framework, ok := LookupFramework(top.Framework); ok {
			best.Language = framework.Language
			best.Category = framework.Category
		}
	}
	return best, nil
}

func buildDetectionCandidates(summaries map[string]*detectionCandidateSummary) []DetectionCandidate {
	items := make([]DetectionCandidate, 0, len(summaries))
	totalFiles := 0
	for _, summary := range summaries {
		totalFiles += summary.files
	}
	for _, summary := range summaries {
		share := 0.0
		if totalFiles > 0 {
			share = float64(summary.files) / float64(totalFiles)
		}
		items = append(items, DetectionCandidate{
			Framework:  summary.framework,
			Confidence: summary.confidence,
			Files:      summary.files,
			FileShare:  share,
			Source:     summary.source,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Files == items[j].Files {
			if items[i].Confidence == items[j].Confidence {
				return items[i].Framework < items[j].Framework
			}
			return items[i].Confidence > items[j].Confidence
		}
		return items[i].Files > items[j].Files
	})
	if len(items) > 0 {
		items[0].Primary = true
	}
	return items
}

func detectionIsAmbiguous(candidates []DetectionCandidate) bool {
	if len(candidates) < 2 {
		return false
	}
	top := candidates[0]
	next := candidates[1]
	if top.Framework == next.Framework {
		return false
	}
	return top.FileShare-next.FileShare <= 0.15 && top.Confidence-next.Confidence <= 0.10
}

func detectionAutoDetectSafe(candidates []DetectionCandidate, ambiguous bool) bool {
	if len(candidates) == 0 {
		return false
	}
	if len(candidates) == 1 {
		return true
	}
	if ambiguous {
		return false
	}
	return candidates[0].FileShare >= 0.60
}

func detectionRecommendation(mixed, ambiguous, autoDetectSafe bool, candidateCount int) string {
	switch {
	case candidateCount == 0:
		return "unknown"
	case !autoDetectSafe && candidateCount == 1:
		return "unknown"
	case !mixed:
		return "safe"
	case ambiguous:
		return "ambiguous"
	case autoDetectSafe:
		return "dominant"
	default:
		return "mixed"
	}
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "dist", "build", ".terrain":
		return true
	default:
		return false
	}
}

func isLikelyTestPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts", ".py", ".java":
	default:
		return false
	}

	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}

	// Pre-0.2.x this matched any path containing "/tests/" anywhere,
	// counting `tests/fixtures/<repo>/src/app.js` as a test file. A
	// terrain doctor on the terrain repo reported 34,399 "test files"
	// (vs 9,852 actual). Tighten to "the immediate parent directory is
	// a recognized test-dir name" — fixture source under tests/fixtures
	// no longer inflates the count, while genuinely-tests files (whose
	// parent IS `__tests__` or `tests` or `e2e` or `integration`) still
	// match.
	dir := filepath.Dir(path)
	parent := strings.ToLower(filepath.Base(dir))
	switch parent {
	case "test", "tests", "__tests__", "e2e", "integration":
		return true
	}
	return false
}
