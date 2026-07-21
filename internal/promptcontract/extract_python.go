package promptcontract

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/pmclSF/terrain/internal/astguard"
	"github.com/pmclSF/terrain/internal/parserpool"
)

// aiImportRoots are the import roots that mark a file as an AI surface. Schema↔
// prompt correlation only runs on repos with AI context, which keeps non-AI
// code (a report template that happens to use {braces}) silent.
var aiImportRoots = map[string]bool{
	"openai": true, "anthropic": true, "langchain": true, "langchain_core": true,
	"langchain_openai": true, "langchain_community": true, "llama_index": true,
	"llamaindex": true, "cohere": true, "mistralai": true, "litellm": true,
	"instructor": true, "dspy": true, "guidance": true, "transformers": true,
	"google": true, "vertexai": true, "groq": true, "ollama": true,
}

var langchainPromptCtors = map[string]bool{
	"PromptTemplate": true, "ChatPromptTemplate": true,
	"from_template": true, "from_messages": true, "PipelinePromptTemplate": true,
}

// pyFile is the extraction result for one Python source file.
type pyFile struct {
	schemas []SchemaDef
	prompts []PromptSurface
	hasAI   bool
}

// extractPython parses one Python source and extracts schemas, prompt surfaces,
// and whether it imports an AI SDK. Returns zero-value (empty) on parse failure.
func extractPython(path string, src []byte) pyFile {
	var out pyFile
	// Skip source whose bracket nesting would make tree-sitter parsing and
	// traversal pathologically slow; real code never approaches the threshold.
	if astguard.LooksPathological(src) {
		return out
	}
	_ = parserpool.With(python.GetLanguage(), func(p *sitter.Parser) error {
		tree, err := p.ParseCtx(context.Background(), nil, src)
		if err != nil || tree == nil {
			return err
		}
		defer tree.Close()
		root := tree.RootNode()
		out.hasAI = pyHasAIImport(root, src)
		imports := pyExtractImports(root, src)
		consts := pyStringConsts(root, src)
		walk(root, func(n *sitter.Node) {
			switch n.Type() {
			case "class_definition":
				if s, ok := pyExtractSchema(n, src, path); ok {
					out.schemas = append(out.schemas, s)
				}
			case "string":
				if ps, ok := pyExtractPrompt(n, src, path); ok {
					ps.Imports = imports
					out.prompts = append(out.prompts, ps)
				}
			case "call":
				if ps, ok := pyExtractLangChain(n, src, path); ok {
					ps.Imports = imports
					out.prompts = append(out.prompts, ps)
				}
				if ps, ok := pyExtractFormatCall(n, src, path, consts); ok {
					ps.Imports = imports
					out.prompts = append(out.prompts, ps)
				}
			}
		})
		return nil
	})
	return out
}

// pyExtractImports maps each imported local name to the module it came from
// (with leading dots preserved for relative imports), so a parameter's type can
// be resolved to the RIGHT schema — a local one only when the file actually
// imports it from an in-repo module, never a library type of the same name.
func pyExtractImports(root *sitter.Node, src []byte) map[string]string {
	out := map[string]string{}
	walk(root, func(n *sitter.Node) {
		if n.Type() != "import_from_statement" {
			return
		}
		mod := childByField(n, "module_name")
		if mod == nil {
			return
		}
		modText := strings.TrimSpace(nodeText(mod, src))
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if c.StartByte() == mod.StartByte() {
				continue // the module_name itself
			}
			switch c.Type() {
			case "dotted_name":
				nm := nodeText(c, src)
				if j := strings.LastIndex(nm, "."); j >= 0 {
					nm = nm[j+1:]
				}
				out[strings.TrimSpace(nm)] = modText
			case "aliased_import":
				if alias := childByField(c, "alias"); alias != nil {
					out[nodeText(alias, src)] = modText
				}
			}
		}
	})
	return out
}

func pyHasAIImport(root *sitter.Node, src []byte) bool {
	found := false
	walk(root, func(n *sitter.Node) {
		if found {
			return
		}
		if n.Type() == "import_statement" || n.Type() == "import_from_statement" {
			// the first identifier of any dotted_name is the import root
			walk(n, func(m *sitter.Node) {
				if m.Type() == "dotted_name" && m.NamedChildCount() > 0 {
					if aiImportRoots[nodeText(m.NamedChild(0), src)] {
						found = true
					}
				}
			})
		}
	})
	return found
}

