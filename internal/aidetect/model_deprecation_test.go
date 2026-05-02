package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestModelDeprecation_FlagsFloatingTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/eval.yaml", `
provider:
  model: gpt-4
  temperature: 0
`)
	got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Type != signals.SignalAIModelDeprecationRisk {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityMedium {
		t.Errorf("severity = %q", got[0].Severity)
	}
	if got[0].Metadata["category"] != "floating" {
		t.Errorf("metadata.category = %v", got[0].Metadata["category"])
	}
}

func TestModelDeprecation_FlagsDeprecatedTag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "promptfoo/eval.yaml", `
providers:
  - id: openai:text-davinci-003
`)
	got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Metadata["category"] != "deprecated" {
		t.Errorf("metadata.category = %v", got[0].Metadata["category"])
	}
}

func TestModelDeprecation_AcceptsDatedVariants(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/eval.yaml", `
provider:
  model: gpt-4-0613
`)
	got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("dated variant should not fire, got %d signals: %+v", len(got), got)
	}
}

func TestModelDeprecation_IgnoresChangelogMention(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "evals/agent.py", `
# Migrated from gpt-4 to gpt-4-0613 to avoid the floating tag.
import openai
client = openai.OpenAI()
response = client.chat.completions.create(model="gpt-4-0613", messages=[])
`)
	got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("changelog comment should not fire, got %d signals: %+v", len(got), got)
	}
}

func TestModelDeprecation_DedupsPerLineMatch(t *testing.T) {
	t.Parallel()

	// Two matches of the same rule on one line — emit once.
	root := t.TempDir()
	rel := writeFile(t, root, "evals/eval.yaml",
		`models: [gpt-4, gpt-4]`)
	got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 1 {
		t.Errorf("got %d signals, want 1 (dedup)", len(got))
	}
}

// TestModelDeprecation_FlagsCodeDavinciDatedVariants locks in the
// 0.2 ship-blocker that pre-0.2 the bare `code-davinci` rule could
// not match the actual identifiers users have in code (`code-davinci-001`,
// `code-davinci-002`) because the trailing boundary class excludes `-`.
// Each dated variant is now its own list entry so the detector fires.
func TestModelDeprecation_FlagsCodeDavinciDatedVariants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"code_davinci_002", `provider:
  model: code-davinci-002
`},
		{"code_davinci_001", `provider:
  model: code-davinci-001
`},
		{"code_davinci_edit_001", `model: "code-davinci-edit-001"`},
		{"code_cushman_001", `provider: openai:code-cushman-001`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := writeFile(t, root, "evals/eval.yaml", tc.body)
			got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
				TestFiles: []models.TestFile{{Path: rel}},
			})
			if len(got) == 0 {
				t.Fatalf("code-davinci dated variant should fire; got 0 signals: body=%q", tc.body)
			}
			if got[0].Metadata["category"] != "deprecated" {
				t.Errorf("category = %v, want deprecated", got[0].Metadata["category"])
			}
		})
	}
}

// TestModelDeprecation_BroaderCommentPrefixes locks in the 0.2
// ship-blocker that pre-0.2 commentLooksLikeChangeLog only recognized
// `#`, `//`, `*`. SQL/Lua `--`, INI `;`, HTML `<!--`, Markdown bullet
// `-` / `*` / `>`, and reStructuredText `..` styles all caused false
// positives in CHANGELOG-shaped snippets that quoted deprecated tags.
func TestModelDeprecation_BroaderCommentPrefixes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"sql_dash_dash", `-- Deprecated: gpt-4 was the floating tag here; switch to gpt-4-0613`},
		{"ini_semicolon", `; Deprecated model gpt-4 — pin to gpt-4-0613`},
		{"html_block", `<!-- Deprecated: gpt-4 floating tag; switched to gpt-4-0613 -->`},
		{"markdown_bullet_dash", `- Deprecated: gpt-4 (floating) — pin gpt-4-0613`},
		{"markdown_bullet_star", `* Deprecated: gpt-4 (floating); now using gpt-4-0613`},
		{"markdown_blockquote", `> Deprecated gpt-4 floating tag; pin gpt-4-0613.`},
		{"rest_comment", `.. Deprecated: gpt-4 (floating)`},
		{"vb_apostrophe", `' Deprecated: gpt-4 floating tag, switched to gpt-4-0613`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := writeFile(t, root, "docs/changelog.md", tc.body+"\n")
			got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
				TestFiles: []models.TestFile{{Path: rel}},
			})
			if len(got) != 0 {
				t.Errorf("CHANGELOG-shaped comment should not fire; got %d signals: %+v", len(got), got)
			}
		})
	}
}

// TestModelDeprecation_DotVersionedDoesNotMatchUndatedParent locks in
// the 0.2 ship-blocker fix — `claude-2.1` and `gpt-3.5-turbo-0125`
// must not match their undated parents (`claude-2`, `gpt-3.5-turbo`).
// Pre-0.2 the trailing-boundary class did not exclude `.`, so any
// dot-versioned variant was a guaranteed false positive.
func TestModelDeprecation_DotVersionedDoesNotMatchUndatedParent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"claude_2_1", `model: claude-2.1`},
		{"claude_2_0", `model: claude-2.0`},
		{"gpt_3_5_turbo_0125", `model: gpt-3.5-turbo-0125`},
		{"gpt_4_0613", `model: gpt-4-0613`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := writeFile(t, root, "evals/eval.yaml", tc.body)
			got := (&ModelDeprecationDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
				TestFiles: []models.TestFile{{Path: rel}},
			})
			if len(got) != 0 {
				t.Errorf("dot-versioned variant should not match undated parent; got %d signals: %+v", len(got), got)
			}
		})
	}
}
