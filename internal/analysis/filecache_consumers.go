package analysis

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// extractCodeUnitsCached is like extractExportedCodeUnitsFromList but reads
// files through the FileCache instead of os.ReadFile.
func extractCodeUnitsCached(root string, testFiles []models.TestFile, sourceFiles []string, fc *FileCache) []models.CodeUnit {
	if fc == nil {
		return extractExportedCodeUnitsFromList(root, testFiles, sourceFiles)
	}
	testPaths := map[string]bool{}
	for _, tf := range testFiles {
		testPaths[tf.Path] = true
	}
	unitsByFile := make([][]models.CodeUnit, len(sourceFiles))
	parallelForEachIndex(len(sourceFiles), func(i int) {
		relPath := sourceFiles[i]
		if testPaths[relPath] {
			return
		}
		ext := strings.ToLower(relPathExt(relPath))
		lang, ok := languageForExt[ext]
		if !ok {
			return
		}
		src, ok := fc.ReadFile(relPath)
		if !ok {
			return
		}
		unitsByFile[i] = extractCodeUnitsFromSource(src, relPath, lang)
	})

	units := make([]models.CodeUnit, 0, len(sourceFiles))
	for i := range unitsByFile {
		units = append(units, unitsByFile[i]...)
	}
	return units
}

// extractCodeUnitsFromSource extracts code units from already-loaded content.
// This mirrors the per-language ExtractExports but without file I/O.
func extractCodeUnitsFromSource(src, relPath, lang string) []models.CodeUnit {
	lines := strings.Split(src, "\n")
	switch lang {
	case "js":
		return extractJSExportsFromLines(relPath, src, lines)
	case "go":
		return extractGoExportsFromLines(relPath, lines)
	case "python":
		return extractPythonExportsFromSource(relPath, src)
	case "java":
		return extractJavaExportsFromLines(relPath, lines)
	default:
		return nil
	}
}

// NOTE: The canonical content-based extractors (extractJSExportsFromLines,
// extractGoExportsFromLines, extractPythonExportsFromSource,
// extractJavaExportsFromLines) are defined in content_analysis.go.
// extractCodeUnitsFromSource (above) delegates to them.

// detectAITemplateFiles scans for .hbs/.j2/.tmpl/.prompt files with AI markers.
func detectAITemplateFiles(root string, existingIDs map[string]bool) []models.CodeSurface {
	var surfaces []models.CodeSurface
	templateExts := map[string]bool{
		".hbs": true, ".handlebars": true, ".j2": true, ".jinja2": true,
		".tmpl": true, ".mustache": true, ".prompt": true,
	}

	_ = walkDir(root, func(relPath string, isDir bool) bool {
		if isDir {
			return skipDirs[relPathBase(relPath)]
		}
		ext := strings.ToLower(filepath.Ext(relPath))
		if !templateExts[ext] {
			return false
		}
		data, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			return false
		}
		if len(aiStringMarkers.FindAllString(string(data), -1)) >= 1 {
			sid := models.BuildSurfaceID(relPath, "template_file", "")
			if !existingIDs[sid] {
				existingIDs[sid] = true
				surfaces = append(surfaces, models.CodeSurface{
					SurfaceID:     sid,
					Name:          "template_file",
					Path:          relPath,
					Kind:          models.SurfacePrompt,
					Language:      "template",
					DetectionTier: models.TierContent,
					Confidence:    0.75,
					Reason:        "[" + models.DetectorTemplateFile + "] template file with AI instruction markers",
				})
			}
		}
		return false
	})
	return surfaces
}
