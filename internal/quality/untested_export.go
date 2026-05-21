package quality

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/barrelresolver"
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
)

// UntestedExportDetector identifies exported/public code units that have
// no linked test coverage in the current analysis model.
//
// In mature JVM/Rust codebases this detector tends to fire on stable
// infrastructure code that doesn't change — the maintenance-debt
// reading, not the regression-risk reading. Adopters in those
// ecosystems should pair with `--new-findings-only` to scope firings
// to newly-added untested exports.
//
// Detection uses a layered approach:
//  1. Import graph (highest confidence): if the snapshot includes an import graph,
//     check whether any test file imports the code unit's module. This is precise
//     because it traces actual import/require statements.
//  2. Heuristic fallback (lower confidence): if no import graph is available or
//     the module isn't found in it, fall back to directory/filename-stem proximity.
//
// Limitations:
//   - Import graph only traces static, relative imports. Dynamic imports, path
//     aliases, and barrel re-exports may not be fully resolved.
//   - Heuristic fallback cannot determine actual runtime coverage.
//   - Code tested via integration tests in a different directory may be flagged
//     unless the import graph captures the linkage.
type UntestedExportDetector struct {
	// RepoRoot enables the a7_barrel_resolver mechanism. When set, the
	// detector consults barrelresolver to follow re-export / barrel
	// indirection that the legacy import graph misses (Jest path
	// aliases, dist-path indirection, Python namespace re-exports).
	// Empty disables the barrel-resolver fallback.
	RepoRoot string
}

// toolingPathPrefixes are repo-root-relative path prefixes that we
// treat as build / CI / tooling code and exclude from untested-export
// detection. Verified against the 30-repo non-AI OSS corpus: Angular
// HEAD flagged `deployToFirebase`, `setupRedirect`, `getCredentialFilePath`
// from `.github/actions/deploy-docs-site/` — deployment scripts that
// don't need unit-test coverage. Same logic applies to repo-wide
// helper scripts (`scripts/`), build tooling (`tools/`, `build-tools/`),
// and shipped artifacts (`bin/`, `dist/`).
//
// Tested ANY path-component match, not just leading slash, because
// monorepos commonly have `packages/foo/scripts/build.ts` etc.
var toolingPathPrefixes = []string{
	".github/",
	".gitlab/",
	".circleci/",
	"scripts/",
	"tools/",
	"build-tools/",
	"build/",
	"bin/",
	"dist/",
	"benchmarks/",
	"benchmark/",
	"examples/",
	"example/",
	"vendor/",
	"third_party/",
	"_vendor/",
	"node_modules/",
	"generated/",
	"_generated/",
	"proto/",
	"protobuf/",
	"autogen/",
	// Additional generated-code paths surfaced by labeled-sample review:
	"tests-gen/",       // Kotlin/JetBrains auto-generated test stubs
	"applyconfigurations/", // Kubernetes auto-generated SDK setters
	"apps/playground/", // React/compiler playground demo
	"playground/",      // generic playground apps
	"docs/",            // documentation snippets
}

// generatedFileRe matches generated-code suffixes that should be
// excluded from untested-export detection regardless of directory.
// Discovered on the post-fix corpus audit: 23% of remaining
// untestedExport firings were on .pb.go / _pb2.py / .bundle.js /
// _generated.* files — protobuf bindings, minified JS, and codegen
// outputs that don't need direct unit tests.
var generatedFileRe = regexp.MustCompile(
	`(?i)(_pb2\.py$|\.pb\.go$|_pb\.go$|\.pb\.cc$|\.pb\.h$|` +
		`\.min\.js$|\.bundle\.js$|\.bundle\.css$|` +
		`_generated\.[a-z]+$|\.generated\.[a-z]+$|` +
		`\.g\.dart$|\.freezed\.dart$|\.gen\.go$)`)

// isToolingPath returns true when path lives under a tooling/CI prefix
// anywhere in its directory chain, OR matches a generated-file suffix
// regardless of directory.
func isToolingPath(path string) bool {
	slashed := filepath.ToSlash(path)
	for _, prefix := range toolingPathPrefixes {
		if strings.HasPrefix(slashed, prefix) {
			return true
		}
		if strings.Contains(slashed, "/"+prefix) {
			return true
		}
	}
	if generatedFileRe.MatchString(slashed) {
		return true
	}
	return false
}

