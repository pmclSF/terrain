package aidetect

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// AICallSite is a single AI SDK invocation discovered by AST traversal.
// One instance per call expression — multiple invocations in the same
// file produce multiple records.
//
// AST-based detection (this type) replaces the regex modelCallPatterns
// fallback over time. AST sees: imports, binding flow, the actual call
// shape, and statically-resolvable string-literal arguments. The regex
// path remains in detect.go as the safety net until LB-5/LB-6 prove the
// AST detector reaches parity on the dogfood corpus.
type AICallSite struct {
	// Path is the repo-relative file path.
	Path string

	// Line is the 1-based line number of the call expression.
	Line int

	// SDK identifies the library family (openai | anthropic | langchain |
	// llamaindex | huggingface | unknown). The classifier looks at the
	// callee root binding and falls through known method shapes when
	// imports don't disambiguate (e.g., bare `chat.completions.create`
	// inside a wrapper).
	SDK string

	// Method is the dotted call path as written at the call site
	// (e.g., "client.chat.completions.create", "anthropic.Anthropic",
	// "openai.embeddings.create"). The exact source text is preserved
	// so consumers can reason about pre-binding vs. post-binding shapes.
	Method string

	// Model carries the model identifier when statically resolvable.
	// Captured from a `model=` keyword argument whose value is a string
	// literal. Empty when the argument is dynamic, missing, or supplied
	// positionally (the latter is rare for these SDKs).
	Model string

	// Confidence reflects how strong the classification is. SDK calls
	// resolved through an import + binding chain score higher than
	// shape-only matches (which can collide with unrelated method names).
	Confidence float64
}

// DetectPythonAISurfaces parses a single Python source buffer and emits
// every AI SDK call site it finds. relPath is recorded on each hit and
// otherwise unused — the parser is stateless and reads only from src.
//
// Two passes:
//
//  1. Walk imports, building a binding map (alias → SDK identity).
//     `from openai import OpenAI` binds `OpenAI → openai`;
//     `import anthropic as ant` binds `ant → anthropic`.
//
//  2. Walk call expressions. For each `call` node, render its callee as a
//     dotted path, look up the root identifier in the binding map, and
//     emit an AICallSite when classified. Falls back to method-shape
//     matching (e.g., `*.chat.completions.create`) when the binding map
//     misses — useful for files that receive a client through a
//     parameter rather than importing the SDK directly.
//
// Returns nil when the parser fails to construct a tree (e.g., severely
// malformed source). Callers fall back to the regex detector in that case.
func DetectPythonAISurfaces(src []byte, relPath string) []AICallSite {
	if len(src) == 0 {
		return nil
	}

	var hits []AICallSite
	var parseOK bool

	_ = parserpool.With(python.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		bindings := collectPythonAIImports(tree.RootNode(), src)
		walkPythonForAICalls(tree.RootNode(), src, relPath, bindings, &hits)
		parseOK = true
		return nil
	})

	if !parseOK {
		return nil
	}
	return hits
}

// aiImportBinding records what an imported name resolves to.
type aiImportBinding struct {
	// SDK is the canonical library identifier this name comes from.
	SDK string

	// SourceModule is the actual import path (e.g., "openai.types",
	// "langchain_core.messages"). Useful for distinguishing the SDK root
	// from re-exported names like `LangChain.OpenAI`.
	SourceModule string
}

// collectPythonAIImports scans `import` / `from ... import ...` statements
// in the tree and returns the mapping of local name → SDK family for
// names that come from a known AI library.
func collectPythonAIImports(root *sitter.Node, src []byte) map[string]aiImportBinding {
	bindings := map[string]aiImportBinding{}

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "import_statement":
			// import X, X as Y, X.sub
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				addPythonImportBinding(child, src, bindings, "")
			}
		case "import_from_statement":
			// from X import Y, Y as Z, *
			moduleNode := n.ChildByFieldName("module_name")
			module := ""
			if moduleNode != nil {
				module = nodeText(moduleNode, src)
			}
			sdk := classifyPythonModule(module)
			if sdk == "" {
				return
			}
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child == moduleNode {
					continue
				}
				addPythonFromImportBinding(child, src, sdk, module, bindings)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	return bindings
}

// addPythonImportBinding handles `import X` and `import X as Y` forms.
func addPythonImportBinding(n *sitter.Node, src []byte, bindings map[string]aiImportBinding, modulePrefix string) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "dotted_name":
		full := nodeText(n, src)
		root := strings.SplitN(full, ".", 2)[0]
		if sdk := classifyPythonModule(full); sdk != "" {
			bindings[root] = aiImportBinding{SDK: sdk, SourceModule: full}
		}
	case "aliased_import":
		nameNode := n.ChildByFieldName("name")
		aliasNode := n.ChildByFieldName("alias")
		if nameNode == nil || aliasNode == nil {
			return
		}
		full := nodeText(nameNode, src)
		alias := nodeText(aliasNode, src)
		if sdk := classifyPythonModule(full); sdk != "" {
			bindings[alias] = aiImportBinding{SDK: sdk, SourceModule: full}
		}
	}
}

