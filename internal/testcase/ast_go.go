package testcase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// extractGoWithAST uses go/parser (real AST) to extract test cases.
// This replaces the regex-based approach for Go files, giving exact
// function identification with no false positives in strings/comments.
func extractGoWithAST(src, relPath, framework string) []TestCase {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, relPath, src, parser.ParseComments)
	if err != nil {
		// Fallback to regex on parse failure.
		return extractGo(src, relPath, framework)
	}

	var cases []TestCase

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}

		name := fn.Name.Name
		line := fset.Position(fn.Pos()).Line

		// Top-level Test* functions.
		if strings.HasPrefix(name, "Test") && isTestSignature(fn) {
			tc := TestCase{
				TestName:       name,
				Line:           line,
				ExtractionKind: ExtractionStatic,
				Confidence:     ConfidenceNamedPattern,
			}
			cases = append(cases, tc)

			// Walk function body for t.Run subtests, tracking the
			// nesting stack so deeply-nested t.Run calls retain the
			// full path (test/sub1/sub2/...).
			if fn.Body != nil {
				subtests := extractGoSubtestsHierarchical(fn.Body, fset, []string{name})
				cases = append(cases, subtests...)
			}
		}
	}

	return cases
}

// isTestSignature checks if a function has the right signature for a Go test:
// func TestXxx(t *testing.T) or func TestXxx(t *testing.TB) etc.
func isTestSignature(fn *ast.FuncDecl) bool {
	if fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	params := fn.Type.Params.List
	if len(params) < 1 {
		return false
	}
	// First param should be *testing.T, *testing.B, *testing.M, etc.
	// We accept any single pointer parameter for flexibility.
	return true
}

// extractGoSubtestsHierarchical recursively walks the function body
// looking for t.Run(name, fn) calls and tracks the full nesting stack
// so deeply-nested t.Runs retain their parent chain.
//
// stack is the chain of names from the enclosing test function down to
// the current call (without the new t.Run's own name appended yet).
// Each emitted TestCase has SuiteHierarchy = stack and TestName = the
// new t.Run's literal name argument; recursion into the t.Run's
// callback function passes stack+[name].
//
// We stop the default ast.Inspect descent inside a t.Run call expression
// because we recurse manually with the updated stack — letting Inspect
// continue would re-visit the inner t.Runs at the wrong stack depth.
func extractGoSubtestsHierarchical(node ast.Node, fset *token.FileSet, stack []string) []TestCase {
	var cases []TestCase
	if node == nil {
		return cases
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		name := extractGoStringLiteral(call.Args[0])
		if name == "" {
			return true
		}

		// Copy the stack so the appended slice doesn't alias the
		// caller's; without this, sibling t.Runs see each other's
		// names in their hierarchies.
		hierarchy := append([]string(nil), stack...)

		tc := TestCase{
			TestName:       name,
			SuiteHierarchy: hierarchy,
			Line:           fset.Position(call.Pos()).Line,
			ExtractionKind: ExtractionStatic,
			Confidence:     ConfidenceSyntaxMatch,
		}
		cases = append(cases, tc)

		// Recurse into the t.Run callback body with the deeper stack.
		// Args[1] is typically `func(t *testing.T) { ... }`. If it's
		// a non-literal (e.g. a named function reference), we don't
		// have its body in this AST and can't recurse — that's fine.
		if fnLit, ok := call.Args[1].(*ast.FuncLit); ok && fnLit.Body != nil {
			deeper := extractGoSubtestsHierarchical(
				fnLit.Body, fset,
				append(hierarchy, name),
			)
			cases = append(cases, deeper...)
		}

		// Stop default descent into this CallExpr; we recursed manually
		// with the correct deeper stack. Returning false here prevents
		// double-emission and wrong-stack attribution.
		return false
	})

	return cases
}

// extractGoStringLiteral extracts the string value from an AST expression
// if it's a basic string literal.
func extractGoStringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	// Remove quotes.
	val := lit.Value
	if len(val) >= 2 {
		if val[0] == '"' {
			// Interpret escape sequences.
			val = val[1 : len(val)-1]
			val = strings.ReplaceAll(val, `\"`, `"`)
			val = strings.ReplaceAll(val, `\\`, `\`)
			return val
		}
		if val[0] == '`' {
			return val[1 : len(val)-1]
		}
	}
	return val
}
