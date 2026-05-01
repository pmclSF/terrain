package airun

import (
	"encoding/json"
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
)

// ToEnvelope converts an EvalRunResult into the snapshot-level
// envelope that gets serialised into TestSuiteSnapshot.EvalRuns. The
// embedded payload is JSON-encoded so the models package can stay
// independent of the airun struct shape.
func (r *EvalRunResult) ToEnvelope(sourcePath string) (models.EvalRunEnvelope, error) {
	if r == nil {
		return models.EvalRunEnvelope{}, fmt.Errorf("nil EvalRunResult")
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return models.EvalRunEnvelope{}, fmt.Errorf("encode EvalRunResult: %w", err)
	}
	return models.EvalRunEnvelope{
		Framework:  r.Framework,
		SourcePath: sourcePath,
		RunID:      r.RunID,
		Aggregates: models.EvalRunAggregates{
			Successes: r.Aggregates.Successes,
			Failures:  r.Aggregates.Failures,
			Errors:    r.Aggregates.Errors,
			Tokens: models.EvalRunTokenUsage{
				Total: r.Aggregates.TokenUsage.Total,
				Cost:  r.Aggregates.TokenUsage.Cost,
			},
		},
		Payload: payload,
	}, nil
}

// ParseEvalRunPayload decodes the embedded JSON in an envelope back
// into the rich EvalRunResult. Returns an error when the payload is
// missing or malformed.
//
// Detectors that need per-case data (aiCostRegression,
// aiHallucinationRate, aiRetrievalRegression) call this on each
// envelope they're given, rather than re-running the framework adapter.
func ParseEvalRunPayload(env models.EvalRunEnvelope) (*EvalRunResult, error) {
	if len(env.Payload) == 0 {
		return nil, fmt.Errorf("envelope has no payload (framework=%s)", env.Framework)
	}
	var out EvalRunResult
	if err := json.Unmarshal(env.Payload, &out); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	return &out, nil
}