// Detect scans code units for untested exports.
func (d *UntestedExportDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal

	if len(snap.CodeUnits) == 0 {
		return nil
	}

	// Layer 1: Build set of source modules imported by test files.
	importedModules := map[string]bool{}
	if snap.ImportGraph != nil {
		for _, imports := range snap.ImportGraph {
			for mod := range imports {
				importedModules[mod] = true
			}
		}
	}
	// Mechanism gate: a7_barrel_resolver. Only when ON does the
	// resolver actively expand the legacy importedModules set; in
	// shadow / off the set stays as the legacy import-graph yielded
	// so downstream user-visible findings are unchanged.
	if d.RepoRoot != "" && mechanisms.Default().State(barrelresolver.MechanismName) == mechanisms.StateOn {
		resolver, err := barrelresolver.New(d.RepoRoot)
		if err == nil {
			for testPath, imports := range snap.ImportGraph {
				fromDir := filepath.Dir(testPath)
				for importPath := range imports {
					for _, res := range resolver.Resolve(mechanisms.Default(), fromDir, importPath) {
						importedModules[res.File] = true
					}
				}
			}
		}
	}
	hasImportGraph := len(importedModules) > 0

	// Layer 2: Build heuristic sets (directories and filename stems).
	testDirs := map[string]bool{}
	testStems := map[string]bool{}
	for _, tf := range snap.TestFiles {
		dir := filepath.Dir(tf.Path)
		testDirs[dir] = true
		// Also consider parent dir (for __tests__/ convention)
		if filepath.Base(dir) == "__tests__" {
			testDirs[filepath.Dir(dir)] = true
		}

		// Extract stem: auth.test.js -> auth
		base := filepath.Base(tf.Path)
		stem := stripTestSuffix(base)
		if stem != "" {
			testStems[stem] = true
		}
	}

	for _, cu := range snap.CodeUnits {
		if !cu.Exported {
			continue
		}
		// Skip code units in CI / tooling / build paths. These are
		// deploy scripts, build helpers, benchmarks, and examples —
		// not the production exports the detector targets. Verified
		// on the OSS corpus: this drops Angular's
		// `.github/actions/deploy-docs-site/` firings (5 sample
		// firings audited, 100% non-actionable for users).
		if isToolingPath(cu.Path) {
			continue
		}

		// Calibration-driven filters:
		//
		// (a) Filenames that self-declare as test infrastructure:
		//     internal-for-testing.ts, test-helpers.*, *-fixture.*, etc.
		//     bun has `internal-for-testing.ts` — the name announces it.
		basename := strings.ToLower(filepath.Base(cu.Path))
		if strings.Contains(basename, "internal-for-testing") ||
			strings.Contains(basename, "test-helpers") ||
			strings.Contains(basename, "testhelpers") ||
			strings.HasSuffix(basename, "-fixture.go") ||
			strings.HasSuffix(basename, "-fixture.ts") ||
			strings.HasSuffix(basename, ".fixture.ts") ||
			strings.HasSuffix(basename, ".fixture.js") {
			continue
		}

		// (b) Go test functions matching ^Test[A-Z] are tests, not
		// production exports. Same convention as `go test`. Hand-labeled
		// go-ethereum/v5test/discv5tests.go `TestHandshakeResend`
		// surfaced this — Go's exported-Test pattern crosses the
		// "production export" line accidentally.
		if (strings.HasSuffix(cu.Path, ".go") || strings.HasSuffix(cu.Path, "_test.go")) &&
			isGoTestFunction(cu.Name) {
			continue
		}

		// (c) Standard JVM Object method overrides — toString,
		// hashCode, equals, finalize. Framework-mandated, every class
		// has them, no separate test expected. Same for AutoCloseable
		// close() and Comparable compareTo().
		if (strings.HasSuffix(cu.Path, ".java") || strings.HasSuffix(cu.Path, ".kt")) &&
			isJVMOverrideMethod(cu.Name) {
			continue
		}

		// (d) Type/schema declaration exports get flagged as "untested"
		// but they're not callable behavior: Zod schemas
		// (CamelCaseSchema), React component Props types (FooProps),
		// config/params/request/response interfaces. Type aliases, not
		// behavior — no unit test expected.
		if (strings.HasSuffix(cu.Path, ".ts") || strings.HasSuffix(cu.Path, ".tsx") ||
			strings.HasSuffix(cu.Path, ".js") || strings.HasSuffix(cu.Path, ".jsx")) &&
			isTypeOrSchemaDecl(cu.Name) {
			continue
		}

		cuPath := filepath.ToSlash(cu.Path)
		cuDir := filepath.Dir(cuPath)
		cuStem := stripExt(filepath.Base(cuPath))

		// Layer 1: Check import graph — if any test imports this module, it's tested.
		if hasImportGraph && importedModules[cuPath] {
			continue // Tested via direct import — high confidence, no signal.
		}

		// Layer 2: Heuristic — check directory/stem proximity.
		hasNearbyTest := testDirs[cuDir] || testStems[cuStem]

		if hasNearbyTest {
			continue // Heuristic says it's likely tested.
		}

		// Determine confidence based on what evidence we have.
		confidence := 0.5
		evidenceStrength := models.EvidenceWeak
		if hasImportGraph {
			// Import graph was available but didn't find a link — higher confidence
			// that this is genuinely untested.
			confidence = 0.7
			evidenceStrength = models.EvidenceModerate
		}

		signals = append(signals, models.Signal{
			Type:             "untestedExport",
			Category:         models.CategoryQuality,
			Severity:         models.SeverityMedium,
			Confidence:       confidence,
			EvidenceStrength: evidenceStrength,
			EvidenceSource:   models.SourcePathName,
			Location: models.SignalLocation{
				File:   cu.Path,
				Symbol: cu.Name,
			},
			Explanation: "Exported " + string(cu.Kind) + " \"" + cu.Name +
				"\" has no linked tests in the current analysis model.",
			// In some ecosystems (JVM/Rust monorepos especially) this
			// detector identifies stable infrastructure rather than
			// regression risk. Adopters who want regression-focused
			// gating should pair it with --new-findings-only to scope
			// to changing code.
			SuggestedAction: "Add direct tests for this exported behavior or improve test-to-code linkage. " +
				"For regression-focused gating, use `terrain analyze --baseline <prev> --new-findings-only` " +
				"so this rule only fires on newly-added untested exports rather than long-standing debt.",
		})
	}

	return signals
}

