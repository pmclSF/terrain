package promptcontract

// Field is one schema field with its declared type.
type Field struct {
	Name string
	Type string
	Line int
}

// SchemaDef is a declared schema (pydantic model, dataclass, zod object, TS
// interface) and its fields — the "contract" side of the boundary.
type SchemaDef struct {
	Name      string
	Path      string
	Line      int
	Kind      string   // "pydantic" | "dataclass" | "zod" | "interface"
	Bases     []string // non-terminal base classes (for inherited-field resolution)
	Fields    []Field
	Methods   []string // method / @property names — also valid attribute accesses
	SelfAttrs []string // attributes assigned imperatively (self.x = ...) in method bodies
	Open      bool     // contract is dynamic/open (setattr, __getattr__, extra="allow") — never assert a missing field
}

// HasField reports whether the schema declares a field of the given name.
func (s SchemaDef) HasField(name string) bool {
	for _, f := range s.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// VarRef is a variable referenced inside a prompt. For `{user.user_id}` Name is
// "user" and Attr is "user_id"; for a bare `{topic}` Name is "topic", Attr "".
type VarRef struct {
	Name string
	Attr string
	Line int
}

// PromptSurface is a prompt string (or LangChain template) and the variables it
// references — the "consumer" side of the boundary.
type PromptSurface struct {
	Path         string
	Line         int
	Vars         []VarRef          // interpolation references
	ParamTypes   map[string]string // enclosing-scope name -> declared schema type name (for attribute binding)
	ExplicitVars []string          // LangChain input_variables (authoritative declared placeholders)
	Imports      map[string]string // imported name -> module path (for import-scoped type resolution)
	Assigned     map[string]bool   // "obj.attr" assigned in the enclosing scope (dynamic attribute, not drift)
}

// Drift is one detected schema↔prompt inconsistency: a bound prompt variable
// that references a field the schema does not declare.
type Drift struct {
	PromptPath string
	PromptLine int
	SchemaName string
	SchemaPath string
	Object     string // the object variable the attribute is accessed on (e.g. `u` in `u.titel`)
	Variable   string // the placeholder / attribute that does not resolve
	Kind       string // "attribute" | "explicit"
	Message    string
}
