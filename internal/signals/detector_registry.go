package signals

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
)

// DefaultDetectorBudget is the per-detector wall-clock ceiling
// applied when DetectorMeta.Budget is zero. 30 seconds is generous
// enough that production-shaped repos clear it on the slowest
// graph-traversal detectors; it primarily catches accidental
// infinite loops or quadratic-or-worse code paths that would
// otherwise hang the whole pipeline.
//
// Override in DetectorMeta.Budget for detectors that legitimately
// need longer (large runtime artifact ingestion, etc.).
const DefaultDetectorBudget = 30 * time.Second

// signalTypeDetectorBudgetExceeded is the local alias for
// SignalDetectorBudgetExceeded (signal_types.go). The marker is
// treated as a quality-domain finding so it surfaces in the
// analyze report alongside the detector-panic marker. Keeping a
// local alias makes the safeDetectWithBudget callsite self-
// contained and protects against the manifest entry being renamed
// under it.
const signalTypeDetectorBudgetExceeded = SignalDetectorBudgetExceeded

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

// safeDetectWithBudget wraps safeDetect with a per-detector wall-
// clock timeout. Track 9.4 — the budget protects the pipeline from
// any single hung detector blocking the rest.
//
// Note: a detector that ignores ctx and runs a tight CPU loop will
// still complete its work after the budget elapses (Go has no
// goroutine kill primitive). The budget here means "stop waiting
// for this result and move on" — the detector's signals from a
// post-budget completion are dropped, the marker stands. This is
// the right trade-off for the failure modes the budget targets:
// runaway regex, accidentally-O(n²) graph walks, blocking I/O on
// a slow filesystem.
func safeDetectWithBudget(reg DetectorRegistration, fn func() []models.Signal) []models.Signal {
	budget := reg.Meta.Budget
	if budget <= 0 {
		budget = DefaultDetectorBudget
	}

	type result struct {
		signals []models.Signal
	}
	done := make(chan result, 1)
	go func() {
		done <- result{signals: safeDetect(reg, fn)}
	}()

	select {
	case r := <-done:
		return r.signals
	case <-time.After(budget):
		return []models.Signal{{
			Type:       signalTypeDetectorBudgetExceeded,
			Category:   models.CategoryQuality,
			Severity:   models.SeverityCritical,
			Confidence: 1.0,
			Explanation: fmt.Sprintf(
				"detector %q exceeded its %s budget and was abandoned by the pipeline",
				reg.Meta.ID, budget),
			SuggestedAction: "If this detector is legitimately slow on your repo, raise its budget in DetectorMeta.Budget. If it should be fast, the runaway suggests a quadratic-or-worse code path or a hung I/O — re-run with --log-level=debug.",
		}}
	}
}

// signalTypeMissingInputDiagnostic is the marker emitted by the
// registry when a detector's RequiresRuntime / RequiresBaseline /
// RequiresEvalArtifact flag is set but the snapshot doesn't carry
// the corresponding input. Track 9.3 — adopters running `terrain
// analyze` without coverage / baseline / eval artifacts get a
// single visible diagnostic per affected detector instead of
// silent zero-output.
const signalTypeMissingInputDiagnostic = SignalDetectorMissingInput

// missingInputs returns a list of human-readable input-name strings
// that the detector's metadata says it needs but the snapshot
// doesn't provide. Empty list means the detector can run; non-empty
// means the registry should emit a missingInputDiagnostic and skip
// invocation. Each input name corresponds to a CLI flag the user
// would set to provide the input.
func missingInputs(meta DetectorMeta, snap *models.TestSuiteSnapshot) []string {
	if snap == nil {
		return nil
	}
	var missing []string
	if meta.RequiresRuntime && !snapshotHasRuntime(snap) {
		missing = append(missing, "runtime artifacts (--runtime path/to/junit.xml or jest.json)")
	}
	if meta.RequiresBaseline && snap.Baseline == nil {
		missing = append(missing, "baseline snapshot (--baseline path/to/old-snapshot.json)")
	}
	if meta.RequiresEvalArtifact && len(snap.EvalRuns) == 0 {
		missing = append(missing, "eval-framework artifact (--promptfoo-results / --deepeval-results / --ragas-results)")
	}
	return missing
}

