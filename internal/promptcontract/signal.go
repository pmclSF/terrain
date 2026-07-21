package promptcontract

import (
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// ToSignal converts a Drift into the canonical models.Signal shape emitted
// across Terrain's report surfaces (JSON, JUnit, SARIF, PR comments). It reuses
// SignalAIPromptSchemaDrift — the same prompt↔schema-field boundary rule — but
// the evidence here is diff-free static consistency (the prompt names a field
// the schema never declares) rather than a git-diff removal, so the Explanation
// carries the resolved static join instead of a before/after render.
func (d Drift) ToSignal() models.Signal {
	return models.Signal{
		Type:     signals.SignalAIPromptSchemaDrift,
		Category: models.CategoryAI,
		Severity: models.SeverityHigh,
		// Floor of the manifest's confidence band (0.85–0.95). The bind is a
		// fully resolved AST join — a typed parameter to an in-repo schema
		// (import-scoped) to its complete field set (own + inherited +
		// methods) — and fires only when the join is unambiguous, so a false
		// positive needs a schema whose real attributes differ from what the
		// parser sees (e.g. a dynamic __getattr__). Raise only with recorded
		// precision evidence.
		Confidence: 0.85,
		Location: models.SignalLocation{
			File: d.PromptPath,
			Line: d.PromptLine,
		},
		Explanation: d.Message + ". The prompt will interpolate a missing value when this path renders.",
		SuggestedAction: "Correct the field name referenced in the prompt, " +
			"or add the missing field to the schema.",
		Metadata: map[string]any{
			"object":     d.Object,
			"variable":   d.Variable,
			"schemaName": d.SchemaName,
			"schemaPath": d.SchemaPath,
			"bindKind":   d.Kind,
			"promptLine": d.PromptLine,
		},
	}
}

// ToSignals converts a slice of Drift into signals.
func ToSignals(drift []Drift) []models.Signal {
	out := make([]models.Signal, 0, len(drift))
	for _, d := range drift {
		out = append(out, d.ToSignal())
	}
	return out
}
