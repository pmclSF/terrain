package convert

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type ExecuteOptions struct {
	Output            string `json:"output,omitempty"`
	PreserveStructure bool   `json:"preserveStructure,omitempty"`
	BatchSize         int    `json:"batchSize,omitempty"`
	Concurrency       int    `json:"concurrency,omitempty"`
}

type FileResult struct {
	SourcePath string `json:"sourcePath"`
	OutputPath string `json:"outputPath,omitempty"`
	Changed    bool   `json:"changed"`
	Status     string `json:"status"`

	// ItemsCovered is the count of test-significant items that
	// appear in BOTH the source and output (test()/it()/describe()
	// calls + assertion-shaped expressions). 0 when the converter
	// did not run or the conversion was a no-op.
	ItemsCovered int `json:"itemsCovered,omitempty"`

	// ItemsLossy is the count of test-significant items that
	// appeared in the source but disappeared from the output —
	// usually a converter that doesn't have an equivalent target
	// API for some construct. Non-zero indicates the user should
	// review the diff before merging.
	ItemsLossy int `json:"itemsLossy,omitempty"`

	// Confidence is ItemsCovered / (ItemsCovered + ItemsLossy),
	// in [0.0, 1.0]. 1.0 means the heuristic saw every input
	// construct survive into the output. 0.0 means a total loss.
	// Empty for source-mode runs that weren't measured.
	Confidence float64 `json:"confidence,omitempty"`
}

type ExecutionResult struct {
	Source         string       `json:"source"`
	Output         string       `json:"output,omitempty"`
	Mode           string       `json:"mode"`
	Direction      Direction    `json:"direction"`
	ValidationMode string       `json:"validationMode,omitempty"`
	Validated      bool         `json:"validated,omitempty"`
	Warnings       []string     `json:"warnings,omitempty"`
	Files          []FileResult `json:"files,omitempty"`
	ConvertedCount int          `json:"convertedCount"`
	UnchangedCount int          `json:"unchangedCount"`
	StdoutContent  string       `json:"-"`
}

func Execute(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	switch direction.From + "-" + direction.To {
	case "jest-vitest":
		return executeSourceConverter(source, direction, options, ConvertJestToVitestSource)
	case "cypress-playwright":
		return executeSourceConverter(source, direction, options, ConvertCypressToPlaywrightSource)
	case "cypress-selenium":
		return executeSourceConverter(source, direction, options, ConvertCypressToSeleniumSource)
	case "cypress-webdriverio":
		return executeSourceConverter(source, direction, options, ConvertCypressToWdioSource)
	case "jasmine-jest":
		return executeSourceConverter(source, direction, options, ConvertJasmineToJestSource)
	case "jest-jasmine":
		return executeSourceConverter(source, direction, options, ConvertJestToJasmineSource)
	case "jest-mocha":
		return executeSourceConverter(source, direction, options, ConvertJestToMochaSource)
	case "junit4-junit5":
		return executeSourceConverter(source, direction, options, ConvertJUnit4ToJunit5Source)
	case "junit5-testng":
		return executeSourceConverter(source, direction, options, ConvertJUnit5ToTestNGSource)
	case "nose2-pytest":
		return executeSourceConverter(source, direction, options, ConvertNose2ToPytestSource)
	case "playwright-cypress":
		return executeSourceConverter(source, direction, options, ConvertPlaywrightToCypressSource)
	case "playwright-puppeteer":
		return executeSourceConverter(source, direction, options, ConvertPlaywrightToPuppeteerSource)
	case "playwright-selenium":
		return executeSourceConverter(source, direction, options, ConvertPlaywrightToSeleniumSource)
	case "playwright-webdriverio":
		return executeSourceConverter(source, direction, options, ConvertPlaywrightToWdioSource)
	case "pytest-unittest":
		return executeSourceConverter(source, direction, options, ConvertPytestToUnittestSource)
	case "puppeteer-playwright":
		return executeSourceConverter(source, direction, options, ConvertPuppeteerToPlaywrightSource)
	case "selenium-cypress":
		return executeSourceConverter(source, direction, options, ConvertSeleniumToCypressSource)
	case "selenium-playwright":
		return executeSourceConverter(source, direction, options, ConvertSeleniumToPlaywrightSource)
	case "testng-junit5":
		return executeSourceConverter(source, direction, options, ConvertTestNGToJunit5Source)
	case "testcafe-cypress":
		return executeSourceConverter(source, direction, options, ConvertTestCafeToCypressSource)
	case "testcafe-playwright":
		return executeSourceConverter(source, direction, options, ConvertTestCafeToPlaywrightSource)
	case "unittest-pytest":
		return executeSourceConverter(source, direction, options, ConvertUnittestToPytestSource)
	case "mocha-jest":
		return executeSourceConverter(source, direction, options, ConvertMochaToJestSource)
	case "webdriverio-cypress":
		return executeSourceConverter(source, direction, options, ConvertWdioToCypressSource)
	case "webdriverio-playwright":
		return executeSourceConverter(source, direction, options, ConvertWdioToPlaywrightSource)
	default:
		return ExecutionResult{}, fmt.Errorf("go-native execution is not implemented for %s -> %s", direction.From, direction.To)
	}
}

