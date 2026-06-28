package outreach

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestRenderRegressionComment_ShowsValidatedRemediation: each card carries
// the proposed fix, marked as validated — the second axis of the product's
// claim ("here's the fix, and Terrain proved it resolves the finding").
func TestRenderRegressionComment_ShowsValidatedRemediation(t *testing.T) {
	t.Parallel()
	f := validatedRegression() // suggestion text "pin"
	var buf bytes.Buffer
	RenderRegressionComment(&buf, []findings.Finding{f})
	s := buf.String()

	if !strings.Contains(s, "→") {
		t.Errorf("missing remediation line; got:\n%s", s)
	}
	if !strings.Contains(s, "pin") {
		t.Error("remediation line must carry the suggested fix text")
	}
	if !strings.Contains(strings.ToLower(s), "validated") {
		t.Error("remediation must be marked validated (the closed-loop proof)")
	}
}
