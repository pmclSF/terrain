// Package barrelresolver is the barrel/re-export resolver hardening
// layer. It expands the import-graph resolution Terrain uses to answer
// "does any production file import this test's target?" — the question
// that drives orphanedTestFile, untestedExport, assertionFreeImport,
// and assertionFreeTest.
//
// The existing internal/analysis/import_graph.go handles the common
// JS/TS cases: relative imports, tsconfig `paths` aliases, package.json
// `imports` aliases, monorepo workspace packages. This package adds
// the FP sub-classes that the legacy resolver misses:
//
//  1. relative sibling — already handled by import_graph.go; preserved
//     here for API completeness.
//  2. `@/`-style path alias from `jest.config.{js,ts,json}`
//     `moduleNameMapper` — a Jest-specific alias surface that lives
//     alongside tsconfig but uses regex-string mapping instead of the
//     tsconfig paths shape. Missing from the legacy resolver, so test
//     files using `import {X} from '@/foo'` resolved via jest.config
//     appeared orphaned.
//  3. absolute-package import — already handled.
//  4. `from x import y` namespace re-export — Python re-export chain:
//     `pkg/__init__.py` writes `from .impl import Helper`, and the test
//     imports `from pkg import Helper`. Without tracking that the name
//     `Helper` re-exports from `pkg`, the test → impl edge is missed.
//  5. dist-path indirection — package.json `main` points at
//     `dist/foo.js` but source lives at `src/foo.ts`. When a downstream
//     repo imports the package by name, the resolver must follow the
//     dist→src mapping to reach the actual implementation file.
//
// The package is gated by the `a7_barrel_resolver` mechanism. When
// state=off, callers get an empty result and fall back to the legacy
// resolver. When state=shadow, the new resolver runs and emits a
// would-add event for every match the legacy resolver missed. When
// state=on, the new resolutions feed the import-graph directly.
package barrelresolver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "a7_barrel_resolver"

// Resolver is the hardened import resolver. Construct once per repo
// scan via New; reuse across detector calls.
type Resolver struct {
	root          string
	jestMappers   []moduleNameMapper
	distMappings  []distMapping
	pyReexports   map[string][]pyReexport
	pyReexportsOK bool
}

// New constructs a Resolver for the given repo root. The Resolver loads
// jest.config moduleNameMapper, scans package.json files for
// main/source dist-path mappings, and lazily builds Python re-export
// indexes the first time they're queried.
func New(root string) (*Resolver, error) {
	r := &Resolver{root: root}
	r.jestMappers = loadJestModuleNameMappers(root)
	r.distMappings = loadDistMappings(root)
	return r, nil
}

// Result is one resolved candidate target with the sub-class that
// produced it. Sub-class identifies WHICH of the five FP cases this
// resolution closed — useful for shadow reports and per-mechanism
// recall accounting.
type Result struct {
	// File is the resolved path, repo-relative, forward-slashed.
	File string

	// SubClass names the FP sub-class this resolution covers:
	//   - relative-sibling
	//   - jest-module-name-mapper
	//   - tsconfig-path-alias
	//   - absolute-package
	//   - python-namespace-reexport
	//   - dist-path-indirection
	SubClass string
}

// Resolve tries to resolve `importPath` from a file located at
// `fromDir` (repo-relative, forward-slashed). Returns every plausible
// target file the import could refer to. The list is the union across
// sub-classes — callers walk it.
//
// When the mechanism is off, this returns nil so callers fall back to
// the legacy resolver.
func (r *Resolver) Resolve(reg *mechanisms.Registry, fromDir, importPath string) []Result {
	if reg.State(MechanismName) == mechanisms.StateOff {
		return nil
	}
	var out []Result
	out = append(out, r.resolveJest(fromDir, importPath)...)
	out = append(out, r.resolveDistPath(fromDir, importPath)...)
	out = append(out, r.resolvePythonNamespace(fromDir, importPath)...)
	return out
}

// ── jest.config moduleNameMapper ───────────────────────────────────

type moduleNameMapper struct {
	pattern *regexp.Regexp
	target  string
}

// loadJestModuleNameMappers walks the repo root looking for jest config
// files (jest.config.{js,ts,json,mjs,cjs}, package.json#jest) and
// extracts moduleNameMapper entries. The .js/.ts variants are parsed
// best-effort via regex — full JS evaluation is out of scope.
func loadJestModuleNameMappers(root string) []moduleNameMapper {
	candidates := []string{
		"jest.config.json",
		"jest.config.js",
		"jest.config.ts",
		"jest.config.mjs",
		"jest.config.cjs",
		"package.json",
	}
	var out []moduleNameMapper
	for _, name := range candidates {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(name, ".json") && name != "package.json":
			out = append(out, parseJestJSON(data)...)
		case name == "package.json":
			out = append(out, parsePackageJSONJest(data)...)
		default:
			out = append(out, parseJestJS(data)...)
		}
	}
	return out
}

