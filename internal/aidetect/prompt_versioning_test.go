package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func writePromptFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestPromptVersioning_FlagsBareFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "prompts/system.yaml", `
role: system
content: |
  You are a helpful assistant.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "system", Kind: models.SurfacePrompt},
		},
	}
	got := (&PromptVersioningDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAIPromptVersioning {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q, want medium", got[0].Severity)
	}
}

func TestPromptVersioning_AcceptsFilenameVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "prompts/system_v2.yaml", `
role: system
content: |
  You are a helpful assistant.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "system", Kind: models.SurfacePrompt},
		},
	}
	if got := (&PromptVersioningDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("filename-versioned prompt should not fire, got %d signals", len(got))
	}
}

func TestPromptVersioning_AcceptsInlineVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "prompts/system.yaml", `
version: 1.0.0
role: system
content: |
  You are a helpful assistant.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "system", Kind: models.SurfacePrompt},
		},
	}
	if got := (&PromptVersioningDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("inline-versioned prompt should not fire, got %d signals", len(got))
	}
}

func TestPromptVersioning_AcceptsCommentVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "prompts/system.txt", `# version: 0.3.1
You are a helpful assistant.
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "system", Kind: models.SurfacePrompt},
		},
	}
	if got := (&PromptVersioningDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("comment-versioned prompt should not fire, got %d signals", len(got))
	}
}

func TestPromptVersioning_OneSignalPerFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "prompts/multi.yaml", `
role: system
content: a
`)
	// Two surfaces in the same file → one signal.
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "a", Path: rel, Name: "a", Kind: models.SurfacePrompt},
			{SurfaceID: "b", Path: rel, Name: "b", Kind: models.SurfacePrompt},
		},
	}
	got := (&PromptVersioningDetector{Root: root}).Detect(snap)
	if len(got) != 1 {
		t.Errorf("got %d signals, want 1 (per-file dedup)", len(got))
	}
}

// TestPromptVersioning_RejectsPlaceholderTokens locks in the 0.2.0
// final-polish fix: pre-fix the inline-version regex's quoted-token
// branch accepted `version: "TODO"`, `version: "tbd"`, `version: ?`,
// etc. — silencing the detector with placeholder text. Now those
// placeholder tokens fall through and the detector fires.
func TestPromptVersioning_RejectsPlaceholderTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		body  string
	}{
		{"todo_quoted", `version: "TODO"` + "\n"},
		{"tbd_unquoted", `version: TBD` + "\n"},
		{"question_marks", `version: ???` + "\n"},
		{"placeholder_word", `version: placeholder` + "\n"},
		{"none_lowercase", `version: none` + "\n"},
		{"unknown", `version: "unknown"` + "\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := writePromptFile(t, root, "prompts/system.yaml", tc.body+"role: system\ncontent: hi\n")
			snap := &models.TestSuiteSnapshot{
				CodeSurfaces: []models.CodeSurface{
					{SurfaceID: "s1", Path: rel, Name: "system", Kind: models.SurfacePrompt},
				},
			}
			got := (&PromptVersioningDetector{Root: root}).Detect(snap)
			if len(got) == 0 {
				t.Fatalf("placeholder version `%s` should NOT satisfy the inline-version requirement; expected detector to fire", tc.body)
			}
		})
	}
}

func TestPromptVersioning_IgnoresInlineSourceFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writePromptFile(t, root, "src/agent.py", `
PROMPT = "You are a helpful assistant."
`)
	snap := &models.TestSuiteSnapshot{
		CodeSurfaces: []models.CodeSurface{
			{SurfaceID: "s1", Path: rel, Name: "PROMPT", Kind: models.SurfacePrompt},
		},
	}
	if got := (&PromptVersioningDetector{Root: root}).Detect(snap); len(got) != 0 {
		t.Errorf("inline source-file prompt should be skipped, got %d", len(got))
	}
}
