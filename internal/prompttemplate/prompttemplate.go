// Package prompttemplate renders prompt templates deterministically.
//
// The package supports two substitution syntaxes (Kinds) — mustache
// `{{var}}` for markdown / text template files, and Python f-string
// `{var}` for prompt bodies extracted from Python sources. The renderer
// is LLM-free, performs pure substitution, and never reads files itself
// — the caller supplies the body and a vars map.
package prompttemplate

import (
	"path/filepath"
	"strings"
)

// Kind identifies a template substitution syntax.
type Kind int

const (
	// KindUnknown means the body's syntax could not be inferred.
	KindUnknown Kind = iota
	// KindMustache uses double-brace placeholders: {{name}}.
	KindMustache
	// KindFString uses single-brace placeholders: {name}.
	// `{{` and `}}` are literal braces.
	KindFString
)

// String returns a short lower-case label for k. Useful in test
// failure messages and shadow-event records.
func (k Kind) String() string {
	switch k {
	case KindMustache:
		return "mustache"
	case KindFString:
		return "fstring"
	default:
		return "unknown"
	}
}

// Detect infers the template Kind from a file path. The body parameter
// is reserved for future body-sniff detection (e.g., recognising a
// Python source file that contains an f-string prompt literal); it is
// unused today.
func Detect(path string, _ []byte) Kind {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown":
		return KindMustache
	default:
		return KindUnknown
	}
}

// Template is a parsed prompt template ready to render.
type Template struct {
	Kind Kind
	Body string
	// Path is the optional repo-relative file path of the template,
	// used in diagnostic messages (e.g. MissingVarError). May be empty
	// for in-memory templates.
	Path string
}

// Render substitutes placeholders in t.Body using vars and returns the
// rendered text. Returns *MissingVarError when t references a
// placeholder name that is not present in vars.
func (t Template) Render(vars map[string]string) (string, error) {
	switch t.Kind {
	case KindMustache:
		return renderMustache(t.Body, t.Path, vars)
	case KindFString:
		return renderFString(t.Body, t.Path, vars)
	default:
		return t.Body, nil
	}
}

// Vars returns the placeholder names referenced in t.Body, in source
// order with duplicates removed.
func (t Template) Vars() []string {
	switch t.Kind {
	case KindMustache:
		return varsMustache(t.Body)
	case KindFString:
		return varsFString(t.Body)
	default:
		return nil
	}
}

func renderFString(body, path string, vars map[string]string) (string, error) {
	var out strings.Builder
	out.Grow(len(body))
	i := 0
	for i < len(body) {
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			out.WriteByte('{')
			i += 2
			continue
		}
		if i+1 < len(body) && body[i] == '}' && body[i+1] == '}' {
			out.WriteByte('}')
			i += 2
			continue
		}
		if body[i] == '{' {
			rel := strings.IndexByte(body[i+1:], '}')
			if rel < 0 {
				out.WriteString(body[i:])
				return out.String(), nil
			}
			name := strings.TrimSpace(body[i+1 : i+1+rel])
			v, ok := vars[name]
			if !ok {
				return "", &MissingVarError{Name: name, Path: path}
			}
			out.WriteString(v)
			i = i + 1 + rel + 1
			continue
		}
		out.WriteByte(body[i])
		i++
	}
	return out.String(), nil
}

func varsFString(body string) []string {
	var out []string
	seen := map[string]struct{}{}
	i := 0
	for i < len(body) {
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			i += 2
			continue
		}
		if i+1 < len(body) && body[i] == '}' && body[i+1] == '}' {
			i += 2
			continue
		}
		if body[i] == '{' {
			rel := strings.IndexByte(body[i+1:], '}')
			if rel < 0 {
				return out
			}
			name := strings.TrimSpace(body[i+1 : i+1+rel])
			if _, dup := seen[name]; !dup {
				seen[name] = struct{}{}
				out = append(out, name)
			}
			i = i + 1 + rel + 1
			continue
		}
		i++
	}
	return out
}

func varsMustache(body string) []string {
	var out []string
	seen := map[string]struct{}{}
	i := 0
	for i < len(body) {
		if hasQuad(body, i, '{') || hasQuad(body, i, '}') {
			i += 4
			continue
		}
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			rel := strings.Index(body[i+2:], "}}")
			if rel < 0 {
				return out
			}
			name := strings.TrimSpace(body[i+2 : i+2+rel])
			if _, dup := seen[name]; !dup {
				seen[name] = struct{}{}
				out = append(out, name)
			}
			i = i + 2 + rel + 2
			continue
		}
		i++
	}
	return out
}

// MissingVarError is returned when Render finds a placeholder whose
// name is not present in the vars map.
type MissingVarError struct {
	Name string
	// Path is the template's optional repo-relative file path, copied
	// from Template.Path at render time. Empty for in-memory templates.
	Path string
}

func (e *MissingVarError) Error() string {
	if e.Path != "" {
		return "prompttemplate: missing variable " + e.Name + " (in " + e.Path + ")"
	}
	return "prompttemplate: missing variable " + e.Name
}

func renderMustache(body, path string, vars map[string]string) (string, error) {
	var out strings.Builder
	out.Grow(len(body))
	i := 0
	for i < len(body) {
		if hasQuad(body, i, '{') {
			out.WriteString("{{")
			i += 4
			continue
		}
		if hasQuad(body, i, '}') {
			out.WriteString("}}")
			i += 4
			continue
		}
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			rel := strings.Index(body[i+2:], "}}")
			if rel < 0 {
				out.WriteString(body[i:])
				return out.String(), nil
			}
			name := strings.TrimSpace(body[i+2 : i+2+rel])
			v, ok := vars[name]
			if !ok {
				return "", &MissingVarError{Name: name, Path: path}
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

func hasQuad(body string, i int, b byte) bool {
	return i+3 < len(body) && body[i] == b && body[i+1] == b && body[i+2] == b && body[i+3] == b
}
