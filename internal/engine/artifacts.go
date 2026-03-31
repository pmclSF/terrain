package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// ArtifactDiscovery holds the results of scanning for coverage and runtime
// artifacts at known locations within a repository. This enables zero-config
// analysis: when the user doesn't specify --coverage or --runtime, Terrain
// can still find and use common artifact locations automatically.
type ArtifactDiscovery struct {
	// CoveragePath is the discovered coverage artifact path, or empty.
	CoveragePath string `json:"coveragePath,omitempty"`

	// CoverageFormat describes the detected format (lcov, istanbul, go-cover).
	CoverageFormat string `json:"coverageFormat,omitempty"`

	// RuntimePaths are the discovered runtime artifact paths.
	RuntimePaths []string `json:"runtimePaths,omitempty"`

	// RuntimeFormats describes each discovered runtime format.
	RuntimeFormats []string `json:"runtimeFormats,omitempty"`

	// CoverageAutoDetected is true when coverage was found by auto-discovery
	// rather than explicit flag.
	CoverageAutoDetected bool `json:"coverageAutoDetected,omitempty"`

	// RuntimeAutoDetected is true when runtime artifacts were found by
	// auto-discovery rather than explicit flag.
	RuntimeAutoDetected bool `json:"runtimeAutoDetected,omitempty"`
}

// coverageCandidates lists known coverage artifact locations in priority order.
// First match wins. These cover the most common CI and local configurations.
var coverageCandidates = []struct {
	path   string
	format string
}{
	// JavaScript/TypeScript — Jest, Vitest, NYC/Istanbul
	{"coverage/lcov.info", "lcov"},
	{"coverage/coverage-final.json", "istanbul"},
	{"coverage/lcov-report/lcov.info", "lcov"},

	// Istanbul JSON alternatives
	{"coverage-final.json", "istanbul"},
	{".nyc_output/coverage-final.json", "istanbul"},

	// LCOV at root
	{"lcov.info", "lcov"},
	{"coverage.lcov", "lcov"},

	// Go coverage
	{"coverage.out", "go-cover"},
	{"cover.out", "go-cover"},
	{"coverage.txt", "go-cover"},

	// Python — pytest-cov, coverage.py
	{"htmlcov/lcov.info", "lcov"},
	{"coverage.xml", "cobertura"},
	{".coverage", "coverage-py"},

	// CI common locations
	{"test-coverage/lcov.info", "lcov"},
	{"reports/coverage/lcov.info", "lcov"},
}

// runtimeCandidates lists known runtime artifact locations in priority order.
var runtimeCandidates = []struct {
	path   string
	format string
}{
	// JUnit XML — produced by most CI systems, Jest (jest-junit), pytest, Go
	{"junit.xml", "junit-xml"},
	{"test-results.xml", "junit-xml"},
	{"reports/junit.xml", "junit-xml"},
	{"junit/junit.xml", "junit-xml"},
	{"test-results/junit.xml", "junit-xml"},
	{"build/test-results/junit.xml", "junit-xml"},

	// Jest/Vitest JSON
	{"test-results.json", "jest-json"},
	{"jest-results.json", "jest-json"},
	{"reports/test-results.json", "jest-json"},

	// Python pytest
	{"report.xml", "junit-xml"},
	{"pytest-results.xml", "junit-xml"},

	// Go test JSON
	{"test-output.json", "go-test-json"},
}

// walkDirs are directories that commonly contain CI/test output artifacts.
// The bounded walk checks these for known file patterns if the static
// candidate checks find nothing.
var walkDirs = []string{
	"coverage", "test-results", "test-reports", "test-output",
	".nyc_output", "build/reports", "build/test-results",
	"target/surefire-reports", "target/test-results",
	"reports", "junit", "htmlcov",
}

// walkMaxDepth is the maximum directory depth for artifact discovery walks.
const walkMaxDepth = 3

// walkMaxFiles is the maximum number of files examined per walk directory.
const walkMaxFiles = 100

// DiscoverArtifacts scans the repository root for common coverage and runtime
// artifacts. It first checks known paths via os.Stat, then does a bounded walk
// of common CI output directories if the static checks didn't find everything.
func DiscoverArtifacts(root string) *ArtifactDiscovery {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return &ArtifactDiscovery{}
	}

	d := &ArtifactDiscovery{}

	// Phase 1: Static candidates (fast, exact paths).

	// Coverage: first match wins.
	for _, c := range coverageCandidates {
		p := filepath.Join(absRoot, c.path)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Size() > 0 {
			d.CoveragePath = p
			d.CoverageFormat = c.format
			break
		}
	}

	// Runtime: collect all matches (user may have multiple artifact files).
	seen := map[string]bool{}
	for _, r := range runtimeCandidates {
		p := filepath.Join(absRoot, r.path)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Size() > 0 {
			if !seen[p] {
				seen[p] = true
				d.RuntimePaths = append(d.RuntimePaths, p)
				d.RuntimeFormats = append(d.RuntimeFormats, r.format)
			}
		}
	}

	// Phase 2: Bounded directory walk for anything not yet found.
	if d.CoveragePath == "" || len(d.RuntimePaths) == 0 {
		discoverByWalk(absRoot, d, seen)
	}

	return d
}

