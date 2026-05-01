package signals

import (
	"fmt"
	"runtime/debug"
	"sort"
	"sync"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
)

// safeDetect wraps a detector call with panic recovery. Pre-0.2.x a
// nil deref or index-out-of-range in any of ~30 detectors would
// terminate the whole pipeline goroutine, taking down `terrain
// analyze` and the calibration test along with the offending fixture.
// With recovery in place, a single broken detector emits zero signals
// for that run instead — the rest of the pipeline continues.
//
// When a panic is caught, we leave a marker in the returned slice
// (Severity=Critical, Type=detectorPanic) so the user sees there was a
// problem and can rerun with --log-level=debug for the stack trace.
func safeDetect(reg DetectorRegistration, fn func() []models.Signal) (out []models.Signal) {
	defer func() {
		if r := recover(); r != nil {
			out = []models.Signal{{
				Type:        "detectorPanic",
				Category:    models.CategoryQuality,
				Severity:    models.SeverityCritical,
				Confidence:  1.0,
				Explanation: fmt.Sprintf("detector %q panicked: %v", reg.Meta.ID, r),
				SuggestedAction: fmt.Sprintf(
					"This is a bug. Re-run with --log-level=debug for the stack trace, then file an issue. Stack: %s",
					string(debug.Stack()),
				),
			}}
		}
	}()
	return fn()
}

// Domain classifies a detector's area of concern.
type Domain string

const (
	DomainQuality    Domain = "quality"
	DomainMigration  Domain = "migration"
	DomainGovernance Domain = "governance"
	DomainHealth     Domain = "health"
	DomainCoverage   Domain = "coverage"
	DomainStructural Domain = "structural"
	// DomainAI is the home for the 0.2 AI-domain detectors (hardcoded
	// API keys, prompt-injection-shaped concatenation, non-deterministic
	// eval configs, etc.). Distinct from DomainStructural because the
	// AI detectors don't need a graph and they read source / config
	// files directly.
	DomainAI Domain = "ai"
)

// EvidenceType describes how a detector obtains its evidence.
type EvidenceType string

const (
	EvidenceStructuralPattern EvidenceType = "structural-pattern"
	EvidencePathName          EvidenceType = "path-name"
	EvidenceRuntime           EvidenceType = "runtime"
	EvidenceCoverage          EvidenceType = "coverage"
	EvidencePolicy            EvidenceType = "policy"
	EvidenceCodeowners        EvidenceType = "codeowners"
	EvidenceGraphTraversal    EvidenceType = "graph-traversal"
)

// DetectorMeta describes a detector's identity and capabilities.
type DetectorMeta struct {
	// ID is a unique, stable identifier for the detector (e.g., "quality.weak-assertion").
	ID string

	// Domain is the detector's area of concern.
	Domain Domain

	// EvidenceType describes how the detector obtains evidence.
	EvidenceType EvidenceType

	// Description is a short human-readable summary.
	Description string

	// SignalTypes lists the signal types this detector may emit.
	SignalTypes []models.SignalType

	// RequiresFileIO indicates the detector reads files from disk beyond the snapshot.
	RequiresFileIO bool

	// DependsOnSignals indicates this detector reads signals from prior detectors.
	DependsOnSignals bool

	// RequiresGraph indicates this detector needs the dependency graph.
	// Graph detectors run in Phase 2 (after flat detectors, before signal-dependent).
	RequiresGraph bool
}

// DetectorRegistration pairs a Detector with its metadata.
type DetectorRegistration struct {
	Meta     DetectorMeta
	Detector Detector
}

// DetectorRegistry holds an ordered set of detector registrations.
//
// Detectors are executed in registration order. Detectors that depend on
// prior signals (DependsOnSignals=true) should be registered after the
// detectors whose signals they read.
type DetectorRegistry struct {
	registrations []DetectorRegistration
}

// NewRegistry creates an empty DetectorRegistry.
func NewRegistry() *DetectorRegistry {
	return &DetectorRegistry{}
}

// Register adds a detector to the registry.
// Returns an error if a non-signal-dependent detector is registered after a
// signal-dependent one, since dependent detectors must run last.
func (r *DetectorRegistry) Register(reg DetectorRegistration) error {
	if !reg.Meta.DependsOnSignals {
		for _, existing := range r.registrations {
			if existing.Meta.DependsOnSignals {
				return fmt.Errorf("signals: cannot register detector %q (DependsOnSignals=false) after dependent detector %q", reg.Meta.ID, existing.Meta.ID)
			}
		}
	}
	r.registrations = append(r.registrations, reg)
	return nil
}

// All returns all registrations in registration order.
func (r *DetectorRegistry) All() []DetectorRegistration {
	out := make([]DetectorRegistration, len(r.registrations))
	copy(out, r.registrations)
	return out
}

