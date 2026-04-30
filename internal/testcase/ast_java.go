package testcase

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// extractJavaWithAST uses tree-sitter to parse Java source and extract test cases.
func extractJavaWithAST(src, relPath, framework string) []TestCase {
	srcBytes := []byte(src)

	var cases []TestCase
	parsed := false
	_ = parserpool.With(java.GetLanguage(), func(parser *sitter.Parser) error {
		tree, perr := parser.ParseCtx(context.Background(), nil, srcBytes)
		if perr != nil || tree == nil {
			return perr
		}
		defer tree.Close()
		walkJavaNode(tree.RootNode(), srcBytes, nil, &cases)
		parsed = true
		return nil
	})
	if !parsed {
		return extractJava(src, relPath, framework)
	}
	return cases
}

// walkJavaNode walks the Java AST looking for test classes and @Test methods.
func walkJavaNode(node *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "class_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			className := nodeText(nameNode, src)
			newStack := append(append([]string{}, suiteStack...), className)
			body := node.ChildByFieldName("body")
			if body != nil {
				walkJavaClassBody(body, src, newStack, cases)
			}
		}
		return

	case "method_declaration":
		// Only process if preceded by @Test or @ParameterizedTest annotation.
		// tree-sitter puts annotations as siblings before the method.
		isTest, isParameterized := checkJavaTestAnnotations(node, src)
		if !isTest && !isParameterized {
			// Also check for method name starting with "test" (JUnit 3 convention).
			nameNode := node.ChildByFieldName("name")
			if nameNode != nil && strings.HasPrefix(nodeText(nameNode, src), "test") {
				isTest = true
			}
		}

		if isTest || isParameterized {
			nameNode := node.ChildByFieldName("name")
			if nameNode != nil {
				methodName := nodeText(nameNode, src)
				tc := TestCase{
					TestName:       methodName,
					SuiteHierarchy: copySuiteStack(suiteStack),
					Line:           int(node.StartPoint().Row) + 1,
					ExtractionKind: ExtractionStatic,
					Confidence:     ConfidenceNamedPattern,
				}
				if isParameterized {
					tc.ExtractionKind = ExtractionParameterizedTemplate
					tc.Parameterized = &ParameterizationInfo{IsTemplate: true}
				}
				*cases = append(*cases, tc)
			}
		}
		return
	}

	// Recurse into children.
	for i := 0; i < int(node.ChildCount()); i++ {
		walkJavaNode(node.Child(i), src, suiteStack, cases)
	}
}

func walkJavaClassBody(body *sitter.Node, src []byte, suiteStack []string, cases *[]TestCase) {
	for i := 0; i < int(body.NamedChildCount()); i++ {
		child := body.NamedChild(i)
		walkJavaNode(child, src, suiteStack, cases)
	}
}

// checkJavaTestAnnotations checks if a method_declaration node is preceded
// by @Test or @ParameterizedTest annotations. In tree-sitter Java grammar,
// annotations are children of the method_declaration or its parent.
func checkJavaTestAnnotations(methodNode *sitter.Node, src []byte) (isTest bool, isParameterized bool) {
	// Annotations may be siblings before the method, or children of the method.
	// Check previous siblings first.
	prev := methodNode.PrevNamedSibling()
	for prev != nil {
		if prev.Type() == "marker_annotation" || prev.Type() == "annotation" {
			_ = strings.TrimSpace(prev.Content(nil))
			// Content may not work without src — check type name child.
			for j := 0; j < int(prev.NamedChildCount()); j++ {
				child := prev.NamedChild(j)
				if child.Type() == "identifier" {
					name := nodeText(child, src)
					if name == "" {
						// Fallback.
						break
					}
					switch name {
					case "Test":
						isTest = true
					case "ParameterizedTest":
						isParameterized = true
					}
				}
			}
			prev = prev.PrevNamedSibling()
		} else {
			break
		}
	}

	// Also check child modifiers (some grammars nest annotations inside).
	for i := 0; i < int(methodNode.NamedChildCount()); i++ {
		child := methodNode.NamedChild(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				mod := child.NamedChild(j)
				if mod.Type() == "marker_annotation" || mod.Type() == "annotation" {
					annotName := extractAnnotationName(mod, src)
					switch annotName {
					case "Test":
						isTest = true
					case "ParameterizedTest":
						isParameterized = true
					}
				}
			}
		}
	}

	return
}

func extractAnnotationName(annotNode *sitter.Node, src []byte) string {
	for i := 0; i < int(annotNode.NamedChildCount()); i++ {
		child := annotNode.NamedChild(i)
		if child.Type() == "identifier" {
			return nodeText(child, src)
		}
	}
	return ""
}
