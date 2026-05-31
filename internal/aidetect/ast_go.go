package aidetect

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// DetectGoAISurfaces parses a Go source buffer and emits AI SDK call
// sites. Mirrors DetectPythonAISurfaces / DetectJSAISurfaces with
// Go-specific binding semantics:
//
//   - Imports bind a package name (the last path component, or an
//     explicit alias when given) to a module path. We classify the
//     module path through classifyGoModule.
//
//   - Call sites either:
//     openai.NewClient(...)       — package-qualified call on a
//     bound AI package.
//     client.CreateChatCompletion(ctx, req)
//     — method on a receiver whose type
//     we can't statically resolve
//     without full type analysis; we
//     classify by call shape against
//     known SDK method names when an
//     AI binding exists in scope.
//
// Go SDKs currently recognized:
//
//	github.com/sashabaranov/go-openai (community)
//	github.com/openai/openai-go (official)
//
// Returns nil on parse failure.
func DetectGoAISurfaces(src []byte, relPath string) []AICallSite {
	if len(src) == 0 {
		return nil
	}

	var hits []AICallSite
	var parseOK bool

	_ = parserpool.With(golang.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		bindings := collectGoAIImports(tree.RootNode(), src)
		walkGoForAICalls(tree.RootNode(), src, relPath, bindings, &hits)
		parseOK = true
		return nil
	})

	if !parseOK {
		return nil
	}
	return hits
}

