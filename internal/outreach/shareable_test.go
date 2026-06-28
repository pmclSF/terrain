package outreach

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

func validatedRegression() findings.Finding {
	return findings.Finding{
		Version: findings.SchemaVersion, RuleID: "terrain/deps/drift-risk",
		Severity: findings.SeverityError, PrimaryLoc: findings.Location{Path: "package.json"},
		ShortMessage: "drift", DocsURL: "d",
		Suggestions: []findings.Suggestion{{
			Text: "pin", Fix: &findings.Fix{Kind: findings.FixEditInPlace, Path: "package.json"},
		}},
	}
}

// TestShareable_ValidatedRegressionWithValidatedRemediation: a true
// regression (present at head, absent at base) carrying a closed-loop-
// validated remediation is safe to share publicly.
func TestShareable_ValidatedRegressionWithValidatedRemediation(t *testing.T) {
	t.Parallel()
	f := validatedRegression()
	reg := remediate.DefaultValidityRegistry()
	if !Shareable(f, nil, []findings.Finding{f}, reg) {
		t.Error("validated regression + validated remediation must be shareable")
	}
}

// TestShareable_NotARegression: a finding present at both base and head is
// pre-existing debt, not introduced by the change — never shared.
func TestShareable_NotARegression(t *testing.T) {
	t.Parallel()
	f := validatedRegression()
	reg := remediate.DefaultValidityRegistry()
	if Shareable(f, []findings.Finding{f}, []findings.Finding{f}, reg) {
		t.Error("pre-existing finding (present at base) must not be shareable")
	}
}

// TestShareable_UnvalidatedRemediation: a true regression whose remediation
// is not closed-loop validated (judge-only) must not be shared.
func TestShareable_UnvalidatedRemediation(t *testing.T) {
	t.Parallel()
	f := validatedRegression()
	f.Suggestions = []findings.Suggestion{{Text: "pin"}} // no Fix → judge-only
	reg := remediate.DefaultValidityRegistry()
	if Shareable(f, nil, []findings.Finding{f}, reg) {
		t.Error("unvalidated/judge-only remediation must not be shareable")
	}
}
