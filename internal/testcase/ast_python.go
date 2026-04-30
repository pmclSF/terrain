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
		walkPyNode(tree.RootNode(), srcBytes, nil, &cases, false, 0)
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
func walkPyNode(node *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase, pendingParametrize bool, estimatedInstances int) {
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
				// Walk class body with this class as suite.
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
						tc := TestCase{
							TestName:       funcName,
							SuiteHierarchy: copySuiteStack(suiteStack),
							Line:           int(node.StartPoint().Row) + 1,
							ExtractionKind: ExtractionStatic,
							Confidence:     ConfidenceHeuristic,
							Parameterized: &ParameterizationInfo{
								IsTemplate:         false,
								ParamSignature:     "case_" + strings.Repeat("0", 0) + string(rune('0'+i%10)),
								EstimatedInstances: count,
							},
						}
						// Use proper formatting for param signature.
						tc.Parameterized.ParamSignature = "case_" + itoa(i)
						*cases = append(*cases, tc)
					}
				} else if pendingParametrize {
					tc := TestCase{
						TestName:       funcName,
						SuiteHierarchy: copySuiteStack(suiteStack),
						Line:           int(node.StartPoint().Row) + 1,
						ExtractionKind: ExtractionParameterizedTemplate,
						Confidence:     ConfidenceInferred,
						Parameterized:  &ParameterizationInfo{IsTemplate: true},
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
		// Check for @pytest.mark.parametrize decorator.
		hasParam := false
		paramInstances := 0
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "decorator" {
				decText := nodeText(child, src)
				if strings.Contains(decText, "parametrize") {
					hasParam = true
					paramInstances = estimateParametrizeInstancesFromAST(child, src)
				}
			}
		}
		// Walk the definition inside the decorator.
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "function_definition" || child.Type() == "class_definition" {
				walkPyNode(child, src, suiteStack, cases, hasParam, paramInstances)
			}
		}
		return
	}

	// Default: recurse into children.
	for i := 0; i < int(node.ChildCount()); i++ {
		walkPyNode(node.Child(i), src, suiteStack, cases, false, 0)
	}
}

// walkPyClassBody walks the body of a test class.
func walkPyClassBody(body *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase) {
	for i := 0; i < int(body.NamedChildCount()); i++ {
		child := body.NamedChild(i)
		walkPyNode(child, src, suiteStack, cases, false, 0)
	}
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
