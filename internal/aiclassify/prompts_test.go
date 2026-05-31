package aiclassify

import "testing"

func TestIsPromptFile_PromptExt(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"foo.prompt", true},
		{"foo.prompt.yaml", true},
		{"foo.prompt.yml", true},
	}
	for _, c := range cases {
		if got := IsPromptFile(c.path); got != c.want {
			t.Errorf("IsPromptFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsPromptFile_JinjaUnderPromptDir(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"prompts/system.jinja2", true},
		{"prompts/user.j2", true},
		{"chat_templates/qwen.jinja", true},
		{"src/prompt_templates/welcome.yaml", true},
		// Same extensions, NOT under prompt dir → reject
		{"templates/system.jinja2", false},
		{"src/main.jinja", false},
	}
	for _, c := range cases {
		if got := IsPromptFile(c.path); got != c.want {
			t.Errorf("IsPromptFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsPromptFile_RejectsCodeGenJinja(t *testing.T) {
	// Even when under a "prompts" dir, code-extension-before-template
	// is code-gen, not a prompt.
	cases := []struct {
		path string
		want bool
	}{
		{"prompts/render.go.j2", false},            // Go codegen
		{"prompts/types.ts.j2", false},             // TS codegen
		{"prompts/header.h.jinja2", false},         // C header codegen
		{"prompts/config.yml.j2", false},           // ansible-style
		{"sky/templates/lambda-ray.yml.j2", false}, // skypilot infra template
	}
	for _, c := range cases {
		if got := IsPromptFile(c.path); got != c.want {
			t.Errorf("IsPromptFile(%q) = %v, want %v (code-gen jinja rejection)", c.path, got, c.want)
		}
	}
}

func TestIsCodeGenTemplate(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"src/types.ts.j2", true},
		{"engine/generators/render.py", true},
		{"_templates/main.go", true},
		// Real prompts → not code-gen
		{"prompts/system.jinja2", false},
		{"foo.prompt.yaml", false},
	}
	for _, c := range cases {
		if got := IsCodeGenTemplate(c.path); got != c.want {
			t.Errorf("IsCodeGenTemplate(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
