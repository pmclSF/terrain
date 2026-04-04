package convert

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/analysis"
)

type Detection struct {
	Path            string  `json:"path"`
	Mode            string  `json:"mode"`
	Framework       string  `json:"framework"`
	Confidence      float64 `json:"confidence"`
	DetectionSource string  `json:"detectionSource,omitempty"`
	Language        string  `json:"language,omitempty"`
	Category        string  `json:"category,omitempty"`
	FilesScanned    int     `json:"filesScanned,omitempty"`
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
	return best, nil
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
