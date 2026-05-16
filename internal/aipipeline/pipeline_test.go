package aipipeline

import (
	"context"
	"testing"
)

// stubStage is a Stage that emits a fixed atom and returns Continue.
type stubStage struct {
	name     string
	atom     EvidenceAtom
	cont     bool
	fallback string
}

func (s *stubStage) Name() string { return s.name }
func (s *stubStage) Run(_ context.Context, c *Candidate) StageResult {
	c.AddAtom(s.atom)
	if s.fallback != "" {
		c.AddFallback(s.fallback)
	}
	return StageResult{Continue: s.cont}
}

func TestPipelineRun_AllStagesEmit(t *testing.T) {
	t.Parallel()
	cal := DefaultCalibration()
	comp := NewComposer(cal, PostureObservability)
	p := NewPipeline(comp,
		&stubStage{name: "a", atom: EvidenceAtom{
			Kind: EvidenceLexical, RuleID: "regex.openai.import", Weight: +0.4,
			Source: "stub-a",
		}, cont: true},
		&stubStage{name: "b", atom: EvidenceAtom{
			Kind: EvidenceLexical, RuleID: "regex.openai.call", Weight: +1.4,
			Source: "stub-b",
		}, cont: true},
		&stubStage{name: "c", atom: EvidenceAtom{
			Kind: EvidenceStructural, RuleID: "ast.bound_call", Weight: +2.0,
			Source: "stub-c",
		}, cont: true},
	)
	cand := &Candidate{
		Path:   "app/handler.py",
		Lang:   "python",
		RuleID: "ai.surface.missing_eval",
		Cohort: "agent-app",
	}
	f, ok := p.Run(context.Background(), cand)
	if !ok {
		t.Fatalf("pipeline dropped candidate")
	}
	if got := len(f.Atoms); got != 3 {
		t.Errorf("expected 3 atoms; got %d", got)
	}
	if f.Confidence <= 0.5 {
		t.Errorf("expected positive confidence; got %v", f.Confidence)
	}
}

func TestPipelineRun_StageDropsCandidate(t *testing.T) {
	t.Parallel()
	comp := NewComposer(DefaultCalibration(), PostureObservability)
	p := NewPipeline(comp,
		&stubStage{name: "a", atom: EvidenceAtom{RuleID: "x"}, cont: true},
		&stubStage{name: "b", atom: EvidenceAtom{RuleID: "y"}, cont: false}, // drops
		&stubStage{name: "c", atom: EvidenceAtom{RuleID: "z"}, cont: true},
	)
	cand := &Candidate{Path: "x.py", Lang: "python", RuleID: "ai.surface.missing_eval"}
	_, ok := p.Run(context.Background(), cand)
	if ok {
		t.Errorf("expected pipeline to drop candidate after b returned Continue=false")
	}
}

func TestPipelineEmittedFindings_FiltersByThreshold(t *testing.T) {
	t.Parallel()
	comp := NewComposer(DefaultCalibration(), PostureObservability)
	p := NewPipeline(comp)
	findings := []Finding{
		{RuleID: "ai.surface.missing_eval", Confidence: 0.85},
		{RuleID: "ai.surface.missing_eval", Confidence: 0.30},
		{RuleID: "ai.surface.missing_eval", Confidence: 0.45, Suppressed: true},
	}
	got := p.EmittedFindings(findings)
	if len(got) != 1 {
		t.Errorf("expected only the 0.85 finding to be emitted; got %d", len(got))
	}
}