// terminalBases carry no user fields we need to resolve — a schema whose bases
// are all terminal (or in-repo and resolvable) has a COMPLETE, knowable contract.
var terminalBases = map[string]bool{
	"BaseModel": true, "BaseSettings": true, "object": true, "Generic": true,
	"ABC": true, "Protocol": true, "Enum": true, "IntEnum": true, "StrEnum": true,
	"TypedDict": true, "NamedTuple": true,
}

// pyExtractSchema recognizes pydantic BaseModel/BaseSettings and @dataclass, and
// records its non-terminal base classes so inherited fields can be resolved.
func pyExtractSchema(cls *sitter.Node, src []byte, path string) (SchemaDef, bool) {
	kind := ""
	var bases []string
	if supers := childByField(cls, "superclasses"); supers != nil {
		for i := 0; i < int(supers.NamedChildCount()); i++ {
			base := baseClassName(nodeText(supers.NamedChild(i), src))
			if base == "BaseModel" || base == "BaseSettings" {
				kind = "pydantic"
			}
			if base != "" && !terminalBases[base] {
				bases = append(bases, base)
			}
		}
	}
	if kind == "" && pyHasDataclassDecorator(cls, src) {
		kind = "dataclass"
	}
	// A class that inherits from a non-terminal (in-repo) base but shows no
	// pydantic/dataclass marker of its own is a candidate schema SUBCLASS —
	// e.g. `class AgentNode(WorkflowNode)`. Recording it (with its bases) lets
	// inheritance and the base↔subclass polymorphism check resolve across the
	// hierarchy. If its base chain never reaches a real schema, fieldsOf reports
	// the contract incomplete and the caller stays silent, so this cannot fire
	// on an unrelated plain-class hierarchy.
	if kind == "" && len(bases) > 0 {
		kind = "derived"
	}
	if kind == "" {
		return SchemaDef{}, false
	}
	s := SchemaDef{
		Name:  nodeText(childByField(cls, "name"), src),
		Path:  path,
		Line:  int(cls.StartPoint().Row) + 1,
		Kind:  kind,
		Bases: bases,
	}
	if body := childByField(cls, "body"); body != nil {
		scanClassBody(body, src, &s, 0)
	}
	// Recover imperatively-set attributes (self.x = ...) and detect dynamic/open
	// contracts — both are essential for precision: a @dataclass that sets
	// self.image_str in __post_init__, or a class with setattr/__getattr__, would
	// otherwise falsely look like it "declares no field x".
	pyCollectDynamicAttrs(cls, src, &s)

	// Collect every recognized schema (even one with no OWN fields) so that
	// classes which inherit all their fields still resolve through the registry.
	return s, true
}

// scanClassBody collects declared fields, methods, and class-level constants
// from a class body, descending into conditional/try/with/loop blocks so a
// field declared under `if TYPE_CHECKING:` (or similar) is still recognized.
// Both behaviors close false-positive drift: a field the code really declares
// must never be mistaken for a missing one.
func scanClassBody(n *sitter.Node, src []byte, s *SchemaDef, depth int) {
	if n == nil || depth > maxClassBodyDepth {
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		stmt := n.NamedChild(i)
		switch stmt.Type() {
		case "expression_statement":
			if stmt.NamedChildCount() == 0 {
				continue
			}
			assign := stmt.NamedChild(0)
			if assign.Type() != "assignment" {
				continue
			}
			left := childByField(assign, "left")
			if left == nil || left.Type() != "identifier" {
				continue
			}
			if typ := childByField(assign, "type"); typ != nil {
				s.Fields = append(s.Fields, Field{
					Name: nodeText(left, src),
					Type: nodeText(typ, src),
					Line: int(left.StartPoint().Row) + 1,
				})
				continue
			}
			// Unannotated class-level assignment (`STATUS = "pending"`): a valid
			// class attribute reachable as obj.STATUS, just not a typed field.
			// Record it so it isn't mistaken for drift. (pydantic v2 rejects
			// unannotated attrs; @dataclass and pydantic v1 allow them, and they
			// are common as class constants.)
			s.SelfAttrs = append(s.SelfAttrs, nodeText(left, src))
		case "function_definition":
			if nm := childByField(stmt, "name"); nm != nil {
				s.Methods = append(s.Methods, nodeText(nm, src)) // method / @property (bare)
			}
		case "decorated_definition":
			// @property / @cached_property / @computed_field def name(self): ...
			for j := 0; j < int(stmt.NamedChildCount()); j++ {
				if fn := stmt.NamedChild(j); fn.Type() == "function_definition" {
					if nm := childByField(fn, "name"); nm != nil {
						s.Methods = append(s.Methods, nodeText(nm, src))
					}
				}
			}
		case "if_statement", "try_statement", "with_statement", "for_statement", "while_statement":
			// A field annotated inside one of these blocks is still declared on
			// the class; descend into the block body/bodies to collect it.
			for j := 0; j < int(stmt.NamedChildCount()); j++ {
				if b := stmt.NamedChild(j); b.Type() == "block" {
					scanClassBody(b, src, s, depth+1)
				}
			}
		}
	}
}

