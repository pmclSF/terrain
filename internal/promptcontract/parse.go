// Package promptcontract detects schema↔prompt drift by building a contract +
// binding model of an AI codebase from the AST: it extracts schema definitions
// (pydantic/dataclass, zod/TS) and prompt surfaces (f-strings, triple-quoted
// strings, LangChain input_variables), binds prompt variables to schema fields
// (explicit input_variables, attribute access on a typed parameter, render
// kwargs), and flags a bound variable that references a field the schema does
// not define. Deterministic, LLM-free, offline — it reads source bytes and
// walks tree-sitter ASTs only.
package promptcontract

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// nodeText returns the source slice a node spans.
func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}

// walk invokes fn for every node in the subtree rooted at n (pre-order).
func walk(n *sitter.Node, fn func(*sitter.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for i := 0; i < int(n.ChildCount()); i++ {
		walk(n.Child(i), fn)
	}
}

// childByField returns the named field child, or nil.
func childByField(n *sitter.Node, field string) *sitter.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName(field)
}
