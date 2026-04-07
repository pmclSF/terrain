package convert

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type jestMochaASTAnalysis struct {
	edits           []textEdit
	unsupportedRows map[int]bool
}

func convertJestToMochaSourceAST(source string) (string, bool) {
	tree, ok := parseJSSyntaxTree(source)
	if !ok {
		return "", false
	}
	defer tree.Close()

	analysis := analyzeJestToMochaAST(tree)
	result := applyTextEdits(source, analysis.edits)
	if len(analysis.unsupportedRows) > 0 {
		result = commentSpecificLines(result, analysis.unsupportedRows, "manual Jest module mock conversion required")
	}
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), true
}

func analyzeJestToMochaAST(tree *jsSyntaxTree) jestMochaASTAnalysis {
	edits := make([]textEdit, 0, 16)
	unsupportedRows := map[int]bool{}
	walkJSNodes(tree.tree.RootNode(), func(node *sitter.Node) bool {
		switch node.Type() {
		case "import_statement":
			module := jsNodeText(node, tree.src)
			if strings.Contains(module, "'@jest/globals'") || strings.Contains(module, "\"@jest/globals\"") ||
				strings.Contains(module, "'chai'") || strings.Contains(module, "\"chai\"") ||
				strings.Contains(module, "'sinon'") || strings.Contains(module, "\"sinon\"") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "lexical_declaration", "variable_declaration":
			text := jsNodeText(node, tree.src)
			if strings.Contains(text, "require('@jest/globals')") || strings.Contains(text, "require(\"@jest/globals\")") ||
				strings.Contains(text, "require('chai')") || strings.Contains(text, "require(\"chai\")") ||
				strings.Contains(text, "require('sinon')") || strings.Contains(text, "require(\"sinon\")") {
				edits = append(edits, textEdit{
					start: int(node.StartByte()),
					end:   int(node.EndByte()),
				})
				return false
			}
		case "call_expression":
			callee := jsCalleeNode(node)
			if callee != nil && callee.Type() == "identifier" {
				switch jsNodeText(callee, tree.src) {
				case "beforeAll":
					edits = append(edits, textEdit{
						start:       int(callee.StartByte()),
						end:         int(callee.EndByte()),
						replacement: "before",
					})
					return true
				case "afterAll":
					edits = append(edits, textEdit{
						start:       int(callee.StartByte()),
						end:         int(callee.EndByte()),
						replacement: "after",
					})
					return true
				}
			}

			replacement, ok := convertJestCallToMocha(node, tree.src)
			if ok {
				edits = append(edits, replacementEditForCall(node, replacement))
				return false
			}

			memberCallee := jsCalleeNode(node)
			if memberCallee == nil || memberCallee.Type() != "member_expression" {
				return true
			}
			if jsBaseIdentifier(memberCallee, tree.src) != "jest" {
				return true
			}
			property := jsNodeText(jsMemberProperty(memberCallee), tree.src)
			if property == "mock" || property == "doMock" {
				unsupportedRows[int(node.StartPoint().Row)] = true
				return false
			}
		}
		return true
	})

	return jestMochaASTAnalysis{
		edits:           edits,
		unsupportedRows: unsupportedRows,
	}
}

