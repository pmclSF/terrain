package regressionsuite

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSuite_Valid(t *testing.T) {
	yaml := []byte(`
schema_version: 1
module: A7-barrel-resolver
max_tp_loss: 10
consumer_detectors:
  - untestedExport
  - orphanedTestFile
frozen_tps:
  - rule_id: untestedExport
    repo: github.com/example/app
    file: src/util/parse.ts
    line: 8
  - rule_id: orphanedTestFile
    repo: github.com/example/other
    file: tests/billing/order_test.py
`)
	s, err := ParseSuite(yaml, "test")
	if err != nil {
		t.Fatalf("ParseSuite: %v", err)
	}
	if s.Module != "A7-barrel-resolver" {
		t.Errorf("Module = %q", s.Module)
	}
	if s.MaxTPLoss != 10 {
		t.Errorf("MaxTPLoss = %d", s.MaxTPLoss)
	}
	if len(s.FrozenTPs) != 2 {
		t.Fatalf("expected 2 frozen TPs, got %d", len(s.FrozenTPs))
	}
}

func TestParseSuite_RejectsBadSchema(t *testing.T) {
	yaml := []byte(`
schema_version: 99
module: x
max_tp_loss: 0
frozen_tps: []
`)
	_, err := ParseSuite(yaml, "bad")
	if err == nil || !strings.Contains(err.Error(), "schema_version 99") {
		t.Errorf("expected schema-version error, got: %v", err)
	}
}

func TestParseSuite_RejectsMissingModule(t *testing.T) {
	yaml := []byte(`
schema_version: 1
module: ""
max_tp_loss: 0
frozen_tps: []
`)
	_, err := ParseSuite(yaml, "nomod")
	if err == nil || !strings.Contains(err.Error(), "module") {
		t.Errorf("expected missing-module error, got: %v", err)
	}
}

func TestParseSuite_RejectsDuplicateFrozenTP(t *testing.T) {
	yaml := []byte(`
schema_version: 1
module: x
max_tp_loss: 5
frozen_tps:
  - rule_id: r
    file: a.py
  - rule_id: r
    file: a.py
`)
	_, err := ParseSuite(yaml, "dup")
	if err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestCheck_AllPresent(t *testing.T) {
	s := &Suite{
		SchemaVersion: 1,
		Module:        "test",
		MaxTPLoss:     2,
		FrozenTPs: []FrozenTP{
			{RuleID: "r1", File: "a.py"},
			{RuleID: "r2", File: "b.py"},
		},
	}
	findings := []Finding{
		{RuleID: "r1", File: "a.py"},
		{RuleID: "r2", File: "b.py"},
		{RuleID: "r3", File: "c.py"},
	}
	report := s.Check(findings)
	if report.Failed {
		t.Errorf("expected pass, got fail")
	}
	if len(report.MissingTPs) != 0 {
		t.Errorf("expected 0 missing, got %d", len(report.MissingTPs))
	}
}

func TestCheck_MissingUnderThreshold(t *testing.T) {
	s := &Suite{
		SchemaVersion: 1,
		Module:        "test",
		MaxTPLoss:     2,
		FrozenTPs: []FrozenTP{
			{RuleID: "r1", File: "a.py"},
			{RuleID: "r2", File: "b.py"},
			{RuleID: "r3", File: "c.py"},
		},
	}
	findings := []Finding{
		{RuleID: "r1", File: "a.py"},
		// r2, r3 missing → 2 missing TPs
	}
	report := s.Check(findings)
	if report.Failed {
		t.Errorf("2 missing TPs at MaxTPLoss=2 should not fail")
	}
	if len(report.MissingTPs) != 2 {
		t.Errorf("expected 2 missing, got %d", len(report.MissingTPs))
	}
}

func TestCheck_MissingOverThreshold(t *testing.T) {
	s := &Suite{
		SchemaVersion: 1,
		Module:        "test",
		MaxTPLoss:     1,
		FrozenTPs: []FrozenTP{
			{RuleID: "r1", File: "a.py"},
			{RuleID: "r2", File: "b.py"},
			{RuleID: "r3", File: "c.py"},
		},
	}
	findings := []Finding{
		{RuleID: "r1", File: "a.py"},
	}
	report := s.Check(findings)
	if !report.Failed {
		t.Errorf("2 missing TPs at MaxTPLoss=1 should fail")
	}
}

func TestCheck_LineMatch(t *testing.T) {
	s := &Suite{
		SchemaVersion: 1,
		Module:        "test",
		MaxTPLoss:     0,
		FrozenTPs: []FrozenTP{
			{RuleID: "r1", File: "a.py", Line: 42},
		},
	}
	// Same rule, same file, different line — should NOT count as match
	findings := []Finding{
		{RuleID: "r1", File: "a.py", Line: 99},
	}
	report := s.Check(findings)
	if !report.Failed {
		t.Errorf("frozen TP at line 42 should not match finding at line 99")
	}
}

func TestCheck_EmptyRepoNoCrossRepoWildcard(t *testing.T) {
	// Empty Repo on the frozen TP must NOT match a finding with a
	// non-empty Repo (the previous wildcard behavior masked
	// regressions across repos).
	s := &Suite{
		SchemaVersion: 1,
		Module:        "test",
		MaxTPLoss:     0,
		FrozenTPs: []FrozenTP{
			{RuleID: "r1", File: "a.py"},
		},
	}
	findings := []Finding{
		{RuleID: "r1", Repo: "github.com/other", File: "a.py"},
	}
	report := s.Check(findings)
	if len(report.MissingTPs) != 1 {
		t.Errorf("empty Repo frozen TP should NOT match cross-repo finding; got %d missing", len(report.MissingTPs))
	}
}

func TestValidate_RejectsLineZeroVsLineNDoubleCount(t *testing.T) {
	// A suite with both a line=0 (file-scope) and a line=42 entry for
	// the same (rule, repo, file) lets ONE finding satisfy both. The
	// validator must reject the conflict.
	yaml := []byte(`
schema_version: 1
module: test
max_tp_loss: 0
frozen_tps:
  - rule_id: r1
    repo: example
    file: a.py
  - rule_id: r1
    repo: example
    file: a.py
    line: 42
`)
	_, err := ParseSuite(yaml, "dup")
	if err == nil {
		t.Errorf("expected validation error for file-scope + line-specific conflict, got nil")
	}
}

func TestLoadAll_EmptyDirIsOK(t *testing.T) {
	tmp := t.TempDir()
	suites, err := LoadAll(tmp)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(suites) != 0 {
		t.Errorf("expected 0 suites in empty dir, got %d", len(suites))
	}
}

func TestLoadAll_SkipsUnderscoreFiles(t *testing.T) {
	// Verifies _README.md doesn't trip the YAML loader.
	dir := filepath.Join("..", "..", "harness", "regression-suites")
	suites, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll on real dir: %v", err)
	}
	// Baseline: 0 suites populated yet; the _README.md should be skipped.
	if len(suites) != 0 {
		t.Logf("note: %d suites present (baseline expected 0)", len(suites))
	}
}
