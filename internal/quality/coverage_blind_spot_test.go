package quality

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestCoverageBlindSpotDetector_FromCoverageInsights(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		CoverageInsights: []models.CoverageInsight{
			{
				Type:            "only_e2e_coverage",
				Severity:        "medium",
				Description:     "3 code unit(s) are covered only by e2e tests.",
				SuggestedAction: "Add unit tests.",
			},
			{
				Type:            "only_e2e_unit",
				Severity:        "low",
				Description:     "AuthService is covered only by e2e tests.",
				Path:            "src/auth.js",
				UnitID:          "src/auth.js:AuthService",
				SuggestedAction: "Add unit tests for AuthService.",
			},
			{
				Type:            "partially_covered",
				Severity:        "low",
				Description:     "Ignored by blind-spot detector.",
				SuggestedAction: "n/a",
			},
			{
				Type:            "uncovered_exported",
				Severity:        "high",
				Description:     "1 exported/public function has no coverage.",
				SuggestedAction: "Prioritize public API tests.",
			},
		},
	}

	d := &CoverageBlindSpotDetector{}
	signals := d.Detect(snap)
	if len(signals) != 3 {
		t.Fatalf("expected 3 coverageBlindSpot signals, got %d", len(signals))
	}

	var repoLevel, fileLevel, high bool
	for _, s := range signals {
		if s.Type != "coverageBlindSpot" {
			t.Fatalf("unexpected signal type %q", s.Type)
		}
		if s.Location.File == "" && s.Location.Repository == "coverage" {
			repoLevel = true
		}
		if s.Location.File == "src/auth.js" {
			fileLevel = true
		}
		if s.Severity == models.SeverityHigh {
			high = true
		}
	}

	if !repoLevel {
		t.Fatal("expected at least one repository-level blind-spot signal")
	}
	if !fileLevel {
		t.Fatal("expected file-level blind-spot signal for src/auth.js")
	}
	if !high {
		t.Fatal("expected high-severity blind-spot signal for uncovered exported insight")
	}
}

// CB1: nil snapshot, nil slice, and empty slice all produce no signals (no panic).
func TestCoverageBlindSpotDetector_EmptyInput(t *testing.T) {
	t.Parallel()
	d := &CoverageBlindSpotDetector{}
	cases := map[string]*models.TestSuiteSnapshot{
		"nil snapshot":   nil,
		"nil insights":   {CoverageInsights: nil},
		"empty insights": {CoverageInsights: []models.CoverageInsight{}},
	}
	for name, snap := range cases {
		if got := d.Detect(snap); len(got) != 0 {
			t.Errorf("%s: want 0 signals, got %d", name, len(got))
		}
	}
}

// CB3: identical (type,path,unitId,description) collapse to one; a differing
// description is kept.
func TestCoverageBlindSpotDetector_Deduplicates(t *testing.T) {
	t.Parallel()
	ci := models.CoverageInsight{Type: "only_e2e_coverage", Severity: "medium", Path: "src/foo.js", UnitID: "src/foo.js:Foo", Description: "covered only by e2e", SuggestedAction: "add unit tests"}
	dup := ci
	diff := ci
	diff.Description = "a different description"
	got := (&CoverageBlindSpotDetector{}).Detect(&models.TestSuiteSnapshot{
		CoverageInsights: []models.CoverageInsight{ci, dup, diff},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 signals after dedup, got %d", len(got))
	}
}

// CB5: an unknown severity string maps to Info (not dropped, not a zero value).
func TestCoverageBlindSpotDetector_UnknownSeverityDefaultsInfo(t *testing.T) {
	t.Parallel()
	got := (&CoverageBlindSpotDetector{}).Detect(&models.TestSuiteSnapshot{
		CoverageInsights: []models.CoverageInsight{
			{Type: "only_e2e_coverage", Severity: "not-a-real-severity", Description: "x"},
		},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 signal, got %d", len(got))
	}
	if got[0].Severity != models.SeverityInfo {
		t.Errorf("unknown severity must map to Info, got %v", got[0].Severity)
	}
}

// CB6: the hardcoded signal fields and conditional metadata the original test
// never asserts (a regression in any of these is otherwise invisible).
func TestCoverageBlindSpotDetector_SignalShape(t *testing.T) {
	t.Parallel()
	got := (&CoverageBlindSpotDetector{}).Detect(&models.TestSuiteSnapshot{
		CoverageInsights: []models.CoverageInsight{
			{Type: "uncovered_exported", Severity: "critical", Path: "internal/auth/h.go", UnitID: "H", Description: "H uncovered", SuggestedAction: "test H"},
			{Type: "only_e2e_coverage", Severity: "medium", Path: "src/u.js", Description: "u e2e-only"}, // no UnitID
		},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 signals, got %d", len(got))
	}
	s := got[0]
	if s.Type != "coverageBlindSpot" || s.Category != models.CategoryQuality {
		t.Errorf("type/category: %q / %v", s.Type, s.Category)
	}
	if s.Severity != models.SeverityCritical {
		t.Errorf("critical insight → SeverityCritical, got %v", s.Severity)
	}
	if s.Confidence != 0.9 {
		t.Errorf("confidence: want 0.9, got %v", s.Confidence)
	}
	if s.EvidenceStrength != models.EvidenceStrong || s.EvidenceSource != models.SourceCoverage {
		t.Errorf("evidence: %v / %v", s.EvidenceStrength, s.EvidenceSource)
	}
	if s.Explanation != "H uncovered" || s.SuggestedAction != "test H" {
		t.Errorf("explanation/action must be copied 1:1: %q / %q", s.Explanation, s.SuggestedAction)
	}
	if s.Metadata["insightType"] != "uncovered_exported" || s.Metadata["unitId"] != "H" {
		t.Errorf("metadata: %+v", s.Metadata)
	}
	// unitId is omitted when the insight has no UnitID.
	if _, present := got[1].Metadata["unitId"]; present {
		t.Errorf("unitId must be omitted when empty; metadata=%+v", got[1].Metadata)
	}
}