// ByDomain returns registrations matching the given domain.
func (r *DetectorRegistry) ByDomain(domain Domain) []DetectorRegistration {
	var out []DetectorRegistration
	for _, reg := range r.registrations {
		if reg.Meta.Domain == domain {
			out = append(out, reg)
		}
	}
	return out
}

// Detectors returns the Detector instances in registration order.
func (r *DetectorRegistry) Detectors() []Detector {
	out := make([]Detector, len(r.registrations))
	for i, reg := range r.registrations {
		out[i] = reg.Detector
	}
	return out
}

// Run executes all registered detectors against the snapshot in
// registration order, appending signals to snap.Signals.
func (r *DetectorRegistry) Run(snap *models.TestSuiteSnapshot) {
	if snap == nil || len(r.registrations) == 0 {
		return
	}

	type result struct {
		idx     int
		signals []models.Signal
	}

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		results    []result
		dependents []DetectorRegistration
	)

	for idx, reg := range r.registrations {
		if reg.Meta.DependsOnSignals {
			dependents = append(dependents, reg)
			continue
		}

		wg.Add(1)
		go func(idx int, reg DetectorRegistration) {
			defer wg.Done()
			found := safeDetect(reg, func() []models.Signal { return reg.Detector.Detect(snap) })
			mu.Lock()
			results = append(results, result{idx: idx, signals: found})
			mu.Unlock()
		}(idx, reg)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})
	for _, res := range results {
		snap.Signals = append(snap.Signals, res.signals...)
	}

	// Dependent detectors run after independent outputs are available.
	for _, reg := range dependents {
		found := safeDetect(reg, func() []models.Signal { return reg.Detector.Detect(snap) })
		snap.Signals = append(snap.Signals, found...)
	}
}

// RunWithGraph executes all registered detectors in three phases:
//
//	Phase 1: Independent flat detectors (concurrent)
//	Phase 2: Graph-powered detectors (concurrent — graph is sealed/immutable)
//	Phase 3: Signal-dependent detectors (sequential)
func (r *DetectorRegistry) RunWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) {
	if snap == nil || len(r.registrations) == 0 {
		return
	}

	type result struct {
		idx     int
		signals []models.Signal
	}

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		results    []result
		graphRegs  []DetectorRegistration
		graphIdxs  []int
		dependents []DetectorRegistration
	)

	// Phase 1: Independent flat detectors (concurrent).
	for idx, reg := range r.registrations {
		if reg.Meta.DependsOnSignals {
			dependents = append(dependents, reg)
			continue
		}
		if reg.Meta.RequiresGraph {
			graphRegs = append(graphRegs, reg)
			graphIdxs = append(graphIdxs, idx)
			continue
		}

		wg.Add(1)
		go func(idx int, reg DetectorRegistration) {
			defer wg.Done()
			found := safeDetect(reg, func() []models.Signal { return reg.Detector.Detect(snap) })
			mu.Lock()
			results = append(results, result{idx: idx, signals: found})
			mu.Unlock()
		}(idx, reg)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].idx < results[j].idx
	})
	for _, res := range results {
		snap.Signals = append(snap.Signals, res.signals...)
	}

	// Phase 2: Graph-powered detectors (concurrent — graph is sealed).
	if g != nil && len(graphRegs) > 0 {
		var graphResults []result
		var wg2 sync.WaitGroup
		for i, reg := range graphRegs {
			gd, ok := reg.Detector.(GraphDetector)
			if !ok {
				continue
			}
			wg2.Add(1)
			go func(idx int, reg DetectorRegistration, gd GraphDetector) {
				defer wg2.Done()
				found := safeDetect(reg, func() []models.Signal { return gd.DetectWithGraph(snap, g) })
				mu.Lock()
				graphResults = append(graphResults, result{idx: idx, signals: found})
				mu.Unlock()
			}(graphIdxs[i], reg, gd)
		}
		wg2.Wait()

		sort.Slice(graphResults, func(i, j int) bool {
			return graphResults[i].idx < graphResults[j].idx
		})
		for _, res := range graphResults {
			snap.Signals = append(snap.Signals, res.signals...)
		}
	}

	// Phase 3: Signal-dependent detectors (sequential).
	for _, reg := range dependents {
		found := safeDetect(reg, func() []models.Signal { return reg.Detector.Detect(snap) })
		snap.Signals = append(snap.Signals, found...)
	}
}

// RunDomain executes only detectors matching the given domain.
func (r *DetectorRegistry) RunDomain(snap *models.TestSuiteSnapshot, domain Domain) {
	for _, reg := range r.registrations {
		if reg.Meta.Domain == domain {
			found := safeDetect(reg, func() []models.Signal { return reg.Detector.Detect(snap) })
			snap.Signals = append(snap.Signals, found...)
		}
	}
}

// Len returns the number of registered detectors.
func (r *DetectorRegistry) Len() int {
	return len(r.registrations)
}
