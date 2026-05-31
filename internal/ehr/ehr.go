// Package ehr is the Eval-Harness Recognizer with surfaces_covered
// output.
//
// The legacy logic recorded eval-config presence as a boolean: "this
// repo has at least one eval config file." That bool was used by
// downstream detectors (promptFileMissingEval, aiNonDeterministicEval,
// aiSafetyEvalMissing) to suppress findings on any prompt or surface
// in the repo. The over-suppression bug: a token-counting eval over
// one surface incorrectly silenced real prompt-missing-eval findings
// on unrelated surfaces.
//
// The fix is structural: every recognized eval config emits the set
// of surfaces it actually covers — prompt file paths, model names,
// dataset paths. Downstream detectors check `surfaces_covered` against
// the specific surface in their finding and only suppress when there's
// a match.
//
// Supported config shapes (best-effort):
//   - Promptfoo YAML  (prompts, providers, tests)
//   - DeepEval YAML    (test_cases with prompts / models)
//   - Ragas YAML       (testset / metrics with prompts / models)
//   - generic YAML     (any top-level `prompts:`, `model:`, `dataset:` key)
//
// The recognizer is mechanism-gated by `ehr_surfaces_covered`. Off →
// returns an empty SurfacesCovered (legacy behavior). Shadow → returns
// the parsed surfaces and emits would-add events for every surface a
// downstream detector decides not to suppress (i.e., a finding that
// would-be-kept once the gate is live).
package ehr

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pmclSF/terrain/internal/mechanisms"
)

// MechanismName is the canonical name in mechanisms.yaml.
const MechanismName = "ehr_surfaces_covered"

// SurfaceKind classifies one entry in SurfacesCovered. Downstream
// detectors filter by kind: promptFileMissingEval cares about Prompt;
// aiModelDeprecationRisk cares about Model.
type SurfaceKind string

const (
	SurfacePrompt  SurfaceKind = "prompt"
	SurfaceModel   SurfaceKind = "model"
	SurfaceDataset SurfaceKind = "dataset"
)

// Surface is one surface the eval config references. Value is a file
// path for Prompt/Dataset surfaces and an identifier for Model surfaces.
type Surface struct {
	Kind  SurfaceKind `json:"kind"`
	Value string      `json:"value"`
}

// Report is the recognizer's output for one eval-config file.
type Report struct {
	ConfigPath       string    `json:"config_path"`
	Format           string    `json:"format"`
	SurfacesCovered  []Surface `json:"surfaces_covered"`
}

// Recognize parses the eval-config at path and returns the surfaces it
// references. Unknown formats return an empty SurfacesCovered (not an
// error) so downstream detectors fall back to legacy behavior.
func Recognize(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return RecognizeBytes(data, path)
}

// RecognizeBytes is the in-memory variant. The path is used to detect
// the eval framework by filename.
func RecognizeBytes(data []byte, path string) (*Report, error) {
	report := &Report{ConfigPath: path}
	switch filepath.Base(strings.ToLower(path)) {
	case "promptfooconfig.yaml", "promptfooconfig.yml":
		report.Format = "promptfoo"
	case "deepeval.yaml", "deepeval.yml":
		report.Format = "deepeval"
	case "ragas.yaml", "ragas.yml":
		report.Format = "ragas"
	default:
		report.Format = "generic"
	}
	report.SurfacesCovered = parseSurfaces(data, report.Format)
	return report, nil
}

// parseSurfaces extracts surface entries from the YAML body. Handles
// multi-doc YAML (each `---`-separated document is parsed); each doc's
// top-level keys are scanned for surface-shaped data.
func parseSurfaces(data []byte, format string) []Surface {
	docs := splitYAMLDocs(data)
	var out []Surface
	for _, raw := range docs {
		var doc map[string]any
		if err := yaml.Unmarshal(raw, &doc); err != nil || doc == nil {
			continue
		}
		out = append(out, parsePrompts(doc)...)
		out = append(out, parseModels(doc)...)
		out = append(out, parseDatasets(doc)...)
		out = append(out, parseTestCases(doc)...)
		out = append(out, parseDeepEvalTestCases(doc)...)
	}
	return dedupSurfaces(out)
}

// splitYAMLDocs splits a multi-doc YAML stream on the `---` separator.
// Single-doc inputs return a single-element slice. yaml.v3's
// yaml.NewDecoder is the canonical approach but using strings.Split
// here keeps the parser tolerant of mildly malformed multi-doc
// boundaries (extra whitespace, trailing separators).
func splitYAMLDocs(data []byte) [][]byte {
	s := string(data)
	parts := strings.Split(s, "\n---")
	if len(parts) == 1 {
		return [][]byte{data}
	}
	out := make([][]byte, 0, len(parts))
	for _, p := range parts {
		// Strip a leading "---" if the separator was at start-of-stream.
		p = strings.TrimPrefix(p, "---")
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, []byte(p))
	}
	return out
}

// parsePrompts handles `prompts: <string>` or `prompts: [<string>, ...]`.
func parsePrompts(doc map[string]any) []Surface {
	v, ok := doc["prompts"]
	if !ok {
		return nil
	}
	return collectStringsAs(v, SurfacePrompt)
}

// parseModels handles `model: <string>`, `models: [<string>, ...]`,
// and `providers: [{id: <string>}, ...]` (the Promptfoo shape).
func parseModels(doc map[string]any) []Surface {
	var out []Surface
	if v, ok := doc["model"]; ok {
		out = append(out, collectStringsAs(v, SurfaceModel)...)
	}
	if v, ok := doc["models"]; ok {
		out = append(out, collectStringsAs(v, SurfaceModel)...)
	}
	if providers, ok := doc["providers"].([]any); ok {
		for _, p := range providers {
			switch x := p.(type) {
			case string:
				out = append(out, Surface{Kind: SurfaceModel, Value: x})
			case map[string]any:
				if id, ok := x["id"].(string); ok {
					out = append(out, Surface{Kind: SurfaceModel, Value: id})
				}
			}
		}
	}
	return out
}

