package airun

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ParseRagasJSON parses a Ragas eval result payload into a
// normalised EvalRunResult. Pairs with ParsePromptfooJSON +
// ParseDeepEvalJSON; same target shape, same downstream detectors.
//
// Ragas typically writes a JSON like:
//
//	{
//	  "run_id": "...",
//	  "created_at": "...",
//	  "results": [
//	    {
//	      "question": "...",
//	      "answer": "...",
//	      "ground_truth": "...",
//	      "context_relevance": 0.85,
//	      "faithfulness": 0.92,
//	      "answer_relevancy": 0.78,
//	      ...
//	    }, ...
//	  ]
//	}
//
// The Ragas DataFrame -> JSON dump uses snake_case field names. The
// adapter pulls every numeric field into NamedScores so the
// retrieval-regression detector can pick up `faithfulness`,
// `context_relevance`, `answer_relevancy` directly.
//
// Success/failure isn't a Ragas concept (it produces continuous
// scores). We synthesize Success := all named scores >= 0.5; flip to
// false if any are below. Score := mean of named scores.
func ParseRagasJSON(data []byte) (*EvalRunResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	var raw ragasEnvelope
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ragas payload: %w", err)
	}
	if len(raw.Results) == 0 {
		return nil, fmt.Errorf("ragas payload has no results")
	}

	out := &EvalRunResult{
		Framework: "ragas",
		RunID:     raw.RunID,
	}
	if t, err := time.Parse(time.RFC3339, raw.CreatedAt); err == nil {
		out.CreatedAt = t.UTC()
	}

	// Reserved metric keys Ragas typically emits. Other keys can
	// appear (e.g. `latency_ms`); we collect them all into
	// NamedScores when they're numeric.
	const successThreshold = 0.5

	out.Cases = make([]EvalCase, 0, len(raw.Results))
	for i, row := range raw.Results {
		named := map[string]float64{}
		for k, v := range row {
			n, ok := numericValue(v)
			if !ok {
				continue
			}
			lk := strings.ToLower(strings.TrimSpace(k))
			if lk == "" {
				continue
			}
			named[lk] = n
		}

		// Description / id fall back to question.
		question, _ := row["question"].(string)
		caseID := stringField(row, "id")
		if caseID == "" {
			caseID = fmt.Sprintf("ragas-row-%d", i+1)
		}

		c := EvalCase{
			CaseID:      caseID,
			Description: question,
			NamedScores: named,
		}
		// Mean score across the named-score axes; success := all >= threshold.
		if len(named) == 0 {
			c.Score = 0
			c.Success = false
		} else {
			var sum float64
			c.Success = true
			for _, v := range named {
				sum += v
				if v < successThreshold {
					c.Success = false
				}
			}
			c.Score = sum / float64(len(named))
		}
		out.Cases = append(out.Cases, c)

		if c.Success {
			out.Aggregates.Successes++
		} else {
			out.Aggregates.Failures++
		}
	}

	return out, nil
}

// LoadRagasFile is the convenience wrapper around ParseRagasJSON.
func LoadRagasFile(path string) (*EvalRunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseRagasJSON(data)
}

// numericValue extracts a float64 from a JSON-decoded value if it
// looks numeric. Booleans / strings / nested objects return false.
func numericValue(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func stringField(row map[string]any, key string) string {
	if v, ok := row[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

type ragasEnvelope struct {
	RunID     string           `json:"run_id,omitempty"`
	CreatedAt string           `json:"created_at,omitempty"`
	Results   []map[string]any `json:"results"`
}
