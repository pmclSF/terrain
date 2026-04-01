package signals

import (
	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
)

// Detector is the interface for all Terrain signal detectors.
//
// A detector examines the snapshot and emits zero or more Signal values.
// Detectors should be:
//   - stateless
//   - deterministic given the same snapshot
//   - honest about confidence
//   - read-only with respect to the snapshot (MUST NOT mutate snap)
//
// Non-dependent detectors run concurrently. Their results are collected
// and appended to snap.Signals only after all concurrent detectors complete.
type Detector interface {
	// Detect examines the snapshot and returns signals found.
	// Implementations MUST treat snap as read-only.
	Detect(snap *models.TestSuiteSnapshot) []models.Signal
}

// GraphDetector is the interface for detectors that require the dependency graph
// for cross-file structural reasoning.
//
// Graph detectors run in Phase 2 (after flat detectors, before signal-dependent
// detectors). The graph is sealed and immutable, safe for concurrent reads.
type GraphDetector interface {
	// DetectWithGraph examines the snapshot and dependency graph and returns signals.
	// Implementations MUST treat both snap and g as read-only.
	DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal
}