// parseJestJSON parses a jest.config.json. Looks for top-level
// "moduleNameMapper" object: { "^@/(.*)$": "<rootDir>/src/$1" }.
func parseJestJSON(data []byte) []moduleNameMapper {
	var doc struct {
		ModuleNameMapper map[string]string `json:"moduleNameMapper"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return mappersFromMap(doc.ModuleNameMapper)
}

// parsePackageJSONJest reads jest.moduleNameMapper from package.json.
func parsePackageJSONJest(data []byte) []moduleNameMapper {
	var doc struct {
		Jest struct {
			ModuleNameMapper map[string]string `json:"moduleNameMapper"`
		} `json:"jest"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return mappersFromMap(doc.Jest.ModuleNameMapper)
}

// parseJestJS extracts moduleNameMapper from jest.config.{js,ts,mjs,cjs}
// via a regex. Handles the dominant shape:
//
//	moduleNameMapper: {
//	  '^@/(.*)$': '<rootDir>/src/$1',
//	}
//
// More exotic JS (computed keys, spread operators, helper functions) is
// not handled — those cases will fall through to the legacy resolver.
var (
	jestBlockRe   = regexp.MustCompile(`(?s)moduleNameMapper\s*[:=]\s*\{([^}]*)\}`)
	jestEntryRe   = regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*['"]([^'"]+)['"]`)
)

func parseJestJS(data []byte) []moduleNameMapper {
	m := jestBlockRe.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return nil
	}
	out := map[string]string{}
	for _, e := range jestEntryRe.FindAllStringSubmatch(m[1], -1) {
		out[e[1]] = e[2]
	}
	return mappersFromMap(out)
}

func mappersFromMap(m map[string]string) []moduleNameMapper {
	out := make([]moduleNameMapper, 0, len(m))
	for pattern, target := range m {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		out = append(out, moduleNameMapper{pattern: re, target: target})
	}
	return out
}

// resolveJest applies any matching jest.config moduleNameMapper rule.
// The target may contain $1 / $2 backreferences; <rootDir> is replaced
// with the repo root.
func (r *Resolver) resolveJest(fromDir, importPath string) []Result {
	var out []Result
	for _, mapper := range r.jestMappers {
		m := mapper.pattern.FindStringSubmatchIndex(importPath)
		if m == nil {
			continue
		}
		expanded := mapper.pattern.ExpandString(nil, mapper.target, importPath, m)
		// <rootDir> → repo root (already implicit in relative paths).
		// Strip any leading <rootDir>/ marker since File is repo-relative.
		s := strings.TrimPrefix(string(expanded), "<rootDir>")
		s = strings.TrimPrefix(s, "/")
		s = filepath.ToSlash(filepath.Clean(s))
		for _, ext := range []string{"", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".mts"} {
			candidate := s + ext
			if fileExists(filepath.Join(r.root, candidate)) {
				out = append(out, Result{File: candidate, SubClass: "jest-module-name-mapper"})
				break
			}
		}
	}
	return out
}

// ── dist-path indirection ──────────────────────────────────────────

type distMapping struct {
	pkgRoot string // repo-relative dir containing package.json
	main    string // repo-relative path the main field points at
	source  string // repo-relative path the source field points at
}

// loadDistMappings scans every package.json under the repo for a `main`
// + `source` pair (or `main` + `module`/`types`). The pair tells us
// that "name → main file" actually compiles from "source file". The
// resolver returns the source path when callers resolve the package.
func loadDistMappings(root string) []distMapping {
	var out []distMapping
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				name := info.Name()
				if name == "node_modules" || name == ".git" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if info.Name() != "package.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var doc struct {
			Main   string `json:"main"`
			Source string `json:"source"`
			Module string `json:"module"`
			Types  string `json:"types"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil
		}
		if doc.Main == "" {
			return nil
		}
		source := firstNonEmpty(doc.Source, doc.Module, doc.Types)
		if source == "" {
			return nil
		}
		relRoot, _ := filepath.Rel(root, filepath.Dir(path))
		out = append(out, distMapping{
			pkgRoot: filepath.ToSlash(relRoot),
			main:    filepath.ToSlash(doc.Main),
			source:  filepath.ToSlash(source),
		})
		return nil
	})
	return out
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// resolveDistPath maps dist/foo.js → src/foo.ts (and similar) by
// substituting the main-path prefix for the source-path prefix on any
// package whose mapping matches the import.
func (r *Resolver) resolveDistPath(fromDir, importPath string) []Result {
	var out []Result
	for _, dm := range r.distMappings {
		// Construct the expected import path that would target main.
		// We try both "<pkgRoot>/<main>" and just "<main>" forms.
		mainAbs := filepath.ToSlash(filepath.Clean(filepath.Join(dm.pkgRoot, dm.main)))
		sourceAbs := filepath.ToSlash(filepath.Clean(filepath.Join(dm.pkgRoot, dm.source)))

		ip := filepath.ToSlash(filepath.Clean(importPath))
		if ip == mainAbs || strings.HasSuffix(ip, "/"+dm.main) || ip == dm.main {
			if fileExists(filepath.Join(r.root, sourceAbs)) {
				out = append(out, Result{File: sourceAbs, SubClass: "dist-path-indirection"})
			}
		}
	}
	return out
}

