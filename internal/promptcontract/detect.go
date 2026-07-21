package promptcontract

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Detect binds prompt surfaces to schemas and returns the drift: a bound prompt
// variable that references a field the schema does not declare. Two binding
// kinds, both DECLARATIVE (the precision spine — an unbound bare `{x}` never
// fires):
//   - attribute: `{user.user_id}` where a parameter `user: UserProfile` types it.
//   - explicit:  LangChain `input_variables` bound to the best-matching schema.
func Detect(schemas []SchemaDef, prompts []PromptSurface) []Drift {
	byName := map[string][]SchemaDef{}
	for _, s := range schemas {
		byName[s.Name] = append(byName[s.Name], s) // multi-valued: detect name collisions
	}
	// subIndex maps a base schema name to the in-repo schemas that extend it, so
	// a base-typed reference can be checked against what its subclasses declare.
	subIndex := map[string][]SchemaDef{}
	for _, defs := range byName {
		for _, s := range defs {
			for _, b := range s.Bases {
				subIndex[b] = append(subIndex[b], s)
			}
		}
	}

	var out []Drift
	for _, p := range prompts {
		// Attribute binding — unambiguous, highest precision.
		for _, v := range p.Vars {
			if v.Attr == "" {
				continue // bare {x} is not attribute-bound; do not guess (precision spine)
			}
			if p.Assigned[v.Name+"."+v.Attr] {
				continue // attribute is assigned (obj.attr = ...) in scope -> dynamic, not drift
			}
			typeName := p.ParamTypes[v.Name]
			if typeName == "" {
				continue // the object is not a typed parameter -> unbound -> skip
			}
			s, ok := resolveSchema(typeName, p.Path, p.Imports, byName)
			if !ok {
				continue // type is a library / not imported here -> not an in-repo contract
			}
			fields, complete := fieldsOf(s, byName, map[string]bool{})
			if !complete {
				continue // an unresolvable base class -> full contract unknown -> stay silent
			}
			if !fields[v.Attr] {
				if attrOnSubclass(s.Name, v.Attr, byName, subIndex, map[string]bool{}) {
					continue // a subclass declares it: a base-typed var is polymorphic, not drift
				}
				out = append(out, Drift{
					PromptPath: p.Path, PromptLine: v.Line,
					SchemaName: s.Name, SchemaPath: s.Path,
					Object: v.Name, Variable: v.Attr, Kind: "attribute",
					Message: fmt.Sprintf("Prompt references %s.%s, but %s (%s) declares no field %q",
						v.Name, v.Attr, s.Name, s.Path, v.Attr),
				})
			}
		}
		// NOTE: explicit LangChain input_variables are extracted (p.ExplicitVars)
		// but NOT correlated to a schema in v1. Binding them to "the best-matching
		// schema" false-positives on generic prompt vars (e.g. `question`) that
		// coincidentally match an unrelated schema's field. A precise fire needs a
		// real dataflow link (`.format(**model)`, `response_model=Schema`).
		// Precision over recall: only attribute-bound drift fires today.
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PromptPath != out[j].PromptPath {
			return out[i].PromptPath < out[j].PromptPath
		}
		if out[i].PromptLine != out[j].PromptLine {
			return out[i].PromptLine < out[j].PromptLine
		}
		return out[i].Variable < out[j].Variable
	})
	return out
}

// resolveSchema picks the ONE in-repo schema a parameter's type refers to, using
// the prompt file's imports: a schema defined in the same file, or one imported
// from an in-repo module of that name. It refuses to bind when the type is
// imported from a library (or not imported at all) — which is what kills the
// dominant real-world FP: a param typed `resp: Response` where Response is
// requests.Response (has status_code), not a local pydantic Response that lacks it.
func resolveSchema(typeName, promptPath string, imports map[string]string, byName map[string][]SchemaDef) (SchemaDef, bool) {
	name := baseTypeName(typeName)
	defs := byName[name]
	if len(defs) == 0 {
		return SchemaDef{}, false
	}
	for _, d := range defs { // a same-file definition needs no import
		if d.Path == promptPath {
			return d, true
		}
	}
	mod, ok := imports[name]
	if !ok {
		return SchemaDef{}, false // not imported here and not local -> do not bind
	}
	for _, d := range defs {
		if importResolvesToPath(mod, promptPath, d.Path) {
			return d, true
		}
	}
	return SchemaDef{}, false // imported from a module with no in-repo schema of that name -> library
}

// importResolvesToPath reports whether importing `mod` from `promptPath` reaches
// the file `defPath` (both repo-relative, slash-separated). Handles absolute
// dotted imports (tolerating src-layout prefixes via suffix match) and relative
// imports (leading dots resolved against the prompt file's package).
func importResolvesToPath(mod, promptPath, defPath string) bool {
	if strings.HasPrefix(mod, ".") {
		dots := 0
		for dots < len(mod) && mod[dots] == '.' {
			dots++
		}
		dir := path.Dir(promptPath)
		for i := 1; i < dots; i++ {
			dir = path.Dir(dir)
		}
		target := path.Join(dir, strings.ReplaceAll(mod[dots:], ".", "/"))
		return defPath == target+".py" || defPath == path.Join(target, "__init__.py")
	}
	suffix := strings.ReplaceAll(mod, ".", "/")
	return defPath == suffix+".py" || strings.HasSuffix(defPath, "/"+suffix+".py") ||
		defPath == suffix+"/__init__.py" || strings.HasSuffix(defPath, "/"+suffix+"/__init__.py")
}

