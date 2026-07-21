package aidetect

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"github.com/pmclSF/terrain/internal/parserpool"
)

// DetectJavaAISurfaces parses a Java source buffer and emits AI SDK
// call sites, bringing Java in line with the Python, JS/TS, and Go detectors.
//
// Java imports bind a class name (the last component of the qualified
// path) to a package. We classify the package through classifyJavaPackage.
//
// SDKs currently recognized:
//
//	com.theokanning.openai.* (community OpenAI for Java)
//	com.openai.* (official OpenAI Java SDK)
//	com.anthropic.* (Anthropic Java)
//	com.azure.ai.openai.* (Azure OpenAI)
//
// Model extraction handles the Java builder pattern:
//
//	ChatCompletionRequest.builder().model("gpt-4o").build()
//
// We find a `.model("...")` invocation chained within the call's
// argument tree and pull the string literal.
//
// Returns nil on parse failure.
func DetectJavaAISurfaces(src []byte, relPath string) []AICallSite {
	if len(src) == 0 {
		return nil
	}

	var hits []AICallSite
	var parseOK bool

	_ = parserpool.With(java.GetLanguage(), func(parser *sitter.Parser) error {
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()

		bindings := collectJavaAIImports(tree.RootNode(), src)
		if len(bindings) == 0 {
			parseOK = true
			return nil
		}
		walkJavaForAICalls(tree.RootNode(), src, relPath, bindings, &hits)
		parseOK = true
		return nil
	})

	if !parseOK {
		return nil
	}
	return hits
}

// collectJavaAIImports walks import_declaration nodes and binds local
// class names to SDK families.
func collectJavaAIImports(root *sitter.Node, src []byte) map[string]aiImportBinding {
	bindings := map[string]aiImportBinding{}

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "import_declaration" {
			fullPath := flattenJavaScopedIdentifier(n, src)
			if fullPath == "" {
				return
			}
			sdk := classifyJavaPackage(fullPath)
			if sdk == "" {
				return
			}
			// Local binding: last component of the dotted path.
			localName := fullPath
			if i := strings.LastIndex(fullPath, "."); i >= 0 {
				localName = fullPath[i+1:]
			}
			// Wildcard imports (`import x.y.*`) bind "*" — we don't
			// resolve specific class names from those, but record the
			// SDK so shape-matching downstream knows it's in scope.
			if localName == "*" {
				bindings["*"+sdk] = aiImportBinding{SDK: sdk, SourceModule: fullPath}
				return
			}
			bindings[localName] = aiImportBinding{SDK: sdk, SourceModule: fullPath}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return bindings
}

// flattenJavaScopedIdentifier reconstructs the dotted path from a
// scoped_identifier subtree. Java tree-sitter builds these left-
// associatively (com.x.y is `((com.x).y)`), so we serialize via the
// raw source text — that's both simpler and avoids edge cases around
// wildcards / static imports.
func flattenJavaScopedIdentifier(importNode *sitter.Node, src []byte) string {
	text := nodeText(importNode, src)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "static")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	return strings.TrimSpace(text)
}

// classifyJavaPackage maps a Java package path to a canonical SDK id.
func classifyJavaPackage(pkg string) string {
	switch {
	case strings.HasPrefix(pkg, "com.theokanning.openai"),
		strings.HasPrefix(pkg, "com.openai."),
		pkg == "com.openai":
		return "openai"
	case strings.HasPrefix(pkg, "com.anthropic"):
		return "anthropic"
	case strings.HasPrefix(pkg, "com.azure.ai.openai"):
		return "openai"
	case strings.HasPrefix(pkg, "dev.langchain4j"):
		return "langchain"
	}
	return ""
}

// walkJavaForAICalls walks the tree for call sites — both method
// invocations and object creation expressions (constructors).
func walkJavaForAICalls(node *sitter.Node, src []byte, relPath string, bindings map[string]aiImportBinding, hits *[]AICallSite) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "object_creation_expression":
		// new Type(args)
		typeNode := node.ChildByFieldName("type")
		if typeNode == nil {
			// fall back to first named child
			if node.NamedChildCount() > 0 {
				typeNode = node.NamedChild(0)
			}
		}
		if typeNode != nil {
			typeName := nodeText(typeNode, src)
			if b, ok := bindings[typeName]; ok && b.SDK != "" {
				hit := AICallSite{
					Path:       relPath,
					Line:       int(node.StartPoint().Row) + 1,
					SDK:        b.SDK,
					Method:     "new " + typeName,
					Confidence: 0.95,
				}
				// Model arg unlikely in a constructor; skip extraction.
				*hits = append(*hits, hit)
			}
		}

	case "method_invocation":
		methodText := nodeText(node, src)
		methodText = collapseJavaWhitespace(methodText)
		if hit := classifyJavaInvocation(node, src, bindings); hit != nil {
			hit.Path = relPath
			hit.Line = int(node.StartPoint().Row) + 1
			hit.Method = collapseMethodInvocationText(node, src)
			hit.Model = extractJavaModelArg(node, src)
			*hits = append(*hits, *hit)
		}
		_ = methodText
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		walkJavaForAICalls(node.Child(i), src, relPath, bindings, hits)
	}
}

