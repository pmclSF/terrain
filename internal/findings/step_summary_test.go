package findings

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteStepSummary_GreenStateLine(t *testing.T) {
	t.Parallel()
	art := NewArtifact(nil)
	var buf bytes.Buffer
	if err := art.WriteStepSummary(&buf, StepSummaryOptions{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No findings — all rules passed.") {
		t.Errorf("missing green-state line: %s", output)
	}
	// Silence-on-green: no severity sections when zero findings.
	if strings.Contains(output, "Errors") || strings.Contains(output, "Warnings") {
		t.Errorf("severity sections leaked into green state: %s", output)
	}
}

func TestWriteStepSummary_GroupBySeverity(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{Version: 1, RuleID: "terrain/x/err", Severity: SeverityError,
			PrimaryLoc: Location{Path: "e.go"}, ShortMessage: "e", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/x/warn", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "w.go"}, ShortMessage: "w", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/x/notice", Severity: SeverityNotice,
			PrimaryLoc: Location{Path: "n.go"}, ShortMessage: "n", DocsURL: "https://x"},
	})
	var buf bytes.Buffer
	_ = art.WriteStepSummary(&buf, StepSummaryOptions{})
	out := buf.String()
	for _, want := range []string{
		"Errors (gate-blocking) (1)",
		"Warnings (1)",
		"Notices (1)",
		"terrain/x/err",
		"terrain/x/warn",
		"terrain/x/notice",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in summary:\n%s", want, out)
		}
	}
}

func TestWriteStepSummary_AnnotationCap(t *testing.T) {
	t.Parallel()
	// 60 errors with cap 5 → 5 inline + "and 55 more" line.
	var findings []Finding
	for i := 0; i < 60; i++ {
		findings = append(findings, Finding{
			Version: 1, RuleID: "terrain/x/err", Severity: SeverityError,
			PrimaryLoc: Location{Path: "p.go"}, ShortMessage: "e", DocsURL: "https://x",
		})
	}
	art := NewArtifact(findings)
	var buf bytes.Buffer
	_ = art.WriteStepSummary(&buf, StepSummaryOptions{AnnotationCap: 5})
	out := buf.String()
	if !strings.Contains(out, "and 55 more") {
		t.Errorf("missing overflow line:\n%s", out)
	}
}

func TestWriteStepSummary_IncludesRepoAndCommit(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{})
	var buf bytes.Buffer
	_ = art.WriteStepSummary(&buf, StepSummaryOptions{
		RepoName: "demo/repo",
		Commit:   "abc1234",
	})
	out := buf.String()
	if !strings.Contains(out, "demo/repo") {
		t.Errorf("missing repo name: %s", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("missing commit SHA: %s", out)
	}
}

func TestWriteStepSummary_CausePathRendering(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{
			Version: 1, RuleID: "terrain/regression/test-failed", Severity: SeverityError,
			PrimaryLoc: Location{Path: "test.py", Line: 42},
			CausePath: []Location{
				{Path: "frontend/x.tsx", Line: 10},
				{Path: "test.py", Line: 42},
			},
			ShortMessage: "test failed", DocsURL: "https://x",
			Reproduction: "terrain test",
		},
	})
	var buf bytes.Buffer
	_ = art.WriteStepSummary(&buf, StepSummaryOptions{})
	out := buf.String()
	if !strings.Contains(out, "Cause path:") {
		t.Errorf("missing cause path label: %s", out)
	}
	if !strings.Contains(out, "frontend/x.tsx:10") {
		t.Errorf("cause path entry missing: %s", out)
	}
	if !strings.Contains(out, "<details><summary>Reproduce locally</summary>") {
		t.Errorf("missing reproduce details block: %s", out)
	}
}
