package testcase

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// extractPythonWithAST uses tree-sitter to parse Python source and extract test cases.
func extractPythonWithAST(src, relPath, framework string) []TestCase {
	srcBytes := []byte(src)

	var cases []TestCase
	parsed := false
	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, perr := parser.ParseCtx(context.Background(), nil, srcBytes)
		if perr != nil || tree == nil {
			return perr
		}
		defer tree.Close()
		walkPyNode(tree.RootNode(), srcBytes, nil, &cases, false, 0, nil)
		parsed = true
		return nil
	})
	if !parsed {
		return extractPython(src, relPath, framework)
	}
	return cases
}

// walkPyNode walks the Python AST looking for test classes, test functions,
// fixtures, and parametrize decorators.
//
// pendingParametrize / estimatedInstances / paramValues form a small
// "decorator hint" that flows from a `decorated_definition` parent down
// to the function it wraps. paramValues holds the literal source text of
// each parametrize entry when statically determinable (one entry per
// row); empty when the value list is dynamic.
func walkPyNode(node *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase, pendingParametrize bool, estimatedInstances int, paramValues []string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "class_definition":
		nameNode := findChildByFieldName(node, "name")
		if nameNode != nil {
			className := nodeText(nameNode, src)
			if strings.HasPrefix(className, "Test") {
				newStack := append(append([]string{}, suiteStack...), className)
				body := findChildByFieldName(node, "body")
				if body != nil {
					walkPyClassBody(body, src, newStack, cases)
				}
				return
			}
		}

	case "function_definition":
		nameNode := findChildByFieldName(node, "name")
		if nameNode != nil {
			funcName := nodeText(nameNode, src)
			if strings.HasPrefix(funcName, "test_") || strings.HasPrefix(funcName, "test") {
				if pendingParametrize && estimatedInstances > 0 {
					const maxEnumerated = 100
					count := estimatedInstances
					if count > maxEnumerated {
						count = maxEnumerated
					}
					for i := 1; i <= count; i++ {
						info := &ParameterizationInfo{
							IsTemplate:         false,
							ParamSignature:     "case_" + itoa(i),
							EstimatedInstances: count,
						}
						// Carry the literal source text of THIS row's
						// parametrize value so consumers can render
						// "test_foo[alice]" / "test_foo[bob]" instead
						// of opaque `case_1` / `case_2`.
						if i-1 < len(paramValues) {
							info.Values = []string{paramValues[i-1]}
						}
						tc := TestCase{
							TestName:       funcName,
							SuiteHierarchy: copySuiteStack(suiteStack),
							Line:           int(node.StartPoint().Row) + 1,
							ExtractionKind: ExtractionStatic,
							Confidence:     ConfidenceHeuristic,
							Parameterized:  info,
						}
						*cases = append(*cases, tc)
					}
				} else if pendingParametrize {
					info := &ParameterizationInfo{IsTemplate: true}
					if len(paramValues) > 0 {
						info.Values = append([]string(nil), paramValues...)
					}
					tc := TestCase{
						TestName:       funcName,
						SuiteHierarchy: copySuiteStack(suiteStack),
						Line:           int(node.StartPoint().Row) + 1,
						ExtractionKind: ExtractionParameterizedTemplate,
						Confidence:     ConfidenceInferred,
						Parameterized:  info,
					}
					*cases = append(*cases, tc)
				} else {
					tc := TestCase{
						TestName:       funcName,
						SuiteHierarchy: copySuiteStack(suiteStack),
						Line:           int(node.StartPoint().Row) + 1,
						ExtractionKind: ExtractionStatic,
						Confidence:     ConfidenceSyntaxMatch,
					}
					*cases = append(*cases, tc)
				}
				return
			}
		}

	case "decorated_definition":
		hasParam := false
		paramInstances := 0
		var values []string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "decorator" {
				decText := nodeText(child, src)
				if strings.Contains(decText, "parametrize") {
					hasParam = true
					paramInstances = estimateParametrizeInstancesFromAST(child, src)
					values = extractParametrizeValuesFromAST(child, src)
				}
			}
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "function_definition" || child.Type() == "class_definition" {
				walkPyNode(child, src, suiteStack, cases, hasParam, paramInstances, values)
			}
		}
		return
	}

	// Default: recurse into children.
	for i := 0; i < int(node.ChildCount()); i++ {
		walkPyNode(node.Child(i), src, suiteStack, cases, false, 0, nil)
	}
}

// walkPyClassBody walks the body of a test class.
func walkPyClassBody(body *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase) {
	for i := 0; i < int(body.NamedChildCount()); i++ {
		child := body.NamedChild(i)
		walkPyNode(child, src, suiteStack, cases, false, 0, nil)
	}
}

// extractParametrizeValuesFromAST returns the literal source text of
// each entry in the value list of @pytest.mark.parametrize(args, [...]).
//
// Returns one string per row. When the row is a tuple (multiple
// parameters per case) the entire tuple text is captured as one entry,
// preserving the original formatting. When the value list is dynamic
// (a name reference, a list comprehension, a function call), returns
// empty — callers fall back to the count-only path.
//
// 0.2 closes the round-4 finding "pytest parametrize value extraction
// (currently estimates count only)".
func extractParametrizeValuesFromAST(decorator *sitter.Node, src []byte) []string {
	argList := findArgListInDecorator(decorator)
	if argList == nil || argList.NamedChildCount() < 2 {
		return nil
	}
	valuesArg := argList.NamedChild(1)
	if valuesArg.Type() != "list" {
		return nil
	}
	out := make([]string, 0, int(valuesArg.NamedChildCount()))
	for i := 0; i < int(valuesArg.NamedChildCount()); i++ {
		child := valuesArg.NamedChild(i)
		text := strings.TrimSpace(nodeText(child, src))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

// findArgListInDecorator returns the argument_list node from a
// parametrize decorator, walking the call structure. Refactored out of
// estimateParametrizeInstancesFromAST so it can be reused.
func findArgListInDecorator(decorator *sitter.Node) *sitter.Node {
	for i := 0; i < int(decorator.NamedChildCount()); i++ {
		child := decorator.NamedChild(i)
		if child.Type() == "call" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				gc := child.NamedChild(j)
				if gc.Type() == "argument_list" {
					return gc
				}
			}
		}
		if child.Type() == "argument_list" {
			return child
		}
	}
	return nil
}

// estimateParametrizeInstancesFromAST counts elements in the second arg of
// @pytest.mark.parametrize("names", [...]).
func estimateParametrizeInstancesFromAST(decorator *sitter.Node, src []byte) int {
	// Structure: decorator → call → argument_list → [string, list]
	var argList *sitter.Node
	for i := 0; i < int(decorator.NamedChildCount()); i++ {
		child := decorator.NamedChild(i)
		if child.Type() == "call" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				gc := child.NamedChild(j)
				if gc.Type() == "argument_list" {
					argList = gc
					break
				}
			}
		}
		if child.Type() == "argument_list" {
			argList = child
		}
	}
	if argList != nil && argList.NamedChildCount() >= 2 {
		valuesArg := argList.NamedChild(1)
		if valuesArg.Type() == "list" {
			return int(valuesArg.NamedChildCount())
		}
	}
	return 0
}

func findChildByFieldName(node *sitter.Node, fieldName string) *sitter.Node {
	return node.ChildByFieldName(fieldName)
}

func itoa(i int) string {
	buf := [20]byte{}
	pos := len(buf)
	for i >= 10 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	pos--
	buf[pos] = byte('0' + i)
	return string(buf[pos:])
}
