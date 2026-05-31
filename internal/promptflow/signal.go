package promptflow

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/schemadiff"
	"github.com/pmclSF/terrain/internal/signals"
)

// ToSignal converts a Finding into the canonical models.Signal shape
// emitted across Terrain's report surfaces (JSON, JUnit, SARIF, PR
// comments via the prtemplates registry).
//
// The rendered before/after text is preserved in Metadata under the
// `renderedBefore` / `renderedAfter` keys so the renderer surfaces can
// reproduce the side-by-side block without re-running the pipeline.
func (f Finding) ToSignal() models.Signal {
	return models.Signal{
		Type:        signals.SignalAIPromptSchemaDrift,
		Category:    models.CategoryAI,
		Severity:    models.SeverityHigh,
		// Confidence is set to the structural-evidence floor — the
		// detector identifies a (template, schema, field) join via
		// AST/JSON parsing with no heuristics, so false positives
		// require either a malformed schema or a template that names
		// the field by coincidence. The number is the floor of the
		// manifest's declared confidence range; raise it (with
		// rationale in the commit body) once adopter-corpus precision
		// data confirms the practical FP rate.
		Confidence:  0.85,
		Location:    models.SignalLocation{File: f.TemplatePath},
		Explanation: explanationFor(f),
		SuggestedAction: "Update the template to use the new schema field, " +
			"restore the old field, or remove the variable reference.",
		Metadata: map[string]any{
			"variable":       f.Risk.Variable,
			"schemaPath":     f.SchemaPath,
			"changeKind":     f.Risk.Change.Kind.String(),
			"oldType":        f.Risk.Change.OldType,
			"newType":        f.Risk.Change.NewType,
			"renderedBefore": f.RenderedBefore,
			"renderedAfter":  f.RenderedAfter,
		},
	}
}

func explanationFor(f Finding) string {
	switch f.Risk.Change.Kind {
	case schemadiff.ChangeRemoved:
		return fmt.Sprintf(
			"Template %s references schema field %q in %s, which this PR removed.",
			f.TemplatePath, f.Risk.Variable, f.SchemaPath)
	case schemadiff.ChangeTypeChanged:
		return fmt.Sprintf(
			"Template %s references schema field %q in %s, whose type changed from %s to %s in this PR.",
			f.TemplatePath, f.Risk.Variable, f.SchemaPath,
			f.Risk.Change.OldType, f.Risk.Change.NewType)
	default:
		return fmt.Sprintf(
			"Template %s references schema field %q in %s, which changed in this PR.",
			f.TemplatePath, f.Risk.Variable, f.SchemaPath)
	}
}

// ToSignals is a convenience over a slice of Findings.
func ToSignals(findings []Finding) []models.Signal {
	out := make([]models.Signal, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ToSignal())
	}
	return out
}
