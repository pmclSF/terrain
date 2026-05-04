package airun

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ShapeInfo captures the detected shape (framework + version family)
// of an eval-output payload, plus any drift warnings the adapter
// produced while parsing. Track 7.1 / 7.2 of the 0.2 release plan
// adds this so adopters can see when Terrain is parsing a payload
// shape it doesn't recognize, before that drift produces silent
// downstream regressions.
//
// Shape detection is best-effort: it reads only the top-level
// envelope (no full payload parse) and uses whatever signal is
// cheapest to look at — version field where present, structural
// shape fingerprint where not.
type ShapeInfo struct {
	// Framework is the canonical framework name ("promptfoo" /
	// "deepeval" / "ragas").
	Framework string `json:"framework"`

	// Version is the detected major-version family — "v3" / "v4"
	// for Promptfoo, "1.x" for DeepEval, "modern" / "legacy" for
	// Ragas. Empty when the version field is absent.
	Version string `json:"version,omitempty"`

	// VersionSource describes where the version label came from:
	//   "field"        — explicit version field in the payload
	//   "shape"        — inferred from the envelope structure
	//   "absent"       — no version signal at all (unknown shape)
	VersionSource string `json:"versionSource,omitempty"`

	// Warnings is the list of drift / unfamiliar-shape warnings
	// the adapter produced. Each warning is a single human-readable
	// sentence with a stable prefix so downstream tooling can grep
	// for it. Empty on a clean parse.
	Warnings []string `json:"warnings,omitempty"`
}

// HasWarnings reports whether the parse surfaced any drift signals.
// Used by the pipeline to log a single per-run notice rather than
// per-case noise.
func (s ShapeInfo) HasWarnings() bool {
	return len(s.Warnings) > 0
}

// FormatWarnings returns the warnings joined with semicolons,
// suitable for a one-line log entry. Stable order — appended in
// detection order, not sorted, so adopters see the first issue
// the adapter hit.
func (s ShapeInfo) FormatWarnings() string {
	return strings.Join(s.Warnings, "; ")
}

// DetectPromptfooShape inspects a Promptfoo eval-output payload and
// returns the detected (Version, Warnings) without doing a full
// parse. Used by the adapter wrapper so callers can log a single
// "running with possibly unfamiliar shape" notice before the
// detector chain consumes the result.
//
// Detection rules — Promptfoo:
//   - v3 (current default): top-level `{ evalId, results: { results: [...] }, ... }`
//   - v4+ (newer): top-level `{ evalId, results: [...], ... }`
//   - missing `evalId` is suspicious but not fatal
//   - missing both `results.results` and a top-level `results` array
//     is a hard drift — adapter will fail to parse, but ShapeInfo
//     surfaces the reason early.
func DetectPromptfooShape(data []byte) ShapeInfo {
	info := ShapeInfo{Framework: "promptfoo"}
	if len(data) == 0 {
		info.Warnings = append(info.Warnings, "shape: empty payload")
		return info
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		info.Warnings = append(info.Warnings,
			fmt.Sprintf("shape: top-level is not a JSON object (%v)", err))
		return info
	}

	if _, ok := probe["evalId"]; !ok {
		info.Warnings = append(info.Warnings,
			"shape: missing evalId field — Promptfoo runs typically include this")
	}

	results, hasResults := probe["results"]
	if !hasResults {
		info.Warnings = append(info.Warnings,
			"shape: missing top-level results — neither v3 nested nor v4 flat shape detected")
		return info
	}

	// v4+ flat: results is an array.
	if firstByte(results) == '[' {
		info.Version = "v4"
		info.VersionSource = "shape"
		return info
	}

	// v3 nested: results is an object containing inner results array.
	if firstByte(results) == '{' {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(results, &inner); err == nil {
			if _, ok := inner["results"]; ok {
				info.Version = "v3"
				info.VersionSource = "shape"
				return info
			}
		}
		info.Warnings = append(info.Warnings,
			"shape: results is an object but lacks an inner results array — unfamiliar v3 variant")
		return info
	}

	info.Warnings = append(info.Warnings,
		"shape: results field is neither an array nor an object — unrecognized shape")
	return info
}

// DetectDeepEvalShape inspects a DeepEval `--export` JSON payload.
//
// Detection rules — DeepEval 1.x:
//   - top-level `{ testCases: [...] }` or `[ ... ]` (some versions
//     dump the array directly)
//   - missing `testCases` and not an array is hard drift
func DetectDeepEvalShape(data []byte) ShapeInfo {
	info := ShapeInfo{Framework: "deepeval"}
	if len(data) == 0 {
		info.Warnings = append(info.Warnings, "shape: empty payload")
		return info
	}

	switch firstByte(data) {
	case '{':
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(data, &probe); err != nil {
			info.Warnings = append(info.Warnings,
				fmt.Sprintf("shape: top-level object failed to parse (%v)", err))
			return info
		}
		if _, ok := probe["testCases"]; ok {
			info.Version = "1.x"
			info.VersionSource = "shape"
			return info
		}
		if _, ok := probe["test_cases"]; ok {
			info.Version = "1.x"
			info.VersionSource = "shape"
			info.Warnings = append(info.Warnings,
				"shape: testCases field uses snake_case (test_cases) — older 1.x export shape")
			return info
		}
		info.Warnings = append(info.Warnings,
			"shape: object payload missing testCases — unrecognized DeepEval shape")
	case '[':
		info.Version = "1.x"
		info.VersionSource = "shape"
		info.Warnings = append(info.Warnings,
			"shape: payload is a bare array — older DeepEval 1.x dump shape, expecting { testCases: [...] }")
	default:
		info.Warnings = append(info.Warnings,
			"shape: top-level is neither object nor array")
	}
	return info
}

// DetectRagasShape inspects a Ragas eval-output payload.
//
// Detection rules — Ragas:
//   - "modern" (>= 0.1): top-level `{ samples: [...], scores: {...} }`
//   - "legacy" (< 0.1): top-level array of per-question records
func DetectRagasShape(data []byte) ShapeInfo {
	info := ShapeInfo{Framework: "ragas"}
	if len(data) == 0 {
		info.Warnings = append(info.Warnings, "shape: empty payload")
		return info
	}

	switch firstByte(data) {
	case '{':
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(data, &probe); err != nil {
			info.Warnings = append(info.Warnings,
				fmt.Sprintf("shape: top-level object failed to parse (%v)", err))
			return info
		}
		_, hasSamples := probe["samples"]
		_, hasScores := probe["scores"]
		if hasSamples && hasScores {
			info.Version = "modern"
			info.VersionSource = "shape"
			return info
		}
		if hasSamples {
			info.Version = "modern"
			info.VersionSource = "shape"
			info.Warnings = append(info.Warnings,
				"shape: samples present but scores missing — partial modern Ragas shape")
			return info
		}
		info.Warnings = append(info.Warnings,
			"shape: object payload lacks samples — unrecognized Ragas shape")
	case '[':
		info.Version = "legacy"
		info.VersionSource = "shape"
	default:
		info.Warnings = append(info.Warnings,
			"shape: top-level is neither object nor array")
	}
	return info
}

// firstByte returns the first non-whitespace byte of the JSON
// payload. Used by shape detectors to decide between
// array-vs-object envelopes without needing a full parse.
func firstByte(data []byte) byte {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}
	return 0
}
