package outreach

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/render"
)

// TestRenderRegressionComment_CarriesProvenanceFooter: the growth-engine
// outreach comment must wear the design-system provenance footer, the same
// key-free signature every Terrain surface carries — so it reads as one
// consistent, clean product, not a parallel style.
func TestRenderRegressionComment_CarriesProvenanceFooter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	RenderRegressionComment(&buf, nil)
	if !strings.Contains(buf.String(), render.ProvenanceFooter("")) {
		t.Errorf("outreach comment must carry the design-system provenance footer; got:\n%s", buf.String())
	}
}
