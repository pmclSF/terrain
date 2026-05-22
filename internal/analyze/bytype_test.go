package analyze

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestBuildSignalSummary_ByType asserts the per-rule_id breakdown in
// SignalBreakdown.ByType is populated correctly. Downstream tools
// (PR-comment renderers, recall harnesses) read this field; a
// serialization regression must not silently zero it.
func TestBuildSignalSummary_ByType(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", Severity: models.SeverityMedium, Category: models.CategoryQuality},
			{Type: "weakAssertion", Severity: models.SeverityHigh, Category: models.CategoryQuality},
			{Type: "untestedExport", Severity: models.SeverityMedium, Category: models.CategoryQuality},
			{Type: "aiModelDeprecationRisk", Severity: models.SeverityHigh, Category: models.CategoryAI},
		},
	}

	sb := buildSignalSummary(snap)
	if sb.Total != 4 {
		t.Errorf("Total = %d, want 4", sb.Total)
	}
	if got := sb.ByType["weakAssertion"]; got != 2 {
		t.Errorf("ByType[weakAssertion] = %d, want 2", got)
	}
	if got := sb.ByType["untestedExport"]; got != 1 {
		t.Errorf("ByType[untestedExport] = %d, want 1", got)
	}
	if got := sb.ByType["aiModelDeprecationRisk"]; got != 1 {
		t.Errorf("ByType[aiModelDeprecationRisk] = %d, want 1", got)
	}
	// Empty bucket: a type that didn't fire should not appear at all.
	if _, ok := sb.ByType["missingType"]; ok {
		t.Errorf("ByType should not include unfired types; got %v", sb.ByType)
	}
}

// TestBuildSignalSummary_ByType_EmptySnapshot pins the zero case so
// an empty signals list yields an empty ByType map (not nil), keeping
// JSON output stable.
func TestBuildSignalSummary_ByType_EmptySnapshot(t *testing.T) {
	sb := buildSignalSummary(&models.TestSuiteSnapshot{})
	if sb.Total != 0 {
		t.Errorf("Total = %d, want 0", sb.Total)
	}
	if sb.ByType == nil {
		t.Error("ByType should be an empty map, not nil, so downstream JSON consumers see {}")
	}
	if len(sb.ByType) != 0 {
		t.Errorf("ByType should be empty; got %v", sb.ByType)
	}
}

// TestBuildFindingRecords asserts the per-finding payload shape that
// PR-comment / suppression consumers depend on.
func TestBuildFindingRecords(t *testing.T) {
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{
				Type:            "weakAssertion",
				Category:        models.CategoryQuality,
				Severity:        models.SeverityMedium,
				Explanation:     "test asserts only on length",
				SuggestedAction: "add a value assertion",
				FindingID:       "wa@a.go:Test#abc",
				Location:        models.SignalLocation{File: "a.go", Line: 42, Symbol: "TestFoo"},
			},
		},
	}
	recs := buildFindingRecords(snap)
	if len(recs) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Type != "weakAssertion" {
		t.Errorf("Type = %q, want weakAssertion", r.Type)
	}
	if r.File != "a.go" || r.Line != 42 || r.Symbol != "TestFoo" {
		t.Errorf("Location not propagated: file=%q line=%d symbol=%q", r.File, r.Line, r.Symbol)
	}
	if r.Severity != "medium" {
		t.Errorf("Severity = %q, want medium", r.Severity)
	}
	if r.Tier == "" {
		t.Errorf("Tier should be populated (gate or observability)")
	}
	if r.FindingID == "" {
		t.Errorf("FindingID should be propagated")
	}
	if r.Evidence == "" {
		t.Errorf("Evidence should carry the explanation")
	}
}

// TestBuildFindingRecords_EmptySnapshot yields nil so JSON omitempty
// suppresses the field on no-finding reports.
func TestBuildFindingRecords_EmptySnapshot(t *testing.T) {
	recs := buildFindingRecords(&models.TestSuiteSnapshot{})
	if recs != nil {
		t.Errorf("expected nil for empty snapshot, got %v", recs)
	}
}