// classifyJavaInvocation classifies a method_invocation node.
//
// Strong path: the call's root identifier (the outermost object in a
// chain) is a class bound to a known SDK via imports.
//
// Shape-based fallback: well-known SDK method names (createChatCompletion,
// createCompletion, createMessage, etc.) trigger a match when there's
// any AI binding in scope.
func classifyJavaInvocation(node *sitter.Node, src []byte, bindings map[string]aiImportBinding) *AICallSite {
	root := rootIdentifierOfInvocation(node, src)
	if root != "" {
		if b, ok := bindings[root]; ok && b.SDK != "" {
			return &AICallSite{SDK: b.SDK, Confidence: 0.95}
		}
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nodeText(nameNode, src)

	hasOpenAI, hasAnthropic, hasLangChain := false, false, false
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
	case hasOpenAI && (name == "createChatCompletion" || name == "createCompletion" ||
		name == "createEmbedding" || name == "createEmbeddings" ||
		name == "createImage"):
		return &AICallSite{SDK: "openai", Confidence: 0.75}
	case hasAnthropic && (name == "createMessage" || name == "messages" ||
		name == "completions"):
		return &AICallSite{SDK: "anthropic", Confidence: 0.75}
	case hasLangChain && (name == "generate" || name == "chat"):
		return &AICallSite{SDK: "langchain", Confidence: 0.65}
	}
	return nil
}

// rootIdentifierOfInvocation returns the leftmost identifier of a
// method-invocation chain. For `A.b().c().d()`, returns "A".
func rootIdentifierOfInvocation(node *sitter.Node, src []byte) string {
	obj := node.ChildByFieldName("object")
	if obj == nil {
		return ""
	}
	for obj.Type() == "method_invocation" {
		inner := obj.ChildByFieldName("object")
		if inner == nil {
			break
		}
		obj = inner
	}
	switch obj.Type() {
	case "identifier":
		return nodeText(obj, src)
	case "field_access", "scoped_identifier":
		text := nodeText(obj, src)
		if i := strings.Index(text, "."); i >= 0 {
			return text[:i]
		}
		return text
	}
	return ""
}

// extractJavaModelArg walks the invocation's children for a
// `.model("...")` builder call and returns the literal.
func extractJavaModelArg(node *sitter.Node, src []byte) string {
	var found string
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if n == nil || found != "" {
			return
		}
		if n.Type() == "method_invocation" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil && nodeText(nameNode, src) == "model" {
				args := n.ChildByFieldName("arguments")
				if args != nil && args.NamedChildCount() > 0 {
					arg := args.NamedChild(0)
					if arg.Type() == "string_literal" {
						found = stripJavaStringQuotes(nodeText(arg, src))
						return
					}
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			visit(n.Child(i))
		}
	}
	visit(node)
	return found
}

// collapseMethodInvocationText returns the dotted call path (without
// arguments). For `service.createChatCompletion(req)`, returns
// "service.createChatCompletion". For chained builders, returns the
// last invocation's dotted form.
func collapseMethodInvocationText(node *sitter.Node, src []byte) string {
	obj := node.ChildByFieldName("object")
	name := node.ChildByFieldName("name")

	if name == nil {
		return ""
	}
	nameText := nodeText(name, src)
	if obj == nil {
		return nameText
	}
	objText := collapseJavaWhitespace(nodeText(obj, src))
	// For long chains, just keep the immediate object.method form.
	if strings.Contains(objText, "\n") || len(objText) > 80 {
		// Try to extract the last identifier from the object expression.
		objText = lastIdentifierOf(obj, src)
	}
	return objText + "." + nameText
}

func lastIdentifierOf(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "identifier":
		return nodeText(n, src)
	case "method_invocation":
		// take the method's name as a synthesized identifier
		name := n.ChildByFieldName("name")
		if name != nil {
			return nodeText(name, src)
		}
	case "field_access":
		field := n.ChildByFieldName("field")
		if field != nil {
			return nodeText(field, src)
		}
	}
	return ""
}

func collapseJavaWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

func stripJavaStringQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
