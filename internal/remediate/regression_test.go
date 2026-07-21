package remediate

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestIsRegression_PresentAtHeadAbsentAtBase: a finding that Terrain reports
// at head but not at base was introduced by the change — a true regression.
func TestIsRegression_PresentAtHeadAbsentAtBase(t *testing.T) {
	t.Parallel()
	target := ruleFinding("terrain/deps/drift-risk", "package.json", "")
	if !IsRegression(target, nil, []findings.Finding{target}) {
		t.Error("present at head and absent at base must be a regression")
	}
}
