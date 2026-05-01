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