// maxClassBodyDepth bounds the conditional/try nesting scanClassBody will
// descend through. Real class bodies nest a level or two at most; the cap keeps
// a pathological input from driving deep recursion (astguard already rejects
// the extreme cases up front).
const maxClassBodyDepth = 50

// pyCollectDynamicAttrs walks a class body for (a) attributes assigned via
// `self.x = ...` (valid attributes invisible to field-declaration parsing) and
// (b) markers that the attribute set is OPEN/dynamic and therefore statically
// unknowable: a `__getattr__`/`__getattribute__` method, a `setattr(self, ...)`
// call, `self.__dict__` manipulation, or a pydantic `extra="allow"` config.
func pyCollectDynamicAttrs(cls *sitter.Node, src []byte, s *SchemaDef) {
	seen := map[string]bool{}
	walk(cls, func(n *sitter.Node) {
		switch n.Type() {
		case "function_definition":
			if nm := childByField(n, "name"); nm != nil {
				switch nodeText(nm, src) {
				case "__getattr__", "__getattribute__":
					s.Open = true // dynamic attribute resolution -> contract is open
				}
			}
		case "call":
			fn := childByField(n, "function")
			if fn != nil && fn.Type() == "identifier" && nodeText(fn, src) == "setattr" {
				if args := childByField(n, "arguments"); args != nil && args.NamedChildCount() > 0 {
					if a0 := args.NamedChild(0); a0.Type() == "identifier" && nodeText(a0, src) == "self" {
						s.Open = true // setattr(self, <dynamic>, ...) -> contract is open
					}
				}
			}
		case "assignment":
			left := childByField(n, "left")
			if left == nil || left.Type() != "attribute" {
				return
			}
			obj := childByField(left, "object")
			attr := childByField(left, "attribute")
			if obj == nil || obj.Type() != "identifier" || nodeText(obj, src) != "self" || attr == nil {
				return
			}
			an := nodeText(attr, src)
			switch {
			case an == "__dict__":
				s.Open = true // self.__dict__ = / .update(...) -> contract is open
			case strings.HasPrefix(an, "__"):
				// dunder attribute — ignore
			case !seen[an]:
				seen[an] = true
				s.SelfAttrs = append(s.SelfAttrs, an)
			}
		}
	})
	// pydantic extra="allow" (ConfigDict or nested class Config) -> extra fields
	// beyond the declared set are accepted, so a "missing" field cannot be asserted.
	if body := childByField(cls, "body"); body != nil {
		compact := strings.ReplaceAll(nodeText(body, src), " ", "")
		if strings.Contains(compact, `extra="allow"`) ||
			strings.Contains(compact, `extra='allow'`) ||
			strings.Contains(compact, "Extra.allow") {
			s.Open = true
		}
	}
}

// baseClassName extracts the outer class name from a superclass expression:
// "Generic[T]" -> "Generic", "pydantic.BaseModel" -> "BaseModel", "Base" -> "Base".
func baseClassName(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.Index(t, "["); i >= 0 {
		t = t[:i]
	}
	if i := strings.LastIndex(t, "."); i >= 0 {
		t = t[i+1:]
	}
	return strings.TrimSpace(t)
}

func pyHasDataclassDecorator(cls *sitter.Node, src []byte) bool {
	parent := cls.Parent()
	if parent == nil || parent.Type() != "decorated_definition" {
		return false
	}
	found := false
	for i := 0; i < int(parent.NamedChildCount()); i++ {
		d := parent.NamedChild(i)
		if d.Type() == "decorator" && strings.Contains(nodeText(d, src), "dataclass") {
			found = true
		}
	}
	return found
}

