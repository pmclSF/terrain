package remediate

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestKey_DistinguishesSameRuleSameLocation: the schema→prompt drift detector
// emits one finding per (template, schema field) at line 0, so rule+path+line
// alone collides. Key folds in the short message so distinct findings stay
// distinct — otherwise a valid remediation reads as "did not clear" and a
// genuinely new finding is dropped from regression selection.
func TestKey_DistinguishesSameRuleSameLocation(t *testing.T) {
	t.Parallel()
	a := findings.Finding{
		RuleID:       "terrain/ai/prompt-schema-drift",
		PrimaryLoc:   findings.Location{Path: "prompts/welcome.md"},
		ShortMessage: "references removed field user_id",
	}
	b := a
	b.ShortMessage = "references removed field account_id"
	if Key(a) == Key(b) {
		t.Fatal("two distinct line-0 findings of the same rule must not share a Key")
	}
}

// TestIsRegression_NewLine0FindingNotMaskedByBase: a genuinely new line-0
// finding must be reported as a regression even when a different same-rule,
// same-file finding was already present at base.
func TestIsRegression_NewLine0FindingNotMaskedByBase(t *testing.T) {
	t.Parallel()
	base := findings.Finding{
		RuleID:       "terrain/ai/prompt-schema-drift",
		PrimaryLoc:   findings.Location{Path: "p.md"},
		ShortMessage: "field A removed",
	}
	fresh := base
	fresh.ShortMessage = "field B removed"
	if !IsRegression(fresh, []findings.Finding{base}, []findings.Finding{base, fresh}) {
		t.Fatal("a new line-0 finding must be a regression despite a same-location base finding")
	}
}
