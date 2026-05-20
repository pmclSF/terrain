package surfacelit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckBytes_Present(t *testing.T) {
	cases := []struct {
		name    string
		surface string
		body    string
	}{
		{"plain identifier in code", "buildPrompt", `function buildPrompt() {}`},
		{"model name in string literal", "gpt-4o", `const m = "gpt-4o";`},
		{"snake_case", "summarizer_template", `template: summarizer_template`},
		{"json key", "chatbot-prompt", `"name": "chatbot-prompt",`},
		{"trailing newline", "x", "x\n"},
		{"surrounded by punctuation", "alpha", `(alpha)`},
		{"between quotes and comma", "gpt-4o-mini", `models = ["gpt-4o-mini", "claude"]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CheckBytes(c.surface, []byte(c.body)); got != Present {
				t.Errorf("CheckBytes(%q, %q) = %v, want Present", c.surface, c.body, got)
			}
		})
	}
}

func TestCheckBytes_Absent(t *testing.T) {
	cases := []struct {
		name    string
		surface string
		body    string
	}{
		{"missing entirely", "buildPrompt", `function other() {}`},
		{"substring of longer identifier", "gpt-4", `model = "gpt-4-turbo"`},
		{"hyphen-extended", "gpt", `model = "gpt-4"`},
		{"in line comment //", "buildPrompt", `// uses buildPrompt`},
		{"in line comment #", "buildPrompt", `# uses buildPrompt`},
		{"in block comment", "buildPrompt", `/* this calls buildPrompt below */`},
		{"prefix of identifier", "build", `function buildPrompt() {}`},
		{"suffix of identifier", "Prompt", `function buildPrompt() {}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CheckBytes(c.surface, []byte(c.body)); got != Absent {
				t.Errorf("CheckBytes(%q, %q) = %v, want Absent", c.surface, c.body, got)
			}
		})
	}
}

func TestCheckBytes_EmptyName(t *testing.T) {
	if got := CheckBytes("", []byte("anything")); got != Skipped {
		t.Errorf("empty name → %v, want Skipped", got)
	}
	if got := CheckBytes("   ", []byte("anything")); got != Skipped {
		t.Errorf("whitespace name → %v, want Skipped", got)
	}
}

func TestCheckBytes_NotInsideString_LooksLikeComment(t *testing.T) {
	// A string literal that happens to contain "//" should NOT be
	// treated as a comment marker.
	body := `const url = "https://example.com/buildPrompt";`
	if got := CheckBytes("buildPrompt", []byte(body)); got != Present {
		t.Errorf("buildPrompt inside URL should be Present, got %v", got)
	}
}

func TestCheckBytes_HashInString(t *testing.T) {
	// A `#` inside a string literal should not start a comment, so a
	// surface following it should still be Present.
	body := `marker = "session#abc" ; model = "claude-3"`
	if got := CheckBytes("claude-3", []byte(body)); got != Present {
		t.Errorf("claude-3 after stringified # should be Present, got %v", got)
	}
}

func TestCheck_FilePresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("model: gpt-4o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Check("gpt-4o", path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if r != Present {
		t.Errorf("Check → %v, want Present", r)
	}
}

func TestCheck_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("model: claude-3-opus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, _ := Check("gpt-4o", path)
	if r != Absent {
		t.Errorf("Check → %v, want Absent", r)
	}
}

func TestCheck_MissingFile(t *testing.T) {
	r, err := Check("anything", "/no/such/path")
	if r != Skipped || err == nil {
		t.Errorf("Check on missing file → (%v, %v), want (Skipped, err)", r, err)
	}
}

func TestCheck_OversizedFileSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	// Write a file just over the max.
	big := make([]byte, MaxFileBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Check("xxx", path)
	if r != Skipped || err != nil {
		t.Errorf("oversized file → (%v, %v), want (Skipped, nil)", r, err)
	}
}

func TestReason(t *testing.T) {
	if got := Reason(Present, "n", "p"); !strings.Contains(got, "present") {
		t.Errorf("Reason(Present) = %q", got)
	}
	if got := Reason(Absent, "n", "p"); !strings.Contains(got, "absent") {
		t.Errorf("Reason(Absent) = %q", got)
	}
	if got := Reason(Skipped, "n", "p"); !strings.Contains(got, "skipped") {
		t.Errorf("Reason(Skipped) = %q", got)
	}
}

func TestStripComments_PreservesNonCommentContent(t *testing.T) {
	input := []byte(`function f() {
	const m = "gpt-4o"; // we like this one
	// const old = "gpt-3";
	return m;
}`)
	stripped := stripComments(input)
	s := string(stripped)
	if !strings.Contains(s, "gpt-4o") {
		t.Errorf("expected non-comment content preserved, got: %s", s)
	}
	if strings.Contains(s, "gpt-3") {
		t.Errorf("expected commented-out model removed, got: %s", s)
	}
}

func TestStripComments_HandlesBlockComments(t *testing.T) {
	input := []byte(`alpha /* beta gamma */ delta`)
	got := string(stripComments(input))
	if strings.Contains(got, "beta") || strings.Contains(got, "gamma") {
		t.Errorf("block comment content not stripped: %s", got)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "delta") {
		t.Errorf("non-comment content lost: %s", got)
	}
}