// fieldsOf returns a schema's COMPLETE attribute set (own fields + methods +
// inherited), and whether it is fully known. Incomplete when a base class cannot
// be resolved to a single in-repo schema — then a "missing field" cannot be
// asserted and the caller stays silent.
func fieldsOf(s SchemaDef, byName map[string][]SchemaDef, seen map[string]bool) (map[string]bool, bool) {
	if seen[s.Name] {
		return map[string]bool{}, true // cycle guard
	}
	seen[s.Name] = true
	if s.Open {
		return nil, false // dynamic/open contract -> field set unknowable -> stay silent
	}
	fields := map[string]bool{}
	for _, f := range s.Fields {
		fields[f.Name] = true
	}
	for _, m := range s.Methods {
		fields[m] = true // a method / @property is a valid attribute access, not drift
	}
	for _, a := range s.SelfAttrs {
		fields[a] = true // self.x = ... assigned imperatively, still a valid attribute
	}
	for _, base := range s.Bases {
		defs := byName[base]
		if len(defs) != 1 {
			return nil, false // base outside the repo or ambiguous -> incomplete contract
		}
		bf, ok := fieldsOf(defs[0], byName, seen)
		if !ok {
			return nil, false
		}
		for k := range bf {
			fields[k] = true
		}
	}
	return fields, true
}

// attrOnSubclass reports whether any in-repo subclass of base declares attr
// (transitively), or whether a subclass's contract is incomplete. A variable
// typed as a base class may hold any subclass instance, so if a subclass
// provides the attribute the access is polymorphically valid — not drift. An
// incomplete subclass (unresolved base) is treated conservatively as "could
// have it", so an unknown subclass never produces a false drift on the base.
func attrOnSubclass(base, attr string, byName, subIndex map[string][]SchemaDef, seen map[string]bool) bool {
	for _, sub := range subIndex[base] {
		if seen[sub.Name] {
			continue
		}
		seen[sub.Name] = true
		sf, complete := fieldsOf(sub, byName, map[string]bool{})
		if !complete {
			return true // subclass contract unknown -> cannot assert the base lacks attr
		}
		if sf[attr] {
			return true
		}
		if attrOnSubclass(sub.Name, attr, byName, subIndex, seen) {
			return true
		}
	}
	return false
}

// baseTypeName resolves a declared type annotation to the type whose fields an
// attribute access `x.attr` actually reaches. This is NOT the innermost type: for
// a generic wrapper `RunContext[MyDeps]` or `List[Order]`, `x.attr` binds to the
// OUTER runtime type (RunContext / list), not the type parameter — unwrapping to
// the inner type is the dominant real-world FP (a `ctx: RunContext[Deps]` whose
// `ctx.deps` is a RunContext attribute, not a Deps field). Only genuinely
// transparent wrappers pass through to their inner type: Optional[X],
// Annotated[X, ...], Union[X, None], and `X | None`.
func baseTypeName(t string) string {
	t = strings.Trim(strings.TrimSpace(t), `"' `)
	// PEP 604 unions: "X | None" is Optional[X]; a union of >1 real type is
	// ambiguous, so it binds to nothing.
	if parts := splitTopLevel(t, '|'); len(parts) > 1 {
		nonNone := dropNone(parts)
		if len(nonNone) == 1 {
			return baseTypeName(nonNone[0])
		}
		return "" // ambiguous union -> no bind
	}
	outer, args := splitGeneric(t)
	switch shortName(outer) {
	case "Optional":
		if len(args) == 1 {
			return baseTypeName(args[0])
		}
	case "Annotated":
		if len(args) >= 1 {
			return baseTypeName(args[0]) // the type is the first arg; the rest is metadata
		}
	case "Union":
		if nonNone := dropNone(args); len(nonNone) == 1 {
			return baseTypeName(nonNone[0])
		}
		return "" // Union of multiple real types is ambiguous -> no bind
	}
	// Any other generic (List, Dict, Sequence, RunContext, a custom generic) or a
	// bare name: the attribute binds to the outer runtime type.
	return shortName(outer)
}

// splitGeneric splits "Foo[a, b]" into ("Foo", ["a","b"]); a bare "Foo" yields
// ("Foo", nil). Only a trailing, balanced "[...]" is treated as a subscript.
func splitGeneric(t string) (string, []string) {
	i := strings.Index(t, "[")
	if i < 0 || !strings.HasSuffix(t, "]") {
		return t, nil
	}
	return strings.TrimSpace(t[:i]), splitTopLevel(t[i+1:len(t)-1], ',')
}

// splitTopLevel splits on sep, ignoring separators nested inside [...] brackets.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

func dropNone(parts []string) []string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "None" {
			out = append(out, p)
		}
	}
	return out
}

func shortName(t string) string {
	if i := strings.LastIndex(t, "."); i >= 0 {
		t = t[i+1:]
	}
	return strings.TrimSpace(t)
}
