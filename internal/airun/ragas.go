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
		// Mean score across the QUALITY axes only; success := all >= threshold.
		// Pre-0.2.x every numeric column flowed into the success vote,
		// including ancillary metrics like `cost`, `latency_ms`, or any
		// custom user-added column. A faithfulness=0.45 alongside
		// `cost: 0.003` flipped the case to failed because cost < 0.5
		// (nonsensical: small cost is GOOD). We now restrict the
		// threshold check to keys that pass `isRagasQualityKey`.
		qualityScores := make([]float64, 0, len(named))
		for k, v := range named {
			if isRagasQualityKey(k) {
				qualityScores = append(qualityScores, v)
			}
		}
		switch {
		case len(qualityScores) == 0 && len(named) == 0:
			c.Score = 0
			c.Success = false
		case len(qualityScores) == 0:
			// Row had only ancillary numerics; no opinion on success.
			c.Score = 0
			c.Success = true
		default:
			var sum float64
			c.Success = true
			for _, v := range qualityScores {
				sum += v
				if v < successThreshold {
					c.Success = false
				}
			}
			c.Score = sum / float64(len(qualityScores))
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

// ragasQualityKeys lists the named-score keys whose semantics are
// "0–1 quality axis where higher is better." Other numeric columns
// (cost, latency, token counts, custom metrics) must NOT flow into
// the success vote because they have different polarity / range.
//
// Keep this aligned with retrieval_regression.go's retrievalScoreKeys
// — anything in that retrieval-detector allowlist is also a quality
// axis here, plus a few more (faithfulness, answer_correctness, etc.)
// that aren't retrieval-shaped but still belong to the quality vote.
var ragasQualityKeys = map[string]bool{
	"context_precision":      true,
	"context_recall":         true,
	"context_entity_recall":  true,
	"context_relevance":      true,
	"faithfulness":           true,
	"answer_relevancy":       true,
	"answer_relevance":       true,
	"answer_correctness":     true,
	"answer_similarity":      true,
	"semantic_similarity":    true,
	"factuality":             true,
	"groundedness":           true,
	"helpfulness":            true,
	"harmfulness":            true, // inverse polarity, but still 0-1
	"coherence":              true,
	"conciseness":            true,
	"relevance":              true,
	"relevance_score":        true,
	"retrieval_score":        true,
	"ndcg":                   true,
	"coverage":               true,
}

// isRagasQualityKey reports whether a NamedScore key is a quality
// axis whose value should flow into success/failure synthesis.
// Variants (hyphens, suffixed `_score`) are normalised.
func isRagasQualityKey(key string) bool {
	low := strings.ToLower(strings.TrimSpace(key))
	low = strings.ReplaceAll(low, "-", "_")
	low = strings.TrimSuffix(low, "_score")
	return ragasQualityKeys[low]
}

// numericValue extracts a float64 from a JSON-decoded value if it
// looks numeric. Booleans / strings / nested objects return false.
//
// Pre-0.2.x this only accepted float64 / json.Number, so wrappers that
// emit ints (Ragas DataFrame export through certain helpers, custom
// JSON encoders) silently dropped the score.
func numericValue(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
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
