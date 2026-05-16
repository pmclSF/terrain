package findings

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

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
			DocsURL:      "https://terrain.dev/rules/coverage/no-tests",
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
