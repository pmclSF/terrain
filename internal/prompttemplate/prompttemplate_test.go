package prompttemplate

import (
	"errors"
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