// ConvertSource runs the Go-native source converter directly on in-memory text for
// a supported direction. It is used by tests and benchmark tooling that need to
// compare converter runtimes without introducing filesystem overhead.
func ConvertSource(direction Direction, input string) (string, error) {
	return convertSourceContent(direction, input)
}

type sourceConverterFunc func(string) (string, error)

func executeSourceConverter(source string, direction Direction, options ExecuteOptions, convert sourceConverterFunc) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeSourceConverterDirectory(source, direction, options, convert)
	}
	return executeSourceConverterFile(source, direction, options, convert)
}

func executeSourceConverterFile(source string, direction Direction, options ExecuteOptions, convert sourceConverterFunc) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := convert(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeSourceConverterDirectory(source string, direction Direction, options ExecuteOptions, convert sourceConverterFunc) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files, err := collectConvertibleSourceFiles(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	batchSize := clampBatchSize(options.BatchSize, len(files))
	workerCount := clampWorkerCount(options.Concurrency, len(files))
	for start := 0; start < len(files); start += batchSize {
		end := start + batchSize
		if end > len(files) {
			end = len(files)
		}
		prepared, err := prepareConvertedOutputs(source, options.Output, files[start:end], direction, convert, workerCount)
		if err != nil {
			return ExecutionResult{}, err
		}
		for _, item := range prepared {
			if err := os.MkdirAll(filepath.Dir(item.outputPath), 0o755); err != nil {
				return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", item.sourcePath, err)
			}
			if err := os.WriteFile(item.outputPath, []byte(item.converted), 0o644); err != nil {
				return ExecutionResult{}, fmt.Errorf("write output file %s: %w", item.outputPath, err)
			}
			if item.changed {
				result.ConvertedCount++
			} else {
				result.UnchangedCount++
			}
			result.Files = append(result.Files, FileResult{
				SourcePath: item.sourcePath,
				OutputPath: item.outputPath,
				Changed:    item.changed,
				Status:     "written",
			})
		}
	}

	return result, nil
}

type preparedConvertedOutput struct {
	sourcePath string
	outputPath string
	converted  string
	changed    bool
}

type preparedConvertedOutputResult struct {
	index int
	item  preparedConvertedOutput
	err   error
}

func collectConvertibleSourceFiles(source string) ([]string, error) {
	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func prepareConvertedOutputs(sourceRoot, outputRoot string, files []string, direction Direction, convert sourceConverterFunc, workerCount int) ([]preparedConvertedOutput, error) {
	if len(files) == 0 {
		return nil, nil
	}
	workerCount = clampWorkerCount(workerCount, len(files))

	type fileJob struct {
		index int
		path  string
	}

	jobs := make(chan fileJob, len(files))
	results := make(chan preparedConvertedOutputResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				input, err := os.ReadFile(job.path)
				if err != nil {
					results <- preparedConvertedOutputResult{
						index: job.index,
						err:   fmt.Errorf("read source file %s: %w", job.path, err),
					}
					continue
				}
				converted, err := convert(string(input))
				if err != nil {
					results <- preparedConvertedOutputResult{
						index: job.index,
						err:   fmt.Errorf("convert %s: %w", job.path, err),
					}
					continue
				}
				rel, err := filepath.Rel(sourceRoot, job.path)
				if err != nil {
					results <- preparedConvertedOutputResult{
						index: job.index,
						err:   fmt.Errorf("compute relative path for %s: %w", job.path, err),
					}
					continue
				}
				results <- preparedConvertedOutputResult{
					index: job.index,
					item: preparedConvertedOutput{
						sourcePath: job.path,
						outputPath: filepath.Join(outputRoot, convertedRelativePath(rel, direction)),
						converted:  converted,
						changed:    converted != string(input),
					},
				}
			}
		}()
	}

	for index, file := range files {
		jobs <- fileJob{index: index, path: file}
	}
	close(jobs)
	wg.Wait()
	close(results)

	prepared := make([]preparedConvertedOutput, len(files))
	var firstErr error
	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			continue
		}
		prepared[result.index] = result.item
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return prepared, nil
}

