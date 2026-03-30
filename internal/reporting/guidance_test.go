package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestWriteHealthGuidance_NoRuntime(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion"},
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
}

func TestWriteHealthGuidance_WithRuntime(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "flakyTest"},
		},
	}
	var buf bytes.Buffer
	WriteHealthGuidance(&buf, snap)
	if buf.Len() != 0 {
		t.Errorf("expected no output when runtime signals present, got: %q", buf.String())
	}
}

func TestWriteHealthGuidance_EmptySignals(t *testing.T) {
	snap := &models.TestSuiteSnapshot{}
	var buf bytes.Buffer
	WriteHealthGuidance(&buf, snap)
	if !strings.Contains(buf.String(), "runtime artifacts") {
		t.Error("expected guidance for empty signals")
	}
}
