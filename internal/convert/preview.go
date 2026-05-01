package convert

import (
	"fmt"
	"os"
	"path/filepath"
)

// runPreview is the implementation of TestMigrationOptions.Preview.
// Runs Execute against a temp directory, reads each (source, output)
// pair, computes a unified diff, then deletes the temp tree. Returns
// one FilePreview per file the converter touched.
func runPreview(source string, direction Direction, options TestMigrationOptions) ([]FilePreview, error) {
	tmp, err := os.MkdirTemp("", "terrain-preview-")
	if err != nil {
		return nil, fmt.Errorf("preview tempdir: %w", err)
	}
	defer os.RemoveAll(tmp)

	exec, err := Execute(source, direction, ExecuteOptions{
		Output:            tmp,
		PreserveStructure: options.PreserveStructure,
		BatchSize:         options.BatchSize,
		Concurrency:       options.Concurrency,
	})
	if err != nil {
		return nil, fmt.Errorf("preview execute: %w", err)
	}

	previews := make([]FilePreview, 0, len(exec.Files))
	for _, f := range exec.Files {
		preview := FilePreview{
			SourcePath: f.SourcePath,
			OutputPath: f.OutputPath,
			Status:     f.Status,
			Changed:    f.Changed,
		}

		oldContent, oldErr := readIfExists(f.SourcePath)
		newContent, newErr := readIfExists(f.OutputPath)

		switch {
		case oldErr != nil && newErr != nil:
			preview.Diff = fmt.Sprintf("(unable to read source or output: %v / %v)\n", oldErr, newErr)
		case oldErr != nil:
			preview.Diff = fmt.Sprintf("--- (no source available)\n+++ %s\n%s\n", f.OutputPath, newContent)
		case newErr != nil:
			preview.Diff = fmt.Sprintf("--- %s\n+++ (no output)\n%s\n", f.SourcePath, oldContent)
		default:
			preview.Diff = UnifiedDiff(f.SourcePath, f.OutputPath, oldContent, newContent)
		}

		previews = append(previews, preview)
	}

	// Single-file converters (mode == "stdout") populate exec.StdoutContent
	// instead of files. Surface that as a preview too.
	if len(previews) == 0 && exec.Mode == "stdout" && exec.StdoutContent != "" {
		oldContent, oldErr := readIfExists(source)
		old := ""
		if oldErr == nil {
			old = oldContent
		}
		previews = append(previews, FilePreview{
			SourcePath: source,
			OutputPath: "(stdout)",
			Status:     "converted",
			Changed:    true,
			Diff:       UnifiedDiff(source, "(stdout)", old, exec.StdoutContent),
		})
	}

	return previews, nil
}

// readIfExists reads a file if it exists and is readable. Used by the
// preview path so missing-file conditions become rendered as part of
// the diff rather than aborting the whole preview.
func readIfExists(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	abs := path
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(path)
		if err != nil {
			return "", err
		}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
