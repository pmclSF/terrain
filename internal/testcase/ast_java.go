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
		if nameNode == nil {
			return
		}
		className := nodeText(nameNode, src)
		// Inner classes only contribute to the suite hierarchy when
		// they're explicitly @Nested. The outer class (suiteStack
		// is empty when we hit it) is always a suite. Without this
		// gate, helper inner classes that happen to live in a test
		// file inflate the hierarchy.
		isNested := hasJavaAnnotation(node, src, "Nested")
		isOuter := len(suiteStack) == 0
		if !isOuter && !isNested {
			// Skip helper inner classes — but still recurse so any
			// @Nested grand-children are caught.
			body := node.ChildByFieldName("body")
			if body != nil {
				walkJavaClassBody(body, src, suiteStack, cases)
			}
			return
		}
		newStack := append(append([]string{}, suiteStack...), className)
		body := node.ChildByFieldName("body")
		if body != nil {
			walkJavaClassBody(body, src, newStack, cases)
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
				if displayName := javaDisplayName(node, src); displayName != "" {
					tc.DisplayName = displayName
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

// hasJavaAnnotation reports whether `target` is decorated with the named
// annotation, examining both sibling annotations (when the grammar
// places them as previous siblings) and child `modifiers` blocks. Used
// by the @Nested gate on class_declaration nodes.
func hasJavaAnnotation(target *sitter.Node, src []byte, name string) bool {
	prev := target.PrevNamedSibling()
	for prev != nil {
		switch prev.Type() {
		case "marker_annotation", "annotation":
			if extractAnnotationName(prev, src) == name {
				return true
			}
			prev = prev.PrevNamedSibling()
		default:
			prev = nil
		}
	}
	for i := 0; i < int(target.NamedChildCount()); i++ {
		child := target.NamedChild(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				mod := child.NamedChild(j)
				if mod.Type() == "marker_annotation" || mod.Type() == "annotation" {
					if extractAnnotationName(mod, src) == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// javaDisplayName returns the string argument of a @DisplayName("...")
// annotation on `target`, or "" if no such annotation. Looks at both
// sibling and child-modifier placements to mirror the same robustness
// in checkJavaTestAnnotations.
func javaDisplayName(target *sitter.Node, src []byte) string {
	if v := annotationStringArg(target.PrevNamedSibling(), src, "DisplayName", true); v != "" {
		return v
	}
	for i := 0; i < int(target.NamedChildCount()); i++ {
		child := target.NamedChild(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				mod := child.NamedChild(j)
				if v := annotationStringArg(mod, src, "DisplayName", false); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// annotationStringArg walks back through annotation siblings (when
// walkPrev is true) or examines a single annotation node (walkPrev
// false) and returns the unquoted string-literal argument of the
// annotation whose name matches `wantName`.
func annotationStringArg(start *sitter.Node, src []byte, wantName string, walkPrev bool) string {
	cur := start
	for cur != nil {
		switch cur.Type() {
		case "annotation", "marker_annotation":
			if extractAnnotationName(cur, src) == wantName {
				if v := firstStringArgInAnnotation(cur, src); v != "" {
					return v
				}
			}
		default:
			if walkPrev {
				return ""
			}
			return ""
		}
		if !walkPrev {
			return ""
		}
		cur = cur.PrevNamedSibling()
	}
	return ""
}

// firstStringArgInAnnotation finds the first string literal in an
// annotation's argument list and returns it without surrounding quotes.
func firstStringArgInAnnotation(annot *sitter.Node, src []byte) string {
	for i := 0; i < int(annot.NamedChildCount()); i++ {
		child := annot.NamedChild(i)
		if child.Type() == "annotation_argument_list" || child.Type() == "argument_list" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				gc := child.NamedChild(j)
				if gc.Type() == "string_literal" {
					return unquoteJavaString(nodeText(gc, src))
				}
			}
		}
		if child.Type() == "string_literal" {
			return unquoteJavaString(nodeText(child, src))
		}
	}
	return ""
}

// unquoteJavaString strips the surrounding double quotes from a Java
// string literal and processes the most common escapes (\" and \\).
// Conservative — doesn't try to handle Unicode escapes or text blocks.
func unquoteJavaString(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}
