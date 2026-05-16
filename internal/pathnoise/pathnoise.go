// Package pathnoise consolidates corpus-validated false-positive
// path patterns into one canonical filter every detector can consult.
//
// Background: every detector that scans files has historically built
// its own ad-hoc list of "skip these paths" — generated code, test
// fixtures, vendored deps, CI tooling. Each detector reinvented the
// list, accumulated different gaps, and drifted over time. Hand-
// validated corpus reviews this session surfaced FP shapes that
// would have been caught earlier if every detector consulted a
// shared filter.
//
// This package is the canonical implementation. Detectors should
// prefer `pathnoise.IsToolingPath(p)` over inline path matching.
//
// The patterns here are all corpus-validated — each was identified
// during a hand-labeled FP analysis (see tier-4/handlabel/*.tsv).
package pathnoise

import (
	"path/filepath"
	"regexp"
	"strings"
)

// toolingPrefixes are path-component patterns where detector firings
// are virtually always FPs. The list was assembled from corpus hand-
// label sessions on uncoveredAISurface, untestedExport, aiHardcoded-
// APIKey, aiToolWithoutSandbox.
var toolingPrefixes = []string{
	// CI/build infrastructure
	".github/",
	".gitlab/",
	".circleci/",
	".buildkite/",
	".azure-pipelines/",

	// Build / packaging output
	"scripts/",
	"tools/",
	"build-tools/",
	"build/",
	"bin/",
	"dist/",
	"out/",
	"target/",

	// Benchmarks / examples / playground (not production code)
	"benchmarks/",
	"benchmark/",
	"examples/",
	"example/",
	"playground/",
	"apps/playground/",
	"demos/",
	"demo/",

	// Vendored / third-party
	"vendor/",
	"third_party/",
	"_vendor/",
	"node_modules/",

	// Editor / framework build output (added 2026-05-12 after self-fire
	// audit: terrain analyzing its own repo fired [critical] on
	// extension/vscode/out/. Same pattern hits any repo with TS-compiled
	// VS Code extensions / Next.js / Nuxt / SvelteKit / Vite / Turborepo
	// outputs that ride alongside source in the same tree.)
	".next/",
	".nuxt/",
	".svelte-kit/",
	".turbo/",
	".parcel-cache/",
	".vscode-test/",
	"extension/vscode/out/",

	// Python / Rust / Node test + build caches
	"__pycache__/",
	".pytest_cache/",
	".mypy_cache/",
	".ruff_cache/",
	".tox/",
	"htmlcov/",
	".coverage/",
	".nyc_output/",
	".cargo/",

	// Snapshot / golden directories
	"__snapshots__/",
	"goldens/",
	"testdata/golden/",

	// Generated code prefixes
	"generated/",
	"_generated/",
	"proto/",
	"protobuf/",
	"applyconfigurations/", // Kubernetes SDK auto-generated
	"tests-gen/",           // JetBrains/Kotlin auto-generated test stubs

	// Test fixtures + mock-recording libraries
	"tests/data/", "tests/fixtures/",
	"test/data/", "test/fixtures/",
	"testdata/", "__fixtures__/",
	"/placebo/",    // botocore placebo recordings
	"/cassettes/",  // vcrpy / betamax
	"/recordings/", // various test recorders

	// Documentation snippets (frequent false-positive zone)
	"docs/",
}

// fileSuffixes are file-name suffix patterns that indicate generated
// or fixture content regardless of directory.
var fileSuffixes = regexp.MustCompile(
	`(?i)(` +
		// Protobuf bindings
		`\.pb\.go|_pb\.go|_pb2\.py|_pb2\.pyi|\.pb\.cc|\.pb\.h|` +
		// Minified/bundled JS
		`\.min\.js|\.bundle\.js|\.bundle\.css|\.umd\.js|` +
		// General "generated" markers
		`_generated\.[a-z]+|\.generated\.[a-z]+|\.gen\.go|` +
		// Dart codegen
		`\.g\.dart|\.freezed\.dart|` +
		// Common fixture filename markers
		`-fixture\.(?:go|ts|js|py)|\.fixture\.(?:ts|js)|` +
		// Self-declared test-infrastructure files
		`internal-for-testing\.(?:ts|js|py)|` +
		// Lockfiles — detectors should never lint these
		`(?:^|/)(?:yarn|pnpm|package|poetry|Pipfile|Cargo)\.lock|` +
		`(?:^|/)go\.sum|(?:^|/)package-lock\.json|` +
		// VS Code extension packaging output
		`\.vsix` +
		`)$`)

// IsToolingPath returns true when the path is in a corpus-validated
// noise location: CI/build tools, generated code, test fixtures,
// vendored deps, benchmarks, or examples.
//
// Detectors should consult this filter before emitting findings.
// Skipping a tooling-path firing is almost always correct.
func IsToolingPath(path string) bool {
	if path == "" {
		return false
	}
	slashed := filepath.ToSlash(path)
	lower := strings.ToLower(slashed)

	for _, prefix := range toolingPrefixes {
		// Some entries start with "/" (e.g. "/placebo/") to require a
		// preceding slash. Others don't (e.g. "vendor/") and should
		// match either at the path start or after any "/".
		if strings.HasPrefix(prefix, "/") {
			if strings.Contains(lower, prefix) {
				return true
			}
			continue
		}
		if strings.HasPrefix(lower, prefix) {
			return true
		}
		if strings.Contains(lower, "/"+prefix) {
			return true
		}
	}
	if fileSuffixes.MatchString(lower) {
		return true
	}
	return false
}

// IsTestPath returns true when the path is inside a test directory.
// Test-targeting detectors should use this to confirm "yes, fire on
// test paths"; non-test detectors should generally skip these.
func IsTestPath(path string) bool {
	slashed := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(slashed, "/test/") ||
		strings.Contains(slashed, "/tests/") ||
		strings.HasSuffix(slashed, "_test.go") ||
		strings.HasSuffix(slashed, ".test.js") ||
		strings.HasSuffix(slashed, ".test.ts") ||
		strings.HasSuffix(slashed, ".test.tsx") ||
		strings.HasSuffix(slashed, ".test.jsx") ||
		strings.HasSuffix(slashed, "_test.py") ||
		strings.HasPrefix(slashed, "test/") ||
		strings.HasPrefix(slashed, "tests/") ||
		strings.HasSuffix(slashed, "spec.ts") ||
		strings.HasSuffix(slashed, "spec.js") ||
		strings.HasSuffix(slashed, "_spec.rb")
}

// IsGeneratedFileSuffix returns true if the filename suffix indicates
// generated content. Useful when the path itself doesn't hint at it
// (e.g. flat directory structures).
func IsGeneratedFileSuffix(path string) bool {
	return fileSuffixes.MatchString(strings.ToLower(filepath.ToSlash(path)))
}
