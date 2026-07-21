package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTmp(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// hiEntropy builds a deterministic high-entropy alphanumeric run of length n —
// a realistic (fake) key body that is NOT a single repeated char, so it clears
// the placeholder/low-entropy filter the way a real leaked key would.
func hiEntropy(n int) string {
	const a = "aB3xK9mP2qR7wL5vN8jT4hF6dY1cG0zE4uWsX"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(a[(i*7+3)%len(a)])
	}
	return b.String()
}

// hiEntropyUpper is hiEntropy restricted to [0-9A-Z] for AWS-shaped keys.
func hiEntropyUpper(n int) string {
	const a = "A3XK9MP2QR7WL5VN8JT4HF6DY1CG0ZE"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(a[(i*7+3)%len(a)])
	}
	return b.String()
}

func TestDetectSecretsInPrompt_OpenAIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	apiKey := "sk-" + hiEntropy(30)
	path := writeTmp(t, filepath.Join(dir, "summarize.txt"),
		"You are a helpful assistant. Use API key "+apiKey+" to call the model.")
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	kinds := sigs[0].Metadata["secretKinds"].([]string)
	if len(kinds) == 0 || kinds[0] != "openai-api-key" {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestDetectSecretsInPrompt_GitHubToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	token := "ghp_" + hiEntropy(38)
	path := writeTmp(t, filepath.Join(dir, "deploy.txt"),
		"Authenticate with token "+token+" then deploy.")
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
}

func TestDetectSecretsInPrompt_JWT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4ifQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	path := writeTmp(t, filepath.Join(dir, "p.txt"), "Use auth: "+jwt)
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 1 {
		t.Fatalf("expected JWT detection, got %+v", sigs)
	}
}

func TestDetectSecretsInPrompt_CleanPromptSuppressed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeTmp(t, filepath.Join(dir, "clean.txt"),
		"You are a helpful assistant. Summarize the user's input clearly.")
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 0 {
		t.Errorf("clean prompt should not fire, got %+v", sigs)
	}
}

func TestDetectSecretsInPrompt_AWSKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	awsKey := "AKIA" + hiEntropyUpper(16)
	path := writeTmp(t, filepath.Join(dir, "p.txt"),
		"Configure AWS with "+awsKey+" and the secret key from env.")
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 1 {
		t.Fatalf("expected AWS key detection, got %+v", sigs)
	}
}

func TestDetectSecretsInPrompt_MultipleKinds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	openAIKey := "sk-" + hiEntropy(30)
	awsKey := "AKIA" + hiEntropyUpper(16)
	path := writeTmp(t, filepath.Join(dir, "p.txt"), `
Configure with:
  OPENAI_KEY=`+openAIKey+`
  AWS_KEY=`+awsKey+`
`)
	sigs := DetectSecretsInPrompt([]string{path})
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal (per-file), got %d", len(sigs))
	}
	kinds := sigs[0].Metadata["secretKinds"].([]string)
	if len(kinds) < 2 {
		t.Errorf("expected ≥2 kinds, got %v", kinds)
	}
}

// TestDetectSecretsInPrompt_SuppressesPlaceholders locks the placeholder defense:
// documentation / example / low-entropy credential shapes in a prompt must not
// fire this Critical detector.
func TestDetectSecretsInPrompt_SuppressesPlaceholders(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"example-marker": "Use API key sk-your_key_here_replace_me_before_running now.",
		"repeated-char":  "key: sk-" + strings.Repeat("a", 30),
		"aws-doc-key":    "aws: AKIA" + "IOSFODNN7EXAMPLE",
		"placeholder":    "token: ghp_" + strings.Repeat("x", 38),
	}
	for name, body := range cases {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := writeTmp(t, filepath.Join(dir, name+".txt"), body)
			if sigs := DetectSecretsInPrompt([]string{path}); len(sigs) != 0 {
				t.Errorf("%s should be suppressed as placeholder, got %+v", name, sigs)
			}
		})
	}
}
