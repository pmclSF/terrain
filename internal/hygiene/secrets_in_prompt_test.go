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

func TestDetectSecretsInPrompt_OpenAIKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	apiKey := "sk-" + strings.Repeat("a", 30)
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
	token := "ghp_" + strings.Repeat("a", 38)
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
	awsKey := "AKIA" + "IOSFODNN7EXAMPLE"
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
	openAIKey := "sk-" + strings.Repeat("a", 30)
	awsKey := "AKIA" + "IOSFODNN7EXAMPLE"
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
