package remediate

import "github.com/pmclSF/terrain/internal/findings"

// IsRegression reports whether target is a true regression across a change:
// present in head findings, absent in base findings (compared by Key). This
// is the temporal mirror of the closed-loop remediation check — Terrain
// validating its own finding by confirming the change introduced it.
func IsRegression(target findings.Finding, base, head []findings.Finding) bool {
	k := Key(target)
	inBase := false
	for _, f := range base {
		if Key(f) == k {
			inBase = true
		}
	}
	inHead := false
	for _, f := range head {
		if Key(f) == k {
			inHead = true
		}
	}
	return inHead && !inBase
}
