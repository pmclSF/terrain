// Package aiclassify holds shared classification helpers for AI/ML
// asset detection — prompt files, eval files, code-gen templates, etc.
//
// Purpose: consolidate the prompt-detection heuristic that previously
// lived only in cmd/terrain-corpus/harvest_boundaries.go. The analyzer's
// CodeSurface extractor (internal/analysis/analyzer.go via
// inferCodeSurfacesCachedCtx) currently has its own prompt-detection
// path; that path will be migrated to consume these helpers in a
// follow-up so both surfaces agree on "what counts as a prompt file."
//
// Calibration surfaced a real divergence where the boundary scanner
// detected a prompt file but the analyzer's surface extractor did
// not. That kind of drift is what this package exists to prevent.
package aiclassify

import (
	"path/filepath"
	"strings"
)

// IsPromptFile returns true if `rel` (a repo-relative path) looks like
// a prompt template asset. Heuristic; favors precision over recall.
//
// Recognized shapes:
//   - extension `.prompt`
//   - suffix `.prompt.yaml` / `.prompt.yml`
//   - bare jinja (`.j2` / `.jinja` / `.jinja2`) UNDER a recognized
//     prompt directory (`prompts/`, `prompt/`, `prompt_templates/`,
//     `system_prompts/`, `chat_templates/`)
//   - yaml/json/markdown/text/tmpl under a recognized prompt directory
//
// Rejects code-gen jinja templates (`.X.j2` / `.X.jinja2` where X is a
// programming-language extension) — those are code generation, not
// prompts. See IsCodeGenTemplate.
func IsPromptFile(rel string) bool {
	low := strings.ToLower(rel)
	ext := strings.ToLower(filepath.Ext(rel))

	if IsCodeGenTemplate(low) {
		return false
	}

	switch {
	case ext == ".prompt":
		return true
	case strings.HasSuffix(low, ".prompt.yaml"), strings.HasSuffix(low, ".prompt.yml"):
		return true
	}

	if !inPromptDir(low) {
		return false
	}
	switch ext {
	case ".j2", ".jinja", ".jinja2":
		return true
	case ".yaml", ".yml", ".json", ".md", ".txt", ".tmpl":
		return true
	}
	return false
}

// inPromptDir returns true if `low` (lowercased path) has a recognized
// prompt directory segment anywhere along its path, including the path
// prefix (for relative paths that start with `prompts/...`).
func inPromptDir(low string) bool {
	return hasSegment(low, "prompts") ||
		hasSegment(low, "prompt") ||
		hasSegment(low, "prompt_templates") ||
		hasSegment(low, "system_prompts") ||
		hasSegment(low, "chat_templates")
}

// hasSegment returns true if `name/` appears as a path segment in `p`,
// either as a substring `/name/` or as a leading prefix `name/`. Handles
// the relative-vs-absolute path difference.
func hasSegment(p, name string) bool {
	if strings.Contains(p, "/"+name+"/") {
		return true
	}
	if strings.HasPrefix(p, name+"/") {
		return true
	}
	if strings.HasSuffix(p, "/"+name) {
		return true
	}
	return false
}

// IsCodeGenTemplate returns true when the filename matches a known
// code-generation template shape — `.X.j2` or `.X.jinja2` where X is a
// programming-language extension, or files under known code-gen
// directories (`_templates/`, `generators/`).
//
// Refined against the calibration set, where code-gen jinja templates
// produced a meaningful share of boundary false positives.
func IsCodeGenTemplate(rel string) bool {
	low := strings.ToLower(rel)

	codeExtBeforeTemplate := []string{
		".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".rs", ".java", ".kt",
		".c", ".cc", ".cpp", ".h", ".hpp", ".m", ".swift",
		".yml", ".yaml", ".tf", ".toml", ".ini",
		".sh", ".bash", ".zsh", ".dockerfile", ".mk",
		".html", ".css", ".sql",
	}
	for _, codeExt := range codeExtBeforeTemplate {
		for _, sfx := range []string{".j2", ".jinja", ".jinja2"} {
			if strings.HasSuffix(low, codeExt+sfx) {
				return true
			}
		}
	}
	if hasSegment(low, "_templates") || hasSegment(low, "generators") {
		return true
	}
	return false
}