func convertJestCallToMocha(node *sitter.Node, src []byte) (string, bool) {
	callee := jsCalleeNode(node)
	if callee == nil || callee.Type() != "member_expression" {
		return "", false
	}

	base, parts, ok := jsMemberChainParts(callee, src)
	if !ok || len(parts) == 0 {
		return "", false
	}
	args := jsArgumentTexts(node, src)

	if base.Type() == "identifier" && jsNodeText(base, src) == "jest" {
		switch {
		case equalStrings(parts, "useFakeTimers"):
			return "sinon.useFakeTimers()", true
		case equalStrings(parts, "useRealTimers"):
			return "clock.restore()", true
		case equalStrings(parts, "advanceTimersByTime") && len(args) == 1:
			return "clock.tick(" + args[0] + ")", true
		case equalStrings(parts, "clearAllMocks"), equalStrings(parts, "resetAllMocks"):
			return "sinon.reset()", true
		case equalStrings(parts, "restoreAllMocks"):
			return "sinon.restore()", true
		case equalStrings(parts, "spyOn") && len(args) == 2:
			return "sinon.spy(" + args[0] + ", " + args[1] + ")", true
		case equalStrings(parts, "fn"):
			if len(args) == 1 {
				return "sinon.stub().callsFake(" + args[0] + ")", true
			}
			return "sinon.stub()", true
		}
	}

	if base.Type() == "call_expression" {
		baseCallee := jsCalleeNode(base)
		baseArgs := jsArgumentTexts(base, src)
		if baseCallee == nil {
			return "", false
		}

		if baseCallee.Type() == "identifier" && jsNodeText(baseCallee, src) == "expect" && len(baseArgs) == 1 {
			target := baseArgs[0]
			switch {
			case equalStrings(parts, "toHaveBeenCalledWith") && len(args) >= 1:
				return "expect(" + target + ").to.have.been.calledWith(" + strings.Join(args, ", ") + ")", true
			case equalStrings(parts, "toHaveBeenCalledTimes") && len(args) == 1:
				if strings.TrimSpace(args[0]) == "1" {
					return "expect(" + target + ").to.have.been.calledOnce", true
				}
				return "expect(" + target + ".callCount).to.equal(" + args[0] + ")", true
			case equalStrings(parts, "not", "toHaveBeenCalled"):
				return "expect(" + target + ").to.not.have.been.called", true
			case equalStrings(parts, "toHaveBeenCalled"):
				return "expect(" + target + ").to.have.been.called", true
			case equalStrings(parts, "toStrictEqual") && len(args) == 1:
				return "expect(" + target + ").to.deep.equal(" + args[0] + ")", true
			case equalStrings(parts, "toEqual") && len(args) == 1:
				return "expect(" + target + ").to.deep.equal(" + args[0] + ")", true
			case equalStrings(parts, "toBe") && len(args) == 1:
				switch strings.TrimSpace(args[0]) {
				case "true":
					return "expect(" + target + ").to.be.true", true
				case "false":
					return "expect(" + target + ").to.be.false", true
				default:
					return "expect(" + target + ").to.equal(" + args[0] + ")", true
				}
			case equalStrings(parts, "toHaveLength") && len(args) == 1:
				return "expect(" + target + ").to.have.lengthOf(" + args[0] + ")", true
			case equalStrings(parts, "toContain") && len(args) == 1:
				return "expect(" + target + ").to.contain(" + args[0] + ")", true
			}
		}

		if baseCallee.Type() == "member_expression" {
			innerBase, innerParts, ok := jsMemberChainParts(baseCallee, src)
			if !ok || innerBase == nil || innerBase.Type() != "identifier" || jsNodeText(innerBase, src) != "jest" {
				return "", false
			}
			switch {
			case equalStrings(innerParts, "spyOn") && len(baseArgs) == 2:
				switch {
				case equalStrings(parts, "mockReturnValue") && len(args) == 1:
					return "sinon.stub(" + baseArgs[0] + ", " + baseArgs[1] + ").returns(" + args[0] + ")", true
				case equalStrings(parts, "mockResolvedValue") && len(args) == 1:
					return "sinon.stub(" + baseArgs[0] + ", " + baseArgs[1] + ").resolves(" + args[0] + ")", true
				case equalStrings(parts, "mockRejectedValue") && len(args) == 1:
					return "sinon.stub(" + baseArgs[0] + ", " + baseArgs[1] + ").rejects(" + args[0] + ")", true
				case equalStrings(parts, "mockImplementation") && len(args) == 1:
					return "sinon.stub(" + baseArgs[0] + ", " + baseArgs[1] + ").callsFake(" + args[0] + ")", true
				}
			case equalStrings(innerParts, "fn"):
				switch {
				case equalStrings(parts, "mockReturnValue") && len(args) == 1:
					return "sinon.stub().returns(" + args[0] + ")", true
				case equalStrings(parts, "mockResolvedValue") && len(args) == 1:
					return "sinon.stub().resolves(" + args[0] + ")", true
				case equalStrings(parts, "mockRejectedValue") && len(args) == 1:
					return "sinon.stub().rejects(" + args[0] + ")", true
				case equalStrings(parts, "mockImplementation") && len(args) == 1:
					return "sinon.stub().callsFake(" + args[0] + ")", true
				}
			}
		}
	}

	return "", false
}
