package outreach

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

// TestRenderRegressionComment_RendersCardPerRegression: each shared
// regression renders as its own card carrying the file path and the
// finding's message — the same card grammar the PR comment uses.
func TestRenderRegressionComment_RendersCardPerRegression(t *testing.T) {
	t.Parallel()
	f1 := validatedRegression() // package.json / "drift"
	f2 := validatedRegression()
	f2.PrimaryLoc.Path = "web/package.json"
	f2.ShortMessage = "web manifest drift"

	var buf bytes.Buffer
	RenderRegressionComment(&buf, []findings.Finding{f1, f2})
	s := buf.String()

	for _, want := range []string{"`package.json`", "drift", "`web/package.json`", "web manifest drift"} {
		if !strings.Contains(s, want) {
			t.Errorf("comment missing %q; got:\n%s", want, s)
		}
	}
}
