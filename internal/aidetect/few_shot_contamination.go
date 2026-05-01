package aidetect

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// FewShotContaminationDetector flags suspected few-shot contamination —
// the case where examples baked into a prompt file overlap verbatim
// with the inputs of eval scenarios that exercise that prompt.
// Contamination inflates eval scores because the model has effectively
// memorised its own test set.
//
// 0.2 ships a narrow heuristic check: for each prompt surface, walk
// the scenarios that cover it and look for chunks of the scenario's
// input text that appear verbatim in the prompt file. The detector is
// marked experimental in the manifest because it's bound to under-
// detect (paraphrased examples won't match) and to over-detect on
// short inputs.
//
// More precise variants (token-level n-gram overlap, semantic
// similarity scores, cross-suite leakage detection) land in 0.3 with
// the calibration corpus calibrating the threshold.
type FewShotContaminationDetector struct {
	// Root is the absolute path of the repo. Snapshot paths are
	// repo-relative.
	Root string

	// MinChunkLen is the minimum length (in characters) of a verbatim
	// substring that counts as contamination. Defaults to 40 — short
	// enough to catch a real example, long enough to avoid matching
	// stop-word fragments.
	MinChunkLen int
}

// promptVersionableExtensions defines the prompt-file extensions we
// scan. Same set as PromptVersioningDetector to keep the universe
// tight.
var fewShotPromptExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".json": true,
	".md": true, ".prompt": true, ".tmpl": true,
	".hbs": true, ".j2": true, ".mustache": true, ".txt": true,
}

// Detect emits SignalAIFewShotContamination per (prompt, scenario)
// pair where contamination is heuristically detected.
func (d *FewShotContaminationDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	threshold := d.MinChunkLen
	if threshold <= 0 {
		threshold = 40
	}

	// Index: prompt surface ID → file content (lowercased for
	// case-insensitive substring matching).
	promptContent := map[string]string{}
	promptPath := map[string]string{}
	for _, surface := range snap.CodeSurfaces {
		if surface.Kind != models.SurfacePrompt {
			continue
		}
		ext := strings.ToLower(filepath.Ext(surface.Path))
		if !fewShotPromptExtensions[ext] {
			continue
		}
		abs := filepath.Join(d.Root, surface.Path)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		promptContent[surface.SurfaceID] = strings.ToLower(string(data))
		promptPath[surface.SurfaceID] = surface.Path
	}
	if len(promptContent) == 0 {
		return nil
	}

	// For each scenario, see if any of its descriptions / steps
	// match a prompt's content. The scenario's Description and
	// Steps are the natural candidates for "this is the test input".
	var out []models.Signal
	emitted := map[string]bool{}
	for _, sc := range snap.Scenarios {
		// Build candidate input strings from the scenario.
		var candidates []string
		if s := strings.TrimSpace(sc.Description); s != "" {
			candidates = append(candidates, s)
		}
		for _, step := range sc.Steps {
			if s := strings.TrimSpace(step); s != "" {
				candidates = append(candidates, s)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		for _, surfaceID := range sc.CoveredSurfaceIDs {
			content, ok := promptContent[surfaceID]
			if !ok {
				continue
			}
			match, matchedCandidate := findContaminationOverlap(content, candidates, threshold)
			if !match {
				continue
			}
			emitKey := sc.ScenarioID + "/" + surfaceID
			if emitted[emitKey] {
				continue
			}
			emitted[emitKey] = true

			out = append(out, models.Signal{
				Type:        signals.SignalAIFewShotContamination,
				Category:    models.CategoryAI,
				Severity:    models.SeverityMedium,
				Confidence:  0.7,
				Location:    models.SignalLocation{File: promptPath[surfaceID], ScenarioID: sc.ScenarioID, Symbol: sc.Name},
				Explanation: "Scenario `" + sc.Name + "` contains text that appears verbatim in prompt `" + promptPath[surfaceID] + "`. Few-shot examples that overlap with the eval test set inflate scores.",
				SuggestedAction: "Hold the matching examples out of the prompt's few-shot block, or rewrite the eval input so it isn't a copy of an example. Re-run the eval after de-duplication.",

				SeverityClauses: []string{"sev-medium-009"},
				Actionability:   models.ActionabilityScheduled,
				LifecycleStages: []models.LifecycleStage{models.StageTestAuthoring, models.StageMaintenance},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-109",
				RuleURI:         "docs/rules/ai/few-shot-contamination.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.7,
					IntervalLow:  0.55,
					IntervalHigh: 0.83,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceModerate,
				Metadata: map[string]any{
					"surfaceId":         surfaceID,
					"scenarioId":        sc.ScenarioID,
					"matchedExcerpt":    truncateExcerpt(matchedCandidate, 80),
					"thresholdChars":    threshold,
				},
			})
		}
	}
	return out
}

// findContaminationOverlap returns (true, candidate) when any
// candidate string of length >= threshold appears verbatim (case-
// insensitive) inside content. The matched candidate is returned for
// reporting.
//
// Candidates shorter than threshold are skipped — short scenario
// descriptions like "happy path" would otherwise match every
// English-language prompt by accident.
func findContaminationOverlap(content string, candidates []string, threshold int) (bool, string) {
	for _, c := range candidates {
		if len(c) < threshold {
			continue
		}
		needle := strings.ToLower(c)
		if strings.Contains(content, needle) {
			return true, c
		}
	}
	return false, ""
}

func truncateExcerpt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
