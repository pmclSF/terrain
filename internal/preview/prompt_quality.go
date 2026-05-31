package preview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// CallSite is the minimum data the preview detectors need from an
// LLM SDK call site. The internal/aidetect package owns the richer
// AICallSite struct; this slim mirror keeps the preview package
// independent of aidetect so the detector adapters can sit in
// internal/aidetect without an import cycle.
type CallSite struct {
	Path   string
	Line   int
	SDK    string
	Method string
}

// promptBloatThreshold is the default token-count budget — 2000
// tokens roughly = 8000 chars (BPE 4 chars/token average). Tunable
// via terrain.yaml rules.prompt-quality/prompt-bloat.threshold.
const promptBloatCharThreshold = 8000

// DetectPromptBloat fires when a prompt-classified file exceeds the
// configured character budget. Implements terrain/prompt-quality/prompt-bloat.
func DetectPromptBloat(promptFiles []string, threshold int) []models.Signal {
	if threshold <= 0 {
		threshold = promptBloatCharThreshold
	}
	var out []models.Signal
	for _, path := range promptFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() < int64(threshold) {
			continue
		}
		out = append(out, signal(
			signals.SignalPromptBloat, models.SeverityLow,
			"terrain/prompt-quality/prompt-bloat",
			"docs/rules/prompt-quality/prompt-bloat.md",
			models.SignalLocation{File: path},
			fmt.Sprintf("Prompt file %s is %d bytes (budget %d).", path, info.Size(), threshold),
			"Trim few-shot examples or move the schema dump to a referenced file. Long prompts inflate per-call cost and latency.",
			map[string]any{"size_bytes": info.Size(), "threshold": threshold},
		))
	}
	return out
}

// DetectPromptWithoutTemperature walks the AST-resolved AI call sites
// and fires when an LLM SDK call has no explicit temperature value.
// Implements terrain/prompt-quality/prompt-without-temperature.
//
// Without temperature pinned, defaults differ across SDKs (OpenAI
// defaults to 1.0; Anthropic to a model-specific value), and the same
// eval can produce different scores depending on which SDK version
// shipped.
func DetectPromptWithoutTemperature(callSites []CallSite, sourcePaths map[string]string) []models.Signal {
	var out []models.Signal
	for _, cs := range callSites {
		// Heuristic: read the source file's line for a `temperature` token.
		// If the line carrying the call doesn't include "temperature", we
		// flag. This is a coarse approximation; precise AST detection
		// of the call's kwargs is followup work.
		path := cs.Path
		if mapped, ok := sourcePaths[path]; ok {
			path = mapped
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if cs.Line <= 0 || cs.Line > len(lines) {
			continue
		}
		// Inspect a window of ±5 lines around the call to handle
		// multi-line call argument layout.
		start := max0(cs.Line - 5)
		end := minN(cs.Line+5, len(lines))
		window := strings.Join(lines[start:end], "\n")
		if strings.Contains(window, "temperature") {
			continue
		}
		out = append(out, signal(
			signals.SignalPromptWithoutTemperature, models.SeverityLow,
			"terrain/prompt-quality/prompt-without-temperature",
			"docs/rules/prompt-quality/prompt-without-temperature.md",
			models.SignalLocation{File: cs.Path, Line: cs.Line},
			fmt.Sprintf("%s call to %s has no explicit temperature.", cs.SDK, cs.Method),
			"Pass temperature=0 (or your team's chosen value) explicitly so eval reproducibility doesn't depend on the SDK's default.",
			map[string]any{"sdk": cs.SDK, "method": cs.Method},
		))
	}
	return out
}

// DetectMissingPromptValidator fires when a Python source uses an LLM
// SDK but lacks a structured-output validator (instructor, guardrails,
// pydantic with response_model). Implements
// terrain/prompt-quality/missing-validator.
func DetectMissingPromptValidator(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		if !strings.HasSuffix(strings.ToLower(path), ".py") {
			continue
		}
		s := string(content)
		// Only fire when the file looks like it's making LLM calls.
		if !looksLikeLLMCallSite(s) {
			continue
		}
		// Skip when a validator is imported / used.
		if hasValidatorMarker(s) {
			continue
		}
		out = append(out, signal(
			signals.SignalMissingPromptValidator, models.SeverityMedium,
			"terrain/prompt-quality/missing-validator",
			"docs/rules/prompt-quality/missing-validator.md",
			models.SignalLocation{File: path},
			"LLM call without a structured-output validator (instructor, guardrails, pydantic response_model).",
			"Wrap the call with instructor.patch / Guardrails / pydantic response_model. The validator catches malformed model output before it propagates.",
			map[string]any{},
		))
	}
	return out
}

func looksLikeLLMCallSite(s string) bool {
	markers := []string{
		"chat.completions.create",
		"messages.create",
		".invoke(",
		"ChatCompletion.create",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func hasValidatorMarker(s string) bool {
	markers := []string{
		"import instructor",
		"from instructor",
		"instructor.patch",
		"from guardrails",
		"import guardrails",
		"response_model=",
		"from pydantic",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// DetectPromptVersionSkew fires when two prompt-classified files
// share substantial content but live under different paths —
// suggesting the prompt was forked rather than versioned.
// Implements terrain/prompt-quality/version-skew.
func DetectPromptVersionSkew(promptFiles []string) []models.Signal {
	type entry struct {
		path string
		body string
	}
	entries := make([]entry, 0, len(promptFiles))
	for _, p := range promptFiles {
		data, err := os.ReadFile(p)
		if err != nil || len(data) < 200 {
			continue
		}
		entries = append(entries, entry{path: p, body: string(data)})
	}

	var out []models.Signal
	seen := map[string]bool{}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if seen[entries[i].path] || seen[entries[j].path] {
				continue
			}
			if normalizedSimilar(entries[i].body, entries[j].body) {
				seen[entries[i].path] = true
				seen[entries[j].path] = true
				out = append(out, signal(
					signals.SignalPromptVersionSkew, models.SeverityMedium,
					"terrain/prompt-quality/version-skew",
					"docs/rules/prompt-quality/version-skew.md",
					models.SignalLocation{File: entries[i].path},
					fmt.Sprintf("Prompt %s shares substantial content with %s.", filepath.Base(entries[i].path), filepath.Base(entries[j].path)),
					"Pick one canonical path. Update consumers and delete the duplicate, or move to a shared template if both are intentionally exported.",
					map[string]any{"other_path": entries[j].path},
				))
			}
		}
	}
	return out
}

// normalizedSimilar returns true when the two prompt contents share
// substantial overlap after whitespace collapse. Heuristic — full
// edit-distance would be more accurate but slower.
func normalizedSimilar(a, b string) bool {
	na := strings.Join(strings.Fields(a), " ")
	nb := strings.Join(strings.Fields(b), " ")
	if len(na) < 200 || len(nb) < 200 {
		return false
	}
	// Cheap similarity: prefix of min length.
	prefix := minN(len(na), len(nb)) / 2
	if prefix > 500 {
		prefix = 500
	}
	return na[:prefix] == nb[:prefix]
}

func max0(x int) int {
	if x < 0 {
		return 0
	}
	return x
}

func minN(a, b int) int {
	if a < b {
		return a
	}
	return b
}
