package changescope

import (
	"fmt"
	"sort"
	"strings"
)

// humanSummary maps a Terrain AI signal type to a one-sentence
// plain-language description suitable for a PR-comment audience. The
// detector taxonomy (`aiPromptInjectionRisk`, `aiToolWithoutSandbox`,
// etc.) is precise but unfamiliar; this map gives the PR author the
// "what does this mean for me?" answer the bare type lacks.
//
// Keep entries short — this becomes a single bullet line in the PR
// comment. The full detector explanation is still available via
// `terrain explain <type>` for readers who want depth.
var humanSummary = map[string]string{
	"aiPromptInjectionRisk":    "User input flows into a prompt without visible escaping or boundary tokens.",
	"aiToolWithoutSandbox":     "Destructive tool can run without an approval gate, sandbox, or dry-run mode.",
	"aiSafetyEvalMissing":      "AI surface ships without an eval scenario covering jailbreak / harm / leak.",
	"aiHardcodedAPIKey":        "API key embedded in source or config — should be in env / secret store.",
	"aiNonDeterministicEval":   "Eval config doesn't pin `temperature: 0` (or seed); CI runs become non-deterministic.",
	"aiModelDeprecationRisk":   "Model tag is sunset or floats — the next API call could break or silently re-resolve.",
	"aiCostRegression":         "Per-case cost rose vs baseline beyond the configured threshold.",
	"aiHallucinationRate":      "Hallucination-shaped failure rate exceeds the project threshold.",
	"aiRetrievalRegression":    "Retrieval-quality score (faithfulness / context_precision / nDCG) dropped vs baseline.",
	"aiEmbeddingModelChange":   "Embedding model referenced without a retrieval-shaped eval scenario — silent quality drift on swap.",
	"aiPromptVersioning":       "Prompt file has no version marker — content changes will silently drift past consumers.",
	"aiFewShotContamination":   "Few-shot example text overlaps verbatim with eval scenario inputs — inflates scores.",
	"uncoveredAISurface":       "AI surface (prompt / tool / retriever / model) has zero test or scenario coverage.",
	"capabilityValidationGap":  "Declared capability has no scenario validating it.",
	"phantomEvalScenario":      "Eval scenario references a surface that doesn't exist.",
	"untestedPromptFlow":       "Prompt invocation path has no covering test or scenario.",
}

// humanAction maps a signal type to a one-sentence concrete next-step
// suggestion. Aimed at "what should the PR author do, today?" — not
// the long-form remediation in `docs/rules/<type>.md`.
var humanAction = map[string]string{
	"aiPromptInjectionRisk":    "Wrap user input through a sanitizer, or use a prompt template with explicit user-content boundaries.",
	"aiToolWithoutSandbox":     "Add `requires_approval: true`, route through a sandbox, or restrict to dry-run.",
	"aiSafetyEvalMissing":      "Add an eval scenario tagged `category: safety` covering this surface.",
	"aiHardcodedAPIKey":        "Move the secret to an env var (or your secrets manager) and reference it from there.",
	"aiNonDeterministicEval":   "Pin `temperature: 0` and a seed in the eval config.",
	"aiModelDeprecationRisk":   "Pin to a dated model variant (e.g. `gpt-4-0613`) or upgrade to a current tier.",
	"aiCostRegression":         "Investigate the prompt or model change for unintended bloat. Bump the baseline if intentional.",
	"aiHallucinationRate":      "Tighten retrieval / grounding before merging; bump the threshold only with documented justification.",
	"aiRetrievalRegression":    "Investigate the regression — revert the offending change or re-tune retrieval before merging.",
	"aiEmbeddingModelChange":   "Add a retrieval-shaped eval scenario (Ragas / Promptfoo / DeepEval) so future swaps surface as quality regressions.",
	"aiPromptVersioning":       "Add a `version:` field, a `_v<N>` filename suffix, or a `# version: ...` comment.",
	"aiFewShotContamination":   "Hold the matching examples out of the few-shot block, or rewrite the eval input.",
	"uncoveredAISurface":       "Add an eval scenario or test that exercises this surface.",
	"capabilityValidationGap":  "Add a scenario validating the declared capability, or remove the declaration.",
	"phantomEvalScenario":      "Fix the surface ID reference, or remove the orphan scenario.",
	"untestedPromptFlow":       "Add a test or scenario that hits this prompt invocation path.",
}

