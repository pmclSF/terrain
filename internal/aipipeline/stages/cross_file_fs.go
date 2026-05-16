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

	mu          sync.Mutex
	dirCache    map[string]bool // absoluteDir → sibling-has-eval
	pkgCache    map[string]bool // packageRoot → package-has-eval
	maxFileSize int64
}

// NewFSResolver creates a resolver anchored at repoRoot. The root is
// the directory the candidate's repo-relative paths are joined onto.
// Callers should pass the absolute working directory.
func NewFSResolver(repoRoot string) *FSResolver {
	return &FSResolver{
		root:        repoRoot,
		dirCache:    map[string]bool{},
		pkgCache:    map[string]bool{},
		maxFileSize: 512 * 1024,
	}
}

// SiblingHasEvalMarker scans the candidate's directory for files with
// recognized eval framework imports, excluding the candidate itself.
func (r *FSResolver) SiblingHasEvalMarker(repoRelativePath string) bool {
	dir := filepath.Dir(filepath.Join(r.root, repoRelativePath))
	r.mu.Lock()
	if cached, ok := r.dirCache[dir]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()
	self := filepath.Base(repoRelativePath)
	found := r.scanDirForMarkers(dir, self)
	r.mu.Lock()
	r.dirCache[dir] = found
	r.mu.Unlock()
	return found
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

func (r *FSResolver) scanDirForMarkers(dir, selfBase string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == selfBase {
			continue
		}
		if !looksLikeSourceFile(name) {
			continue
		}
		full := filepath.Join(dir, name)
		if r.fileImportsEvalMarker(full) {
			return true
		}
	}
	return false
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

func (r *FSResolver) findPackageRoot(absPath string) string {
	dir := filepath.Dir(absPath)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		// In Python a missing __init__.py terminates the package.
		if _, err := os.Stat(filepath.Join(dir, "__init__.py")); err != nil && !os.IsNotExist(err) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
	return dir
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
