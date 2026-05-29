// Package data implements the data/* stable rules.
package data

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

// DetectMissingTrainTestSplit AST-walks Python ML training source for
// `.fit(...)` calls that aren't preceded by a train/test split call.
// Implements terrain/data/missing-train-test-split.
//
// Heuristics:
//   - Fires only on files in training paths (looksLikeTrainingFile).
//   - A file fires when it has a `.fit(X, y)`-style call but no
//     preceding split helper (train_test_split, StratifiedKFold,
//     TimeSeriesSplit, KFold, GroupKFold, etc.).
//   - One signal per file at the first un-split fit site.
func DetectMissingTrainTestSplit(src []byte, relPath string) []models.Signal {
	if len(src) == 0 || !looksLikeTrainingFile(relPath) {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		analyzeSplitFile(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func looksLikeTrainingFile(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range trainingPathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var trainingPathMarkers = []string{
	"/train/", "/training/", "/models/", "/notebooks/",
	"/experiments/", "/ml/", "/pipelines/",
}

// splitPrimitives is the vocabulary recognized as splitting data
// before training. Any of these in the file suppresses the rule.
var splitPrimitives = []string{
	"train_test_split(",
	"StratifiedKFold(",
	"KFold(",
	"GroupKFold(",
	"TimeSeriesSplit(",
	"StratifiedShuffleSplit(",
	"ShuffleSplit(",
	"LeaveOneOut(",
	"LeavePOut(",
	".cv_results_", // sklearn cross-validation result marker
	"cross_val_score(",
	"cross_validate(",
	"GridSearchCV(",
	"RandomizedSearchCV(",
}

// fitMethodNames is the set of fit-shaped methods we treat as training.
var fitMethodNames = map[string]bool{
	"fit":             true,
	"fit_transform":   true,
	"partial_fit":     true,
	"train":           true,
	"fit_one_cycle":   true,
}

func analyzeSplitFile(root *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	srcStr := string(src)
	hasSplit := false
	for _, p := range splitPrimitives {
		if strings.Contains(srcStr, p) {
			hasSplit = true
			break
		}
	}
	if hasSplit {
		return
	}

	var firstFit *sitter.Node
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil || firstFit != nil {
			return
		}
		if n.Type() == "call" {
			fn := n.ChildByFieldName("function")
			if fn != nil {
				// Look for `<obj>.fit(...)` or `<obj>.train(...)`.
				name := lastDottedSegment(string(src[fn.StartByte():fn.EndByte()]))
				if fitMethodNames[name] {
					firstFit = n
					return
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(root)

	if firstFit == nil {
		return
	}
	*out = append(*out, models.Signal{
		Type:             signals.SignalMissingTrainTestSplit,
		Category:         models.CategoryAI,
		Severity:         models.SeverityHigh,
		Confidence:       0.8,
		EvidenceStrength: models.EvidenceModerate,
		EvidenceSource:   models.SourceStructuralPattern,
		Location: models.SignalLocation{
			File: relPath,
			Line: int(firstFit.StartPoint().Row) + 1,
		},
		Explanation: fmt.Sprintf(
			"Training call in %s without a preceding train/test split. The model is fit on the full dataset; evaluation against the same data measures memorization, not generalization.",
			relPath,
		),
		SuggestedAction: "Split the dataset before training (sklearn.model_selection.train_test_split, KFold, or TimeSeriesSplit for temporal data).",
		RuleID:          "terrain/data/missing-train-test-split",
		RuleURI:         "docs/rules/data/missing-train-test-split.md",
		DetectorVersion: "0.2.0",
	})
}

func lastDottedSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}
