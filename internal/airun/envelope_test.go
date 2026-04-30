package airun

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	in := &EvalRunResult{
		Framework: "promptfoo",
		RunID:     "eval-1",
		Cases: []EvalCase{
			{CaseID: "a", Description: "x", Success: true, Score: 1.0,
				TokenUsage: TokenUsage{Total: 10, Cost: 0.001}},
			{CaseID: "b", Description: "y", Success: false, Score: 0.0,
				TokenUsage: TokenUsage{Total: 20, Cost: 0.002}},
		},
		Aggregates: EvalAggregates{
			Successes: 1, Failures: 1,
			TokenUsage: TokenUsage{Total: 30, Cost: 0.003},
		},
	}

	env, err := in.ToEnvelope("evals/run.json")
	if err != nil {
		t.Fatalf("ToEnvelope: %v", err)
	}
	if env.Framework != "promptfoo" {
		t.Errorf("framework = %q", env.Framework)
	}
	if env.SourcePath != "evals/run.json" {
		t.Errorf("sourcePath = %q", env.SourcePath)
	}
	if env.Aggregates.Tokens.Total != 30 {
		t.Errorf("aggregates.Tokens.Total = %d", env.Aggregates.Tokens.Total)
	}
	if len(env.Payload) == 0 {
		t.Fatal("payload empty")
	}

	out, err := ParseEvalRunPayload(env)
	if err != nil {
		t.Fatalf("ParseEvalRunPayload: %v", err)
	}
	if len(out.Cases) != 2 {
		t.Fatalf("cases = %d", len(out.Cases))
	}
	if out.Cases[1].FailureReason != in.Cases[1].FailureReason {
		t.Errorf("FailureReason round-trip lost: %+v vs %+v", in.Cases[1], out.Cases[1])
	}
}

func TestParseEvalRunPayload_Empty(t *testing.T) {
	t.Parallel()
	if _, err := ParseEvalRunPayload(models.EvalRunEnvelope{}); err == nil {
		t.Error("expected error on empty envelope")
	}
}
