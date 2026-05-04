package aidetect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DetectResult holds all AI/ML detection findings for a repository.
type DetectResult struct {
	// Frameworks detected in the repository.
	Frameworks []Framework `json:"frameworks"`

	// EvalConfigs are paths to eval framework config files found.
	EvalConfigs []string `json:"evalConfigs,omitempty"`

	// PromptFiles are source files containing prompt patterns.
	PromptFiles []string `json:"promptFiles,omitempty"`

	// DatasetFiles are source files containing dataset patterns.
	DatasetFiles []string `json:"datasetFiles,omitempty"`

	// ModelFiles are source files with model invocation patterns.
	ModelFiles []string `json:"modelFiles,omitempty"`
}

// maxSourceFileSize is the maximum file size (256 KB) to scan for AI patterns.
// Files larger than this are skipped to avoid slow analysis on generated code.
const maxSourceFileSize = 256 * 1024

// Detect scans a repository root for AI/ML frameworks, prompt patterns,
// dataset usage, and model invocations. No configuration required.
//
// This is a convenience wrapper that uses context.Background(). For
// cancellation support — required by callers driving Terrain from a
// CI workflow with a `--timeout`, or `terrain ai run` invoked from
// within an already-cancelling pipeline — use DetectContext.
func Detect(root string) *DetectResult {
	return DetectContext(context.Background(), root)
}

// DetectContext is like Detect but respects ctx for cancellation. The
// file-walking phase (Phase 3) checks ctx at each entry and aborts
// the walk cleanly when cancelled, returning whatever has been
// collected so far. Phases 1 and 2 (config-file probes and dependency
// manifest reads) are bounded — at most a few stat / open calls — so
// they don't need granular cancellation; we still check ctx between
// phases so a caller cancelling between Phase 2 and Phase 3 doesn't
// pay for the source walk.
//
// Track 5.3 — added in 0.2 to prove cancellation through the AI
// detector path. The pre-0.2 shape (`Detect(root)` only) silently
// ignored ctx, so a slow AI scan would block until the walk
// completed even when the calling pipeline had already cancelled.
func DetectContext(ctx context.Context, root string) *DetectResult {
	result := &DetectResult{}

	// Phase 1: Check config files.
	detectConfigFiles(root, result)
	if ctx.Err() != nil {
		return result
	}

	// Phase 2: Check dependency manifests.
	detectDependencies(root, result)
	if ctx.Err() != nil {
		return result
	}

	// Phase 3: Scan source files for import patterns and AI code patterns.
	detectFromSourceCtx(ctx, root, result)

	// Deduplicate frameworks by name, keeping highest confidence.
	result.Frameworks = deduplicateFrameworks(result.Frameworks)

	return result
}

// detectConfigFiles looks for framework-specific config files at the repo root.
func detectConfigFiles(root string, result *DetectResult) {
	for _, sig := range KnownFrameworks {
		for _, cfgName := range sig.ConfigFiles {
			// Check root and common subdirectories.
			candidates := []string{
				filepath.Join(root, cfgName),
				filepath.Join(root, ".config", cfgName),
			}
			for _, path := range candidates {
				if _, err := os.Stat(path); err == nil {
					rel, _ := filepath.Rel(root, path)
					result.Frameworks = append(result.Frameworks, Framework{
						Name:       sig.Name,
						Source:     "config",
						ConfigFile: rel,
						Confidence: 0.95,
					})
					result.EvalConfigs = append(result.EvalConfigs, rel)
				}
			}
		}
	}
}

// detectDependencies checks package.json and pyproject.toml for AI framework deps.
func detectDependencies(root string, result *DetectResult) {
	// Check package.json
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg map[string]interface{}
		if json.Unmarshal(data, &pkg) == nil {
			allDeps := map[string]string{}
			for _, field := range []string{"dependencies", "devDependencies", "peerDependencies"} {
				if deps, ok := pkg[field].(map[string]interface{}); ok {
					for k, v := range deps {
						if vs, ok := v.(string); ok {
							allDeps[k] = vs
						}
					}
				}
			}

			for _, sig := range KnownFrameworks {
				for _, key := range sig.DependencyKeys {
					if ver, ok := allDeps[key]; ok {
						result.Frameworks = append(result.Frameworks, Framework{
							Name:       sig.Name,
							Version:    ver,
							Source:     "dependency",
							ConfigFile: "package.json",
							Confidence: 0.9,
						})
					}
				}
			}
		}
	}

	// Check pyproject.toml (simple string matching, no TOML parser to avoid deps).
	pyPath := filepath.Join(root, "pyproject.toml")
	if data, err := os.ReadFile(pyPath); err == nil {
		content := string(data)
		for _, sig := range KnownFrameworks {
			for _, key := range sig.DependencyKeys {
				if strings.Contains(content, key) {
					result.Frameworks = append(result.Frameworks, Framework{
						Name:       sig.Name,
						Source:     "dependency",
						ConfigFile: "pyproject.toml",
						Confidence: 0.85,
					})
				}
			}
		}
	}

	// Check requirements.txt
	reqPath := filepath.Join(root, "requirements.txt")
	if data, err := os.ReadFile(reqPath); err == nil {
		content := string(data)
		for _, sig := range KnownFrameworks {
			for _, key := range sig.DependencyKeys {
				if strings.Contains(content, key) {
					result.Frameworks = append(result.Frameworks, Framework{
						Name:       sig.Name,
						Source:     "dependency",
						ConfigFile: "requirements.txt",
						Confidence: 0.8,
					})
				}
			}
		}
	}
}

