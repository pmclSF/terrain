package terrainconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_MinimalValid(t *testing.T) {
	t.Parallel()
	c, err := Parse([]byte("version: 1\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Version != 1 {
		t.Errorf("version = %d", c.Version)
	}
}

func TestParse_VersionRequired(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte("on_terrain_error: block\n"))
	if err == nil {
		t.Error("expected error when version missing")
	}
}

func TestParse_RuleSpec_BareSeverity(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
rules:
  regression/eval-regression: warning
  coverage/no-tests: off
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Rules["regression/eval-regression"].BareSeverity != "warning" {
		t.Errorf("bare severity = %+v", c.Rules["regression/eval-regression"])
	}
	if c.Rules["coverage/no-tests"].BareSeverity != "off" {
		t.Errorf("off severity not parsed")
	}
}

func TestParse_RuleSpec_BlockForm(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
rules:
  regression/eval-regression:
    severity: error
    threshold: 0.05
    samples_per_run: 5
    seed_strategy: fixed
    confidence_alpha: 0.05
    base_strategy: cached
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b := c.Rules["regression/eval-regression"].Block
	if b == nil {
		t.Fatal("block nil")
	}
	if b.Severity != "error" {
		t.Errorf("severity = %q", b.Severity)
	}
	if b.Threshold != 0.05 {
		t.Errorf("threshold = %v", b.Threshold)
	}
	if b.SamplesPerRun != 5 {
		t.Errorf("samples_per_run = %d", b.SamplesPerRun)
	}
}

func TestParse_RuleBlock_InvalidEnums(t *testing.T) {
	t.Parallel()
	cases := []string{
		"version: 1\nrules:\n  x/y: bogus\n",
		"version: 1\nrules:\n  x/y:\n    seed_strategy: bogus\n",
		"version: 1\nrules:\n  x/y:\n    base_strategy: bogus\n",
		"version: 1\nrules:\n  x/y:\n    scope: bogus\n",
		"version: 1\nrules:\n  x/y:\n    pii_engine: bogus\n",
		"version: 1\nrules:\n  x/y:\n    confidence_alpha: 1.5\n",
	}
	for _, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("expected error for: %s", c)
		}
	}
}

func TestParse_Ignore(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
ignore:
  paths:
    - "vendor/**"
    - "third_party/**"
  rules:
    coverage/no-tests:
      - "scripts/**"
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Ignore.Paths) != 2 {
		t.Errorf("paths = %v", c.Ignore.Paths)
	}
	if len(c.Ignore.Rules["coverage/no-tests"]) != 1 {
		t.Errorf("rule ignores = %v", c.Ignore.Rules)
	}
}

func TestParse_AISection(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
ai:
  framework: promptfoo
  scenarios_dir: evals/
  baselines_dir: evals/baselines/
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.AI.Framework != "promptfoo" {
		t.Errorf("framework = %q", c.AI.Framework)
	}
}

func TestParse_AIFramework_InvalidEnum(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
ai:
  framework: bogus
`)
	if _, err := Parse(src); err == nil {
		t.Error("expected error on bogus framework")
	}
}

func TestParse_Surfaces(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
surfaces:
  summarizer:
    description: "Summarizes user comments via LLM."
    type: llm
    model: gpt-4o-mini
  classifier:
    description: "Spam classifier."
    type: classical_ml
    file_path: src/classifier.py
`)
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Surfaces["summarizer"].Type != "llm" {
		t.Errorf("summarizer type = %q", c.Surfaces["summarizer"].Type)
	}
	if c.Surfaces["classifier"].FilePath != "src/classifier.py" {
		t.Errorf("classifier file_path = %q", c.Surfaces["classifier"].FilePath)
	}
}

func TestParse_Surfaces_BadType(t *testing.T) {
	t.Parallel()
	src := []byte(`
version: 1
surfaces:
  x:
    description: foo
    type: bogus
`)
	if _, err := Parse(src); err == nil {
		t.Error("expected error")
	}
}

