package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestPromptInjection_FlagsPythonFString(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "agent.py", `
import openai

def chat(user_input):
    prompt = f"You are an assistant. The user said: {user_input}"
    return openai.ChatCompletion.create(model="gpt-4-0613", messages=[{"role":"user","content":prompt}])
`)
	got := (&PromptInjectionDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) == 0 {
		t.Fatalf("expected at least 1 signal, got 0")
	}
	if got[0].Type != signals.SignalAIPromptInjectionRisk {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want high", got[0].Severity)
	}
}

func TestPromptInjection_FlagsConcatAssignment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "handler.js", `
function handle(req, res) {
  let prompt = "You are an assistant. ";
  prompt += req.body.message;
  callLLM(prompt);
}
`)
	got := (&PromptInjectionDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) == 0 {
		t.Errorf("expected concat-assignment to fire, got 0 signals")
	}
}

func TestPromptInjection_IgnoresClean(t *testing.T) {
	t.Parallel()

	// Templated prompt with sanitised input — no concatenation, no
	// f-string boundary issue.
	root := t.TempDir()
	rel := writeFile(t, root, "agent.py", `
import openai
TEMPLATE = "You are an assistant. User said: {user_message}"
def chat(user_input):
    safe = sanitise(user_input)
    prompt = TEMPLATE.format(user_message=safe)
    return openai.ChatCompletion.create(model="gpt-4-0613", messages=[{"role":"user","content":prompt}])
`)
	got := (&PromptInjectionDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	// .format() with sanitised input should not fire — neither pattern
	// matches user_input on the .format line.
	if len(got) != 0 {
		t.Errorf("clean handler should not fire, got %d signals: %+v", len(got), got)
	}
}

func TestPromptInjection_IgnoresComments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "agent.py", `
# Example of bad code: prompt = f"You are an assistant. The user said: {user_input}"
import openai

def chat():
    pass
`)
	got := (&PromptInjectionDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("comment-only mention should not fire, got %d signals", len(got))
	}
}
