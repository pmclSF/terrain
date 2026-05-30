package reproducibility

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/parserpool"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectNoSeed AST-walks Python eval / training source for calls into
// stochastic libraries (np.random, torch.rand*, random.random, etc.)
// without a preceding seed call in the same module scope.
// Implements terrain/reproducibility/no-seed.
//
// Heuristics:
//   - The detector only fires on files in eval / training paths
//     (looksLikeStochasticFile below). The intent isn't to flag every
//     random call in a codebase; it's to flag stochastic patterns in
//     contexts where reproducibility is load-bearing.
//   - A file is flagged when a stochastic-library call exists AND no
//     seed call from the same library appears earlier in the module.
//   - One signal per file, attached to the first un-seeded stochastic
//     call site.
func DetectNoSeed(src []byte, relPath string) []models.Signal {
	if len(src) == 0 || !looksLikeStochasticFile(relPath) {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		analyzeSeedingFile(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func looksLikeStochasticFile(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range stochasticPathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var stochasticPathMarkers = []string{
	"/eval/", "/evals/", "/evaluations/", "/__evals__/",
	"/train/", "/training/", "/models/", "/notebooks/",
	"/experiments/",
}

// seedCall classifies a Python call's text against known seeding /
// stochastic primitives. Returns the library family (numpy / torch /
// random / tf), whether the call is a seed setter, and whether it's
// a stochastic source.
type seedCall struct {
	library      string
	isSeed       bool
	isStochastic bool
}

var seedCallClassifiers = []struct {
	pattern string
	library string
	isSeed  bool
}{
	{"np.random.seed(", "numpy", true},
	{"numpy.random.seed(", "numpy", true},
	{"torch.manual_seed(", "torch", true},
	{"torch.cuda.manual_seed", "torch", true},
	{"random.seed(", "random", true},
	{"tf.random.set_seed(", "tf", true},
	{"tensorflow.random.set_seed(", "tf", true},
	{"set_seed(", "any", true}, // HuggingFace transformers helper

	// Stochastic sources.
	{"np.random.", "numpy", false},
	{"numpy.random.", "numpy", false},
	{"torch.rand", "torch", false},
	{"torch.randn", "torch", false},
	{"torch.randint", "torch", false},
	{"random.random(", "random", false},
	{"random.choice(", "random", false},
	{"random.uniform(", "random", false},
	{"tf.random.", "tf", false},
	{"tensorflow.random.", "tf", false},
}

func classifyCallText(text string) (seedCall, bool) {
	for _, c := range seedCallClassifiers {
		if strings.Contains(text, c.pattern) {
			return seedCall{library: c.library, isSeed: c.isSeed, isStochastic: !c.isSeed}, true
		}
	}
	return seedCall{}, false
}

// analyzeSeedingFile walks the tree in order and tracks which libraries
// have been seeded. Emits one signal at the first un-seeded stochastic
// call.
func analyzeSeedingFile(root *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	seeded := map[string]bool{}
	var firstUnseeded *sitter.Node
	var unseededLibrary string

	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "call" {
			text := string(src[n.StartByte():n.EndByte()])
			if call, ok := classifyCallText(text); ok {
				switch {
				case call.isSeed:
					seeded[call.library] = true
					if call.library == "any" {
						// HuggingFace set_seed sets all of numpy / torch / random.
						seeded["numpy"] = true
						seeded["torch"] = true
						seeded["random"] = true
					}
				case call.isStochastic:
					if !seeded[call.library] && firstUnseeded == nil {
						firstUnseeded = n
						unseededLibrary = call.library
					}
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(root)

	if firstUnseeded != nil {
		*out = append(*out, models.Signal{
			Type:             signals.SignalNoSeed,
			Category:         models.CategoryAI,
			Severity:         models.SeverityMedium,
			Confidence:       0.85,
			EvidenceStrength: models.EvidenceModerate,
			EvidenceSource:   models.SourceStructuralPattern,
			Location: models.SignalLocation{
				File: relPath,
				Line: int(firstUnseeded.StartPoint().Row) + 1,
			},
			Explanation: fmt.Sprintf(
				"Stochastic call into %q in %s without a preceding seed in module scope. Run-to-run results will vary, masking real regressions.",
				unseededLibrary, relPath,
			),
			SuggestedAction: fmt.Sprintf(
				"Add a seed call (%s) at module scope or in a pytest fixture.",
				seedExampleFor(unseededLibrary),
			),
			RuleID:          "terrain/reproducibility/no-seed",
			RuleURI:         "docs/rules/reproducibility/no-seed.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"library": unseededLibrary,
			},
		})
	}
}

func seedExampleFor(library string) string {
	switch library {
	case "numpy":
		return "np.random.seed(42)"
	case "torch":
		return "torch.manual_seed(42)"
	case "tf":
		return "tf.random.set_seed(42)"
	case "random":
		return "random.seed(42)"
	}
	return "set_seed(42)"
}
