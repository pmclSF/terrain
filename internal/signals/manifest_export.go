package signals

import (
	"encoding/json"

	"github.com/pmclSF/terrain/internal/models"
)

// ManifestExportEntry is the wire-format projection of a ManifestEntry for
// `docs/signals/manifest.json`. It uses explicit JSON tags so the generated
// file stays stable across Go-struct-tag changes inside the package, and
// flattens enum types to plain strings so non-Go consumers (the eventual
// docs site, third-party readers) don't need to learn the in-tree types.
//
// Field order in this struct dictates field order in the emitted JSON when
// combined with the deterministic key emission `encoding/json` performs
// (alphabetical-by-default). We keep the json tags ordered by intent so
// downstream readers see Type/ConstName/Domain/Status first.
type ManifestExportEntry struct {
	Type              models.SignalType     `json:"type"`
	ConstName         string                `json:"constName"`
	Domain            models.SignalCategory `json:"domain"`
	Status            SignalStatus          `json:"status"`
	// Tier classifies the rule as "gate" (counts toward
	// `--fail-on=*` gate decisions) or "observability" (informational
	// only). The field is always emitted — no default tier — so external
	// consumers reading this manifest can deterministically predict
	// whether a finding contributes to Terrain's gate decision.
	Tier              SignalTier            `json:"tier"`
	DisabledByDefault bool                  `json:"disabledByDefault,omitempty"`
	Title             string                `json:"title"`
	Description       string                `json:"description"`
	Remediation       string                `json:"remediation,omitempty"`
	DefaultSeverity   models.SignalSeverity `json:"defaultSeverity"`
	ConfidenceMin     float64               `json:"confidenceMin"`
	ConfidenceMax     float64               `json:"confidenceMax"`
	EvidenceSources   []string              `json:"evidenceSources,omitempty"`
	RuleID            string                `json:"ruleId"`
	RuleURI           string                `json:"ruleUri,omitempty"`
	PromotionPlan     string                `json:"promotionPlan,omitempty"`
}

// ManifestExport is the top-level shape of `docs/signals/manifest.json`.
// SchemaVersion is bumped whenever the export shape changes — consumers
// can refuse loads of unsupported majors.
type ManifestExport struct {
	SchemaVersion string                `json:"schemaVersion"`
	Entries       []ManifestExportEntry `json:"entries"`
}

// CurrentManifestSchemaVersion is the wire-format version of the export.
// Bump the major if a field becomes required, the minor if a field is
// added in an additive way.
//
// 1.2.0 changes:
//   - "tier" is now required (always emitted, no longer omitempty).
//   - "disabledByDefault" is a new omitempty field.
//   - "ruleUri" is now omitempty (was always-emitted in 1.1.0).
//
// Pre-1.2.0 parsers that ignored unknown fields keep working. Two
// behavioral breaks to flag in release notes for external consumers:
//   - Any consumer that defaulted missing "tier" to "gate" must update
//     — the field is now always present, so a present-but-empty value
//     is a producer bug (not a default-to-gate hint).
//   - A consumer that expected "ruleUri" on every entry must handle
//     its absence — emission now skips the field on entries whose rule
//     doc has not yet shipped.
const CurrentManifestSchemaVersion = "1.2.0"

// BuildManifestExport projects the in-memory manifest into a stable wire
// format suitable for marshaling to JSON. The result is deterministic:
// entries appear in the order declared in manifest.go (which is itself
// stable for documentation purposes).
func BuildManifestExport() ManifestExport {
	out := ManifestExport{
		SchemaVersion: CurrentManifestSchemaVersion,
		Entries:       make([]ManifestExportEntry, 0, len(allSignalManifest)),
	}
	for _, e := range allSignalManifest {
		ruleURI := e.RuleURI
		// docs-gen skips rule-doc emission for StatusPlanned entries
		// (their docs haven't shipped yet). Suppress the RuleURI here
		// too so external consumers don't dereference a stale path.
		if e.Status == StatusPlanned {
			ruleURI = ""
		}
		out.Entries = append(out.Entries, ManifestExportEntry{
			Type:              e.Type,
			ConstName:         e.ConstName,
			Domain:            e.Domain,
			Status:            e.Status,
			Tier:              e.Tier,
			DisabledByDefault: e.DisabledByDefault,
			Title:             e.Title,
			Description:       e.Description,
			Remediation:       e.Remediation,
			DefaultSeverity:   e.DefaultSeverity,
			ConfidenceMin:     e.ConfidenceMin,
			ConfidenceMax:     e.ConfidenceMax,
			EvidenceSources:   e.EvidenceSources,
			RuleID:            e.RuleID,
			RuleURI:           ruleURI,
			PromotionPlan:     e.PromotionPlan,
		})
	}
	return out
}

// MarshalManifestJSON emits the canonical JSON for the manifest export.
// Output is indented with two spaces and terminates with a newline so the
// committed `docs/signals/manifest.json` plays nicely with text-mode tools
// and the `git diff --check` style trailing-newline rules.
func MarshalManifestJSON() ([]byte, error) {
	data, err := json.MarshalIndent(BuildManifestExport(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
