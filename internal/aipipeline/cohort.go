package aipipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cohort labels a repository by its product shape. Cohort selection
// happens once per repo at pipeline start and is cached. The selected
// cohort drives the calibration row used during composition.
type Cohort string

const (
	CohortUnknown        Cohort = "unknown"
	CohortRAGApp         Cohort = "rag-app"
	CohortAgentApp       Cohort = "agent-app"
	CohortAIFeatureInApp Cohort = "ai-feature-in-app"
	CohortMLPipeline     Cohort = "ml-pipeline"
	CohortAIDevTool      Cohort = "ai-dev-tool"
	CohortNotebookHeavy  Cohort = "notebook-heavy"
	CohortLibrarySDK     Cohort = "library-sdk"
)

// RepoShape captures the repo-level signals used to derive a cohort.
// Multi-language: manifests for Python, Node, Go, Java, Rust, Ruby.
type RepoShape struct {
	// Python manifests
	HasSetupPy       bool
	HasPyproject     bool
	HasPyprojectName bool // [project] name= declared

	// Node manifests
	HasPackageJson     bool
	HasPackageJsonName bool // top-level "name" present
	HasPackageJsonPriv bool // "private": true → application-shaped

	// Go manifests
	HasGoMod    bool // go.mod present
	HasGoModule bool // module path declared

	// Java / Kotlin manifests
	HasPomXML      bool
	HasBuildGradle bool

	// Rust manifests
	HasCargoToml bool

	// Ruby manifests
	HasGemfile bool
	HasGemspec bool

	// .NET manifests
	HasCsproj bool

	// Generic release-workflow signal (any language)
	HasReleaseWf bool

	// App-shape signals — multi-language
	HasDockerfile  bool
	HasCompose     bool
	HasFlyToml     bool
	HasVercelJson  bool
	HasNetlifyToml bool
	HasRailwayToml bool
	HasProcfile    bool

	// Entry-point signals — Python
	HasMainPy   bool
	HasAppPy    bool
	HasServerPy bool
	HasManagePy bool // Django

	// Entry-point signals — Node
	HasIndexJS  bool
	HasIndexTS  bool
	HasServerJS bool
	HasServerTS bool

	// Entry-point signals — Go
	HasMainGo bool

	// Directory signals
	HasPromptsDir   bool
	HasEvalsDir     bool
	HasAgentsDir    bool
	HasChainsDir    bool
	HasNotebooksDir bool
	HasPipelinesDir bool
	HasSrcDir       bool
	HasAppDir       bool
	HasApiDir       bool
	HasFrontendDir  bool
	HasBackendDir   bool

	// Topic / language signals (free-form; cohort detector may consume)
	Topics    []string
	Languages []string
}

// IsLibraryShape reports whether the repo's manifest signals suggest
// it ships as a library/package across any supported language.
func (s RepoShape) IsLibraryShape() bool {
	return (s.HasPyprojectName && s.HasReleaseWf) ||
		(s.HasPackageJsonName && !s.HasPackageJsonPriv && s.HasReleaseWf) ||
		(s.HasGoModule && s.HasReleaseWf) ||
		s.HasCargoToml || s.HasGemspec
}

// IsApplicationShape reports whether the repo has app-shaped signals
// (deploy configs, entry-point files, etc.) across any language.
func (s RepoShape) IsApplicationShape() bool {
	if s.HasDockerfile || s.HasCompose || s.HasFlyToml || s.HasVercelJson ||
		s.HasNetlifyToml || s.HasRailwayToml || s.HasProcfile {
		return true
	}
	if s.HasMainPy || s.HasAppPy || s.HasServerPy || s.HasManagePy {
		return true
	}
	if s.HasIndexJS || s.HasIndexTS || s.HasServerJS || s.HasServerTS {
		return true
	}
	if s.HasMainGo {
		return true
	}
	if s.HasPackageJsonPriv {
		return true
	}
	return false
}

// DetectCohort returns the most-specific cohort label given the repo
// shape. Falls back to CohortUnknown when no signal dominates.
//
// The detection order is deliberate:
//
//  1. Specific AI-product shapes win first (RAG, Agent — distinctive directories)
//  2. Library-SDK wins when the repo's primary product IS the package
//  3. ML-pipeline wins when pipeline+app entry exist
//  4. Notebook-heavy wins when notebooks are the primary surface
//  5. Generic AI-feature-in-app catches everything else with app shape
//
// Multi-language: library detection consults setup.py/pyproject (Python),
// package.json (Node), go.mod (Go), Cargo.toml (Rust), Gemspec (Ruby).
// App detection consults entry-point files across all languages.
func DetectCohort(s RepoShape) Cohort {
	// AI-product cohorts win on directory shape regardless of language
	if s.HasPromptsDir && s.HasEvalsDir {
		return CohortRAGApp
	}
	if s.HasAgentsDir || s.HasChainsDir {
		return CohortAgentApp
	}

	// Library shape — multi-language
	if s.IsLibraryShape() {
		return CohortLibrarySDK
	}

	// ML pipeline shape
	if s.HasPipelinesDir && s.IsApplicationShape() {
		return CohortMLPipeline
	}

	// Notebook-heavy without app entry
	if s.HasNotebooksDir && !s.IsApplicationShape() {
		return CohortNotebookHeavy
	}

	// Generic AI feature in an app
	if s.IsApplicationShape() {
		return CohortAIFeatureInApp
	}

	return CohortUnknown
}