// collectGoAIImports walks import declarations and binds local names
// to SDK families.
//
// Go imports are one of:
//
//	import "github.com/sashabaranov/go-openai"     → bound name = path's last component
//	import openai "github.com/openai/openai-go"    → bound name = explicit alias
//	import . "github.com/x/y"                       → dot import (rare, hard to support)
//	import _ "github.com/x/y"                       → blank import (side-effect only)
//
// Both single import_declaration and grouped `import ( ... )` forms
// produce one or more import_spec nodes.
func collectGoAIImports(root *sitter.Node, src []byte) map[string]aiImportBinding {
	bindings := map[string]aiImportBinding{}

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "import_spec" {
			collectGoImportSpec(n, src, bindings)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return bindings
}

func collectGoImportSpec(spec *sitter.Node, src []byte, bindings map[string]aiImportBinding) {
	nameNode := spec.ChildByFieldName("name") // optional alias
	pathNode := spec.ChildByFieldName("path")
	if pathNode == nil {
		return
	}
	module := stripGoStringQuotes(nodeText(pathNode, src))
	sdk := classifyGoModule(module)
	if sdk == "" {
		return
	}

	bound := ""
	if nameNode != nil {
		alias := nodeText(nameNode, src)
		if alias == "_" || alias == "." {
			// Blank import contributes a file-level signal but no
			// binding for a specific identifier. We still record the
			// SDK by using a sentinel key the call walker can probe.
			bindings["_"+sdk] = aiImportBinding{SDK: sdk, SourceModule: module}
			return
		}
		bound = alias
	} else {
		// Last path component is the default package name.
		bound = module
		if i := strings.LastIndex(module, "/"); i >= 0 {
			bound = module[i+1:]
		}
		// `go-openai` → `openai` is the conventional package name
		// (set by `package openai` inside the module). Strip leading
		// `go-` prefix when present.
		bound = strings.TrimPrefix(bound, "go-")
	}

	bindings[bound] = aiImportBinding{SDK: sdk, SourceModule: module}
}

// classifyGoModule maps a Go module path to a canonical SDK identifier.
func classifyGoModule(module string) string {
	switch {
	case strings.HasPrefix(module, "github.com/sashabaranov/go-openai"),
		strings.HasPrefix(module, "github.com/openai/openai-go"):
		return "openai"
	case strings.HasPrefix(module, "github.com/anthropic/anthropic-sdk-go"):
		return "anthropic"
	case strings.HasPrefix(module, "github.com/tmc/langchaingo"):
		return "langchain"
	}
	return ""
}

// walkGoForAICalls walks call_expression nodes and classifies.
func walkGoForAICalls(node *sitter.Node, src []byte, relPath string, bindings map[string]aiImportBinding, hits *[]AICallSite) {
	if node == nil {
		return
	}
	if node.Type() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		argsNode := node.ChildByFieldName("arguments")
		if funcNode != nil {
			method := nodeText(funcNode, src)
			if hit := classifyGoCall(method, bindings); hit != nil {
				hit.Path = relPath
				hit.Method = method
				hit.Line = int(funcNode.StartPoint().Row) + 1
				hit.Model = extractGoModelArg(argsNode, src)
				*hits = append(*hits, *hit)
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkGoForAICalls(node.Child(i), src, relPath, bindings, hits)
	}
}

// classifyGoCall classifies a call's selector path.
//
// Strong: root identifier is bound to a known SDK via imports
// (openai.NewClient, openai.ChatCompletion).
//
// Shape-based fallback: the method name matches a known SDK call
// shape AND there's at least one AI binding in scope (so we don't
// fire on coincidental method names in unrelated files).
func classifyGoCall(method string, bindings map[string]aiImportBinding) *AICallSite {
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

	// Shape-based fallback: only fire when the file has any AI SDK
	// import. This guards against accidentally matching unrelated
	// methods named `CreateCompletion`, `Messages`, etc.
	hasOpenAI := false
	hasAnthropic := false
	hasLangChain := false
	for _, b := range bindings {
		switch b.SDK {
		case "openai":
			hasOpenAI = true
		case "anthropic":
			hasAnthropic = true
		case "langchain":
			hasLangChain = true
		}
	}

	switch {
	case hasOpenAI && (strings.HasSuffix(method, ".CreateChatCompletion") ||
		strings.HasSuffix(method, ".CreateCompletion") ||
		strings.HasSuffix(method, ".CreateEmbeddings") ||
		strings.HasSuffix(method, ".CreateImage")):
		return &AICallSite{SDK: "openai", Confidence: 0.75}
	case hasAnthropic && (strings.HasSuffix(method, ".Messages") ||
		strings.HasSuffix(method, ".Messages.Create") ||
		strings.HasSuffix(method, ".Complete")):
		return &AICallSite{SDK: "anthropic", Confidence: 0.75}
	case hasLangChain && (strings.HasSuffix(method, ".Call") ||
		strings.HasSuffix(method, ".Generate")):
		return &AICallSite{SDK: "langchain", Confidence: 0.65}
	}
	return nil
}

// extractGoModelArg walks a call's arguments for an inline struct
// literal with a Model: field whose value is a string literal or a
// known constant. Go SDKs (e.g., sashabaranov/go-openai) pass model
// via a request struct:
//
//	openai.ChatCompletionRequest{
//	    Model:    "gpt-4o-mini",
//	    Messages: ...,
//	}
//
// We accept either string literal or unqualified/qualified constant
// reference; for constants we return the raw text (e.g.,
// "openai.GPT4oMini") since resolving the constant requires
// cross-file type analysis.
func extractGoModelArg(argsNode *sitter.Node, src []byte) string {
	if argsNode == nil {
		return ""
	}
	var found string
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil || found != "" {
			return
		}
		if n.Type() == "keyed_element" {
			// Go tree-sitter wraps both key and value in
			// `literal_element` nodes — unwrap to inspect the inner
			// content. Two named children: [literal_element key,
			// literal_element value].
			if n.NamedChildCount() < 2 {
				return
			}
			keyText := nodeText(n.NamedChild(0), src)
			if strings.TrimSpace(keyText) != "Model" {
				return
			}
			value := n.NamedChild(1)
			// Unwrap literal_element to get the actual expression.
			if value.Type() == "literal_element" && value.NamedChildCount() > 0 {
				value = value.NamedChild(0)
			}
			switch value.Type() {
			case "interpreted_string_literal", "raw_string_literal":
				found = stripGoStringQuotes(nodeText(value, src))
			case "identifier", "selector_expression":
				found = nodeText(value, src)
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(argsNode)
	return found
}

func stripGoStringQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	first := s[0]
	last := s[len(s)-1]
	if (first == '"' || first == '`') && first == last {
		return s[1 : len(s)-1]
	}
	return s
}
