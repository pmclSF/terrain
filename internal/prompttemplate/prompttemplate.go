// Package prompttemplate renders prompt templates deterministically.
//
// The package supports two substitution syntaxes (Kinds) — mustache
// `{{var}}` for markdown / text template files, and Python f-string
// `{var}` for prompt bodies extracted from Python sources. The renderer
// is LLM-free, performs pure substitution, and never reads files itself
// — the caller supplies the body and a vars map.
package prompttemplate

import "strings"

// Kind identifies a template substitution syntax.
type Kind int

const (
	// KindUnknown means the body's syntax could not be inferred.
	KindUnknown Kind = iota
	// KindMustache uses double-brace placeholders: {{name}}.
	KindMustache
)

// Template is a parsed prompt template ready to render.
type Template struct {
	Kind Kind
	Body string
}

// Render substitutes placeholders in t.Body using vars and returns the
// rendered text. Returns *MissingVarError when t references a
// placeholder name that is not present in vars.
func (t Template) Render(vars map[string]string) (string, error) {
	switch t.Kind {
	case KindMustache:
		return renderMustache(t.Body, vars)
	default:
		return t.Body, nil
	}
}

// MissingVarError is returned when Render finds a placeholder whose
// name is not present in the vars map.
type MissingVarError struct {
	Name string
}

func (e *MissingVarError) Error() string {
	return "prompttemplate: missing variable " + e.Name
}

func renderMustache(body string, vars map[string]string) (string, error) {
	var out strings.Builder
	out.Grow(len(body))
	i := 0
	for i < len(body) {
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			rel := strings.Index(body[i+2:], "}}")
			if rel < 0 {
				out.WriteString(body[i:])
				return out.String(), nil
			}
			name := strings.TrimSpace(body[i+2 : i+2+rel])
			v, ok := vars[name]
			if !ok {
				return "", &MissingVarError{Name: name}
			}
			out.WriteString(v)
			i = i + 2 + rel + 2
			continue
		}
		out.WriteByte(body[i])
		i++
	}
	return out.String(), nil
}
