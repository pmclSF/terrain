package findings

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestArtifact_RedactSource(t *testing.T) {
	t.Parallel()
	art := &Artifact{Findings: []Finding{
		{RuleID: "r1", Evidence: &Evidence{
			CodeExcerpt: "api_key = \"sk-secret\"",
			IOExamples:  []IOExample{{Input: "x", Expected: "y"}},
		}},
		{RuleID: "r2", Evidence: nil}, // nil evidence must be safe
	}}
	art.RedactSource()
	if got := art.Findings[0].Evidence.CodeExcerpt; got != "" {
		t.Errorf("code excerpt not redacted: %q", got)
	}
	// Non-source eval data is preserved; redact_source is about code only.
	if len(art.Findings[0].Evidence.IOExamples) != 1 {
		t.Errorf("io examples should be preserved, got %d", len(art.Findings[0].Evidence.IOExamples))
	}
	if art.Findings[1].Evidence != nil {
		t.Errorf("nil evidence should remain nil")
	}
}

// TestReadArtifact_RoundTripsWriteJSON pins the contract that ReadArtifact
// inverts WriteJSON: a written artifact decodes back to the same findings,
// including nested evidence — so downstream consumers (e.g. /terrain expand)
// read exactly what was written. TestArtifact_RoundTrip uses raw
// json.Unmarshal and does NOT exercise ReadArtifact.
func TestReadArtifact_RoundTripsWriteJSON(t *testing.T) {
	t.Parallel()
	in := []Finding{{
		Version:      1,
		RuleID:       "terrain/ai/missing-eval",
		Severity:     SeverityWarning,
		PrimaryLoc:   Location{Path: "prompts/main.md", Line: 7},
		ShortMessage: "no eval covers this prompt",
		Evidence:     &Evidence{CodeExcerpt: "render(prompt)"},
	}}
	var buf bytes.Buffer
	if err := NewArtifact(in).WriteJSON(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadArtifact(&buf)
	if err != nil {
		t.Fatalf("ReadArtifact: %v", err)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(got.Findings))
	}
	f := got.Findings[0]
	if f.RuleID != "terrain/ai/missing-eval" || f.ShortMessage != "no eval covers this prompt" {
		t.Errorf("round-trip lost finding fields: %+v", f)
	}
	if f.PrimaryLoc.Path != "prompts/main.md" || f.PrimaryLoc.Line != 7 {
		t.Errorf("round-trip lost location: %+v", f.PrimaryLoc)
	}
	if f.Evidence == nil || f.Evidence.CodeExcerpt != "render(prompt)" {
		t.Errorf("round-trip lost nested evidence: %+v", f.Evidence)
	}
}

func TestArtifact_RoundTrip(t *testing.T) {
	t.Parallel()
	in := []Finding{
		{
			Version:      1,
			RuleID:       "terrain/coverage/no-tests",
			Severity:     SeverityWarning,
			Tier:         TierStable,
			PrimaryLoc:   Location{Path: "src/auth.go", Line: 42, NodeKind: "code_unit"},
			ShortMessage: "untested code unit",
			DocsURL:      "https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/no-tests.md",
			Reproduction: "terrain test --selector coverage/no-tests",
		},
	}
	art := NewArtifact(in)
	if art.Version != 1 {
		t.Errorf("version = %d", art.Version)
	}

	var buf bytes.Buffer
	if err := art.WriteJSON(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}

	var decoded Artifact
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Findings) != 1 {
		t.Fatalf("decoded findings = %d", len(decoded.Findings))
	}
	if decoded.Findings[0].RuleID != "terrain/coverage/no-tests" {
		t.Errorf("rule_id = %q", decoded.Findings[0].RuleID)
	}
}

func TestArtifact_SortStable(t *testing.T) {
	t.Parallel()
	in := []Finding{
		{Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "z.go"}, ShortMessage: "z", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/coverage/no-tests", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "a.go"}, ShortMessage: "a", DocsURL: "https://x"},
		{Version: 1, RuleID: "terrain/hygiene/weak-assertion", Severity: SeverityWarning,
			PrimaryLoc: Location{Path: "m.go"}, ShortMessage: "m", DocsURL: "https://x"},
	}
	art := NewArtifact(in)
	if art.Findings[0].PrimaryLoc.Path != "a.go" {
		t.Errorf("first = %q", art.Findings[0].PrimaryLoc.Path)
	}
	if art.Findings[2].RuleID != "terrain/hygiene/weak-assertion" {
		t.Errorf("last rule = %q", art.Findings[2].RuleID)
	}
}

func TestValidate_HappyPath(t *testing.T) {
	t.Parallel()
	f := Finding{
		Version:      1,
		RuleID:       "terrain/coverage/no-tests",
		Severity:     SeverityWarning,
		PrimaryLoc:   Location{Path: "x"},
		ShortMessage: "x",
		DocsURL:      "https://x",
	}
	if err := f.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestValidate_RuleIDFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID string
		ok     bool
	}{
		{"terrain/coverage/no-tests", true},
		{"terrain/regression/eval-regression", true},
		{"terrain/ai/model-deprecation-risk", true},
		{"terrain/hygiene/permanently-skipped", true},
		{"terrain/category/rule-name-2", true},
		{"terrain/CATEGORY/lowercase", false},
		{"terrain//no-category", false},
		{"terrain/category", false},
		{"not-terrain/category/rule", false},
		{"terrain/category/rule/extra", false},
		{"", false},
	}
	for _, c := range cases {
		f := Finding{
			Version:      1,
			RuleID:       c.ruleID,
			Severity:     SeverityError,
			PrimaryLoc:   Location{Path: "x"},
			ShortMessage: "x",
			DocsURL:      "https://x",
		}
		err := f.Validate()
		if c.ok && err != nil {
			t.Errorf("%q: expected valid, got %v", c.ruleID, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%q: expected invalid", c.ruleID)
		}
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	t.Parallel()
	base := Finding{
		Version:      1,
		RuleID:       "terrain/x/y",
		Severity:     SeverityError,
		PrimaryLoc:   Location{Path: "a.go"},
		ShortMessage: "x",
		DocsURL:      "https://x",
	}
	cases := []struct {
		mutate func(*Finding)
		want   string
	}{
		{func(f *Finding) { f.Version = 0 }, "version"},
		{func(f *Finding) { f.RuleID = "" }, "rule_id"},
		{func(f *Finding) { f.Severity = "bogus" }, "severity"},
		{func(f *Finding) { f.PrimaryLoc.Path = "" }, "primary_loc"},
		{func(f *Finding) { f.ShortMessage = "" }, "short_message"},
		{func(f *Finding) { f.DocsURL = "" }, "docs_url"},
		{func(f *Finding) { f.Tier = "draft" }, "tier"},
	}
	for _, c := range cases {
		f := base
		c.mutate(&f)
		err := f.Validate()
		if err == nil {
			t.Errorf("expected error mentioning %q, got nil", c.want)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Errorf("error %q should mention %q", err.Error(), c.want)
		}
	}
}

func TestLint_Soft(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 300)
	f := Finding{
		ShortMessage: long,
		CausePath:    []Location{{Path: "x"}},
		// CauseLoc deliberately empty
		// Reproduction deliberately empty
	}
	warnings := f.Lint()
	if len(warnings) != 3 {
		t.Errorf("expected 3 lint warnings, got %d: %+v", len(warnings), warnings)
	}
}
