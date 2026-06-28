package outreach

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

func firstNonEmptyLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// TestRenderRegressionComment_LeadsWithVerdict: the first line is a bold,
// count-bearing verdict — the Swiss principle that the lead carries the
// message even when a notification email flattens everything below it.
func TestRenderRegressionComment_LeadsWithVerdict(t *testing.T) {
	t.Parallel()
	cases := map[int]string{
		1: "**Terrain found 1 regression in this repo.**",
		3: "**Terrain found 3 regressions in this repo.**",
	}
	for n, want := range cases {
		regs := make([]findings.Finding, n)
		for i := range regs {
			regs[i] = validatedRegression()
		}
		var buf bytes.Buffer
		RenderRegressionComment(&buf, regs)
		if got := firstNonEmptyLine(buf.String()); got != want {
			t.Errorf("n=%d: first line = %q, want %q", n, got, want)
		}
	}
}