// snapshotHasRuntime reports whether the snapshot carries any
// runtime test result data. We look at the test-file inventory
// rather than walking every signal — the runtime stats live on the
// TestFile, not on signals.
func snapshotHasRuntime(snap *models.TestSuiteSnapshot) bool {
	for i := range snap.TestFiles {
		if snap.TestFiles[i].RuntimeStats != nil {
			return true
		}
	}
	return false
}

// missingInputDiagnostic builds the marker signal emitted when one
// or more required inputs are absent. The explanation lists every
// missing input so adopters can fix them all in one re-run rather
// than playing whack-a-mole.
func missingInputDiagnostic(meta DetectorMeta, missing []string) models.Signal {
	return models.Signal{
		Type:       signalTypeMissingInputDiagnostic,
		Category:   models.CategoryQuality,
		Severity:   models.SeverityLow,
		Confidence: 1.0,
		Explanation: fmt.Sprintf(
			"detector %q requires inputs the current snapshot doesn't carry: %s",
			meta.ID, joinInputNames(missing)),
		SuggestedAction: "Re-run `terrain analyze` with the listed flags to enable this detector. If you don't need its signals, leave the inputs absent — this diagnostic surfaces the gap without blocking the rest of the pipeline.",
	}
}

// safeDetectChecked is the registry's canonical detector-invocation
// path. It composes Track 9.3 (missing-input check) with Track 9.4
// (per-detector budget) over Track 9.2's panic recovery: input
// gates first (skip detectors that can't fire), then budget-bounded
// invocation that delegates to safeDetect for panic handling.
// All call sites in Run / RunWithGraph route through here.
func safeDetectChecked(reg DetectorRegistration, snap *models.TestSuiteSnapshot, fn func() []models.Signal) []models.Signal {
	if missing := missingInputs(reg.Meta, snap); len(missing) > 0 {
		return []models.Signal{missingInputDiagnostic(reg.Meta, missing)}
	}
	return safeDetectWithBudget(reg, fn)
}

func joinInputNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		// Oxford comma, plain English: "a, b, c, and d".
		// Join all but the last with ", " then append ", and <last>".
		head := names[:len(names)-1]
		return strings.Join(head, ", ") + ", and " + names[len(names)-1]
	}
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

	// Budget is the maximum wall-clock time this detector is allowed
	// to run before the pipeline cancels it and treats it as a no-op
	// for the run. Zero means "use the registry default" (see
	// DefaultDetectorBudget). Track 9.4 — protects analyze runs from
	// a single hung detector blocking the whole pipeline.
	//
	// When the budget elapses, safeDetectWithBudget emits a
	// SignalDetectorBudgetExceeded marker so the user sees the
	// detector name + budget that was hit, rather than silent
	// truncation.
	//
	// Detectors that legitimately need longer (large-graph traversal,
	// runtime artifact ingestion) should set this explicitly. The
	// default is generous enough that production-shaped repos clear
	// it; setting a tighter budget on simple structural detectors
	// catches accidental quadratic-or-worse code paths.
	Budget time.Duration

	// --- Track 9.1 capability metadata ---
	//
	// The fields below describe what a detector consumes beyond the
	// in-memory snapshot. They're descriptive (so docs / `terrain
	// doctor` can surface "this detector needs runtime data") AND
	// load-bearing (Track 9.3 — when a required input is missing
	// the registry emits a single per-detector missingInputDiagnostic
	// instead of silently running a detector that can't fire).
	//
	// All zero values mean "don't require this input", which keeps
	// the existing detector roster behaving exactly as before. New
	// detectors that genuinely need runtime / baseline / eval data
	// should set the relevant flag so the diagnostic surfaces when
	// inputs are absent.

	// RequiresRuntime indicates the detector reads RuntimeStats from
	// the snapshot (populated by JUnit XML / Jest JSON / Go test
	// JSON ingestion). Without runtime artifacts the snapshot's
	// runtime fields are empty and the detector cannot fire.
	RequiresRuntime bool

	// RequiresBaseline indicates the detector compares the current
	// snapshot against a baseline snapshot (passed via
	// `terrain analyze --baseline`). Without it, the regression
	// detectors (aiCostRegression, aiHallucinationRate,
	// aiRetrievalRegression) have no point of comparison.
	RequiresBaseline bool

	// RequiresEvalArtifact indicates the detector reads EvalRuns
	// from the snapshot (populated by Promptfoo / DeepEval / Ragas
	// adapter ingestion). Without an artifact path passed via the
	// `--{promptfoo,deepeval,ragas}-results` flags, the snapshot's
	// EvalRuns is empty and these detectors can't fire.
	RequiresEvalArtifact bool

	// ContextAware reports whether the detector honors ctx.Err() in
	// its inner loops. Detectors that don't are still safe — they
	// run inside safeDetectWithBudget and get abandoned at the
	// budget cap — but ctx-aware detectors can react faster to
	// pipeline cancellation. Surfaced in `terrain doctor` so
	// reviewers can see the cancellation posture per-detector.
	ContextAware bool

	// Experimental marks the detector as not-yet-stable. Distinct
	// from the manifest's Status field on individual signals: a
	// stable signal type can still have an experimental detector
	// (the type is locked, the detector implementation is not).
	// Experimental detectors are excluded from the recommended
	// `--fail-on critical` gate per the trust-tier framing.
	Experimental bool
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

	// Pre-allocate results to len(r.registrations) so the per-goroutine
	// append doesn't trigger repeated copy-grow under the mutex. Cheap
	// micro-optimization, but useful at scale: with ~30 detectors the
	// pre-fix slice grew through 0/1/2/4/8/16/32 reallocations.
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		results    = make([]result, 0, len(r.registrations))
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
			found := safeDetectChecked(reg, snap, func() []models.Signal { return reg.Detector.Detect(snap) })
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
		found := safeDetectChecked(reg, snap, func() []models.Signal { return reg.Detector.Detect(snap) })
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
			found := safeDetectChecked(reg, snap, func() []models.Signal { return reg.Detector.Detect(snap) })
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
		graphResults := make([]result, 0, len(graphRegs))
		var wg2 sync.WaitGroup
		for i, reg := range graphRegs {
			gd, ok := reg.Detector.(GraphDetector)
			if !ok {
				// 0.2.0 final-polish: pre-fix this branch silently
				// dropped the registration with no signal, no log, no
				// diagnostic — a detector declared `RequiresGraph: true`
				// but whose runtime type didn't satisfy the GraphDetector
				// interface vanished from the pipeline entirely. Now we
				// emit a detectorPanic-shaped diagnostic so the user
				// sees something is wrong instead of getting a quietly
				// half-empty snapshot.
				snap.Signals = append(snap.Signals, models.Signal{
					Type:        "detectorPanic",
					Category:    models.CategoryQuality,
					Severity:    models.SeverityCritical,
					Confidence:  1.0,
					Explanation: fmt.Sprintf("detector %q declared RequiresGraph=true but does not implement GraphDetector — registration silently skipped pre-0.2.x; surfaced now as a configuration bug.", reg.Meta.ID),
					SuggestedAction: "Verify that the detector's concrete type implements DetectWithGraph(*TestSuiteSnapshot, *Graph), or set RequiresGraph=false in the registration.",
				})
				continue
			}
			wg2.Add(1)
			go func(idx int, reg DetectorRegistration, gd GraphDetector) {
				defer wg2.Done()
				found := safeDetectChecked(reg, snap, func() []models.Signal { return gd.DetectWithGraph(snap, g) })
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
		found := safeDetectChecked(reg, snap, func() []models.Signal { return reg.Detector.Detect(snap) })
		snap.Signals = append(snap.Signals, found...)
	}
}

// RunDomain executes only detectors matching the given domain.
func (r *DetectorRegistry) RunDomain(snap *models.TestSuiteSnapshot, domain Domain) {
	for _, reg := range r.registrations {
		if reg.Meta.Domain == domain {
			found := safeDetectChecked(reg, snap, func() []models.Signal { return reg.Detector.Detect(snap) })
			snap.Signals = append(snap.Signals, found...)
		}
	}
}

// Len returns the number of registered detectors.
func (r *DetectorRegistry) Len() int {
	return len(r.registrations)
}
