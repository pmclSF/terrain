package prompttemplate

import (
	"errors"
	"strings"
	"testing"
)

func TestRender_Mustache_SinglePlaceholder(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "Hello, {{name}}!"}
	got, err := tpl.Render(map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hello, World!"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_Mustache_MissingVarReturnsError(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "Hello, {{name}}!"}
	_, err := tpl.Render(map[string]string{})
	if err == nil {
		t.Fatalf("expected MissingVarError, got nil")
	}
	var mv *MissingVarError
	if !errors.As(err, &mv) {
		t.Fatalf("expected *MissingVarError, got %T: %v", err, err)
	}
	if mv.Name != "name" {
		t.Errorf("MissingVarError.Name = %q, want %q", mv.Name, "name")
	}
}

func TestRender_MissingVarError_CarriesTemplatePath(t *testing.T) {
	tpl := Template{
		Kind: KindMustache,
		Body: "Hello, {{name}}!",
		Path: "prompts/welcome.md",
	}
	_, err := tpl.Render(map[string]string{})
	var mv *MissingVarError
	if !errors.As(err, &mv) {
		t.Fatalf("expected *MissingVarError, got %T: %v", err, err)
	}
	if mv.Path != "prompts/welcome.md" {
		t.Errorf("MissingVarError.Path = %q, want %q", mv.Path, "prompts/welcome.md")
	}
	if !strings.Contains(mv.Error(), "prompts/welcome.md") {
		t.Errorf("Error() should mention path; got %q", mv.Error())
	}
}

func TestRender_Mustache_WhitespaceInsideBraces(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "Hello, {{ name }}! {{  greeting  }}"}
	got, err := tpl.Render(map[string]string{
		"name":     "World",
		"greeting": "Hi",
	})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hello, World! Hi"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestVars_Mustache_SourceOrder(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "{{greeting}}, {{name}}! You have {{count}} items."}
	got := tpl.Vars()
	want := []string{"greeting", "name", "count"}
	if !equalStrings(got, want) {
		t.Errorf("Vars = %v, want %v", got, want)
	}
}

func TestVars_Mustache_DedupesDuplicates(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "{{name}} {{name}} {{ name }} {{other}}"}
	got := tpl.Vars()
	want := []string{"name", "other"}
	if !equalStrings(got, want) {
		t.Errorf("Vars = %v, want %v", got, want)
	}
}

func TestRender_FString_SinglePlaceholder(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "Hello, {name}!"}
	got, err := tpl.Render(map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hello, World!"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_FString_MultiplePlaceholders(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "{greeting}, {name}! You have {count} items."}
	got, err := tpl.Render(map[string]string{
		"greeting": "Hi",
		"name":     "Ada",
		"count":    "3",
	})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hi, Ada! You have 3 items."
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_FString_WhitespaceInsideBraces(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "Hello, { name }!"}
	got, err := tpl.Render(map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hello, World!"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_FString_MissingVarReturnsError(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "Hello, {name}!"}
	_, err := tpl.Render(map[string]string{})
	var mv *MissingVarError
	if !errors.As(err, &mv) {
		t.Fatalf("expected *MissingVarError, got %T: %v", err, err)
	}
	if mv.Name != "name" {
		t.Errorf("MissingVarError.Name = %q, want %q", mv.Name, "name")
	}
}

func TestRender_FString_EscapedBraces(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "Use {{escaped}} like {name}"}
	got, err := tpl.Render(map[string]string{"name": "this"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Use {escaped} like this"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestVars_FString_SourceOrder(t *testing.T) {
	tpl := Template{Kind: KindFString, Body: "{a} {b} {{escaped}} {a} {c}"}
	got := tpl.Vars()
	want := []string{"a", "b", "c"}
	if !equalStrings(got, want) {
		t.Errorf("Vars = %v, want %v", got, want)
	}
}

func TestRender_Mustache_DoubledBracesEscape(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "Use {{{{escaped}}}} like {{name}}"}
	got, err := tpl.Render(map[string]string{"name": "this"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Use {{escaped}} like this"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestVars_Mustache_IgnoresDoubledBraces(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "{{{{escaped}}}} {{name}}"}
	got := tpl.Vars()
	want := []string{"name"}
	if !equalStrings(got, want) {
		t.Errorf("Vars = %v, want %v", got, want)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		path string
		want Kind
	}{
		{"md is mustache", "prompts/system.md", KindMustache},
		{"markdown is mustache", "prompts/system.markdown", KindMustache},
		{"upper case extension", "prompts/SYSTEM.MD", KindMustache},
		{"txt is unknown", "prompts/system.txt", KindUnknown},
		{"py is unknown", "agent.py", KindUnknown},
		{"no extension is unknown", "PROMPT", KindUnknown},
		{"empty path is unknown", "", KindUnknown},
		{"trailing dot is unknown", "prompt.", KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect(c.path, nil); got != c.want {
				t.Errorf("Detect(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestKind_String(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindMustache, "mustache"},
		{KindFString, "fstring"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.k.String(); got != c.want {
				t.Errorf("Kind(%d).String() = %q, want %q", int(c.k), got, c.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRender_Mustache_MultiplePlaceholders(t *testing.T) {
	tpl := Template{Kind: KindMustache, Body: "{{greeting}}, {{name}}! You have {{count}} items."}
	got, err := tpl.Render(map[string]string{
		"greeting": "Hi",
		"name":     "Ada",
		"count":    "3",
	})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "Hi, Ada! You have 3 items."
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}
