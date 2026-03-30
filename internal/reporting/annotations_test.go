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
			KeyFindings: []analyze.KeyFinding{
				{Title: "Test finding", Severity: tt.severity, Category: "coverage_debt"},
			},
		}
		var buf bytes.Buffer
		RenderGitHubAnnotations(&buf, r)
		if !strings.HasPrefix(buf.String(), tt.prefix) {
			t.Errorf("severity %q: expected prefix %q, got %q", tt.severity, tt.prefix, buf.String())
		}
	}
}

func TestRenderGitHubAnnotations_CoverageWithFiles(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Weak coverage", Severity: "high", Category: "coverage_debt"},
		},
		WeakCoverageAreas: []analyze.WeakArea{
			{Path: "src/api/handlers.go"},
			{Path: "src/api/routes.go"},
		},
	}
	var buf bytes.Buffer
	RenderGitHubAnnotations(&buf, r)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 annotations (one per file), got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "file=src/api/handlers.go") {
		t.Errorf("expected file annotation, got: %s", lines[0])
	}
}

func TestRenderGitHubAnnotations_MetricInMessage(t *testing.T) {
	r := &analyze.Report{
		KeyFindings: []analyze.KeyFinding{
			{Title: "Duplicate tests", Severity: "medium", Category: "optimization", Metric: "340 tests"},
		},
	}
	var buf bytes.Buffer
	RenderGitHubAnnotations(&buf, r)
	if !strings.Contains(buf.String(), "(340 tests)") {
		t.Errorf("expected metric in message, got: %s", buf.String())
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