// parseDatasets handles `dataset: <string>`, `datasets: [<string>, ...]`,
// and `testset: <string>` (Ragas).
func parseDatasets(doc map[string]any) []Surface {
	var out []Surface
	for _, key := range []string{"dataset", "datasets", "testset"} {
		if v, ok := doc[key]; ok {
			out = append(out, collectStringsAs(v, SurfaceDataset)...)
		}
	}
	return out
}

// parseDeepEvalTestCases handles DeepEval's `test_cases:` shape, where
// each entry carries inline `prompt` and `model` fields:
//
//	test_cases:
//	  - prompt: "Summarize: {{document}}"
//	    model: gpt-4o
//	    input: ...
//
// Both single-entry and multi-entry forms are supported.
func parseDeepEvalTestCases(doc map[string]any) []Surface {
	cases, ok := doc["test_cases"].([]any)
	if !ok {
		return nil
	}
	var out []Surface
	for _, c := range cases {
		entry, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := entry["prompt"].(string); ok {
			out = append(out, Surface{Kind: SurfacePrompt, Value: v})
		}
		if v, ok := entry["model"].(string); ok {
			out = append(out, Surface{Kind: SurfaceModel, Value: v})
		}
		if v, ok := entry["dataset"].(string); ok {
			out = append(out, Surface{Kind: SurfaceDataset, Value: v})
		}
	}
	return out
}

// parseTestCases handles `tests: [{vars: {dataset: ..., prompt: ...}}]`
// — common in Promptfoo test arrays.
func parseTestCases(doc map[string]any) []Surface {
	tests, ok := doc["tests"].([]any)
	if !ok {
		return nil
	}
	var out []Surface
	for _, t := range tests {
		tc, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if vars, ok := tc["vars"].(map[string]any); ok {
			if prompt, ok := vars["prompt"].(string); ok {
				out = append(out, Surface{Kind: SurfacePrompt, Value: prompt})
			}
			if dataset, ok := vars["dataset"].(string); ok {
				out = append(out, Surface{Kind: SurfaceDataset, Value: dataset})
			}
		}
	}
	return out
}

// collectStringsAs walks a YAML value (string or []any) and returns
// each string as a Surface with the given kind.
func collectStringsAs(v any, kind SurfaceKind) []Surface {
	switch x := v.(type) {
	case string:
		return []Surface{{Kind: kind, Value: x}}
	case []any:
		var out []Surface
		for _, e := range x {
			switch y := e.(type) {
			case string:
				out = append(out, Surface{Kind: kind, Value: y})
			case map[string]any:
				if s, ok := y["id"].(string); ok {
					out = append(out, Surface{Kind: kind, Value: s})
				} else if s, ok := y["name"].(string); ok {
					out = append(out, Surface{Kind: kind, Value: s})
				} else if s, ok := y["path"].(string); ok {
					out = append(out, Surface{Kind: kind, Value: s})
				}
			}
		}
		return out
	}
	return nil
}

func dedupSurfaces(in []Surface) []Surface {
	seen := map[string]bool{}
	var out []Surface
	for _, s := range in {
		key := string(s.Kind) + "|" + s.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// ── Gate helper ────────────────────────────────────────────────────

// Covers reports whether a given surface name is plausibly covered by
// any of the supplied reports. Matching is by exact value OR by
// suffix-match for path surfaces (the eval may reference
// `prompts/foo.txt` while the detector reports `foo.txt`).
func Covers(reports []*Report, kind SurfaceKind, name string) bool {
	for _, r := range reports {
		for _, s := range r.SurfacesCovered {
			if s.Kind != kind {
				continue
			}
			if s.Value == name {
				return true
			}
			if kind == SurfacePrompt || kind == SurfaceDataset {
				// Path-suffix match: 'prompts/foo.txt' covers 'foo.txt'
				// and vice-versa.
				if strings.HasSuffix(s.Value, "/"+name) || strings.HasSuffix(name, "/"+s.Value) {
					return true
				}
			}
		}
	}
	return false
}

// GateSuppression is the canonical wire-up for downstream detectors
// (promptFileMissingEval, aiNonDeterministicEval, aiSafetyEvalMissing).
// Returns Keep=true when the surface is NOT covered by any eval
// report, so the finding should fire. Returns Keep=false when the
// mechanism is on AND the surface IS covered — the eval covers this
// surface, so suppress.
//
// In shadow mode, never suppresses but emits would-add events for
// findings that legacy behavior would have suppressed but the
// per-surface check now keeps — i.e., the detector should fire
// despite some other eval being present.
func GateSuppression(reg *mechanisms.Registry, reports []*Report, kind SurfaceKind, name, ruleID, file string, legacyHadEvalConfig bool) bool {
	keepLegacy := !legacyHadEvalConfig

	// Gate fires when legacy would have suppressed (eval config present)
	// AND the per-surface check says this specific surface is NOT
	// covered — the gate's job is to LIFT the over-broad suppression.
	shouldAdd := mechanisms.GateAdd(reg, MechanismName,
		mechanisms.EventContext{RuleID: ruleID, File: file},
		func() mechanisms.PredicateResult {
			if !legacyHadEvalConfig {
				return mechanisms.PredicateResult{Fired: false}
			}
			if Covers(reports, kind, name) {
				return mechanisms.PredicateResult{Fired: false}
			}
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"eval config present but does not cover surface " + name},
			}
		})

	if shouldAdd {
		return true // gate lifted the legacy suppression
	}
	return keepLegacy
}
