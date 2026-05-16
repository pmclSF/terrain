// Package hygiene implements the §9 hygiene/* stable rules — pattern-
// based code-quality checks that don't depend on runtime artifacts.
package hygiene

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

// DetectEvalNoAssertion AST-walks Python eval test files and emits a
// Signal for any test function that has no assertion / score / metric
// call. Implements terrain/hygiene/eval-no-assertion.
//
// Heuristics:
//   - Treats the file as an eval test when its path contains
//     "/eval", "/evaluations/", "/evals/", "/__evals__/", or
//     "/benchmarks/".
//   - Walks every `def test_*` function and inspects its body for any
//     of the assertion vocabulary: `assert`, `expect`, `pytest.fail`,
//     `unittest.TestCase` method calls (assertEqual, assertTrue, ...),
//     `score = ...`, `metric.add(...)`, eval-framework assertion calls
//     (`assert_response`, `evaluator.assert_*`, ragas / deepeval
//     metric usage).
//   - A test that's empty or contains only print/log calls without an
//     assertion-shaped check fires the rule.
//
// Returns nil on parse failure (callers handle the absence).
func DetectEvalNoAssertion(src []byte, relPath string) []models.Signal {
	if len(src) == 0 || !looksLikeEvalFile(relPath) {
		return nil
	}

	var out []models.Signal
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		walkPyTestFns(tree.RootNode(), src, relPath, &out)
		return nil
	})
	return out
}

func looksLikeEvalFile(path string) bool {
	// Normalize: add leading / and convert windows paths so the markers
	// match positionally regardless of how the caller passed the path.
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range evalPathMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var evalPathMarkers = []string{
	"/eval/", "/evals/", "/evaluations/", "/__evals__/", "/benchmarks/",
}

func walkPyTestFns(node *sitter.Node, src []byte, relPath string, out *[]models.Signal) {
	if node == nil {
		return
	}
	if node.Type() == "function_definition" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			name := string(src[nameNode.StartByte():nameNode.EndByte()])
			if isPyTestFunction(name) {
				if !hasAssertionShape(node, src) {
					*out = append(*out, models.Signal{
						Type:             signals.SignalEvalNoAssertion,
						Category:         models.CategoryAI,
						Severity:         models.SeverityHigh,
						Confidence:       0.85,
						EvidenceStrength: models.EvidenceModerate,
						EvidenceSource:   models.SourceStructuralPattern,
						Location: models.SignalLocation{
							File:   relPath,
							Symbol: name,
							Line:   int(node.StartPoint().Row) + 1,
						},
						Explanation: fmt.Sprintf(
							"Eval test %q in %s has no assertion / score / metric call. The test runs to completion regardless of model output, so it can't detect regressions.",
							name, relPath,
						),
						SuggestedAction: "Add an assert / score check that fails when the eval output deviates from expectations.",
						RuleID:          "terrain/hygiene/eval-no-assertion",
						RuleURI:         "docs/rules/hygiene/eval-no-assertion.md",
						DetectorVersion: "0.2.0",
						Metadata: map[string]any{
							"function": name,
						},
					})
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkPyTestFns(node.Child(i), src, relPath, out)
	}
}

func isPyTestFunction(name string) bool {
	return strings.HasPrefix(name, "test_") || strings.HasPrefix(name, "eval_")
}

// hasAssertionShape returns true when the function body contains any
// expression shape we recognize as an assertion / scoring / metric
// call.
func hasAssertionShape(fnNode *sitter.Node, src []byte) bool {
	body := fnNode.ChildByFieldName("body")
	if body == nil {
		return false
	}
	text := strings.ToLower(string(src[body.StartByte():body.EndByte()]))
	for _, marker := range assertionShapes {
		if strings.Contains(text, marker) {
			return true
		}
	}
	// AST-level check for raw `assert` statements — text match above
	// catches them too, but the AST walk avoids matching `assert` in
	// strings/comments.
	return walkForAssertStmt(body)
}

var assertionShapes = []string{
	"assertequal", "asserttrue", "assertfalse", "assertin",
	"assertgreater", "assertless", "assertraises",
	".score", "metric.", "evaluator.", "assert_response",
	"deepeval.assert", "ragas.evaluate", "promptfoo",
	"expect(", "should.", ".tobe", ".toequal",
	"pytest.fail(", "pytest.skip(",
}

func walkForAssertStmt(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	if node.Type() == "assert_statement" {
		return true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if walkForAssertStmt(node.Child(i)) {
			return true
		}
	}
	return false
}
