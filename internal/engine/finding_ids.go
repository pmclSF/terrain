package engine

import (
	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/models"
)

// assignFindingIDs walks every signal in the snapshot (both top-level
// `snapshot.Signals` and per-test-file `TestFile.Signals`) and populates
// the stable `FindingID` field for any signal that doesn't already have
// one, plus the `Pillar` derived from Category. Detectors that need a
// non-default ID (e.g. signals attached to virtual locations like a
// manifest entry) can pre-set FindingID and this pass leaves them alone.
//
// Idempotent — calling twice produces the same result.
//
// Called from RunPipelineContext after SortSnapshot so the IDs land in
// canonical order. Order matters: the assignment uses Type +
// Location.{File,Symbol,Line} as the inputs, so signals that are
// indistinguishable on those four fields get the same ID by design
// (deduplication during snapshot construction is upstream's job).
func assignFindingIDs(snapshot *models.TestSuiteSnapshot) {
	if snapshot == nil {
		return
	}
	for i := range snapshot.Signals {
		finalizeSignal(&snapshot.Signals[i])
	}
	for fi := range snapshot.TestFiles {
		tf := &snapshot.TestFiles[fi]
		for si := range tf.Signals {
			finalizeSignal(&tf.Signals[si])
		}
	}
}

func finalizeSignal(s *models.Signal) {
	if s.FindingID == "" {
		s.FindingID = identity.BuildFindingID(
			string(s.Type),
			s.Location.File,
			s.Location.Symbol,
			s.Location.Line,
		)
	}
	if s.Pillar == "" {
		s.Pillar = models.PillarFor(s.Category)
	}
}
