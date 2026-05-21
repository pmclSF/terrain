package ascg

import (
	"github.com/pmclSF/terrain/internal/mechanisms"
)

// RoleMechanismName is the canonical name in mechanisms.yaml.
const RoleMechanismName = "ascg_live_vs_catalog"

// Role names two structural occurrences distinguished by import-graph
// position, not text content (the Phase 1 Classify function classified
// by file context; roles add the cross-file structural test).
type Role int

const (
	// RoleUnknown means the import-graph data doesn't satisfy either
	// pinning condition. Callers fall back to the Phase 1 Classify
	// verdict.
	RoleUnknown Role = iota

	// RoleLiveConfigValue means: top-level const exported AND consumed
	// by a call-site in the same module/package via import-graph. The
	// value influences runtime behavior — findings should fire as-is.
	RoleLiveConfigValue

	// RoleCatalogOrExampleString means: defined inside a Map / Record
	// / dict literal of ≥3 entries; NOT consumed by any call-site in
	// the same file; the map keys ARE referenced by call arguments in
	// import-reached call-sites elsewhere. The value is a lookup
	// target, not an active config — findings demote to NOTE.
	RoleCatalogOrExampleString
)

func (r Role) String() string {
	switch r {
	case RoleLiveConfigValue:
		return "live_config_value"
	case RoleCatalogOrExampleString:
		return "catalog_or_example_string"
	default:
		return "unknown_role"
	}
}

// GraphFacts is the import-graph evidence the role test consumes. The
// caller (typically a detector after the depgraph + AST scan) fills
// in the facts it has access to. Roles that need a fact absent from
// GraphFacts evaluate to RoleUnknown so the gate fails open.
type GraphFacts struct {
	// IsExported is true when the symbol is exported from its file
	// (re-exported via index, named export, or top-level binding in
	// Python/Go).
	IsExported bool

	// ConsumedInSameFile is true when at least one call-site in the
	// same file references the symbol.
	ConsumedInSameFile bool

	// ImportReachedFromCallSites is true when the symbol is named in
	// at least one call argument from a file that imports the defining
	// module (via the depgraph).
	ImportReachedFromCallSites bool

	// InMapLiteralOfSize is non-zero when the symbol lives inside a
	// Map / Record / dict literal with this many sibling entries. The
	// catalog role requires ≥3 entries (so a 2-element pair isn't
	// classified as catalog).
	InMapLiteralOfSize int

	// MapKeysReferencedElsewhere is true when the map's keys appear as
	// call arguments in import-reached call-sites. This is the
	// "consumers use the catalog by-key" signal.
	MapKeysReferencedElsewhere bool
}

// RoleOf returns the structural role for `name` given the supplied
// graph facts. Used by consumer detectors after they collect the facts
// during their scan.
func RoleOf(facts GraphFacts) Role {
	// Live: exported + consumed in same file via call-site, OR
	// exported + consumed cross-file. We accept either as "live."
	// Direct consumption beats the map structure when both apply.
	if facts.IsExported && (facts.ConsumedInSameFile || facts.ImportReachedFromCallSites) {
		return RoleLiveConfigValue
	}

	// Catalog: in a map of ≥3 entries, NOT consumed in same file, but
	// keys referenced elsewhere via import.
	if facts.InMapLiteralOfSize >= 3 &&
		!facts.ConsumedInSameFile &&
		facts.MapKeysReferencedElsewhere {
		return RoleCatalogOrExampleString
	}

	return RoleUnknown
}

// GateRole is the canonical mechanism-state wire-up for live-vs-
// catalog consumers. Given the symbol facts + the finding's
// surrounding context, returns a structural Decision telling the
// caller whether to keep the finding, demote it, or take no action.
//
// Off → no role test runs, falls back to the Classify verdict.
// Shadow → role test runs, emits would_suppress / would_demote events
//   on disagreement with legacy behavior, user-visible findings
//   unchanged.
// On → role test result is authoritative.
type RoleDecision struct {
	Role   Role
	Keep   bool  // false → suppress (or demote)
	Demote bool  // true → keep but demote severity
}

func GateRole(reg *mechanisms.Registry, facts GraphFacts, loc Location, ruleID string) RoleDecision {
	// Off short-circuits: no role test runs, fall back to Classify.
	if reg.State(RoleMechanismName) == mechanisms.StateOff {
		return RoleDecision{Role: RoleUnknown, Keep: true}
	}

	role := RoleOf(facts)
	dec := RoleDecision{Role: role, Keep: true}

	// Route the catalog-demotion through the canonical state machine;
	// returns true when state=on AND role is catalog (the demote
	// trigger), false otherwise. Shadow-mode emit is handled inside.
	demote := mechanisms.GateDemote(reg, RoleMechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: loc.Path, Line: loc.Line},
		func() mechanisms.PredicateResult {
			return mechanisms.PredicateResult{
				Fired:   role == RoleCatalogOrExampleString,
				Reasons: []string{"symbol is a catalog/example-string per import-graph role"},
			}
		})
	dec.Demote = demote
	return dec
}

// CombineWithClassify merges a structural role decision with the
// Phase 1 Classify verdict. Returns the strictest verdict — if either
// says "catalog/example," demote; if either says "live," keep.
func CombineWithClassify(roleDec RoleDecision, classifyRes Result) RoleDecision {
	// If the role gate already produced a strong verdict, trust it.
	if roleDec.Role != RoleUnknown {
		return roleDec
	}
	switch classifyRes.Class {
	case CatalogOrExample:
		return RoleDecision{Role: RoleCatalogOrExampleString, Keep: true, Demote: true}
	case Live:
		return RoleDecision{Role: RoleLiveConfigValue, Keep: true}
	default:
		return roleDec
	}
}

