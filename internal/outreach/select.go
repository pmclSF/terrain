package outreach

import (
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

// SelectShareable filters a head scan down to the findings that are safe to
// share: validated regressions (new at head) with closed-loop-validated
// remediations. base is the scan at the change's base ref. This is the
// funnel every outreach comment passes through.
func SelectShareable(base, head []findings.Finding, reg *remediate.ValidityRegistry) []findings.Finding {
	var out []findings.Finding
	for _, f := range head {
		if Shareable(f, base, head, reg) {
			out = append(out, f)
		}
	}
	return out
}
