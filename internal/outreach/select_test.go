package outreach

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

// TestSelectShareable_KeepsOnlyValidatedRegressions: from a head scan, the
// selector returns only findings that are both new (regressions) and carry a
// validated remediation — the rest are filtered out before any comment.
func TestSelectShareable_KeepsOnlyValidatedRegressions(t *testing.T) {
	t.Parallel()
	shareable := validatedRegression() // new + validated fix

	preExisting := validatedRegression()
	preExisting.PrimaryLoc.Path = "old/package.json" // present at base too

	judgeOnly := validatedRegression()
	judgeOnly.PrimaryLoc.Path = "other/package.json"
	judgeOnly.Suggestions = []findings.Suggestion{{Text: "pin"}} // no Fix

	base := []findings.Finding{preExisting}
	head := []findings.Finding{shareable, preExisting, judgeOnly}

	got := SelectShareable(base, head, remediate.DefaultValidityRegistry())
	if len(got) != 1 {
		t.Fatalf("SelectShareable returned %d, want 1 (only the validated regression)", len(got))
	}
	if got[0].PrimaryLoc.Path != "package.json" {
		t.Errorf("kept the wrong finding: %s", got[0].PrimaryLoc.Path)
	}
}
