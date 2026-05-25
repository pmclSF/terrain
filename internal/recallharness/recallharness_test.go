package recallharness

import (
	"strings"
	"testing"
)

func TestParseHarness_Valid(t *testing.T) {
	yaml := []byte(`
schema_version: 1
rule_id: untestedExport
union_min_recall: 0.8
mechanisms:
  - id: barrel-export
    description: barrel resolver
    min_recall: 0.4
  - id: scope-classifier
    min_recall: 0.3
golden_tps:
  - repo: a
    file: src/x.ts
    line: 1
    mechanism_hint: barrel-export
  - repo: b
    file: src/y.ts
    line: 2
    mechanism_hint: scope-classifier
  - file: src/z.ts
    line: 3
`)
	h, err := ParseHarness(yaml, "test")
	if err != nil {
		t.Fatalf("ParseHarness: %v", err)
	}
	if h.RuleID != "untestedExport" {
		t.Errorf("RuleID = %q", h.RuleID)
	}
	if len(h.GoldenTPs) != 3 {
		t.Errorf("expected 3 golden TPs, got %d", len(h.GoldenTPs))
	}
}

func TestParseHarness_RejectsUnknownMechanismHint(t *testing.T) {
	yaml := []byte(`
schema_version: 1
rule_id: r
mechanisms:
  - id: m1
golden_tps:
  - file: a.py
    mechanism_hint: nonexistent
`)
	_, err := ParseHarness(yaml, "bad")
	if err == nil || !strings.Contains(err.Error(), "undeclared mechanism") {
		t.Errorf("expected undeclared-mechanism error, got: %v", err)
	}
}

func TestParseHarness_RejectsDuplicateMechanism(t *testing.T) {
	yaml := []byte(`
schema_version: 1
rule_id: r
mechanisms:
  - id: m1
  - id: m1
golden_tps: []
`)
	_, err := ParseHarness(yaml, "dup")
	if err == nil || !strings.Contains(err.Error(), "duplicates id") {
		t.Errorf("expected duplicate-mechanism error, got: %v", err)
	}
}

func TestParseHarness_RejectsOutOfRangeRecall(t *testing.T) {
	yaml := []byte(`
schema_version: 1
rule_id: r
union_min_recall: 1.5
mechanisms: []
golden_tps: []
`)
	_, err := ParseHarness(yaml, "oor")
	if err == nil || !strings.Contains(err.Error(), "union_min_recall") {
		t.Errorf("expected out-of-range error, got: %v", err)
	}
}

func TestCheck_UnionRecall(t *testing.T) {
	h := &Harness{
		SchemaVersion:  1,
		RuleID:         "r",
		UnionMinRecall: 0.5,
		Mechanisms: []Mechanism{
			{ID: "m1"},
			{ID: "m2"},
		},
		GoldenTPs: []GoldenTP{
			{File: "a.py"},
			{File: "b.py"},
			{File: "c.py"},
			{File: "d.py"},
		},
	}
	// m1 catches a and b; m2 catches b and c → union = {a, b, c} = 3/4
	findings := []Finding{
		{RuleID: "r", Mechanism: "m1", File: "a.py"},
		{RuleID: "r", Mechanism: "m1", File: "b.py"},
		{RuleID: "r", Mechanism: "m2", File: "b.py"},
		{RuleID: "r", Mechanism: "m2", File: "c.py"},
	}
	report := h.Check(findings)
	if report.UnionCaught != 3 {
		t.Errorf("UnionCaught = %d, want 3", report.UnionCaught)
	}
	if report.UnionRecall != 0.75 {
		t.Errorf("UnionRecall = %f, want 0.75", report.UnionRecall)
	}
	if report.UnionMinFail {
		t.Errorf("0.75 >= 0.5 floor; UnionMinFail should be false")
	}
}

