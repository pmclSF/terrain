package stages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// FSResolver is the production CrossFileResolver. It walks the
// filesystem rooted at a repo and answers eval-marker questions by
// scanning candidate sibling files for known framework imports.
//
// FSResolver caches per-directory results so that batched candidates
// in the same package share a single scan. The cache is bounded only
// by the size of the repo's directory tree, which is fine for typical
// runs (a few hundred directories).
type FSResolver struct {
	root string

	mu sync.Mutex
	// dirCache stores the SET of basenames in a directory that contain
	// eval markers. Cache key is the absolute directory. Caller checks
	// whether the set is non-empty after subtracting its own basename
	// — this lets us cache the directory scan once and still get
	// correct per-candidate "is there a sibling with eval" answers.
	dirCache    map[string]map[string]struct{}
	pkgCache    map[string]bool
	maxFileSize int64
}

// NewFSResolver creates a resolver anchored at repoRoot. The root is
// the directory the candidate's repo-relative paths are joined onto.
// Callers should pass the absolute working directory.
func NewFSResolver(repoRoot string) *FSResolver {
	return &FSResolver{
		root:        repoRoot,
		dirCache:    map[string]map[string]struct{}{},
		pkgCache:    map[string]bool{},
		maxFileSize: 512 * 1024,
	}
}

// SiblingHasEvalMarker scans the candidate's directory for files with
// recognized eval framework imports, excluding the candidate itself.
//
// Cache invariant: dirCache holds the full set of marker-bearing files
// per directory. Two candidates from the same directory hit one scan,
// and each gets a correct answer because we subtract the candidate's
// own basename at check time. Caching just `bool` is unsafe — the
// first candidate's self-exclusion would poison every subsequent
// candidate's lookup (this exact bug was found 2026-05-15).
func (r *FSResolver) SiblingHasEvalMarker(repoRelativePath string) bool {
	dir := filepath.Dir(filepath.Join(r.root, repoRelativePath))
	self := filepath.Base(repoRelativePath)
	markers := r.markersInDir(dir)
	for name := range markers {
		if name != self {
			return true
		}
	}
	return false
}

// markersInDir returns the set of file basenames in `dir` that import
// eval frameworks. Results are cached per directory.
func (r *FSResolver) markersInDir(dir string) map[string]struct{} {
	r.mu.Lock()
	if cached, ok := r.dirCache[dir]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()
	out := map[string]struct{}{}
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !looksLikeSourceFile(name) {
				continue
			}
			if r.fileImportsEvalMarker(filepath.Join(dir, name)) {
				out[name] = struct{}{}
			}
		}
	}
	r.mu.Lock()
	r.dirCache[dir] = out
	r.mu.Unlock()
	return out
}

// PackageHasEvalMarker walks the candidate's package looking for eval
// markers in any reachable file. Packages are inferred from common
// language conventions: a Python package is the chain of directories
// containing __init__.py; a Node/TS package is the directory up to
// the nearest package.json; otherwise we walk up two levels.
func (r *FSResolver) PackageHasEvalMarker(repoRelativePath string) bool {
	pkgDir := r.findPackageRoot(filepath.Join(r.root, repoRelativePath))
	r.mu.Lock()
	if cached, ok := r.pkgCache[pkgDir]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()
	found := r.scanPackageForMarkers(pkgDir, filepath.Base(repoRelativePath))
	r.mu.Lock()
	r.pkgCache[pkgDir] = found
	r.mu.Unlock()
	return found
}

func (r *FSResolver) scanPackageForMarkers(pkgDir, selfBase string) bool {
	found := false
	_ = filepath.WalkDir(pkgDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == selfBase {
			return nil
		}
		if !looksLikeSourceFile(d.Name()) {
			return nil
		}
		if r.fileImportsEvalMarker(path) {
			found = true
		}
		return nil
	})
	return found
}

// findPackageRoot walks upward from absPath looking for a package-root
// marker (package.json for JS/TS, __init__.py-terminated chain for
// Python). The walk is bounded by:
//
//   1. The repo root r.root — escaping it would scan unrelated
//      filesystem regions (this caused a hard hang on synthetic
//      single-file repos where the walk reached `/` and then scanned
//      the entire filesystem).
//   2. A fixed depth of 6 levels — repos with deeper packages get the
//      6th ancestor, still bounded by r.root.
//
// Returns the candidate's own directory when no marker is found and
// the bound is reached — the safe default scans only the immediate
// directory and is consistent with SiblingHasEvalMarker behavior.
func (r *FSResolver) findPackageRoot(absPath string) string {
	absRoot, _ := filepath.Abs(r.root)
	dir := filepath.Dir(absPath)
	candidateDir := dir
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		// In Python a missing __init__.py terminates the package.
		// (The original logic here was inverted — it returned on an
		// error other than ENOENT, which never fires in practice.)
		if i > 0 {
			if _, err := os.Stat(filepath.Join(dir, "__init__.py")); os.IsNotExist(err) {
				// Reached a directory without __init__.py: the
				// previous level was the package root.
				return candidateDir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return candidateDir
		}
		// Guard against escaping the repo root. Without this the walk
		// reaches `/` on minimal synthetic repos (no package.json, no
		// __init__.py chain) and scanPackageForMarkers then walks the
		// entire filesystem.
		if absRoot != "" && !pathInside(parent, absRoot) {
			return candidateDir
		}
		candidateDir = dir
		dir = parent
	}
	return candidateDir
}

// pathInside reports whether `child` is at or below `parent` in the
// filesystem hierarchy. Both paths are assumed absolute.
func pathInside(child, parent string) bool {
	if child == parent {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func (r *FSResolver) fileImportsEvalMarker(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.Size() > r.maxFileSize {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return evalMarkerRE.Match(data)
}

func looksLikeSourceFile(name string) bool {
	for _, ext := range sourceExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func shouldSkipDir(name string) bool {
	switch name {
	case "node_modules", "venv", ".venv", "__pycache__", ".git", "dist", "build", "target":
		return true
	}
	return false
}

var (
	sourceExts = []string{".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".go", ".rb"}

	// evalMarkerRE matches imports of recognized eval / test / metric
	// frameworks. The regex is intentionally permissive — partial
	// substring matches are acceptable because false positives here
	// only suppress findings, never produce new ones. Markers cover:
	//   Python:   pytest, deepeval, ragas, promptfoo, mlflow, wandb,
	//             tensorboard, langsmith, trulens
	//   Node/TS:  jest, vitest, mocha, ava, langsmith, deepeval-ts,
	//             promptfoo (via require), playwright (for agent UIs)
	//   Go:       testing (stdlib), evals/ subdirectory imports
	evalMarkerRE = regexp.MustCompile(
		`(?:import|from|require)[\s(]*['"]?` +
			`(?:pytest|deepeval|ragas|promptfoo|mlflow|wandb|tensorboard|langsmith|trulens|jest|vitest|mocha|ava|playwright)` +
			`(?:[/.\w-]*)?['"]?`)
)