// pyExtractPrompt pulls interpolation variables out of an f-string and records
// the enclosing function's typed parameters (for attribute binding).
func pyExtractPrompt(str *sitter.Node, src []byte, path string) (PromptSurface, bool) {
	var vars []VarRef
	for i := 0; i < int(str.NamedChildCount()); i++ {
		child := str.NamedChild(i)
		if child.Type() != "interpolation" {
			continue
		}
		expr := childByField(child, "expression")
		if expr == nil {
			continue
		}
		switch expr.Type() {
		case "attribute":
			obj := childByField(expr, "object")
			attr := childByField(expr, "attribute")
			if obj != nil && obj.Type() == "identifier" && attr != nil {
				an := nodeText(attr, src)
				if strings.HasPrefix(an, "__") {
					continue // dunder attribute (__module__, __class__) — always valid
				}
				vars = append(vars, VarRef{Name: nodeText(obj, src), Attr: an,
					Line: int(expr.StartPoint().Row) + 1})
			}
		case "identifier":
			vars = append(vars, VarRef{Name: nodeText(expr, src), Line: int(expr.StartPoint().Row) + 1})
		}
	}
	if len(vars) == 0 {
		return PromptSurface{}, false
	}
	return PromptSurface{
		Path:       path,
		Line:       int(str.StartPoint().Row) + 1,
		Vars:       vars,
		ParamTypes: pyEnclosingParamTypes(str, src),
		Assigned:   pyEnclosingAssignedAttrs(str, src),
	}, true
}

// pyEnclosingAssignedAttrs returns the set of "obj.attr" that are ASSIGNED
// (obj.attr = ...) anywhere in the nearest enclosing function, so a prompt that
// reads an attribute the code sets dynamically in the same scope is not treated
// as drift (the attribute exists at read time even if the schema never declares it).
func pyEnclosingAssignedAttrs(n *sitter.Node, src []byte) map[string]bool {
	out := map[string]bool{}
	for a := n.Parent(); a != nil; a = a.Parent() {
		if a.Type() != "function_definition" {
			continue
		}
		body := childByField(a, "body")
		if body != nil {
			walk(body, func(m *sitter.Node) {
				if m.Type() != "assignment" {
					return
				}
				left := childByField(m, "left")
				if left == nil || left.Type() != "attribute" {
					return
				}
				obj := childByField(left, "object")
				attr := childByField(left, "attribute")
				if obj != nil && obj.Type() == "identifier" && attr != nil {
					out[nodeText(obj, src)+"."+nodeText(attr, src)] = true
				}
			})
		}
		break // nearest enclosing function only
	}
	return out
}

// pyEnclosingParamTypes returns {param name -> declared type name} for the
// nearest enclosing function, so `user.user_id` can bind to `user: UserProfile`.
func pyEnclosingParamTypes(n *sitter.Node, src []byte) map[string]string {
	out := map[string]string{}
	for a := n.Parent(); a != nil; a = a.Parent() {
		if a.Type() != "function_definition" {
			continue
		}
		params := childByField(a, "parameters")
		if params == nil {
			break
		}
		for i := 0; i < int(params.NamedChildCount()); i++ {
			tp := params.NamedChild(i)
			if tp.Type() != "typed_parameter" {
				continue
			}
			typ := childByField(tp, "type")
			var name string
			for j := 0; j < int(tp.NamedChildCount()); j++ {
				if tp.NamedChild(j).Type() == "identifier" {
					name = nodeText(tp.NamedChild(j), src)
					break
				}
			}
			if name != "" && typ != nil {
				out[name] = strings.TrimSpace(nodeText(typ, src))
			}
		}
		break // nearest enclosing function only
	}
	return out
}

