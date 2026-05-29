// Package promptflow correlates prompt-template variable references
// with schema-diff changes. Used to surface "your prompt template
// references a schema field that changed in this PR" findings.
//
// Slice 3 ships single-hop correlation: a variable matches a
// changed field iff their names are equal. Future slices add
// transformation-function tracing (variable goes through a function
// before reaching the template) and rename-detection.
package promptflow

import (
	"sort"

	"github.com/pmclSF/terrain/internal/prompttemplate"
	"github.com/pmclSF/terrain/internal/schemadiff"
)

// Risk is one correlated (variable, change) pair.
type Risk struct {
	Variable string
	Change   schemadiff.Change
}

// CorrelateVars returns the set of risks where a name in vars matches
// the Field of a Removed or TypeChanged entry in changes. Added
// fields do not produce risks — a new field can't break an existing
// template reference.
//
// Results are sorted by Variable for stable downstream consumers.
func CorrelateVars(vars []string, changes []schemadiff.Change) []Risk {
	changeByField := map[string]schemadiff.Change{}
	for _, c := range changes {
		if c.Kind == schemadiff.ChangeRemoved || c.Kind == schemadiff.ChangeTypeChanged {
			changeByField[c.Field] = c
		}
	}
	var risks []Risk
	for _, v := range vars {
		if c, ok := changeByField[v]; ok {
			risks = append(risks, Risk{Variable: v, Change: c})
		}
	}
	sort.Slice(risks, func(i, j int) bool {
		return risks[i].Variable < risks[j].Variable
	})
	return risks
}

// CorrelateTemplate is a convenience wrapper that pulls variables
// from t and forwards them to CorrelateVars.
func CorrelateTemplate(t prompttemplate.Template, changes []schemadiff.Change) []Risk {
	return CorrelateVars(t.Vars(), changes)
}
