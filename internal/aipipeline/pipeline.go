package aipipeline

import "context"

// Pipeline orchestrates Stages and a Composer for one or more
// (path, rule) candidates. Stages are run in order; if any stage sets
// Continue=false the candidate is dropped before composition.
type Pipeline struct {
	Stages   []Stage
	Composer *Composer
}

// NewPipeline constructs a Pipeline with the given stages and composer.
// Stage order is preserved.
func NewPipeline(composer *Composer, stages ...Stage) *Pipeline {
	return &Pipeline{
		Stages:   append([]Stage(nil), stages...),
		Composer: composer,
	}
}

// Run evaluates a single candidate through the configured stages,
// returning the composed Finding. Returns (Finding{}, false) when the
// candidate was dropped by a stage before composition.
func (p *Pipeline) Run(ctx context.Context, cand *Candidate) (Finding, bool) {
	for _, st := range p.Stages {
		if ctx.Err() != nil {
			return Finding{}, false
		}
		res := st.Run(ctx, cand)
		if !res.Continue {
			return Finding{}, false
		}
	}
	if p.Composer == nil {
		// Without a composer there is nothing meaningful to return.
		return Finding{}, false
	}
	return p.Composer.Compose(cand), true
}

// RunAll evaluates a batch of candidates. The returned slice is the
// subset that survived all stages and were composed into findings.
func (p *Pipeline) RunAll(ctx context.Context, cands []*Candidate) []Finding {
	out := make([]Finding, 0, len(cands))
	for _, c := range cands {
		f, ok := p.Run(ctx, c)
		if !ok {
			continue
		}
		out = append(out, f)
	}
	return out
}

// EmittedFindings filters a slice of findings to those that meet the
// posture threshold. Caller can use this to render only the verdicts
// that should appear in PR comments or gate decisions.
func (p *Pipeline) EmittedFindings(findings []Finding) []Finding {
	if p.Composer == nil {
		return findings
	}
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if p.Composer.ShouldEmit(f) {
			out = append(out, f)
		}
	}
	return out
}
