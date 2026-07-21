package findings

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestWriteJUnit_GroupedByRule(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityError,
			PrimaryLoc: Location{Path: "a.go", Line: 12}, ShortMessage: "untested A",
			DocsURL: "https://x", Reproduction: "terrain test",
		},
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityError,
			PrimaryLoc: Location{Path: "b.go", Line: 7}, ShortMessage: "untested B",
			DocsURL: "https://x", Reproduction: "terrain test",
		},
		{
			Version: 1, RuleID: "terrain/hygiene/weak-assertion", Severity: SeverityError,
			PrimaryLoc: Location{Path: "c_test.go", Line: 3}, ShortMessage: "weak",
			DocsURL: "https://x", Reproduction: "terrain test",
		},
	})

	var buf bytes.Buffer
	if err := art.WriteJUnit(&buf, JUnitOptions{}); err != nil {
		t.Fatalf("write: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `<testsuite name="terrain/coverage/no-tests"`) {
		t.Errorf("missing coverage suite: %s", output)
	}
	if !strings.Contains(output, `<testsuite name="terrain/hygiene/weak-assertion"`) {
		t.Errorf("missing hygiene suite")
	}
	if !strings.Contains(output, `tests="3"`) {
		t.Errorf("test count wrong; output:\n%s", output)
	}
	if !strings.Contains(output, `failures="3"`) {
		t.Errorf("failure count wrong")
	}

	// Verify XML is structurally valid.
	var parsed struct {
		Tests    int `xml:"tests,attr"`
		Failures int `xml:"failures,attr"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid XML: %v\noutput:\n%s", err, output)
	}
	if parsed.Tests != 3 || parsed.Failures != 3 {
		t.Errorf("parsed: tests=%d failures=%d", parsed.Tests, parsed.Failures)
	}
}

func TestWriteJUnit_FailureBodyIncludesCauseAndRepro(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{
			Version: 1, RuleID: "terrain/regression/test-failed", Severity: SeverityError,
			PrimaryLoc:   Location{Path: "api/test_summarize.py", Line: 42},
			ShortMessage: "test_summarize failed",
			LongMessage:  "AssertionError: expected refusal in response",
			CausePath: []Location{
				{Path: "frontend/CommentInput.tsx", Line: 42, NodeKind: "code_unit"},
				{Path: "backend/api/summarize.py", Line: 18, NodeKind: "handler"},
				{Path: "api/test_summarize.py", Line: 42, NodeKind: "test"},
			},
			DocsURL:      "https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/test-failed.md",
			Reproduction: "terrain test --selector regression/test-failed --filter test_summarize",
		},
	})

	var buf bytes.Buffer
	_ = art.WriteJUnit(&buf, JUnitOptions{})
	out := buf.String()

	for _, expected := range []string{
		"AssertionError: expected refusal",
		"Cause path:",
		"1. frontend/CommentInput.tsx:42",
		"3. api/test_summarize.py:42",
		"Reproduce locally:",
		"terrain test --selector regression/test-failed",
		"Docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/regression/test-failed.md",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q in body:\n%s", expected, out)
		}
	}
}

func TestWriteJUnit_WarningsOmittedByDefault(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "a.go"}, ShortMessage: "advisory",
			DocsURL: "https://x",
		},
	})
	var buf bytes.Buffer
	_ = art.WriteJUnit(&buf, JUnitOptions{})
	if strings.Contains(buf.String(), "advisory") {
		t.Errorf("warning leaked into JUnit by default: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `tests="0"`) {
		t.Errorf("tests count = 0 when only warnings: %s", buf.String())
	}
}

func TestWriteJUnit_WarningsIncludedOnOpt(t *testing.T) {
	t.Parallel()
	art := NewArtifact([]Finding{
		{
			Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "a.go"}, ShortMessage: "advisory",
			DocsURL: "https://x",
		},
	})
	var buf bytes.Buffer
	_ = art.WriteJUnit(&buf, JUnitOptions{EmitWarnings: true})
	if !strings.Contains(buf.String(), "a.go") {
		t.Errorf("warning case missing when opt-in: %s", buf.String())
	}
}

func TestWriteJUnit_StableSort(t *testing.T) {
	t.Parallel()
	// Findings deliberately ordered to test sort stability across runs.
	a := NewArtifact([]Finding{
		{Version: 1, RuleID: "terrain/z/y", Severity: SeverityError,
			PrimaryLoc: Location{Path: "z.go"}, ShortMessage: "z", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/a/y", Severity: SeverityError,
			PrimaryLoc: Location{Path: "a.go"}, ShortMessage: "a", DocsURL: "https://x"},
	})
	b := NewArtifact([]Finding{
		{Version: 1, RuleID: "terrain/a/y", Severity: SeverityError,
			PrimaryLoc: Location{Path: "a.go"}, ShortMessage: "a", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/z/y", Severity: SeverityError,
			PrimaryLoc: Location{Path: "z.go"}, ShortMessage: "z", DocsURL: "https://x"},
	})
	var bufA, bufB bytes.Buffer
	_ = a.WriteJUnit(&bufA, JUnitOptions{})
	_ = b.WriteJUnit(&bufB, JUnitOptions{})
	if bufA.String() != bufB.String() {
		t.Errorf("JUnit output not stable across input order")
	}
}
