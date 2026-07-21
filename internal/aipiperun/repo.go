// Package aipiperun is the production-side runner that wires the
// aipipeline stages, calibration, and FS resolver into a single
// "walk a repo and emit findings" entry point. It lives outside the
// aipipeline core package to avoid an import cycle between core and
// stages while keeping the production wire-up in one place.
package aipiperun

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipipeline/fixscaffold"
	"github.com/pmclSF/terrain/internal/aipipeline/stages"
)

// RunRepo walks repoRoot and emits calibrated findings for every
// source file that survives the pipeline.
//
// Pipeline chain (production):
//
//	path-prefilter  → regex-fastscan → ast-confirm →
//	cross-file-scope (FSResolver) → change-scope → composer
//
// Cohort is detected once via DetectCohortFromDir and applied to every
// candidate. The FS resolver memoizes per-directory eval-marker scans
// so cross-file evidence costs O(directories), not O(files²).
//
// Typical use:
//
//	findings, err := aipiperun.RunRepo(ctx, root,
//	    []string{"ai.surface.missing_eval"},
//	    aipipeline.PostureObservability)
//
// Returns findings in walk order. Callers should sort by Confidence
// (descending) or group by Path for presentation.
func RunRepo(ctx context.Context, repoRoot string, rules []string, posture aipipeline.Posture) ([]aipipeline.Finding, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	cohort, _, _ := aipipeline.DetectCohortFromDir(repoRoot)
	resolver := stages.NewFSResolver(repoRoot)

	cal := aipipeline.DefaultCalibration()
	comp := aipipeline.NewComposer(cal, posture)
	// Attach the fix-scaffold registry so every emitted finding carries a
	// runnable protection patch (e.g. an eval-coverage scaffold for
	// ai.surface.missing_eval) plus the path to write it. This turns a
	// finding from a diagnostic into something actionable — and once the
	// scaffold is written and references the surface, the next run resolves
	// the finding (the closed loop).
	comp.Scaffolds = fixscaffold.NewRegistryAdapter(fixscaffold.NewRegistry())
	pipeline := aipipeline.NewPipeline(comp,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(resolver),
		stages.NewChangeScope(),
	)

	var findings []aipipeline.Finding
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !isSourceFileName(d.Name()) {
			return nil
		}
		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		info, statErr := d.Info()
		// !IsRegular rejects a symlink on its own type: without it, a symlink to
		// /dev/zero passes the size check (the link is tiny) and ReadFile then
		// follows it and grows the buffer unbounded.
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > maxRepoFileSize {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, rule := range rules {
			cand := &aipipeline.Candidate{
				Path:   filepath.ToSlash(relPath),
				Lang:   string(aipipeline.LanguageFromPath(relPath)),
				RuleID: rule,
				Cohort: string(cohort),
				Src:    src,
			}
			if _, ok := pipeline.Run(ctx, cand); !ok {
				continue
			}
			f := comp.Compose(cand)
			if !comp.ShouldEmit(f) {
				continue
			}
			findings = append(findings, f)
		}
		return nil
	})
	return findings, err
}

// maxRepoFileSize caps the per-file source read. 1 MB is well above
// any hand-written source file; bigger files are nearly always
// generated, vendored, or fixtures.
const maxRepoFileSize = 1 * 1024 * 1024

// shouldSkipDir mirrors the FS resolver's skip set plus a few
// build-output conventions and Terrain-specific bulk dirs.
func shouldSkipDir(name string) bool {
	switch name {
	// Standard build / dependency / cache dirs across ecosystems.
	case "node_modules", "venv", ".venv", "env", "__pycache__",
		".git", "dist", "build", "target", "out", "bin", "obj",
		".next", ".nuxt", ".cache", ".pytest_cache", ".mypy_cache",
		".ruff_cache", ".tox", ".gradle", ".idea", ".vscode",
		"vendor", "Pods", "DerivedData":
		return true
	// Terrain's runtime cache shouldn't be evaluated as real source.
	case ".terrain":
		return true
	}
	return false
}

// isSourceFileName accepts files whose extension matches a language
// the pipeline knows how to evaluate.
func isSourceFileName(name string) bool {
	for _, ext := range sourceExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

var sourceExts = []string{
	".py",
	".ts", ".tsx", ".js", ".jsx", ".mjs",
	".go",
	".java", ".kt",
	".rs",
	".rb",
	".cs",
}
