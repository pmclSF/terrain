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

// TestToolWithoutSandbox_BenignDestructiveObjects locks in the 0.2.0
// final-polish fix for the long-running false-positive class:
// `delete_cache`, `purge_logs`, `remove_session`, `truncate_buffer`,
// etc. The verb matches but the blast radius is bounded by the
// object noun. Always-high verbs (exec, send_payment, transfer) stay
// flagged regardless of object — covered by the next test.
func TestToolWithoutSandbox_BenignDestructiveObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := writeFile(t, root, "agents/tools.yaml", `
tools:
  - name: delete_cache
    description: clear the request-scope cache
  - name: purge_logs
    description: roll the in-memory log buffer
  - name: remove_session
    description: invalidate the current user session
  - name: truncate_buffer
    description: drop the recent-input buffer
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 0 {
		t.Errorf("benign-object destructive verbs should not fire; got %d signals: %+v", len(got), got)
	}
}

// TestToolWithoutSandbox_AlwaysHighVerbsStillFire ensures unbounded-
// blast-radius verbs (exec/eval/send_payment/transfer/charge) keep
// firing regardless of object noun. Pre-fix `exec_<anything>` would
// have been suppressed by a benign-object substring; post-fix
// always-high verbs short-circuit the benign check.
func TestToolWithoutSandbox_AlwaysHighVerbsStillFire(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := writeFile(t, root, "agents/tools.yaml", `
tools:
  - name: exec_command
    description: run an arbitrary shell command
  - name: send_payment
    description: charge the customer's saved card
`)
	got := (&ToolWithoutSandboxDetector{Root: root}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: rel}},
	})
	if len(got) != 2 {
		t.Errorf("always-high destructive verbs should fire even with mild objects; got %d signals: %+v", len(got), got)
	}
}