func resolveOutputFilePath(source, output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) {
		return filepath.Join(output, filepath.Base(source)), nil
	}
	return output, nil
}

func resolveOutputFilePathForDirection(source, output string, direction Direction) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	targetName := convertedOutputBaseName(source, direction)
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		return filepath.Join(output, targetName), nil
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) {
		return filepath.Join(output, targetName), nil
	}
	if filepath.Ext(output) == "" {
		return filepath.Join(output, targetName), nil
	}
	return output, nil
}

func convertedRelativePath(rel string, direction Direction) string {
	base := convertedOutputBaseName(rel, direction)
	dir := filepath.Dir(rel)
	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}

func convertedOutputBaseName(source string, direction Direction) string {
	base := filepath.Base(source)
	switch direction.From + "-" + direction.To {
	case "cypress-playwright":
		return renameCypressSpec(base, ".cy.", ".spec.")
	case "playwright-cypress":
		return renameCypressSpec(base, ".spec.", ".cy.")
	default:
		return base
	}
}

func renameCypressSpec(name, fromFragment, toFragment string) string {
	if strings.Contains(name, fromFragment) {
		return strings.Replace(name, fromFragment, toFragment, 1)
	}
	return name
}

func executeCypressToPlaywright(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeCypressToPlaywrightDirectory(source, direction, options)
	}
	return executeCypressToPlaywrightFile(source, direction, options)
}

func executeCypressToPlaywrightFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertCypressToPlaywrightSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeCypressToPlaywrightDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertCypressToPlaywrightSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeCypressToSelenium(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeCypressToSeleniumDirectory(source, direction, options)
	}
	return executeCypressToSeleniumFile(source, direction, options)
}

func executeCypressToSeleniumFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertCypressToSeleniumSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeCypressToSeleniumDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertCypressToSeleniumSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executePlaywrightToCypress(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executePlaywrightToCypressDirectory(source, direction, options)
	}
	return executePlaywrightToCypressFile(source, direction, options)
}

func executePlaywrightToCypressFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertPlaywrightToCypressSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executePlaywrightToCypressDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertPlaywrightToCypressSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeWdioToPlaywright(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeWdioToPlaywrightDirectory(source, direction, options)
	}
	return executeWdioToPlaywrightFile(source, direction, options)
}

func executeWdioToPlaywrightFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertWdioToPlaywrightSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeWdioToPlaywrightDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertWdioToPlaywrightSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executePlaywrightToWdio(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executePlaywrightToWdioDirectory(source, direction, options)
	}
	return executePlaywrightToWdioFile(source, direction, options)
}

func executePlaywrightToWdioFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertPlaywrightToWdioSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executePlaywrightToWdioDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertPlaywrightToWdioSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeCypressToWdio(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeCypressToWdioDirectory(source, direction, options)
	}
	return executeCypressToWdioFile(source, direction, options)
}

func executeCypressToWdioFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertCypressToWdioSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeCypressToWdioDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertCypressToWdioSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeWdioToCypress(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeWdioToCypressDirectory(source, direction, options)
	}
	return executeWdioToCypressFile(source, direction, options)
}

func executeWdioToCypressFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertWdioToCypressSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeWdioToCypressDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertWdioToCypressSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executePuppeteerToPlaywright(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executePuppeteerToPlaywrightDirectory(source, direction, options)
	}
	return executePuppeteerToPlaywrightFile(source, direction, options)
}

func executePuppeteerToPlaywrightFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertPuppeteerToPlaywrightSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executePuppeteerToPlaywrightDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertPuppeteerToPlaywrightSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executePlaywrightToPuppeteer(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executePlaywrightToPuppeteerDirectory(source, direction, options)
	}
	return executePlaywrightToPuppeteerFile(source, direction, options)
}

func executePlaywrightToPuppeteerFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertPlaywrightToPuppeteerSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executePlaywrightToPuppeteerDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertPlaywrightToPuppeteerSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeSeleniumToCypress(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeSeleniumToCypressDirectory(source, direction, options)
	}
	return executeSeleniumToCypressFile(source, direction, options)
}

func executeSeleniumToCypressFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertSeleniumToCypressSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeSeleniumToCypressDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertSeleniumToCypressSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeSeleniumToPlaywright(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeSeleniumToPlaywrightDirectory(source, direction, options)
	}
	return executeSeleniumToPlaywrightFile(source, direction, options)
}

func executeSeleniumToPlaywrightFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertSeleniumToPlaywrightSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeSeleniumToPlaywrightDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertSeleniumToPlaywrightSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executePlaywrightToSelenium(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executePlaywrightToSeleniumDirectory(source, direction, options)
	}
	return executePlaywrightToSeleniumFile(source, direction, options)
}

func executePlaywrightToSeleniumFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertPlaywrightToSeleniumSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executePlaywrightToSeleniumDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertPlaywrightToSeleniumSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeTestCafeToPlaywright(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeTestCafeToPlaywrightDirectory(source, direction, options)
	}
	return executeTestCafeToPlaywrightFile(source, direction, options)
}

func executeTestCafeToPlaywrightFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertTestCafeToPlaywrightSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeTestCafeToPlaywrightDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertTestCafeToPlaywrightSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func executeTestCafeToCypress(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if info.IsDir() {
		return executeTestCafeToCypressDirectory(source, direction, options)
	}
	return executeTestCafeToCypressFile(source, direction, options)
}

func executeTestCafeToCypressFile(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	input, err := os.ReadFile(source)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read source: %w", err)
	}
	converted, err := ConvertTestCafeToCypressSource(string(input))
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{
		Source:         source,
		Direction:      direction,
		ConvertedCount: 1,
	}
	changed := converted != string(input)

	if strings.TrimSpace(options.Output) == "" {
		result.Mode = "stdout"
		result.StdoutContent = converted
		result.Files = []FileResult{{
			SourcePath: source,
			Changed:    changed,
			Status:     "stdout",
		}}
		return result, nil
	}

	outputPath, err := resolveOutputFilePathForDirection(source, options.Output, direction)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
		return ExecutionResult{}, fmt.Errorf("write output: %w", err)
	}

	result.Mode = "file"
	result.Output = outputPath
	result.Files = []FileResult{{
		SourcePath: source,
		OutputPath: outputPath,
		Changed:    changed,
		Status:     "written",
	}}
	return result, nil
}

func executeTestCafeToCypressDirectory(source string, direction Direction, options ExecuteOptions) (ExecutionResult, error) {
	if strings.TrimSpace(options.Output) == "" {
		return ExecutionResult{}, fmt.Errorf("directory conversion requires --output")
	}

	files := make([]string, 0, 32)
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConvertibleSourceFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("scan source directory: %w", err)
	}

	result := ExecutionResult{
		Source:    source,
		Output:    options.Output,
		Mode:      "directory",
		Direction: direction,
		Files:     make([]FileResult, 0, len(files)),
	}

	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read source file %s: %w", file, err)
		}
		converted, err := ConvertTestCafeToCypressSource(string(input))
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("convert %s: %w", file, err)
		}
		rel, err := filepath.Rel(source, file)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("compute relative path for %s: %w", file, err)
		}
		outputPath := filepath.Join(options.Output, convertedRelativePath(rel, direction))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return ExecutionResult{}, fmt.Errorf("create output directory for %s: %w", file, err)
		}
		if err := os.WriteFile(outputPath, []byte(converted), 0o644); err != nil {
			return ExecutionResult{}, fmt.Errorf("write output file %s: %w", outputPath, err)
		}
		changed := converted != string(input)
		if changed {
			result.ConvertedCount++
		} else {
			result.UnchangedCount++
		}
		result.Files = append(result.Files, FileResult{
			SourcePath: file,
			OutputPath: outputPath,
			Changed:    changed,
			Status:     "written",
		})
	}

	return result, nil
}

func shouldSkipExecutionDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "dist", "build", ".terrain":
		return true
	default:
		return false
	}
}

func isConvertibleSourceFile(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(lower, ".snap") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts", ".java", ".py":
		return true
	default:
		return false
	}
}