// pyExtractLangChain pulls the declared placeholders out of a
// PromptTemplate(input_variables=[...]) construction.
func pyExtractLangChain(call *sitter.Node, src []byte, path string) (PromptSurface, bool) {
	fn := childByField(call, "function")
	if fn == nil {
		return PromptSurface{}, false
	}
	name := nodeText(fn, src)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	if !langchainPromptCtors[name] {
		return PromptSurface{}, false
	}
	args := childByField(call, "arguments")
	if args == nil {
		return PromptSurface{}, false
	}
	var explicit []string
	for i := 0; i < int(args.NamedChildCount()); i++ {
		kw := args.NamedChild(i)
		if kw.Type() != "keyword_argument" {
			continue
		}
		if nodeText(childByField(kw, "name"), src) != "input_variables" {
			continue
		}
		list := childByField(kw, "value")
		if list == nil {
			continue
		}
		for j := 0; j < int(list.NamedChildCount()); j++ {
			s := list.NamedChild(j)
			if s.Type() == "string" {
				explicit = append(explicit, pyStringContent(s, src))
			}
		}
	}
	if len(explicit) == 0 {
		return PromptSurface{}, false
	}
	return PromptSurface{Path: path, Line: int(call.StartPoint().Row) + 1, ExplicitVars: explicit}, true
}

// pyStringContent returns the inner text of a string node (without quotes).
func pyStringContent(s *sitter.Node, src []byte) string {
	for i := 0; i < int(s.NamedChildCount()); i++ {
		if s.NamedChild(i).Type() == "string_content" {
			return nodeText(s.NamedChild(i), src)
		}
	}
	return strings.Trim(nodeText(s, src), `"'`)
}

// pyRawStringContent concatenates all string_content pieces of a string node
// (multi-line / triple-quoted strings have several), preserving literal braces.
func pyRawStringContent(s *sitter.Node, src []byte) string {
	var b strings.Builder
	for i := 0; i < int(s.NamedChildCount()); i++ {
		if s.NamedChild(i).Type() == "string_content" {
			b.WriteString(nodeText(s.NamedChild(i), src))
		}
	}
	if b.Len() == 0 {
		return strings.Trim(nodeText(s, src), `"'`)
	}
	return b.String()
}

// pyIsPlainString reports whether a string node is a plain string (no f-string
// interpolation children) — i.e. a `.format()` template rather than an already-
// interpolated f-string.
func pyIsPlainString(s *sitter.Node, src []byte) bool {
	for i := 0; i < int(s.NamedChildCount()); i++ {
		if s.NamedChild(i).Type() == "interpolation" {
			return false
		}
	}
	return true
}

// pyStringConsts maps a name to its string-literal value for names that are
// assigned a plain string literal EXACTLY once (module constant or local). Names
// assigned more than one distinct literal, or ever assigned a non-literal, are
// excluded — so a `.format()` template variable resolves only when its value is
// unambiguous (precision over recall).
func pyStringConsts(root *sitter.Node, src []byte) map[string]string {
	vals := map[string]string{}
	poisoned := map[string]bool{}
	walk(root, func(n *sitter.Node) {
		if n.Type() != "assignment" {
			return
		}
		left := childByField(n, "left")
		right := childByField(n, "right")
		if left == nil || left.Type() != "identifier" || right == nil {
			return
		}
		name := nodeText(left, src)
		if right.Type() == "string" && pyIsPlainString(right, src) {
			content := pyRawStringContent(right, src)
			if existing, seen := vals[name]; seen && existing != content {
				poisoned[name] = true
			}
			vals[name] = content
			return
		}
		poisoned[name] = true // assigned something that isn't a plain string literal
	})
	for p := range poisoned {
		delete(vals, p)
	}
	return vals
}