// Model invocation patterns in source code.
var modelCallPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bOpenAI\b|ChatCompletion|chat\.completions\.create`),
	regexp.MustCompile(`\bAnthropic\b|messages\.create|claude`),
	regexp.MustCompile(`\.(invoke|predict|generate)\s*\(`),
	regexp.MustCompile(`\bLLMChain\b|\bChatPromptTemplate\b|\bPromptTemplate\b`),
	regexp.MustCompile(`\bpipeline\s*\(|\bAutoModel\b|\bAutoTokenizer\b`),
}

// detectFromSource walks source files looking for AI import patterns.
func detectFromSource(root string, result *DetectResult) {
	detectFromSourceCtx(context.Background(), root, result)
}

func detectFromSourceCtx(ctx context.Context, root string, result *DetectResult) {
	// Build pattern index for framework detection.
	type patternEntry struct {
		framework string
		pattern   string
	}
	var patterns []patternEntry
	for _, sig := range KnownFrameworks {
		for _, p := range sig.ImportPatterns {
			patterns = append(patterns, patternEntry{sig.Name, p})
		}
	}

	promptFiles := map[string]bool{}
	datasetFiles := map[string]bool{}
	modelFiles := map[string]bool{}
	frameworkHits := map[string]bool{}

	// fileCount tracks how many files we've examined so the ctx check
	// fires every 64 files rather than on every entry — checking
	// ctx.Err() is a system call on some platforms, and AI detection
	// already walks ~the whole repo, so the per-entry overhead is
	// noticeable.
	fileCount := 0

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Honor cancellation. Returning a non-nil error from WalkDir
		// stops the walk; we use ctx.Err() so callers can distinguish
		// "user cancelled" from "filesystem error" if they choose to
		// inspect the walk error.
		fileCount++
		if fileCount&0x3F == 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		if d.IsDir() {
			// Use the same canonical skip set as walkRepoForConfigs and
			// internal/analysis/repository_scan.go. Pre-0.2.x this site
			// only skipped 5 dirs and would descend into dist/, build/,
			// .terrain/, vendor/, target/, etc. — a major contributor to
			// multi-walk amplification on real repos.
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".js" && ext != ".py" && ext != ".tsx" && ext != ".jsx" && ext != ".mjs" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || len(data) > maxSourceFileSize {
			return nil
		}
		content := string(data)
		rel, _ := filepath.Rel(root, path)

		// Check framework import patterns.
		for _, pe := range patterns {
			if strings.Contains(content, pe.pattern) && !frameworkHits[pe.framework+":"+rel] {
				frameworkHits[pe.framework+":"+rel] = true
				result.Frameworks = append(result.Frameworks, Framework{
					Name:       pe.framework,
					Source:     "import",
					ConfigFile: rel,
					Confidence: 0.75,
				})
			}
		}

		// Check for prompt patterns (broader than code surface detection).
		if hasPromptPatterns(content) {
			promptFiles[rel] = true
		}

		// Check for dataset patterns.
		if hasDatasetPatterns(content) {
			datasetFiles[rel] = true
		}

		// Check for model invocation patterns.
		for _, pat := range modelCallPatterns {
			if pat.MatchString(content) {
				modelFiles[rel] = true
				break
			}
		}

		return nil
	})

	result.PromptFiles = sortedKeyList(promptFiles)
	result.DatasetFiles = sortedKeyList(datasetFiles)
	result.ModelFiles = sortedKeyList(modelFiles)
}

func containsAny(content string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return true
		}
	}
	return false
}

var promptPatterns = []string{
	"PromptTemplate", "ChatPromptTemplate", "SystemMessage",
	"HumanMessage", "AIMessage", "prompt_template",
	"system_prompt", "systemPrompt", "buildPrompt",
	"PROMPT", "prompt =", "template =",
}

var datasetPatterns = []string{
	"load_dataset", "Dataset", "DataLoader", "dataloader",
	"training_data", "eval_data", "test_data",
	"evalDataset", "trainingDataset",
}

func hasPromptPatterns(content string) bool  { return containsAny(content, promptPatterns) }
func hasDatasetPatterns(content string) bool { return containsAny(content, datasetPatterns) }

func deduplicateFrameworks(frameworks []Framework) []Framework {
	best := map[string]Framework{}
	for _, f := range frameworks {
		if existing, ok := best[f.Name]; !ok || f.Confidence > existing.Confidence {
			best[f.Name] = f
		}
	}
	out := make([]Framework, 0, len(best))
	for _, f := range best {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortedKeyList(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
