package remediate

import "github.com/pmclSF/terrain/internal/findings"

// FixProducer returns a structured, mechanically-applicable Fix for a
// finding, or nil when no deterministic fix applies (the remediation is then
// judge-only). root is the repository root; producers may read files under
// it to compute the fix. Producers must only return a Fix they have reason
// to believe resolves the finding — the closed-loop validator is the proof,
// the producer is the claim.
type FixProducer func(root string, f findings.Finding) *findings.Fix

// FixRegistry maps a canonical ruleID to the producer that knows how to
// remediate it. It is the signal-side analog of the AI composer's
// fixscaffold registry: where the AI path attaches FixScaffold during
// composition, the signal path attaches Suggestion.Fix here, after
// detection, so the canonical finding carries an applicable remediation.
type FixRegistry struct {
	producers map[string]FixProducer
}

// NewFixRegistry returns an empty registry.
func NewFixRegistry() *FixRegistry {
	return &FixRegistry{producers: map[string]FixProducer{}}
}

// Register binds a producer to a ruleID. Last registration wins.
func (r *FixRegistry) Register(ruleID string, p FixProducer) {
	r.producers[ruleID] = p
}

// Attach walks the findings and, for each one whose rule has a registered
// producer, attaches the produced Fix to the finding's primary suggestion
// (preserving its existing remediation text). Findings without a producer,
// or whose producer declines, are left untouched (judge-only). Mutates fs in
// place; returns the number of fixes attached.
func (r *FixRegistry) Attach(root string, fs []findings.Finding) int {
	attached := 0
	for i := range fs {
		p := r.producers[fs[i].RuleID]
		if p == nil {
			continue
		}
		fix := p(root, fs[i])
		if fix == nil {
			continue
		}
		if len(fs[i].Suggestions) == 0 {
			fs[i].Suggestions = []findings.Suggestion{{Text: "Apply the suggested fix.", Fix: fix}}
		} else {
			fs[i].Suggestions[0].Fix = fix
		}
		attached++
	}
	return attached
}
