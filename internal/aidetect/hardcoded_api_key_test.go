package aidetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestHardcodedAPIKey_DetectsRealKeys(t *testing.T) {
	t.Parallel()

	// Split at compile time so this source file does not itself
	// match GitHub's secret-scanning patterns.
	apiKey := "sk-" + "proj-abcdefghijklmnop1234567890ABCDEFGH"
	root := t.TempDir()
	rel := writeFile(t, root, "evals/agent.yaml",
		"\nname: classifier\nprovider:\n  name: openai\n  api_key: "+apiKey+"\n")
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}

	d := &HardcodedAPIKeyDetector{Root: root}
	got := d.Detect(snap)
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	sig := got[0]
	if sig.Type != signals.SignalAIHardcodedAPIKey {
		t.Errorf("type = %q, want aiHardcodedAPIKey", sig.Type)
	}
	if sig.Severity != models.SeverityCritical {
		t.Errorf("severity = %q, want critical", sig.Severity)
	}
	if sig.Location.File != rel {
		t.Errorf("location.file = %q, want %q", sig.Location.File, rel)
	}
	if sig.Location.Line != 5 {
		t.Errorf("location.line = %d, want 5", sig.Location.Line)
	}
	if len(sig.SeverityClauses) != 1 || sig.SeverityClauses[0] != "sev-critical-001" {
		t.Errorf("severityClauses = %v, want [sev-critical-001]", sig.SeverityClauses)
	}
	if sig.RuleID != "TER-AI-103" {
		t.Errorf("ruleId = %q, want TER-AI-103", sig.RuleID)
	}
	if sig.ConfidenceDetail == nil || sig.ConfidenceDetail.Quality != "heuristic" {
		t.Errorf("confidenceDetail wrong: %+v", sig.ConfidenceDetail)
	}
}

func TestHardcodedAPIKey_IgnoresPlaceholders(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/example.yaml", `
provider:
  api_key: sk-fake-key-do-not-use-replace-with-real
  another:  AKIAXXXXXXXXXXXXXXXX
  also:     ghp_exampleexampleexampleexampleexample
`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}

	d := &HardcodedAPIKeyDetector{Root: root}
	if got := d.Detect(snap); len(got) != 0 {
		t.Errorf("expected no signals on placeholders, got %d: %+v", len(got), got)
	}
}

func TestHardcodedAPIKey_SkipsNonConfigExtensions(t *testing.T) {
	t.Parallel()

	apiKey := "sk-" + "proj-abcdefghijklmnop1234567890ABCDEFGH"
	root := t.TempDir()
	rel := writeFile(t, root, "src/login.test.js",
		`const key = "`+apiKey+`";`)
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	}

	d := &HardcodedAPIKeyDetector{Root: root}
	if got := d.Detect(snap); len(got) != 0 {
		t.Errorf("expected detector to skip .js files, got %d signals", len(got))
	}
}

func TestHardcodedAPIKey_DetectsAcrossProviders(t *testing.T) {
	t.Parallel()

	// Provider-key shapes are split at compile time so GitHub's
	// secret-scanning patterns don't match this source file. Each
	// fragment alone fails the scanner's regex; concatenated at
	// runtime, the bytes written to the fixture file exercise our
	// detector.
	openaiKey := "sk-" + "proj-realKEY1234567890abcdefghijkl"
	anthropicKey := "sk-" + "ant-realToken1234567890abcdef"
	googleKey := "AIza" + "SyAVeryRealLookingKey12345678901234"
	awsKey := "AKIA" + "REALKEY1234567XY"
	githubKey := "ghp" + "_realtokenrealtokenrealtokenrealtoken12"

	cases := []struct {
		name     string
		filename string
		content  string
		want     int // signals expected (one per matching line, capped)
	}{
		{"openai", "a.yaml", "key: " + openaiKey, 1},
		{"anthropic", "b.yaml", "ANTHROPIC_API_KEY=" + anthropicKey, 1},
		{"google", "c.json", `{"key":"` + googleKey + `"}`, 1},
		{"aws", "d.toml", `aws = "` + awsKey + `"`, 1},
		{"github", "e.yml", "token: " + githubKey, 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := writeFile(t, root, tc.filename, tc.content+"\n")
			snap := &models.TestSuiteSnapshot{
				TestFiles: []models.TestFile{{Path: rel}},
			}
			got := (&HardcodedAPIKeyDetector{Root: root}).Detect(snap)
			if len(got) != tc.want {
				t.Errorf("%s: got %d signals, want %d", tc.name, len(got), tc.want)
			}
		})
	}
}