// DetectCohortFromDir walks `root` shallowly, populates a RepoShape,
// and returns the cohort label. Errors are surfaced unwrapped so the
// caller can decide whether cohort-unknown is acceptable.
func DetectCohortFromDir(root string) (Cohort, RepoShape, error) {
	shape := RepoShape{}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return CohortUnknown, shape, fmt.Errorf("stat root: %w", err)
	}
	if !rootInfo.IsDir() {
		return CohortUnknown, shape, fmt.Errorf("root is not a directory: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return CohortUnknown, shape, fmt.Errorf("read root: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		lower := strings.ToLower(name)
		if e.IsDir() {
			switch lower {
			case "prompts":
				shape.HasPromptsDir = true
			case "evals", "eval", "evaluations", "evaluation":
				shape.HasEvalsDir = true
			case "agents", "agent":
				shape.HasAgentsDir = true
			case "chains", "chain":
				shape.HasChainsDir = true
			case "notebooks":
				shape.HasNotebooksDir = true
			case "pipelines", "pipeline":
				shape.HasPipelinesDir = true
			case "src":
				shape.HasSrcDir = true
			case "app":
				shape.HasAppDir = true
			case "api":
				shape.HasApiDir = true
			case "frontend", "web", "client":
				shape.HasFrontendDir = true
			case "backend", "server":
				shape.HasBackendDir = true
			}
			continue
		}
		switch lower {
		// Python manifests
		case "setup.py":
			shape.HasSetupPy = true
		case "pyproject.toml":
			shape.HasPyproject = true
			if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				if strings.Contains(string(data), "[project]") &&
					strings.Contains(string(data), "name") {
					shape.HasPyprojectName = true
				}
			}

		// Node manifests
		case "package.json":
			shape.HasPackageJson = true
			if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				text := string(data)
				if strings.Contains(text, `"name"`) {
					shape.HasPackageJsonName = true
				}
				if strings.Contains(text, `"private": true`) ||
					strings.Contains(text, `"private":true`) {
					shape.HasPackageJsonPriv = true
				}
			}

		// Go manifests
		case "go.mod":
			shape.HasGoMod = true
			if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				if strings.Contains(string(data), "module ") {
					shape.HasGoModule = true
				}
			}

		// Java / Kotlin manifests
		case "pom.xml":
			shape.HasPomXML = true
		case "build.gradle", "build.gradle.kts":
			shape.HasBuildGradle = true

		// Rust manifests
		case "cargo.toml":
			shape.HasCargoToml = true

		// Ruby manifests
		case "gemfile":
			shape.HasGemfile = true
		case "gemspec":
			shape.HasGemspec = true

		// Deployment / app signals
		case "dockerfile":
			shape.HasDockerfile = true
		case "compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml":
			shape.HasCompose = true
		case "fly.toml":
			shape.HasFlyToml = true
		case "vercel.json":
			shape.HasVercelJson = true
		case "netlify.toml":
			shape.HasNetlifyToml = true
		case "railway.toml":
			shape.HasRailwayToml = true
		case "procfile":
			shape.HasProcfile = true

		// Python entry points
		case "main.py":
			shape.HasMainPy = true
		case "app.py":
			shape.HasAppPy = true
		case "server.py":
			shape.HasServerPy = true
		case "manage.py":
			shape.HasManagePy = true

		// Node entry points
		case "index.js", "index.mjs", "index.cjs":
			shape.HasIndexJS = true
		case "index.ts":
			shape.HasIndexTS = true
		case "server.js", "server.mjs":
			shape.HasServerJS = true
		case "server.ts":
			shape.HasServerTS = true

		// Go entry points
		case "main.go":
			shape.HasMainGo = true
		}

		// .NET csproj detection (case-insensitive suffix)
		if strings.HasSuffix(lower, ".csproj") {
			shape.HasCsproj = true
		}
		// Ruby gemspec (case-insensitive suffix)
		if strings.HasSuffix(lower, ".gemspec") {
			shape.HasGemspec = true
		}
	}

	// Release workflow check
	wf := filepath.Join(root, ".github", "workflows")
	if items, err := os.ReadDir(wf); err == nil {
		for _, it := range items {
			lower := strings.ToLower(it.Name())
			if strings.Contains(lower, "release") || strings.Contains(lower, "publish") ||
				strings.Contains(lower, "pypi") {
				shape.HasReleaseWf = true
				break
			}
		}
	}

	return DetectCohort(shape), shape, nil
}

// CohortCache holds previously-detected cohort labels for the duration
// of a process. The pipeline consults it before triggering filesystem
// I/O for new roots.
type CohortCache struct {
	entries map[string]Cohort
}

// NewCohortCache returns an empty cache.
func NewCohortCache() *CohortCache { return &CohortCache{entries: map[string]Cohort{}} }

// Lookup returns the cached cohort and whether it was present.
func (c *CohortCache) Lookup(root string) (Cohort, bool) {
	if c == nil {
		return CohortUnknown, false
	}
	v, ok := c.entries[root]
	return v, ok
}

// Set records a cohort for a root.
func (c *CohortCache) Set(root string, cohort Cohort) {
	if c == nil {
		return
	}
	c.entries[root] = cohort
}

// SaveJSON serializes the cache to a JSON file (best-effort).
func (c *CohortCache) SaveJSON(path string) error {
	if c == nil {
		return nil
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
