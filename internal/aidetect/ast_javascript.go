package aidetect

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	tsTypescript "github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// DetectJSAISurfaces parses a single JavaScript or TypeScript source
// buffer and emits every AI SDK call site it finds. relPath drives the
// language choice (TS/TSX → TypeScript grammar; everything else →
// JavaScript) and is recorded on each hit.
//
// Same two-pass shape as the Python detector:
//
//  1. Collect imports / requires, binding alias → SDK identity.
//     `import OpenAI from "openai"` and
//     `const { Anthropic } = require("@anthropic-ai/sdk")` both
//     produce bindings for the local names.
//
//  2. Walk call expressions, classify each by binding or by shape,
//     extract the `model:` option from an object_expression arg when
//     statically resolvable.
//
// Returns nil on parser failure so callers fall back to the regex path.
func DetectJSAISurfaces(src []byte, relPath string) []AICallSite {
	if len(src) == 0 {
		return nil
	}

	lang := javascript.GetLanguage()
	if strings.HasSuffix(relPath, ".ts") || strings.HasSuffix(relPath, ".tsx") {
		lang = tsTypescript.GetLanguage()
	}

	var hits []AICallSite
	var parseOK bool

	_ = parserpool.With(lang, func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		bindings := collectJSAIImports(tree.RootNode(), src)
		walkJSForAICalls(tree.RootNode(), src, relPath, bindings, &hits)
		parseOK = true
		return nil
	})

	if !parseOK {
		return nil
	}
	return hits
}

// collectJSAIImports walks ESM imports and CommonJS requires, binding
// every local name brought in from an AI-known module to its SDK family.
func collectJSAIImports(root *sitter.Node, src []byte) map[string]aiImportBinding {
	bindings := map[string]aiImportBinding{}

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "import_statement":
			collectJSESMImport(n, src, bindings)
		case "variable_declarator":
			// const X = require("...")  and  const { X } = require("...")
			collectJSRequire(n, src, bindings)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	return bindings
}

// collectJSESMImport handles ESM `import` declarations:
//
//	import OpenAI from "openai"
//	import { Anthropic } from "@anthropic-ai/sdk"
//	import * as openai from "openai"
//	import "side-effect-only"  // produces no bindings
func collectJSESMImport(n *sitter.Node, src []byte, bindings map[string]aiImportBinding) {
	sourceNode := n.ChildByFieldName("source")
	if sourceNode == nil {
		return
	}
	module := stripJSStringQuotes(nodeText(sourceNode, src))
	sdk := classifyJSModule(module)
	if sdk == "" {
		return
	}

	// import_clause is sibling of source; it carries default/named imports.
	for i := 0; i < int(n.NamedChildCount()); i++ {
		child := n.NamedChild(i)
		if child.Type() == "import_clause" {
			collectJSImportClause(child, src, sdk, module, bindings)
		}
	}
}