func TestParse_OnTerrainError(t *testing.T) {
	t.Parallel()
	for _, val := range []string{"block", "pass"} {
		src := []byte("version: 1\non_terrain_error: " + val + "\n")
		if _, err := Parse(src); err != nil {
			t.Errorf("expected valid for %q, got %v", val, err)
		}
	}
	if _, err := Parse([]byte("version: 1\non_terrain_error: bogus\n")); err == nil {
		t.Error("expected error")
	}
}

func TestParse_ExplainProvider(t *testing.T) {
	t.Parallel()
	for _, val := range []string{"ollama", "openai", "anthropic", "custom", "none"} {
		src := []byte("version: 1\nexplain:\n  provider: " + val + "\n")
		if _, err := Parse(src); err != nil {
			t.Errorf("expected valid for %q, got %v", val, err)
		}
	}
}

func TestLoad_NoFileReturnsNil(t *testing.T) {
	t.Parallel()
	c, err := Load("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("expected nil, no error: got %v", err)
	}
	if c != nil {
		t.Errorf("expected nil config, got %+v", c)
	}
}

func TestLoad_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "terrain.yaml")
	_ = os.WriteFile(path, []byte("version: 1\n"), 0o644)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Version != 1 {
		t.Errorf("version: %d", c.Version)
	}
}

func TestSeverityFor(t *testing.T) {
	t.Parallel()
	c, _ := Parse([]byte(`
version: 1
rules:
  regression/eval-regression: warning
  coverage/no-tests:
    severity: error
`))
	if c.SeverityFor("terrain/regression/eval-regression", "high") != "warning" {
		t.Error("bare severity override missed")
	}
	if c.SeverityFor("regression/eval-regression", "high") != "warning" {
		t.Error("trimmed-prefix lookup missed")
	}
	if c.SeverityFor("terrain/coverage/no-tests", "high") != "error" {
		t.Error("block severity override missed")
	}
	if c.SeverityFor("terrain/unmapped/rule", "low") != "low" {
		t.Error("default not returned for unmapped rule")
	}
	var nilCfg *Config
	if nilCfg.SeverityFor("any/rule", "default") != "default" {
		t.Error("nil config should return default")
	}
}

func TestIsPathIgnored(t *testing.T) {
	t.Parallel()
	c, _ := Parse([]byte(`
version: 1
ignore:
  paths:
    - "vendor/**"
    - "*.gen.go"
  rules:
    coverage/no-tests:
      - "scripts/**"
`))
	cases := []struct {
		path   string
		ruleID string
		want   bool
	}{
		{"vendor/lib/x.go", "anything", true},
		{"x.gen.go", "anything", true},
		{"src/util.go", "anything", false},
		{"scripts/build.sh", "terrain/coverage/no-tests", true},
		{"scripts/build.sh", "terrain/other/rule", false},
	}
	for _, c1 := range cases {
		got := c.IsPathIgnored(c1.path, c1.ruleID)
		if got != c1.want {
			t.Errorf("IsPathIgnored(%q, %q) = %v, want %v", c1.path, c1.ruleID, got, c1.want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"vendor/**", "vendor/x/y.go", true},
		{"vendor/**", "vendor", false},
		{"vendor/**", "vendorish/x", false},
		{"*.go", "x.go", true},
		{"*.go", "x.py", false},
		{"*.go", "sub/x.go", false},
		{"sub/*.go", "sub/x.go", true},
		{"sub/*.go", "sub/inner/x.go", false},
		{"**/x.go", "deep/sub/x.go", true},
		{"**/x.go", "x.go", true},
		{"a/?.go", "a/b.go", true},
		{"a/?.go", "a/ab.go", false},
	}
	for _, tc := range cases {
		got := matchGlob(tc.pat, tc.name)
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pat, tc.name, got, tc.want)
		}
	}
}

func TestParse_RuleKey_Invalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"version: 1\nrules:\n  BadCategory/rule: error\n",
		"version: 1\nrules:\n  category/Rule: error\n",
		"version: 1\nrules:\n  no-slash: error\n",
		"version: 1\nrules:\n  too/many/slashes: error\n",
	}
	for _, c := range cases {
		_, err := Parse([]byte(c))
		if err == nil || !strings.Contains(err.Error(), "rule key") {
			t.Errorf("expected rule-key error for:\n%s\ngot: %v", c, err)
		}
	}
}
