// Package manifest parses dependency manifests across ecosystems
// (Python, Node, Go, Rust, Ruby) and reports each declared dependency
// alongside its version-pinning state. Used by the reproducibility /
// version-floating rule to flag unpinned dependencies that introduce
// non-determinism into test runs and CI evals.
package manifest

// Ecosystem identifies a package ecosystem.
type Ecosystem string

const (
	EcosystemPython Ecosystem = "python"
	EcosystemNode   Ecosystem = "node"
	EcosystemGo     Ecosystem = "go"
	EcosystemRust   Ecosystem = "rust"
	EcosystemRuby   Ecosystem = "ruby"
)

// Pinning classifies how strictly a dependency's version is constrained.
// The classification feeds the reproducibility rule's risk assessment:
// exact-pinned deps lock the resolved version across runs; range-pinned
// allow patch-or-minor drift; unpinned float freely across versions.
type Pinning string

const (
	// PinningExact: version specifier resolves to a single version on every
	// run (e.g., "foo==1.2.3", "foo @ git+https://...@sha256:abc",
	// package.json "foo": "1.2.3" without ^/~).
	PinningExact Pinning = "exact"

	// PinningRange: version specifier admits multiple compatible versions
	// (e.g., "foo>=1.0,<2.0", "^1.2.3", "~1.2.3", "foo>=1.0").
	PinningRange Pinning = "range"

	// PinningUnpinned: no version specifier at all (e.g., "foo" by itself,
	// package.json "foo": "*" or "latest").
	PinningUnpinned Pinning = "unpinned"

	// PinningGit: version is a VCS reference (branch, tag, or commit).
	// May or may not be reproducible depending on whether the reference
	// is a commit SHA (reproducible) or a moving tag (not).
	PinningGit Pinning = "git"

	// PinningURL: version is a direct URL reference (e.g., tarball URL,
	// non-git VCS). Reproducibility depends on whether the URL is
	// content-addressed.
	PinningURL Pinning = "url"

	// PinningPath: dependency points to a local filesystem path
	// (e.g., "-e ./local-pkg", "file:../sibling"). Reproducible across
	// runs within the same checkout but not portable.
	PinningPath Pinning = "path"

	// PinningUnknown: parser could not classify (malformed spec, unsupported
	// syntax). Treated as a parse-quality issue distinct from PinningUnpinned.
	PinningUnknown Pinning = "unknown"
)

// Section identifies which group a dependency belongs to within a manifest.
// Different manifests use different conventions; this is a normalized view.
type Section string

const (
	SectionRuntime  Section = "runtime"  // production deps
	SectionDev      Section = "dev"      // dev/test-only deps
	SectionBuild    Section = "build"    // build-system requires (e.g., pyproject build-system.requires)
	SectionOptional Section = "optional" // optional / peer / extras
)

// Dependency is a single declared dependency in a manifest.
type Dependency struct {
	// Name is the package name as declared in the manifest. Case is
	// preserved (some ecosystems are case-sensitive).
	Name string

	// Spec is the raw version specifier as written in the manifest
	// (e.g., ">=1.2,<2.0", "^1.2.3", "1.2.3", or "" for unpinned).
	// Empty when the dependency has no version constraint.
	Spec string

	// Pinning classifies the spec.
	Pinning Pinning

	// Section indicates which group the dependency belongs to.
	Section Section

	// Extras lists optional features requested for this dependency
	// (Python PEP-508 syntax: foo[ext1,ext2]). Empty for ecosystems
	// that don't support extras.
	Extras []string

	// Markers carries platform-or-environment markers (Python PEP-508:
	// `; python_version >= '3.10'`). Empty for ecosystems without markers.
	Markers string

	// Line is the 1-based line number in the source manifest, when
	// available. Zero when the parser doesn't preserve line info
	// (e.g., TOML libraries that materialize a tree without positions).
	Line int
}

// Manifest is the parsed contents of one dependency manifest.
type Manifest struct {
	// Path is the repository-relative path to the manifest file.
	Path string

	// Ecosystem identifies the manifest's ecosystem.
	Ecosystem Ecosystem

	// Format identifies the specific manifest format within an ecosystem
	// (e.g., "pyproject.toml", "requirements.txt", "setup.py", "Pipfile.lock").
	// Helps consumers reason about ecosystem-specific semantics.
	Format string

	// Dependencies lists every declared dependency. Order is preserved
	// from the source manifest when the underlying parser allows it.
	Dependencies []Dependency
}