// isGoTestFunction returns true for Go function names matching `Test[A-Z]…`,
// the convention `go test` uses to discover tests. These ARE tests even when
// they happen to be exported — flagging them as "untested exports" is a
// category error.
func isGoTestFunction(name string) bool {
	if len(name) < 5 || !strings.HasPrefix(name, "Test") {
		return false
	}
	r := name[4]
	return r >= 'A' && r <= 'Z'
}

// isJVMOverrideMethod returns true for methods that are nearly-universal
// JVM Object / interface overrides — `toString`, `hashCode`, `equals`,
// `finalize`, `clone`, `compareTo`, `close`, `dispose`. Per-class tests for
// these are vanishingly rare in practice; the interface contract usually has
// its own conformance tests.
func isJVMOverrideMethod(name string) bool {
	switch name {
	case "toString", "hashCode", "equals", "finalize", "clone",
		"compareTo", "close", "dispose", "reset", "init", "<init>",
		"readResolve", "writeReplace":
		return true
	}
	return false
}

// typeOrSchemaSuffixes are name-suffix conventions for type / schema /
// configuration declarations. These are non-callable declarations
// (Zod schemas, TypeScript type aliases, config interfaces, DTO
// definitions) — adding unit tests for them isn't expected.
//
// PascalCase prefix check is intentional — a function named `parseSchema`
// (camelCase) does behavior and should NOT be excluded. Only PascalCase
// declarations matching these suffixes get filtered.
var typeOrSchemaSuffixes = []string{
	"Schema", "Props", "Type", "Config", "Params", "Request", "Response",
	"Options", "Settings", "Args", "Input", "Output", "Variables", "Result",
	"State", "Context",
}

// isTypeOrSchemaDecl returns true when name is a PascalCase identifier
// ending in a type/schema/config suffix. Treats `FooSchema`, `UserProps`,
// `ApiResponse` as non-behavioral type exports.
func isTypeOrSchemaDecl(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Must start uppercase (PascalCase).
	if name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for _, suffix := range typeOrSchemaSuffixes {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			return true
		}
	}
	return false
}

// stripTestSuffix removes test/spec suffixes to get the base module name.
// "auth.test.js" -> "auth", "auth.spec.ts" -> "auth"
func stripTestSuffix(filename string) string {
	name := stripExt(filename)
	name = strings.TrimSuffix(name, ".test")
	name = strings.TrimSuffix(name, ".spec")
	name = strings.TrimSuffix(name, "_test")
	name = strings.TrimPrefix(name, "test_")
	return name
}

func stripExt(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}
