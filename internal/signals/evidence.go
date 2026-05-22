package signals

import (
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/pmclSF/terrain/internal/models"
)

// Evidence captures the corpus-measured evidence for one detector.
// Mirrors the structure of `internal/explain/data/detector-evidence.json`
// but with only the fields severity-from-lift actually consumes.
type Evidence struct {
	HandPrecision *EvidenceCI `json:"heuristic_precision,omitempty"`
	HandValidated *HandValidated `json:"hand_validated,omitempty"`
	GlobalLift    *EvidenceCI    `json:"global_lift,omitempty"`
}

// EvidenceCI is a point estimate with a 95% CI.
type EvidenceCI struct {
	Point  float64 `json:"point,omitempty"`
	Lift   float64 `json:"lift,omitempty"`
	Low95  float64 `json:"low_95,omitempty"`
	High95 float64 `json:"high_95,omitempty"`
	Sample int     `json:"sample_size,omitempty"`
}

// HandValidated is the result of a sampled review against a labeled
// subset.
type HandValidated struct {
	TruePositives  int     `json:"tp,omitempty"`
	FalsePositives int     `json:"fp,omitempty"`
	Unknown        int     `json:"unknown,omitempty"`
	PointPrecision float64 `json:"point_precision,omitempty"`
}

// LiftPoint returns the lift value, preferring the explicit Lift field
// over the Point field (different JSON shapes use one or the other).
func (e EvidenceCI) LiftPoint() float64 {
	if e.Lift != 0 {
		return e.Lift
	}
	return e.Point
}

//go:embed evidence_data.json
var embeddedEvidenceJSON []byte

var (
	evidenceOnce sync.Once
	evidenceMap  map[string]Evidence
)

// loadEvidence parses the embedded evidence JSON once. The JSON file is a
// thin alias for `internal/explain/data/detector-evidence.json`; we keep
// a local copy so the signals package can depend on the data without
// pulling in the internal/explain tree at compile time.
func loadEvidence() {
	evidenceOnce.Do(func() {
		var doc struct {
			Detectors map[string]Evidence `json:"detectors"`
		}
		if err := json.Unmarshal(embeddedEvidenceJSON, &doc); err != nil {
			evidenceMap = map[string]Evidence{}
			return
		}
		evidenceMap = doc.Detectors
	})
}

// LookupEvidence returns the evidence row for a signal type, or zero
// value + false if the detector has no measured evidence.
func LookupEvidence(t models.SignalType) (Evidence, bool) {
	loadEvidence()
	ev, ok := evidenceMap[string(t)]
	return ev, ok
}

// EffectiveSeverity adjusts a signal's declared severity based on the
// validation evidence for its detector. The ladder is one-way:
// evidence can DEMOTE severity, never promote.
//
// The ladder requires lift >= 2 with a CI > 1.5 before declared
// severity stands at High, and lift >= 3 with CI > 1.5 before
// Critical is allowed:
//
//   - lift CI upper < 1.0                          → floor to Low
//   - lift CI upper < 1.5 (any hand-prec)          → cap at Low
//   - lift CI upper < 2.0                          → cap at Medium
//   - lift CI upper < 3.0 OR CI lower ≤ 1.5        → cap at High
//   - lift CI upper ≥ 3.0 AND CI lower > 1.5       → as declared (Critical allowed)
//
// Detectors with NO evidence row hit the fail-closed path: cap to Medium
// (treat absence of evidence as "evidence rejects critical/high").
//
// Returns (effective, adjusted) where `adjusted` indicates whether the
// declared severity was changed. Callers can surface this in metadata.
func EffectiveSeverity(t models.SignalType, declared models.SignalSeverity) (effective models.SignalSeverity, adjusted bool) {
	// Observability-tier detectors short-circuit the lift ladder.
	// Their failure mode is silent quality degradation that the
	// PR-revert proxy cannot measure, so lift-based demotion would be
	// wrong-headed. Cap at Medium so they never gate CI; declared
	// severity otherwise stands.
	if isObservabilityTier(t) {
		return capSeverity(declared, models.SeverityMedium)
	}

	ev, ok := LookupEvidence(t)
	if !ok {
		// Fail-closed: no evidence ≤ Medium.
		return capSeverity(declared, models.SeverityMedium)
	}
	if ev.GlobalLift == nil {
		return capSeverity(declared, models.SeverityMedium)
	}
	upper := ev.GlobalLift.High95
	lower := ev.GlobalLift.Low95

	switch {
	case upper < 1.0:
		return capSeverity(declared, models.SeverityLow)
	case upper < 1.5:
		return capSeverity(declared, models.SeverityLow)
	case upper < 2.0:
		return capSeverity(declared, models.SeverityMedium)
	case upper >= 3.0 && lower > 1.5:
		// Strong empirical lift with tight CI — declared severity stands.
		return declared, false
	default:
		// Mid-range (lift CI upper ≥ 2.0 but < 3.0, or CI lower ≤ 1.5):
		// cap at High. Critical only earned by the strong-lift branch.
		return capSeverity(declared, models.SeverityHigh)
	}
}

// isObservabilityTier returns true when the signal type's manifest
// entry does NOT declare Tier == TierGate. Empty tier defaults to
// observability — every detector with a manifest entry must
// explicitly opt in to TierGate (CI-blocking) by setting
// Tier: TierGate. Rationale: a detector that has not been
// lift-validated should not silently block CI just because its
// manifest entry forgot to set Tier.
//
// Signal types with NO manifest entry (runtime/eval-derived signals
// such as safetyFailure or costRegression that originate from
// adapter ingestion rather than the static detector registry) keep
// the legacy "treat as gate-relevant" behavior so they continue to
// surface in PR-comment Blocking sections.
func isObservabilityTier(t models.SignalType) bool {
	entry, ok := ManifestByType(t)
	if !ok {
		return false
	}
	return entry.Tier != TierGate
}

// IsGateRelevant returns true when the signal type can block CI via
// `--fail-on=*`. Equivalent to "not observability tier." Consumed by
// the gate logic in cmd/terrain.
func IsGateRelevant(t models.SignalType) bool {
	return !isObservabilityTier(t)
}

// capSeverity returns the lesser of declared and cap, by severity rank.
// Returns (effective, true) when the cap actually lowered the severity.
func capSeverity(declared, cap models.SignalSeverity) (models.SignalSeverity, bool) {
	if severityRank(declared) <= severityRank(cap) {
		return declared, false
	}
	return cap, true
}

func severityRank(s models.SignalSeverity) int {
	switch s {
	case models.SeverityCritical:
		return 4
	case models.SeverityHigh:
		return 3
	case models.SeverityMedium:
		return 2
	case models.SeverityLow:
		return 1
	case models.SeverityInfo:
		return 0
	}
	return 2 // unknown ≈ medium
}
