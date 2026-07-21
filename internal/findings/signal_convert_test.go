package findings

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestFromSignal_CarriesSuggestedAction pins the regression that motivated
// 0.4.0's remediation axis: the detector's SuggestedAction used to be
// dropped in conversion, so the canonical finding shipped no remediation.
// It must now surface as a text Suggestion anchored at the finding location.
func TestFromSignal_CarriesSuggestedAction(t *testing.T) {
	t.Parallel()
	s := models.Signal{
		Type:            "untestedExport",
		Severity:        models.SeverityMedium,
		Location:        models.SignalLocation{File: "pkg/foo.go", Line: 42},
		Explanation:     "exported Foo has no direct test",
		SuggestedAction: "Add a direct test exercising Foo in pkg/foo_test.go",
	}
	f := FromSignal(s, "terrain/quality/untested-export")

	if len(f.Suggestions) != 1 {
		t.Fatalf("Suggestions len = %d, want 1", len(f.Suggestions))
	}
	got := f.Suggestions[0]
	if got.Text != s.SuggestedAction {
		t.Errorf("Text = %q, want %q", got.Text, s.SuggestedAction)
	}
	if got.AppliesTo == nil || got.AppliesTo.Path != "pkg/foo.go" || got.AppliesTo.Line != 42 {
		t.Errorf("AppliesTo = %+v, want pkg/foo.go:42", got.AppliesTo)
	}
	// Text-only suggestions are the judge-only floor; no structured Fix yet.
	if got.Fix != nil {
		t.Errorf("Fix = %+v, want nil for a text-only suggestion", got.Fix)
	}
}

// TestFromSignal_NoActionNoSuggestion keeps the artifact clean: a signal
// without a SuggestedAction must not synthesize an empty suggestion.
func TestFromSignal_NoActionNoSuggestion(t *testing.T) {
	t.Parallel()
	s := models.Signal{
		Type:     "untestedExport",
		Severity: models.SeverityLow,
		Location: models.SignalLocation{File: "pkg/foo.go", Line: 7},
	}
	f := FromSignal(s, "terrain/quality/untested-export")
	if f.Suggestions != nil {
		t.Errorf("Suggestions = %+v, want nil", f.Suggestions)
	}
}
