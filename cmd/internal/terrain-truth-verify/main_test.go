package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestExtractDocSignalNames_HappyPath(t *testing.T) {
	t.Parallel()
	doc := `# Doc

## Workflows

| ` + "`terrain analyze`" + ` | stable | … |

## Detectors / signal types

### Stable in 0.2

| Signal | Detector | Notes |
|---|---|---|
| ` + "`weakAssertion`" + ` | … | … |
| ` + "`untestedExport`" + ` | … | … |
| ` + "`aiHardcodedAPIKey`" + ` | … | … |

### Planned (referenced in docs but not yet implemented)

| Signal | Earliest |
|---|---|
| ` + "`xfailAccumulation`" + ` (age-based) | 0.3 |
`
	got := extractDocSignalNames(doc)
	want := map[string]bool{
		"weakAssertion":    true,
		"untestedExport":   true,
		"aiHardcodedAPIKey": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (planned subsection should be excluded; CLI verbs above the anchor too)",
			sortedKeys(got), sortedKeys(want))
	}
}

func TestExtractDocSignalNames_NoAnchor(t *testing.T) {
	t.Parallel()
	doc := "# Random doc\n\nNo detector section here.\n"
	got := extractDocSignalNames(doc)
	if len(got) != 0 {
		t.Errorf("doc with no anchor should produce no names, got %v", sortedKeys(got))
	}
}

func TestExtractDocSignalNames_ExcludesPlannedSubsection(t *testing.T) {
	t.Parallel()
	doc := `## Detectors / signal types

### Stable in 0.2

| ` + "`realSignal`" + ` | … | … |

### Planned (referenced in docs but not yet implemented)

| ` + "`futureSignal`" + ` | 0.3 |
`
	got := extractDocSignalNames(doc)
	if !got["realSignal"] {
		t.Error("realSignal should be extracted")
	}
	if got["futureSignal"] {
		t.Error("futureSignal in planned subsection should NOT be extracted")
	}
}

func TestExtractDocSignalNames_ExcludesAllLowercaseTokens(t *testing.T) {
	t.Parallel()
	// `report`, `eval`, `policy` are CLI verbs / English words that
	// appear in code spans throughout the doc but aren't signal types.
	// The camelCase pattern should reject them.
	doc := `## Detectors / signal types

| ` + "`report`" + ` | … |
| ` + "`eval`" + ` | … |
| ` + "`policy`" + ` | … |
| ` + "`weakAssertion`" + ` | … |
`
	got := extractDocSignalNames(doc)
	if got["report"] || got["eval"] || got["policy"] {
		t.Errorf("all-lowercase tokens should be rejected; got %v", sortedKeys(got))
	}
	if !got["weakAssertion"] {
		t.Error("camelCase token should be extracted")
	}
}

func TestIsEngineDiagnostic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		typ  string
		want bool
	}{
		{"detectorPanic", true},
		{"detectorBudgetExceeded", true},
		{"detectorMissingInput", true},
		{"suppressionExpired", true},
		{"weakAssertion", false},
		{"aiHardcodedAPIKey", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isEngineDiagnostic(models.SignalType(tt.typ))
		if got != tt.want {
			t.Errorf("isEngineDiagnostic(%q) = %v, want %v", tt.typ, got, tt.want)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
