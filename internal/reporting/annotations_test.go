package reporting

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/analyze"
)

func TestRenderGitHubAnnotations_SeverityMapping(t *testing.T) {
	tests := []struct {
		severity string
		prefix   string
	}{
		{"critical", "::error"},
		{"high", "::error"},
		{"medium", "::warning"},
		{"low", "::notice"},
	}
	for _, tt := range tests {
		r := &analyze.Report{
			Signals: []analyze.FindingRecord{
				{RuleID: "terrain/coverage/blind-spot", Type: "coverageBlindSpot", Severity: tt.severity},
			},
		}
		var buf bytes.Buffer
		RenderGitHubAnnotations(&buf, r)
		if !strings.HasPrefix(buf.String(), tt.prefix) {
			t.Errorf("severity %q: expected prefix %q, got %q", tt.severity, tt.prefix, buf.String())
		}
	}
}

func TestRenderGitHubAnnotations_OnePerSignalWithLocation(t *testing.T) {
	r := &analyze.Report{
		Signals: []analyze.FindingRecord{
			{RuleID: "terrain/ai/prompt-schema-drift", Type: "aiPromptSchemaDrift", Severity: "high", File: "app.py", Line: 4, Evidence: "prompt references a missing field"},
			{RuleID: "terrain/quality/untested-export", Type: "untestedExport", Severity: "medium", File: "api/routes.go", Line: 12},
		},
	}
	var buf bytes.Buffer
	RenderGitHubAnnotations(&buf, r)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected one annotation per signal (2), got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "file=app.py,line=4") {
		t.Errorf("expected file+line location, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "prompt references a missing field") {
		t.Errorf("expected evidence in message, got: %s", lines[0])
	}
}

func TestRenderGitHubAnnotations_SkipsBlankRuleID(t *testing.T) {
	r := &analyze.Report{
		Signals: []analyze.FindingRecord{{Type: "x", Severity: "high"}},
	}
	var buf bytes.Buffer
	RenderGitHubAnnotations(&buf, r)
	if buf.Len() != 0 {
		t.Errorf("expected no annotation for a blank RuleID, got: %q", buf.String())
	}
}

func TestRenderGitHubAnnotations_Empty(t *testing.T) {
	r := &analyze.Report{}
	var buf bytes.Buffer
	RenderGitHubAnnotations(&buf, r)
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty report, got: %q", buf.String())
	}
}
