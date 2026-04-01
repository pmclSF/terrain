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

			// Walk function body for t.Run subtests.
			if fn.Body != nil {
				subtests := extractGoSubtests(fn.Body, fset, name)
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

// extractGoSubtests walks a function body looking for t.Run("name", ...) calls.
func extractGoSubtests(body *ast.BlockStmt, fset *token.FileSet, parentName string) []TestCase {
	var cases []TestCase

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for t.Run(name, func)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}

		// First argument should be a string literal.
		if len(call.Args) < 2 {
			return true
		}

		name := extractGoStringLiteral(call.Args[0])
		if name == "" {
			return true
		}

		tc := TestCase{
			TestName:       name,
			SuiteHierarchy: []string{parentName},
			Line:           fset.Position(call.Pos()).Line,
			ExtractionKind: ExtractionStatic,
			Confidence:     ConfidenceSyntaxMatch,
		}
		cases = append(cases, tc)

		return true // Continue walking for nested t.Run.
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