// fileLine returns "file:line" if Line > 0, otherwise just "file".
func fileLine(s AISignalSummary) string {
	if s.Line > 0 {
		return fmt.Sprintf("%s:%d", s.File, s.Line)
	}
	return s.File
}

// groupedSignal aggregates AISignalSummary entries that share a file
// and signal type so the renderer can output one bullet per
// (file, type) instead of N identical lines.
type groupedSignal struct {
	File        string
	Type        string
	Severity    string // worst severity in the group (for sorting)
	Lines       []int  // unique line numbers, sorted ascending
	Symbols     []string
	Explanation string // first non-empty (they're all the same shape)
}

// groupSignalsByFileAndType is the core of the "don't dump 25 identical
// lines" presentation fix. Two signals that share both file and type
// are aggregated into one bullet whose Lines slice carries every
// distinct line number. Symbols come along for the ride for tool-style
// findings where the line is 0 but the symbol identifies the offender.
func groupSignalsByFileAndType(signals []AISignalSummary) []groupedSignal {
	type key struct{ file, sigType string }
	idx := map[key]*groupedSignal{}
	var keys []key
	for _, s := range signals {
		k := key{s.File, s.Type}
		g, ok := idx[k]
		if !ok {
			g = &groupedSignal{
				File: s.File, Type: s.Type, Severity: s.Severity,
				Explanation: s.Explanation,
			}
			idx[k] = g
			keys = append(keys, k)
		}
		// Track the worst severity seen for sort priority.
		if severityRank(s.Severity) > severityRank(g.Severity) {
			g.Severity = s.Severity
		}
		if s.Line > 0 && !containsInt(g.Lines, s.Line) {
			g.Lines = append(g.Lines, s.Line)
		}
		if s.Symbol != "" && !containsString(g.Symbols, s.Symbol) {
			g.Symbols = append(g.Symbols, s.Symbol)
		}
	}
	out := make([]groupedSignal, 0, len(keys))
	for _, k := range keys {
		g := idx[k]
		sort.Ints(g.Lines)
		sort.Strings(g.Symbols)
		out = append(out, *g)
	}
	// Sort bullets: highest severity first, then file path, then type.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func containsInt(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// renderGroupedSignal produces the user-facing bullet for one
// (file, type) group. Format:
//
//	**`path/to/file.go:42, 47, 51`** — <plain-language summary>
//	  → <concrete next step>
//
// When Lines is empty (e.g., tool-config findings keyed by symbol),
// the symbol list takes its place; when both are empty, the file
// alone carries the locator.
func renderGroupedSignal(g groupedSignal) []string {
	summary := humanSummary[g.Type]
	if summary == "" {
		summary = g.Explanation // fall back to detector text
	}
	action := humanAction[g.Type]

	loc := g.File
	switch {
	case len(g.Lines) > 0:
		// `path:42, 47, 51` for multi-line; `path:42` for single.
		strs := make([]string, len(g.Lines))
		for i, ln := range g.Lines {
			strs[i] = fmt.Sprintf("%d", ln)
		}
		loc = fmt.Sprintf("%s:%s", g.File, strings.Join(strs, ", "))
	case len(g.Symbols) > 0:
		loc = fmt.Sprintf("%s (%s)", g.File, strings.Join(g.Symbols, ", "))
	}

	header := fmt.Sprintf("- **`%s`** — %s", loc, summary)
	out := []string{header}
	if action != "" {
		out = append(out, fmt.Sprintf("  → %s", action))
	}
	return out
}
