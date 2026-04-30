package aidetect

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestToolWithoutSandbox_FlagsUngatedDestructive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "agents/tools.yaml", `
tools:
  - name: delete_user
    description: Delete a user account by id.
    parameters:
      type: object
      properties:
        user_id: {type: string}
  - name: get_user
    description: Look up a user by id.
    parameters:
      type: object
      properties:
        user_id: {type: string}
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1: %+v", len(got), got)
	}
	if got[0].Type != signals.SignalAIToolWithoutSandbox {
		t.Errorf("type = %q", got[0].Type)
	}
	if got[0].Severity != models.SeverityHigh {
		t.Errorf("severity = %q, want high", got[0].Severity)
	}
	if got[0].Metadata["tool"] != "delete_user" {
		t.Errorf("metadata.tool = %v", got[0].Metadata["tool"])
	}
}

func TestToolWithoutSandbox_AcceptsApprovalGate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "agents/tools.yaml", `
tools:
  - name: delete_user
    description: Delete a user account by id. Requires approval.
    parameters:
      type: object
      properties:
        user_id: {type: string}
    requires_approval: true
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("approval-gated tool should not fire, got %d signals: %+v", len(got), got)
	}
}

func TestToolWithoutSandbox_AcceptsSandboxFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "mcp/tools.json", `
{
  "tools": [
    {"name": "exec_command", "description": "Run shell command in sandbox", "sandbox": true}
  ]
}
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("sandboxed tool should not fire, got %d signals: %+v", len(got), got)
	}
}

func TestToolWithoutSandbox_IgnoresNonDestructive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := writeFile(t, root, "agents/tools.yaml", `
tools:
  - name: get_weather
    description: Look up the weather for a city.
    parameters:
      type: object
      properties:
        city: {type: string}
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("non-destructive tool should not fire, got %d signals", len(got))
	}
}

func TestToolWithoutSandbox_IgnoresNonToolYAML(t *testing.T) {
	t.Parallel()

	// Non-tool config — should not fire even if it contains the word
	// "delete" somewhere.
	root := t.TempDir()
	rel := writeFile(t, root, "config/db.yaml", `
host: localhost
on_drop: confirm
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("non-tool YAML should not fire, got %d signals", len(got))
	}
}
