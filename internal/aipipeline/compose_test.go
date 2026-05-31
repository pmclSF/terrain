package aipipeline

import (
	"math"
	"testing"
)

func TestSigmoid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   float64
		want float64
	}{
		{0.0, 0.5},
		{2.0, 0.880797},
		{-2.0, 0.119203},
		{10.0, 0.999955},
		{-10.0, 0.000045},
	}
	for _, c := range cases {
		got := sigmoid(c.in)
		if math.Abs(got-c.want) > 0.001 {
			t.Errorf("sigmoid(%v) = %v; want %v", c.in, got, c.want)
		}
	}
}

func TestComposerCompose_WeightedLogOdds(t *testing.T) {
	t.Parallel()
	cal := DefaultCalibration()
	c := NewComposer(cal, PostureObservability)

	// Synthetic candidate: regex-flagged openai import + call.
	cand := &Candidate{
		Path:   "app/agent/handler.py",
		Lang:   "python",
		RuleID: "ai.surface.missing_eval",
		Cohort: "agent-app",
		Atoms: []EvidenceAtom{
			{Kind: EvidenceLexical, RuleID: "regex.openai.import", Source: "regex-fastscan"},
			{Kind: EvidenceLexical, RuleID: "regex.openai.call", Source: "regex-fastscan"},
			{Kind: EvidenceStructural, RuleID: "ast.bound_call", Source: "ast-confirm"},
		},
	}
	f := c.Compose(cand)
	if f.Confidence <= 0.5 {
		t.Errorf("expected confidence > 0.5 for positive evidence stack; got %v (logOdds=%v)",
			f.Confidence, f.LogOdds)
	}
	if f.Suppressed {
		t.Errorf("did not expect suppression in observability mode without fallbacks")
	}
}

func TestComposerCompose_NegativeEvidenceWins(t *testing.T) {
	t.Parallel()
	cal := DefaultCalibration()
	c := NewComposer(cal, PostureObservability)

	cand := &Candidate{
		Path:   "src/llm/providers/openai_provider.py",
		Lang:   "python",
		RuleID: "ai.surface.missing_eval",
		Cohort: "agent-app",
		Atoms: []EvidenceAtom{
			{Kind: EvidenceLexical, RuleID: "regex.openai.import", Source: "regex-fastscan"},
			{Kind: EvidenceLexical, RuleID: "regex.openai.call", Source: "regex-fastscan"},
			{Kind: EvidenceNegative, RuleID: "wrapper.class.match", Source: "regex-fastscan"},
			{Kind: EvidenceNegative, RuleID: "ast.no_call_despite_regex", Source: "ast-confirm"},
			{Kind: EvidenceNegative, RuleID: "path.providers", Source: "path-prefilter"},
		},
	}
	f := c.Compose(cand)
	if f.Confidence >= 0.4 {
		t.Errorf("expected provider-wrapper to suppress confidence; got %v (logOdds=%v)",
			f.Confidence, f.LogOdds)
	}
}

func TestComposerShouldEmit_PosturesDiffer(t *testing.T) {
	t.Parallel()
	cal := DefaultCalibration()
	obs := NewComposer(cal, PostureObservability)
	gate := NewComposer(cal, PostureGate)

	cand := &Candidate{
		Path:   "app/handlers/chat.py",
		Lang:   "python",
		RuleID: "ai.surface.missing_eval",
		Cohort: "agent-app",
		Atoms: []EvidenceAtom{
			{Kind: EvidenceLexical, RuleID: "regex.openai.import", Source: "regex-fastscan"},
			{Kind: EvidenceLexical, RuleID: "regex.openai.call", Source: "regex-fastscan"},
		},
	}
	fObs := obs.Compose(cand)
	fGate := gate.Compose(cand)

	if fObs.Confidence != fGate.Confidence {
		t.Errorf("composer score should be identical across postures; obs=%v gate=%v",
			fObs.Confidence, fGate.Confidence)
	}
	// Observability emits at >=0.40; Gate at >=0.80. With moderate
	// positive evidence the finding should land in the gap.
	if fObs.Confidence < 0.4 && obs.ShouldEmit(fObs) {
		t.Errorf("observability should not emit below threshold")
	}
	if fGate.Confidence < 0.8 && gate.ShouldEmit(fGate) {
		t.Errorf("gate should not emit below threshold")
	}
}

func TestComposerGateGatesASTUnavailable(t *testing.T) {
	t.Parallel()
	cal := DefaultCalibration()
	gate := NewComposer(cal, PostureGate)

	cand := &Candidate{
		Path:      "app/handlers/chat.rb", // Ruby — no AST detector
		Lang:      "ruby",
		RuleID:    "ai.surface.missing_eval",
		Cohort:    "agent-app",
		Atoms:     []EvidenceAtom{{Kind: EvidenceLexical, RuleID: "regex.openai.call", Weight: +1.4}},
		Fallbacks: []string{"ast=unavailable"},
	}
	f := gate.Compose(cand)
	if !f.Suppressed {
		t.Errorf("gate posture should suppress ast=unavailable findings; got %+v", f)
	}
}

func TestAdjustSeverity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		declared Severity
		conf     float64
		want     Severity
	}{
		{SeverityCritical, 0.30, SeverityHigh},
		{SeverityCritical, 0.80, SeverityCritical},
		{SeverityHigh, 0.30, SeverityMedium},
		{SeverityHigh, 0.80, SeverityHigh},
		{SeverityMedium, 0.30, SeverityLow},
		{SeverityMedium, 0.95, SeverityHigh},
		{SeverityLow, 0.95, SeverityMedium},
		{SeverityLow, 0.20, SeverityLow},
	}
	for _, c := range cases {
		got := adjustSeverity(c.declared, c.conf)
		if got != c.want {
			t.Errorf("adjustSeverity(%v, %v) = %v; want %v",
				c.declared, c.conf, got, c.want)
		}
	}
}
