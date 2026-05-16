package apispec

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	tsTypescript "github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// tRPC router parsing. tRPC defines its API in TypeScript code rather
// than a declarative schema; this parser walks the AST of a router
// definition file and emits one Operation per procedure (query /
// mutation / subscription).
//
// Recognized shape:
//
//	export const appRouter = router({
//	  getUser: publicProcedure
//	    .input(z.object({ id: z.string() }))
//	    .query(async ({ input }) => { ... }),
//	  createUser: protectedProcedure
//	    .input(z.object({ name: z.string(), email: z.string() }))
//	    .mutation(async ({ input }) => { ... }),
//	});
//
// Each procedure key in the object literal becomes a separate
// Operation with:
//   - Method: "Query" / "Mutation" / "Subscription" (from .query() /
//     .mutation() / .subscription() in the chain)
//   - Path: "<routerName>.<procedureName>"
//   - OperationID: "<routerName>.<procedureName>"
//   - FieldsWrite: keys from z.object({...}) inside .input(...)
//
// Nested routers (e.g., `users: router({...})` inside another router)
// flatten to dotted paths so impact analysis can match a route
// reference like `trpc.users.getById.useQuery()` to the procedure.

// ContractTRPC identifies tRPC router contracts.
const ContractTRPC ContractKind = "trpc"

// ParseTRPCFile reads a TypeScript file and extracts tRPC routers.
// Returns nil + nil error when the file isn't a tRPC router (no router
// import + no router(...) call site).
func ParseTRPCFile(path string) (*APIContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("apispec: read tRPC %s: %w", path, err)
	}
	return ParseTRPC(data, path), nil
}

// ParseTRPC parses the bytes of a TypeScript file and returns one
// APIContract aggregating every router it finds. Returns nil when no
// tRPC-shaped router is detected.
func ParseTRPC(src []byte, path string) *APIContract {
	if len(src) == 0 {
		return nil
	}

	lang := javascript.GetLanguage()
	if strings.HasSuffix(strings.ToLower(path), ".ts") || strings.HasSuffix(strings.ToLower(path), ".tsx") {
		lang = tsTypescript.GetLanguage()
	}

	var operations []Operation
	hasRouter := false

	_ = parserpool.With(lang, func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		if !fileImportsTRPC(tree.RootNode(), src) {
			return nil
		}
		hasRouter = true

		walkTRPCRouters(tree.RootNode(), src, "", &operations)
		return nil
	})

	if !hasRouter || len(operations) == 0 {
		return nil
	}
	return &APIContract{
		Path:       path,
		Kind:       ContractTRPC,
		Operations: operations,
	}
}

