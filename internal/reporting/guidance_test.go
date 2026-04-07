package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestWriteHealthGuidance_NoRuntime(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: signals.SignalWeakAssertion},
		},
	}
	var buf bytes.Buffer
	WriteHealthGuidance(&buf, snap)
	out := buf.String()
	if !strings.Contains(out, "runtime artifacts") {
		t.Errorf("expected guidance message, got: %q", out)
	}
	if !strings.Contains(out, "Jest:") || !strings.Contains(out, "Pytest:") {
		t.Error("expected framework-specific generation commands")
	}
	// Dead test detection is static — the runtime-required line should not mention it.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "require runtime") && strings.Contains(line, "dead") {
			t.Error("guidance should not list dead tests as requiring runtime (they use AST analysis)")
		}
	}
}

func TestWriteHealthGuidance_WithRuntime(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: signals.SignalFlakyTest},
		},
	}
	var buf bytes.Buffer
	WriteHealthGuidance(&buf, snap)
	if buf.Len() != 0 {
		t.Errorf("expected no output when runtime signals present, got: %q", buf.String())
	}
}

func TestWriteHealthGuidance_EmptySignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	var buf bytes.Buffer
	WriteHealthGuidance(&buf, snap)
	if !strings.Contains(buf.String(), "runtime artifacts") {
		t.Error("expected guidance for empty signals")
	}
}