// discoverByWalk does a bounded walk of common CI output directories,
// looking for coverage and runtime artifacts by file name patterns.
func discoverByWalk(root string, d *ArtifactDiscovery, seen map[string]bool) {
	for _, dir := range walkDirs {
		walkRoot := filepath.Join(root, dir)
		info, err := os.Stat(walkRoot)
		if err != nil || !info.IsDir() {
			continue
		}

		filesExamined := 0
		_ = filepath.Walk(walkRoot, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // skip errors
			}
			if fi.IsDir() {
				// Enforce depth limit.
				rel, _ := filepath.Rel(walkRoot, path)
				depth := strings.Count(rel, string(filepath.Separator))
				if depth >= walkMaxDepth {
					return filepath.SkipDir
				}
				return nil
			}
			filesExamined++
			if filesExamined > walkMaxFiles {
				return filepath.SkipDir
			}
			if fi.Size() == 0 {
				return nil
			}

			name := fi.Name()

			// Coverage patterns.
			if d.CoveragePath == "" {
				if name == "lcov.info" || name == "coverage.lcov" {
					d.CoveragePath = path
					d.CoverageFormat = "lcov"
				} else if name == "coverage-final.json" {
					d.CoveragePath = path
					d.CoverageFormat = "istanbul"
				} else if name == "coverage.out" || name == "cover.out" {
					d.CoveragePath = path
					d.CoverageFormat = "go-cover"
				} else if name == "coverage.xml" {
					d.CoveragePath = path
					d.CoverageFormat = "cobertura"
				}
			}

			// Runtime patterns.
			if !seen[path] {
				if strings.HasSuffix(name, ".xml") && (strings.Contains(name, "junit") || strings.Contains(name, "test-result") || name == "report.xml") {
					seen[path] = true
					d.RuntimePaths = append(d.RuntimePaths, path)
					d.RuntimeFormats = append(d.RuntimeFormats, "junit-xml")
				} else if name == "test-results.json" || name == "jest-results.json" || name == "test-output.json" {
					seen[path] = true
					d.RuntimePaths = append(d.RuntimePaths, path)
					d.RuntimeFormats = append(d.RuntimeFormats, "jest-json")
				}
			}

			return nil
		})
	}
}

// ApplyDiscovery merges auto-discovered artifacts into PipelineOptions,
// respecting explicit flag precedence. Returns a description of what was
// applied for user-facing messaging.
//
// Rules:
//   - If opts.CoveragePath is already set (explicit flag), discovery is skipped.
//   - If opts.RuntimePaths is already set, discovery is skipped.
//   - Only high-confidence artifacts (known formats at known paths) are used.
func ApplyDiscovery(opts *PipelineOptions, discovery *ArtifactDiscovery) []string {
	if discovery == nil {
		return nil
	}

	var messages []string

	// Coverage: apply only if not explicitly set.
	if opts.CoveragePath == "" && discovery.CoveragePath != "" {
		opts.CoveragePath = discovery.CoveragePath
		discovery.CoverageAutoDetected = true
		messages = append(messages,
			"Auto-detected coverage: "+RelativePath(discovery.CoveragePath)+" ("+discovery.CoverageFormat+")")
	}

	// Runtime: apply only if not explicitly set.
	if len(opts.RuntimePaths) == 0 && len(discovery.RuntimePaths) > 0 {
		// Use only the first runtime artifact to avoid noise.
		opts.RuntimePaths = discovery.RuntimePaths[:1]
		discovery.RuntimeAutoDetected = true
		messages = append(messages,
			"Auto-detected runtime: "+RelativePath(discovery.RuntimePaths[0])+" ("+discovery.RuntimeFormats[0]+")")
	}

	return messages
}

// MissingArtifactHints returns user-facing hints for artifacts that were
// not found and not explicitly provided. Hints are tailored to detected
// languages so that irrelevant suggestions (e.g., JUnit XML for a pure
// JS repo) are suppressed. Returns nil if everything is available.
func MissingArtifactHints(opts *PipelineOptions, discovery *ArtifactDiscovery, languages []string) []string {
	var hints []string

	if opts.CoveragePath == "" {
		hint := coverageHint(languages)
		if hint != "" {
			hints = append(hints, hint)
		}
	}

	if len(opts.RuntimePaths) == 0 {
		hint := runtimeHint(languages)
		if hint != "" {
			hints = append(hints, hint)
		}
	}

	return hints
}

func coverageHint(languages []string) string {
	for _, lang := range languages {
		switch lang {
		case "javascript", "typescript":
			return "Coverage not found. Run: npx jest --coverage --coverageReporters=lcov (or provide --coverage <path>)"
		case "go":
			return "Coverage not found. Run: go test -coverprofile=coverage.out ./... (or provide --coverage <path>)"
		case "python":
			return "Coverage not found. Run: pytest --cov --cov-report=lcov (or provide --coverage <path>)"
		case "java":
			return "Coverage not found. Run your build tool's coverage target (or provide --coverage <path>)"
		}
	}
	return "Coverage data not found. Provide with --coverage <path> to unlock coverage signals."
}

func runtimeHint(languages []string) string {
	for _, lang := range languages {
		switch lang {
		case "javascript", "typescript":
			return "Runtime not found. Run: npx jest --json --outputFile=jest-results.json (or provide --runtime <path>)"
		case "go":
			return "Runtime not found. Run: go test -json ./... > test-output.json (or provide --runtime <path>)"
		case "python":
			return "Runtime not found. Run: pytest --junitxml=junit.xml (or provide --runtime <path>)"
		case "java":
			return "Runtime not found. Run: mvn test (JUnit XML in target/surefire-reports/) or provide --runtime <path>"
		}
	}
	return "Runtime data not found. Provide with --runtime <path> to unlock health signals."
}

// RelativePath returns the path relative to the current working directory,
// falling back to the absolute path if the relative form would escape.
func RelativePath(absPath string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return absPath
}
