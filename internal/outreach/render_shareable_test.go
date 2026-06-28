package outreach

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

// TestRenderShareableComment_OmitsNonShareable: the public entry point must
// never render a finding that isn't a validated regression with a validated
// remediation — the safety gate is enforced at the render boundary, so no
// caller can accidentally post a false positive to a stranger's repo.
func TestRenderShareableComment_OmitsNonShareable(t *testing.T) {
	t.Parallel()
	shareable := validatedRegression() // package.json, validated fix

	judgeOnly := validatedRegression()
	judgeOnly.PrimaryLoc.Path = "x/package.json"
	judgeOnly.ShortMessage = "judge only drift"
	judgeOnly.Suggestions = []findings.Suggestion{{Text: "pin"}} // no Fix

	var buf bytes.Buffer
	RenderShareableComment(&buf, nil, []findings.Finding{shareable, judgeOnly}, remediate.DefaultValidityRegistry())
	s := buf.String()

	if !strings.Contains(s, "`package.json`") {
		t.Error("the shareable regression must appear")
	}
	if strings.Contains(s, "judge only drift") {
		t.Error("a non-shareable finding must NOT appear in a public comment")
	}
	if !strings.Contains(s, "found 1 regression") {
		t.Errorf("verdict must count only shareable findings; got:\n%s", s)
	}
}