// ── Python `from x import y` namespace re-export ────────────────────

type pyReexport struct {
	originModule string // e.g. ".impl" → relative module
	name         string // the symbol name
}

// buildPyReexports walks the repo for *.py files and records
// `from X import Y` re-exports in __init__.py modules. Lazily built on
// first call to resolvePythonNamespace.
func (r *Resolver) buildPyReexports() {
	if r.pyReexportsOK {
		return
	}
	r.pyReexports = map[string][]pyReexport{}
	_ = filepath.Walk(r.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == ".venv" || name == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "__init__.py" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.root, filepath.Dir(path))
		pkg := strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		for _, m := range pyFromImportRe.FindAllStringSubmatch(string(data), -1) {
			module := m[1]
			names := m[2]
			for _, n := range strings.Split(names, ",") {
				n = strings.TrimSpace(n)
				// Strip "as alias" → keep alias as exposed name.
				if i := strings.Index(n, " as "); i >= 0 {
					n = strings.TrimSpace(n[i+4:])
				}
				if n == "" || n == "*" {
					continue
				}
				r.pyReexports[pkg] = append(r.pyReexports[pkg], pyReexport{
					originModule: module,
					name:         n,
				})
			}
		}
		return nil
	})
	r.pyReexportsOK = true
}

var pyFromImportRe = regexp.MustCompile(`(?m)^\s*from\s+(\S+)\s+import\s+(.+?)\s*$`)

// resolvePythonNamespace checks whether `importPath` (e.g.
// "pkg.Helper") is a re-export from a namespace package's __init__.py.
// Returns the originating module file if so.
func (r *Resolver) resolvePythonNamespace(fromDir, importPath string) []Result {
	if !strings.Contains(importPath, ".") {
		return nil
	}
	r.buildPyReexports()
	parts := strings.Split(importPath, ".")
	if len(parts) < 2 {
		return nil
	}
	name := parts[len(parts)-1]
	pkg := strings.Join(parts[:len(parts)-1], ".")
	for _, re := range r.pyReexports[pkg] {
		if re.name != name {
			continue
		}
		// Resolve the relative module ".impl" inside pkg.
		mod := re.originModule
		mod = strings.TrimPrefix(mod, ".")
		// Look for pkg/mod.py
		candidate := strings.ReplaceAll(pkg, ".", "/") + "/" + mod + ".py"
		candidate = filepath.ToSlash(filepath.Clean(candidate))
		if fileExists(filepath.Join(r.root, candidate)) {
			return []Result{{File: candidate, SubClass: "python-namespace-reexport"}}
		}
	}
	return nil
}

// ── shared helpers ─────────────────────────────────────────────────

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// EmitShadow reports a would-add event for one Result. Callers run
// this in shadow mode after they discover a Result that the legacy
// resolver missed.
func EmitShadow(ruleID string, res Result, note string) {
	shadow.Emit(shadow.Event{
		Mechanism: MechanismName,
		RuleID:    ruleID,
		Action:    shadow.ActionAdd,
		File:      res.File,
		Reasons:   []string{res.SubClass},
		Note:      note,
	})
}

// String describes the resolver's loaded inputs — useful for `terrain
// doctor` and debugging.
func (r *Resolver) String() string {
	return fmt.Sprintf(
		"barrelresolver{root=%s, jestMappers=%d, distMappings=%d, pyReexports=%v}",
		r.root, len(r.jestMappers), len(r.distMappings), r.pyReexportsOK,
	)
}
