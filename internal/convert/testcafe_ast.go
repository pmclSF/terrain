package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func testCafeFixtureStatementInfo(node *sitter.Node, src []byte) (string, string, bool) {
	if node == nil || node.Type() != "expression_statement" || node.NamedChildCount() != 1 {
		return "", "", false
	}
	return testCafeFixtureExprInfo(node.NamedChild(0), src)
}

func testCafeFixtureExprInfo(node *sitter.Node, src []byte) (string, string, bool) {
	if node == nil {
		return "", "", false
	}

	switch node.Type() {
	case "call_expression":
		callee := jsCalleeNode(node)
		if callee == nil {
			return "", "", false
		}

		switch callee.Type() {
		case "identifier":
			if jsNodeText(callee, src) != "fixture" {
				return "", "", false
			}
			suiteName, ok := testCafeSingleLiteralCallArg(node, src)
			if !ok {
				return "", "", false
			}
			return suiteName, "", true
		case "member_expression":
			if jsNodeText(jsMemberProperty(callee), src) != "page" {
				return "", "", false
			}
			suiteName, pageURL, ok := testCafeFixtureExprInfo(jsMemberObject(callee), src)
			if !ok || pageURL != "" {
				return "", "", false
			}
			pageURL, ok = testCafeSingleLiteralCallArg(node, src)
			if !ok {
				return "", "", false
			}
			return suiteName, pageURL, true
		}
	case "tagged_template":
		tag := node.ChildByFieldName("tag")
		template := node.ChildByFieldName("template")
		if tag == nil || template == nil {
			return "", "", false
		}

		value, ok := testCafeLiteralText(jsNodeText(template, src))
		if !ok {
			return "", "", false
		}

		switch tag.Type() {
		case "identifier":
			if jsNodeText(tag, src) != "fixture" {
				return "", "", false
			}
			return value, "", true
		case "member_expression":
			if jsNodeText(jsMemberProperty(tag), src) != "page" {
				return "", "", false
			}
			suiteName, pageURL, ok := testCafeFixtureExprInfo(jsMemberObject(tag), src)
			if !ok || pageURL != "" {
				return "", "", false
			}
			return suiteName, value, true
		}
	}

	return "", "", false
}

func testCafeSingleLiteralCallArg(node *sitter.Node, src []byte) (string, bool) {
	args := jsArgumentTexts(node, src)
	if len(args) == 1 {
		return testCafeLiteralText(args[0])
	}
	if len(args) != 0 {
		return "", false
	}

	callee := jsCalleeNode(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child == callee {
			continue
		}
		if child.Type() == "string" || child.Type() == "template_string" {
			return testCafeLiteralText(jsNodeText(child, src))
		}
	}
	return "", false
}

func testCafeLiteralText(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		return "", false
	}

	switch text[0] {
	case '\'', '"':
		if text[len(text)-1] != text[0] {
			return "", false
		}
	case '`':
		if text[len(text)-1] != '`' || strings.Contains(text, "${") {
			return "", false
		}
	default:
		return "", false
	}

	return normalizeJSLiteral(text), true
}
