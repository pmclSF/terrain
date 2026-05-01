package severity

import (
	"regexp"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

var clauseIDFormat = regexp.MustCompile(`^sev-(critical|high|medium|low|info)-[0-9]{3}$`)

// TestRubric_ClauseIDFormat ensures every clause ID matches the canonical
// pattern. Detectors and consumers rely on the format to parse the
// severity out of an ID without a lookup.
func TestRubric_ClauseIDFormat(t *testing.T) {
	t.Parallel()

	for _, c := range All() {
		if !clauseIDFormat.MatchString(c.ID) {
			t.Errorf("clause ID %q does not match %s", c.ID, clauseIDFormat)
		}
	}
}

// TestRubric_IDsUnique guards against accidentally reusing an ID after a
// copy-paste. IDs are part of the public contract (cited in Signal
// payloads) so collisions are a release blocker.
func TestRubric_IDsUnique(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, c := range All() {
		if seen[c.ID] {
			t.Errorf("duplicate clause ID %q", c.ID)
		}
		seen[c.ID] = true
	}
}

// TestRubric_IDPrefixMatchesSeverity confirms that a clause's parsed
// severity (from its ID) agrees with its declared Severity. Detectors
// shouldn't have to look up a clause to know what severity it justifies.
func TestRubric_IDPrefixMatchesSeverity(t *testing.T) {
	t.Parallel()

	for _, c := range All() {
		parts := strings.Split(c.ID, "-")
		if len(parts) < 3 {
			continue
		}
		gotSev := models.SignalSeverity(parts[1])
		if gotSev != c.Severity {
			t.Errorf("clause %q has Severity=%q but ID encodes %q",
				c.ID, c.Severity, gotSev)
		}
	}
}

// TestRubric_EveryClauseHasDescription is a presentation-quality check:
// the rubric is the user-facing source of truth. Empty descriptions
// would render as blank rows in the generated doc.
func TestRubric_EveryClauseHasDescription(t *testing.T) {
	t.Parallel()

	for _, c := range All() {
		if strings.TrimSpace(c.Description) == "" {
			t.Errorf("clause %q has empty Description", c.ID)
		}
		if strings.TrimSpace(c.Title) == "" {
			t.Errorf("clause %q has empty Title", c.ID)
		}
	}
}

// TestRubric_AtLeastOneClausePerSeverity protects against a future PR
// emptying out a severity tier by accident.
func TestRubric_AtLeastOneClausePerSeverity(t *testing.T) {
	t.Parallel()

	for _, sev := range SeverityOrder() {
		if len(BySeverity(sev)) == 0 {
			t.Errorf("severity %q has zero clauses", sev)
		}
	}
}

// TestRubric_ValidateClauseIDs spot-checks the helper used by detectors.
func TestRubric_ValidateClauseIDs(t *testing.T) {
	t.Parallel()

	missing := ValidateClauseIDs([]string{"sev-critical-001", "sev-bogus-999"})
	if len(missing) != 1 || missing[0] != "sev-bogus-999" {
		t.Errorf("expected [sev-bogus-999], got %v", missing)
	}
}