// fileImportsTRPC walks imports looking for a tRPC dependency
// (`@trpc/server` or `@trpc/client`) or a local re-export named
// "router" / "publicProcedure" / "protectedProcedure".
func fileImportsTRPC(root *sitter.Node, src []byte) bool {
	var found bool
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil || found {
			return
		}
		if n.Type() == "import_statement" {
			text := nodeText(n, src)
			if strings.Contains(text, "@trpc/") {
				found = true
				return
			}
			// Local re-export: import { router, publicProcedure } from './trpc'
			if strings.Contains(text, "publicProcedure") ||
				strings.Contains(text, "protectedProcedure") ||
				(strings.Contains(text, "router") && strings.Contains(text, "from")) {
				// Heuristic: any import that names router + procedure
				// is likely from a local tRPC initialization module.
				found = true
				return
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return found
}

// walkTRPCRouters finds router({...}) call sites and processes their
// procedure entries. prefix is the dotted path accumulated through
// nested routers (e.g., "users" when the parent router has key
// `users: router({...})`).
func walkTRPCRouters(node *sitter.Node, src []byte, prefix string, out *[]Operation) {
	if node == nil {
		return
	}

	// Recognize `router(<object>)` call expressions.
	if node.Type() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			name := nodeText(funcNode, src)
			if name == "router" || name == "t.router" || strings.HasSuffix(name, ".router") {
				args := node.ChildByFieldName("arguments")
				if args != nil && args.NamedChildCount() > 0 {
					arg := args.NamedChild(0)
					if arg.Type() == "object" || arg.Type() == "object_expression" {
						processRouterObject(arg, src, prefix, out)
						return // don't recurse into the body — processRouterObject did it
					}
				}
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkTRPCRouters(node.Child(i), src, prefix, out)
	}
}

// processRouterObject walks the keys of a router({...}) object. Each
// key is either:
//   - a procedure (publicProcedure.input(...).query(...))
//   - a sub-router (router({...}) call)
func processRouterObject(obj *sitter.Node, src []byte, prefix string, out *[]Operation) {
	for i := 0; i < int(obj.NamedChildCount()); i++ {
		prop := obj.NamedChild(i)
		if prop.Type() != "pair" && prop.Type() != "property" {
			continue
		}
		keyNode := prop.ChildByFieldName("key")
		valueNode := prop.ChildByFieldName("value")
		if keyNode == nil || valueNode == nil {
			continue
		}
		key := strings.Trim(nodeText(keyNode, src), `"'`)
		dottedKey := key
		if prefix != "" {
			dottedKey = prefix + "." + key
		}

		// Check whether the value is a nested router call.
		if isRouterCall(valueNode, src) {
			// Recurse with extended prefix into the router(...) argument.
			walkTRPCRouters(valueNode, src, dottedKey, out)
			continue
		}

		// Otherwise treat as a procedure chain.
		if op := parseTRPCProcedure(valueNode, src, dottedKey); op != nil {
			*out = append(*out, *op)
		}
	}
}

func isRouterCall(node *sitter.Node, src []byte) bool {
	if node.Type() != "call_expression" {
		return false
	}
	fn := node.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	name := nodeText(fn, src)
	return name == "router" || strings.HasSuffix(name, ".router")
}

// parseTRPCProcedure parses a procedure value expression — a chained
// call sequence starting from `publicProcedure` or `protectedProcedure`
// (or a custom-named procedure base).
//
// Walks the chain looking for `.input(...)` (for FieldsWrite) and
// `.query(...)` / `.mutation(...)` / `.subscription(...)` (for Method).
func parseTRPCProcedure(node *sitter.Node, src []byte, dottedKey string) *Operation {
	method := ""
	var fieldsWrite []string

	// The chain is a method_invocation tree; walk to collect each link.
	visit := node
	for visit != nil && visit.Type() == "call_expression" {
		fn := visit.ChildByFieldName("function")
		if fn == nil {
			break
		}
		name := lastMemberName(fn, src)
		args := visit.ChildByFieldName("arguments")
		switch name {
		case "query":
			method = "Query"
		case "mutation":
			method = "Mutation"
		case "subscription":
			method = "Subscription"
		case "input":
			fieldsWrite = extractZodObjectKeys(args, src)
		}
		// Step to the inner call (the object of this call).
		visit = innerCall(fn)
	}

	if method == "" {
		return nil
	}

	sortStrings(fieldsWrite)
	return &Operation{
		Method:      method,
		Path:        dottedKey,
		OperationID: dottedKey,
		FieldsWrite: fieldsWrite,
	}
}

// lastMemberName returns the last `.name` from a member-access
// expression, or the identifier itself for a bare identifier.
func lastMemberName(n *sitter.Node, src []byte) string {
	switch n.Type() {
	case "member_expression":
		prop := n.ChildByFieldName("property")
		if prop != nil {
			return nodeText(prop, src)
		}
	case "identifier":
		return nodeText(n, src)
	}
	return ""
}

// innerCall returns the call_expression that's the object of a
// member-expression-as-function. For `a.b().c()`, given the function
// node `a.b().c`, returns the call_expression `a.b()`.
func innerCall(fn *sitter.Node) *sitter.Node {
	if fn == nil || fn.Type() != "member_expression" {
		return nil
	}
	obj := fn.ChildByFieldName("object")
	if obj != nil && obj.Type() == "call_expression" {
		return obj
	}
	return nil
}

// extractZodObjectKeys walks the `.input(...)` arguments looking for
// `z.object({ field1: z.x(), field2: z.y() })` and returns the keys.
func extractZodObjectKeys(args *sitter.Node, src []byte) []string {
	if args == nil {
		return nil
	}
	var keys []string
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil {
			return
		}
		// Look for z.object({...}) calls.
		if n.Type() == "call_expression" {
			fn := n.ChildByFieldName("function")
			if fn != nil {
				name := nodeText(fn, src)
				if name == "z.object" || strings.HasSuffix(name, ".object") {
					callArgs := n.ChildByFieldName("arguments")
					if callArgs != nil {
						for i := 0; i < int(callArgs.NamedChildCount()); i++ {
							arg := callArgs.NamedChild(i)
							if arg.Type() == "object" || arg.Type() == "object_expression" {
								for j := 0; j < int(arg.NamedChildCount()); j++ {
									prop := arg.NamedChild(j)
									if prop.Type() != "pair" && prop.Type() != "property" {
										continue
									}
									kn := prop.ChildByFieldName("key")
									if kn != nil {
										key := strings.Trim(nodeText(kn, src), `"'`)
										keys = append(keys, key)
									}
								}
							}
						}
					}
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(args)
	return keys
}

// nodeText reads source bytes for a tree-sitter node.
// (Declared here separately from apispec.go for symmetry with the
// Java/Go/Python AST packages — each has its own local nodeText.)
func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}
