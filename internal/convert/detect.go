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
	detection := Detection{
		Path:            path,
		Mode:            "file",
		Framework:       result.Framework,
		Confidence:      result.Confidence,
		DetectionSource: result.Source,
		FilesScanned:    1,
	}
	if result.Framework != "" && result.Framework != "unknown" {
		detection.Candidates = []DetectionCandidate{{
			Framework:  result.Framework,
			Confidence: result.Confidence,
			Files:      1,
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

		if detection.Confidence > best.Confidence {
			best.Framework = detection.Framework
			best.Confidence = detection.Confidence
			best.DetectionSource = detection.DetectionSource
			best.Language = detection.Language
			best.Category = detection.Category
		}
		return nil
	})
	if walkErr != nil {
		return Detection{}, fmt.Errorf("scan directory: %w", walkErr)
	}
	best.Candidates = buildDetectionCandidates(summaries)
	best.Mixed = len(best.Candidates) > 1
	best.Ambiguous = detectionIsAmbiguous(best.Candidates)
	return best, nil
}

func buildDetectionCandidates(summaries map[string]*detectionCandidateSummary) []DetectionCandidate {
	items := make([]DetectionCandidate, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, DetectionCandidate{
			Framework:  summary.framework,
			Confidence: summary.confidence,
			Files:      summary.files,
			Source:     summary.source,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Confidence == items[j].Confidence {
			if items[i].Files == items[j].Files {
				return items[i].Framework < items[j].Framework
			}
			return items[i].Files > items[j].Files
		}
		return items[i].Confidence > items[j].Confidence
	})
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
	return top.Confidence-next.Confidence <= 0.10
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

	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, fragment := range []string{"/test/", "/tests/", "/__tests__/", "/e2e/", "/integration/"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