// pyExtractFormatCall recognizes `TEMPLATE.format(**model)` where model is a
// typed schema parameter, and models it as an attribute binding: each `{field}`
// placeholder becomes `model.field`, so a placeholder the schema does not
// declare drifts. The `**model` unpack is what makes this declarative — every
// placeholder MUST be a field of the unpacked model or `.format` raises at
// runtime — so it reaches the common bare-`{field}` template without guessing.
func pyExtractFormatCall(call *sitter.Node, src []byte, path string, consts map[string]string) (PromptSurface, bool) {
	fn := childByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return PromptSurface{}, false
	}
	if nodeText(childByField(fn, "attribute"), src) != "format" {
		return PromptSurface{}, false
	}
	recv := childByField(fn, "object")
	if recv == nil {
		return PromptSurface{}, false
	}
	var tmpl string
	switch recv.Type() {
	case "string":
		if !pyIsPlainString(recv, src) {
			return PromptSurface{}, false // an f-string is already interpolated
		}
		tmpl = pyRawStringContent(recv, src)
	case "identifier":
		c, ok := consts[nodeText(recv, src)]
		if !ok {
			return PromptSurface{}, false // template value not resolvable to a single literal
		}
		tmpl = c
	default:
		return PromptSurface{}, false
	}

	args := childByField(call, "arguments")
	if args == nil {
		return PromptSurface{}, false
	}
	varName := ""
	for i := 0; i < int(args.NamedChildCount()); i++ {
		arg := args.NamedChild(i)
		if arg.Type() != "dictionary_splat" { // the **model argument
			continue
		}
		if arg.NamedChildCount() == 0 {
			continue
		}
		if v := unwrapModelVar(arg.NamedChild(0), src); v != "" {
			varName = v
			break
		}
	}
	if varName == "" {
		return PromptSurface{}, false // no **model unpack of a resolvable variable
	}
	paramTypes := pyEnclosingParamTypes(call, src)
	typ, ok := paramTypes[varName]
	if !ok {
		return PromptSurface{}, false // the unpacked model is not a typed parameter -> unbound
	}

	placeholders := parseFormatPlaceholders(tmpl)
	if len(placeholders) == 0 {
		return PromptSurface{}, false
	}
	line := int(call.StartPoint().Row) + 1
	var vars []VarRef
	for _, ph := range placeholders {
		vars = append(vars, VarRef{Name: varName, Attr: ph, Line: line})
	}
	return PromptSurface{
		Path:       path,
		Line:       line,
		Vars:       vars,
		ParamTypes: map[string]string{varName: typ},
	}, true
}

// unwrapModelVar returns the variable name behind a `**` unpack expression when
// it is a typed model instance: `order`, `order.dict()`, `order.model_dump()`,
// or `asdict(order)`. Anything else (a bare dict, a function result) returns ""
// so the binding stays unresolved.
func unwrapModelVar(expr *sitter.Node, src []byte) string {
	switch expr.Type() {
	case "identifier":
		return nodeText(expr, src)
	case "call":
		fn := childByField(expr, "function")
		if fn == nil {
			return ""
		}
		if fn.Type() == "attribute" {
			meth := nodeText(childByField(fn, "attribute"), src)
			obj := childByField(fn, "object")
			if (meth == "dict" || meth == "model_dump") && obj != nil && obj.Type() == "identifier" {
				return nodeText(obj, src)
			}
			if meth == "asdict" {
				return firstIdentArg(expr, src)
			}
		}
		if fn.Type() == "identifier" && nodeText(fn, src) == "asdict" {
			return firstIdentArg(expr, src)
		}
	}
	return ""
}

// firstIdentArg returns the first positional argument of a call when it is a
// bare identifier (e.g. the X in asdict(X)); otherwise "".
func firstIdentArg(call *sitter.Node, src []byte) string {
	args := childByField(call, "arguments")
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.NamedChildCount()); i++ {
		a := args.NamedChild(i)
		if a.Type() == "identifier" {
			return nodeText(a, src)
		}
		return "" // first arg isn't a bare identifier
	}
	return ""
}

// parseFormatPlaceholders extracts the simple field names from a str.format()
// template: `{account_id}` and `{account_id:>10}` and `{account_id.name}` all
// yield "account_id". Escaped braces (`{{`, `}}`), positional (`{}`, `{0}`), and
// non-identifier contents are skipped.
func parseFormatPlaceholders(tmpl string) []string {
	var out []string
	seen := map[string]bool{}
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] != '{' {
			continue
		}
		if i+1 < len(tmpl) && tmpl[i+1] == '{' { // escaped {{
			i++
			continue
		}
		end := strings.IndexByte(tmpl[i+1:], '}')
		if end < 0 {
			break
		}
		content := tmpl[i+1 : i+1+end]
		i += end + 1
		// field name is the part before any .attr, [index], :spec, or !conv
		if j := strings.IndexAny(content, ".[:!"); j >= 0 {
			content = content[:j]
		}
		content = strings.TrimSpace(content)
		if content == "" || !isIdentifier(content) || seen[content] {
			continue
		}
		seen[content] = true
		out = append(out, content)
	}
	return out
}

// isIdentifier reports whether s is a valid Python identifier (used to reject
// positional `{0}` placeholders and format noise).
func isIdentifier(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return len(s) > 0
}
