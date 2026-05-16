package stages

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// CrossFileResolver answers per-file questions that require visibility
// into sibling files in the candidate's package or directory. The
// pipeline injects a resolver implementation that knows how to walk the
// real filesystem in production; the validation harness leaves it nil,
// in which case the stage emits no atoms.
//
// Implementations MUST be concurrency-safe — the pipeline may call them
// from multiple goroutines when batching candidates.
type CrossFileResolver interface {
	// SiblingHasEvalMarker returns true when any file in the same
	// directory as the candidate (or in a sibling tests/ directory at
	// the same level) imports a recognized eval/test framework. The
	// recognized set is documented in the production resolver: pytest,
	// deepeval, ragas, promptfoo, mlflow, wandb, tensorboard, jest,
	// vitest, mocha, langsmith.
	//
	// The candidate's own file is excluded from the scan. A true result
	// means the team already runs evals nearby, so a "missing eval"
	// finding on this file is almost certainly a false positive.
	SiblingHasEvalMarker(repoRelativePath string) bool

	// PackageHasEvalMarker is like SiblingHasEvalMarker but checks the
	// candidate's whole package — all files under the same Python/Go
	// module, or all files reachable by relative import in JS/TS. This
	// catches the case where the evals live in a parallel tests/
	// directory at the package root rather than inline.
	PackageHasEvalMarker(repoRelativePath string) bool
}

// CrossFileScope is Stage 4: cross-file eval-presence detection. It
// emits negative atoms when the candidate's package already runs evals
// — the strongest signal we have that "missing eval" is wrong here.
//
// In Observability mode without a resolver (e.g. the corpus validator),
// the stage is a no-op. In production the Pipeline constructor injects
// a filesystem-walking resolver.
type CrossFileScope struct {
	Resolver CrossFileResolver
}

// NewCrossFileScope returns the stage. Pass nil for the corpus case;
// pass a real CrossFileResolver in production.
func NewCrossFileScope(r CrossFileResolver) *CrossFileScope {
	return &CrossFileScope{Resolver: r}
}

// Name implements pipeline.Stage.
func (s *CrossFileScope) Name() string { return "cross-file-scope" }

// Run emits scope atoms when a sibling or package-mate file contains
// eval-framework imports.
func (s *CrossFileScope) Run(_ context.Context, c *aipipeline.Candidate) aipipeline.StageResult {
	if s == nil || s.Resolver == nil {
		return aipipeline.StageResult{Continue: true}
	}
	if c.Path == "" {
		return aipipeline.StageResult{Continue: true}
	}
	if s.Resolver.SiblingHasEvalMarker(c.Path) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceScope,
			RuleID: "scope.sibling_has_eval",
			Source: "cross-file-scope",
			Weight: -1.8,
			Span:   aipipeline.Span{Snippet: filepath.Dir(c.Path)},
		})
		return aipipeline.StageResult{Continue: true}
	}
	if s.Resolver.PackageHasEvalMarker(c.Path) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceScope,
			RuleID: "scope.package_has_eval",
			Source: "cross-file-scope",
			Weight: -1.4,
			Span:   aipipeline.Span{Snippet: packageRoot(c.Path)},
		})
	}
	return aipipeline.StageResult{Continue: true}
}

// packageRoot returns a short directory label suitable for evidence
// rendering ("src/agents" rather than "src/agents/openai_helper.py").
func packageRoot(p string) string {
	dir := filepath.Dir(p)
	if dir == "." || dir == "" {
		return p
	}
	return strings.ReplaceAll(dir, string(filepath.Separator), "/")
}