// addPythonFromImportBinding handles `from X import Y[, Z as W]` forms,
// given that X has already been classified as an AI module.
func addPythonFromImportBinding(n *sitter.Node, src []byte, sdk, module string, bindings map[string]aiImportBinding) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "dotted_name", "identifier":
		name := nodeText(n, src)
		bindings[name] = aiImportBinding{SDK: sdk, SourceModule: module}
	case "aliased_import":
		nameNode := n.ChildByFieldName("name")
		aliasNode := n.ChildByFieldName("alias")
		if aliasNode == nil {
			if nameNode != nil {
				bindings[nodeText(nameNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
			}
			return
		}
		bindings[nodeText(aliasNode, src)] = aiImportBinding{SDK: sdk, SourceModule: module}
	case "wildcard_import":
		// `from openai import *` — we can't statically enumerate names,
		// so fall back to shape-based matching downstream. Record the
		// SDK so the wildcard at least flags the file as AI-touching.
		bindings["*"+sdk] = aiImportBinding{SDK: sdk, SourceModule: module}
	}
}

// classifyPythonModule returns the canonical SDK identifier for an import
// module path, or "" if the module isn't recognized as an AI library.
//
// Matching is prefix-based on the dotted path: `openai.types` → "openai",
// `langchain_core.messages` → "langchain", `llama_index.core` → "llamaindex".
func classifyPythonModule(module string) string {
	if module == "" {
		return ""
	}
	root := strings.SplitN(module, ".", 2)[0]
	switch {
	case root == "openai":
		return "openai"
	case root == "anthropic":
		return "anthropic"
	case root == "langchain" || strings.HasPrefix(root, "langchain_"):
		return "langchain"
	case root == "langsmith":
		return "langchain"
	case root == "llama_index" || root == "llamaindex":
		return "llamaindex"
	case root == "transformers" || root == "huggingface_hub" || root == "datasets":
		return "huggingface"
	case root == "promptfoo":
		return "promptfoo"
	case root == "deepeval":
		return "deepeval"
	case root == "ragas":
		return "ragas"
	}
	return ""
}

// walkPythonForAICalls traverses the tree looking for `call` nodes whose
// callee resolves (through the binding map or by shape) to an AI SDK
// invocation.
func walkPythonForAICalls(node *sitter.Node, src []byte, relPath string, bindings map[string]aiImportBinding, hits *[]AICallSite) {
	if node == nil {
		return
	}

	if node.Type() == "call" {
		funcNode := node.ChildByFieldName("function")
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			method := nodeText(funcNode, src)
			if hit := classifyPythonCall(method, bindings); hit != nil {
				hit.Path = relPath
				hit.Method = method
				hit.Line = int(funcNode.StartPoint().Row) + 1
				hit.Model = extractPythonModelArg(argsNode, src)
				*hits = append(*hits, *hit)
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkPythonForAICalls(node.Child(i), src, relPath, bindings, hits)
	}
}

// classifyPythonCall takes a dotted method path (the literal source text
// of the call's callee, e.g., `client.chat.completions.create`) and
// returns a partially-filled AICallSite when it looks like an AI SDK
// invocation. Returns nil for non-matching calls.
func classifyPythonCall(method string, bindings map[string]aiImportBinding) *AICallSite {
	if method == "" {
		return nil
	}
	root := method
	if i := strings.Index(method, "."); i >= 0 {
		root = method[:i]
	}

	// Strong path: the root is bound to a known SDK via imports.
	if b, ok := bindings[root]; ok && b.SDK != "" {
		return &AICallSite{SDK: b.SDK, Confidence: 0.95}
	}

	// Shape-based fallback: a wrapper method whose name matches a known
	// SDK call path even when the client object isn't imported in this
	// file (e.g., it was passed in as a parameter).
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
		// LangChain runnable interface; ambiguous on its own. Only
		// emit if we have ANY langchain binding in the file.
		for _, b := range bindings {
			if b.SDK == "langchain" {
				return &AICallSite{SDK: "langchain", Confidence: 0.65}
			}
		}
	}
	return nil
}

// extractPythonModelArg pulls the `model=` keyword argument out of a
// Python call's argument list when its value is a string literal.
// Returns empty when the argument is missing, dynamic, or not a string.
//
// Tree-sitter Python represents `model="gpt-4o"` as a keyword_argument
// node with `name` field "model" and `value` field a `string` node
// whose text includes the surrounding quotes.
func extractPythonModelArg(argsNode *sitter.Node, src []byte) string {
	if argsNode == nil {
		return ""
	}
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		child := argsNode.NamedChild(i)
		if child.Type() != "keyword_argument" {
			continue
		}
		nameNode := child.ChildByFieldName("name")
		valueNode := child.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil {
			continue
		}
		if nodeText(nameNode, src) != "model" {
			continue
		}
		if valueNode.Type() != "string" {
			return ""
		}
		// Strip the leading/trailing quote character. Triple-quoted
		// strings and f-strings have nested string_start / string_end
		// children; the slice approach below handles plain "..." and
		// '...' correctly. The triple-quoted case stays empty because
		// model names aren't multi-line strings in practice.
		raw := nodeText(valueNode, src)
		raw = strings.TrimSpace(raw)
		if len(raw) >= 2 {
			first := raw[0]
			last := raw[len(raw)-1]
			if (first == '"' || first == '\'') && first == last {
				return raw[1 : len(raw)-1]
			}
		}
		return raw
	}
	return ""
}

// nodeText returns the source text covered by an AST node.
func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}
