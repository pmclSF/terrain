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
	Type            models.SignalType     `json:"type"`
	ConstName       string                `json:"constName"`
	Domain          models.SignalCategory `json:"domain"`
	Status          SignalStatus          `json:"status"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Remediation     string                `json:"remediation,omitempty"`
	DefaultSeverity models.SignalSeverity `json:"defaultSeverity"`
	ConfidenceMin   float64               `json:"confidenceMin"`
	ConfidenceMax   float64               `json:"confidenceMax"`
	EvidenceSources []string              `json:"evidenceSources,omitempty"`
	RuleID          string                `json:"ruleId"`
	RuleURI         string                `json:"ruleUri"`
	PromotionPlan   string                `json:"promotionPlan,omitempty"`
}

// ManifestExport is the top-level shape of `docs/signals/manifest.json`.
// SchemaVersion is bumped whenever the export shape changes — consumers
// can refuse loads of unsupported majors.
type ManifestExport struct {
	SchemaVersion string                `json:"schemaVersion"`
	Entries       []ManifestExportEntry `json:"entries"`
}

// CurrentManifestSchemaVersion is the wire-format version of the export.
// 1.0.0 ships in 0.2.0; bump the major if a field becomes required, the
// minor if a field is added in an additive way.
const CurrentManifestSchemaVersion = "1.0.0"

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
		out.Entries = append(out.Entries, ManifestExportEntry{
			Type:            e.Type,
			ConstName:       e.ConstName,
			Domain:          e.Domain,
			Status:          e.Status,
			Title:           e.Title,
			Description:     e.Description,
			Remediation:     e.Remediation,
			DefaultSeverity: e.DefaultSeverity,
			ConfidenceMin:   e.ConfidenceMin,
			ConfidenceMax:   e.ConfidenceMax,
			EvidenceSources: e.EvidenceSources,
			RuleID:          e.RuleID,
			RuleURI:         e.RuleURI,
			PromotionPlan:   e.PromotionPlan,
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