func TestCheck_PerMechanismHintedRecall(t *testing.T) {
	h := &Harness{
		SchemaVersion: 1,
		RuleID:        "r",
		Mechanisms: []Mechanism{
			{ID: "m1", MinRecall: 0.5},
			{ID: "m2"},
		},
		GoldenTPs: []GoldenTP{
			{File: "a.py", MechanismHint: "m1"},
			{File: "b.py", MechanismHint: "m1"},
			{File: "c.py", MechanismHint: "m2"},
		},
	}
	// m1 catches only a (1/2 = 0.5, equals floor → not a fail)
	findings := []Finding{
		{RuleID: "r", Mechanism: "m1", File: "a.py"},
	}
	report := h.Check(findings)
	if len(report.PerMechanism) != 2 {
		t.Fatalf("expected 2 mechanism reports, got %d", len(report.PerMechanism))
	}
	m1 := report.PerMechanism[0]
	if m1.ID != "m1" {
		t.Errorf("first mechanism = %q, want m1", m1.ID)
	}
	if m1.HintedGolden != 2 || m1.HintedCaught != 1 {
		t.Errorf("m1 hinted = %d/%d", m1.HintedCaught, m1.HintedGolden)
	}
	if m1.HintedRecall != 0.5 {
		t.Errorf("m1.HintedRecall = %f, want 0.5", m1.HintedRecall)
	}
	if m1.MinRecallFail {
		t.Errorf("0.5 == 0.5 floor — not a fail")
	}
}

func TestCheck_PerMechanismFailsFloor(t *testing.T) {
	h := &Harness{
		SchemaVersion: 1,
		RuleID:        "r",
		Mechanisms: []Mechanism{
			{ID: "m1", MinRecall: 0.5},
		},
		GoldenTPs: []GoldenTP{
			{File: "a.py", MechanismHint: "m1"},
			{File: "b.py", MechanismHint: "m1"},
			{File: "c.py", MechanismHint: "m1"},
			{File: "d.py", MechanismHint: "m1"},
		},
	}
	// m1 catches only a (1/4 = 0.25 < 0.5 floor)
	findings := []Finding{
		{RuleID: "r", Mechanism: "m1", File: "a.py"},
	}
	report := h.Check(findings)
	if !report.PerMechanism[0].MinRecallFail {
		t.Errorf("0.25 < 0.5 should fail floor")
	}
	if !report.AnyFail() {
		t.Errorf("AnyFail should be true")
	}
}

func TestCheck_UnionFailsFloor(t *testing.T) {
	h := &Harness{
		SchemaVersion:  1,
		RuleID:         "r",
		UnionMinRecall: 0.8,
		Mechanisms:     []Mechanism{{ID: "m1"}},
		GoldenTPs: []GoldenTP{
			{File: "a.py"}, {File: "b.py"}, {File: "c.py"}, {File: "d.py"},
		},
	}
	// Only 2/4 = 0.5 < 0.8
	findings := []Finding{
		{RuleID: "r", Mechanism: "m1", File: "a.py"},
		{RuleID: "r", Mechanism: "m1", File: "b.py"},
	}
	report := h.Check(findings)
	if !report.UnionMinFail {
		t.Errorf("0.5 < 0.8 should fail union floor")
	}
}

func TestCheck_IgnoresUndeclaredMechanism(t *testing.T) {
	h := &Harness{
		SchemaVersion: 1,
		RuleID:        "r",
		Mechanisms:    []Mechanism{{ID: "declared"}},
		GoldenTPs:     []GoldenTP{{File: "a.py"}},
	}
	// Finding from undeclared mechanism should not count.
	findings := []Finding{
		{RuleID: "r", Mechanism: "undeclared", File: "a.py"},
	}
	report := h.Check(findings)
	if report.UnionCaught != 0 {
		t.Errorf("undeclared mechanism finding should not count toward recall")
	}
}

func TestCheck_EmptyRepoNoCrossRepoWildcard(t *testing.T) {
	h := &Harness{
		SchemaVersion: 1,
		RuleID:        "r",
		Mechanisms:    []Mechanism{{ID: "m1"}},
		GoldenTPs: []GoldenTP{
			{File: "a.py"}, // empty Repo
		},
	}
	// A finding from a different repo should NOT count toward recall.
	findings := []Finding{
		{RuleID: "r", Mechanism: "m1", Repo: "github.com/foreign", File: "a.py"},
	}
	report := h.Check(findings)
	if report.UnionCaught != 0 {
		t.Errorf("foreign-repo finding should not count toward recall for empty-Repo golden TP; got %d", report.UnionCaught)
	}
}

func TestLoadAll_SkipsReadme(t *testing.T) {
	harnesses, err := LoadAll("../../harness/recall-harnesses")
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	// Baseline: 0 populated harnesses. _README.md must be skipped.
	if len(harnesses) != 0 {
		t.Logf("note: %d harnesses present (baseline expected 0)", len(harnesses))
	}
}