// collectJSImportClause unpacks an import_clause:
//
//	default          → identifier
//	named            → { foo, bar as baz }
//	namespace        → * as ns
func collectJSImportClause(clause *sitter.Node, src []byte, sdk, module string, bindings map[string]aiImportBinding) {
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		child := clause.NamedChild(i)
		switch child.Type() {
		case "identifier":
			bindings[nodeText(child, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
		case "named_imports":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				spec := child.NamedChild(j)
				if spec.Type() != "import_specifier" {
					continue
				}
				nameNode := spec.ChildByFieldName("name")
				aliasNode := spec.ChildByFieldName("alias")
				if aliasNode != nil {
					bindings[nodeText(aliasNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
				} else if nameNode != nil {
					bindings[nodeText(nameNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
				}
			}
		case "namespace_import":
			// * as ns
			for j := 0; j < int(child.NamedChildCount()); j++ {
				ident := child.NamedChild(j)
				if ident.Type() == "identifier" {
					bindings[nodeText(ident, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
				}
			}
		}
	}
}

// collectJSRequire handles CommonJS `const X = require("...")` and
// destructured `const { X, Y } = require("...")` forms.
func collectJSRequire(n *sitter.Node, src []byte, bindings map[string]aiImportBinding) {
	nameNode := n.ChildByFieldName("name")
	valueNode := n.ChildByFieldName("value")
	if nameNode == nil || valueNode == nil {
		return
	}
	if valueNode.Type() != "call_expression" {
		return
	}
	funcNode := valueNode.ChildByFieldName("function")
	if funcNode == nil || nodeText(funcNode, src) != "require" {
		return
	}
	argsNode := valueNode.ChildByFieldName("arguments")
	if argsNode == nil || argsNode.NamedChildCount() == 0 {
		return
	}
	moduleNode := argsNode.NamedChild(0)
	if moduleNode.Type() != "string" {
		return
	}
	module := stripJSStringQuotes(nodeText(moduleNode, src))
	sdk := classifyJSModule(module)
	if sdk == "" {
		return
	}

	switch nameNode.Type() {
	case "identifier":
		bindings[nodeText(nameNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
	case "object_pattern":
		for i := 0; i < int(nameNode.NamedChildCount()); i++ {
			prop := nameNode.NamedChild(i)
			// `{ foo }` → shorthand_property_identifier_pattern
			// `{ foo: bar }` → pair_pattern (key=foo, value=bar)
			switch prop.Type() {
			case "shorthand_property_identifier_pattern":
				bindings[nodeText(prop, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
			case "pair_pattern":
				valNode := prop.ChildByFieldName("value")
				if valNode != nil && valNode.Type() == "identifier" {
					bindings[nodeText(valNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
				}
			}
		}
	}
}

// classifyJSModule maps a bare module specifier to a canonical SDK
// identifier, or "" if the module isn't a recognized AI library.
//
// Handles scoped packages (`@anthropic-ai/sdk`) and the npm scope of
// langchain (`@langchain/core`, `langchain`, etc.).
func classifyJSModule(module string) string {
	switch {
	case module == "openai" || strings.HasPrefix(module, "openai/"):
		return "openai"
	case module == "@anthropic-ai/sdk" || strings.HasPrefix(module, "@anthropic-ai/"):
		return "anthropic"
	case module == "langchain" || strings.HasPrefix(module, "langchain/") ||
		strings.HasPrefix(module, "@langchain/"):
		return "langchain"
	case strings.HasPrefix(module, "@llamaindex/") || module == "llamaindex":
		return "llamaindex"
	case strings.HasPrefix(module, "@huggingface/") || module == "@huggingface/inference":
		return "huggingface"
	case module == "promptfoo" || strings.HasPrefix(module, "promptfoo/"):
		return "promptfoo"
	}
	return ""
}

// walkJSForAICalls walks the AST looking for call_expression nodes that
// classify as AI SDK calls — either constructor calls (`new OpenAI()`,
// `new Anthropic()`) or method calls on a bound client.
func walkJSForAICalls(node *sitter.Node, src []byte, relPath string, bindings map[string]aiImportBinding, hits *[]AICallSite) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "call_expression", "new_expression":
		funcNode := node.ChildByFieldName("function")
		// new_expression uses `constructor` field in some grammars; check both.
		if funcNode == nil {
			funcNode = node.ChildByFieldName("constructor")
		}
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			method := nodeText(funcNode, src)
			if hit := classifyJSCall(method, bindings); hit != nil {
				hit.Path = relPath
				hit.Method = method
				hit.Line = int(funcNode.StartPoint().Row) + 1
				hit.Model = extractJSModelArg(argsNode, src)
				*hits = append(*hits, *hit)
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkJSForAICalls(node.Child(i), src, relPath, bindings, hits)
	}
}

// classifyJSCall takes a dotted callee path and returns a partially-
// filled AICallSite when the call looks like an AI SDK invocation.
//
// Symmetric to classifyPythonCall: strong path is binding-resolved,
// fallback path is shape-based. The JS SDK call shapes are:
//
//	openai     - client.chat.completions.create / .embeddings.create
//	            client.images.generate / .completions.create
//	anthropic  - client.messages.create / .messages.stream
//	langchain  - chain.invoke / .stream / .batch (and async variants)
func classifyJSCall(method string, bindings map[string]aiImportBinding) *AICallSite {
	if method == "" {
		return nil
	}
	root := method
	if i := strings.Index(method, "."); i >= 0 {
		root = method[:i]
	}

	if b, ok := bindings[root]; ok && b.SDK != "" {
		return &AICallSite{SDK: b.SDK, Confidence: 0.95}
	}

	switch {
	case strings.HasSuffix(method, ".chat.completions.create"),
		strings.HasSuffix(method, ".embeddings.create"),
		strings.HasSuffix(method, ".completions.create"),
		strings.HasSuffix(method, ".images.generate"):
		return &AICallSite{SDK: "openai", Confidence: 0.75}
	case strings.HasSuffix(method, ".messages.create"),
		strings.HasSuffix(method, ".messages.stream"):
		return &AICallSite{SDK: "anthropic", Confidence: 0.75}
	case strings.HasSuffix(method, ".invoke"),
		strings.HasSuffix(method, ".ainvoke"),
		strings.HasSuffix(method, ".stream"),
		strings.HasSuffix(method, ".batch"):
		for _, b := range bindings {
			if b.SDK == "langchain" {
				return &AICallSite{SDK: "langchain", Confidence: 0.65}
			}
		}
	}
	return nil
}

// extractJSModelArg looks for the `model:` property in the first
// arguments object passed to a call and returns its string value when
// statically resolvable.
//
// The JS SDK convention is `{ model: "gpt-4o", messages: [...] }` as
// the single options object. We search the first argument's
// object_expression for a `model` property whose value is a string
// literal. Template strings without interpolation (`` `gpt-4o` ``) are
// accepted; interpolated templates and references resolve to empty.
func extractJSModelArg(argsNode *sitter.Node, src []byte) string {
	if argsNode == nil || argsNode.NamedChildCount() == 0 {
		return ""
	}
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := argsNode.NamedChild(i)
		if arg.Type() != "object" && arg.Type() != "object_expression" {
			continue
		}
		for j := 0; j < int(arg.NamedChildCount()); j++ {
			prop := arg.NamedChild(j)
			if prop.Type() != "pair" && prop.Type() != "property" {
				continue
			}
			keyNode := prop.ChildByFieldName("key")
			valueNode := prop.ChildByFieldName("value")
			if keyNode == nil || valueNode == nil {
				continue
			}
			keyText := nodeText(keyNode, src)
			// Object property keys may be identifier or string literal.
			keyText = strings.Trim(keyText, `"'`)
			if keyText != "model" {
				continue
			}
			switch valueNode.Type() {
			case "string":
				return stripJSStringQuotes(nodeText(valueNode, src))
			case "template_string":
				// Only return the literal text if the template has no
				// interpolations (no template_substitution children).
				for k := 0; k < int(valueNode.NamedChildCount()); k++ {
					if valueNode.NamedChild(k).Type() == "template_substitution" {
						return ""
					}
				}
				raw := nodeText(valueNode, src)
				return strings.Trim(raw, "`")
			}
			return ""
		}
		// Only inspect the first object argument.
		break
	}
	return ""
}

// stripJSStringQuotes removes the surrounding " or ' or ` from a string
// literal source slice. Returns the slice unchanged if it's not quoted.
func stripJSStringQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	first := s[0]
	last := s[len(s)-1]
	if (first == '"' || first == '\'' || first == '`') && first == last {
		return s[1 : len(s)-1]
	}
	return s
}
