package outreach

import (
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

// Shareable is the public-sharing safety gate: a finding may appear in an
// outreach comment ONLY when it is a validated regression (present at head,
// absent at base) AND its remediation is closed-loop validated. Posting a
// false positive — or a fix Terrain can't prove — to a stranger's repo is
// the failure mode that kills the growth motion, so both axes must hold.
func Shareable(f findings.Finding, base, head []findings.Finding, reg *remediate.ValidityRegistry) bool {
	return remediate.IsRegression(f, base, head) && remediate.GateEligible(f, reg)
}
