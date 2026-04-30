package aidetect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// PromptInjectionDetector flags source patterns that concatenate
// user-controlled input into a prompt without obvious escaping or
// structured input boundaries. The 0.2 detector is regex-based and
// intentionally heuristic — the round-4 review confirmed taint-flow
// analysis is the right destination but lives in 0.3.
//
// Detection model:
//
//   - look for "prompt-shaped" identifiers (variables named prompt,
//     system_prompt, user_prompt, instruction, message)
//   - look for "user-input-shaped" identifiers (request.body, req.query,
//     params.*, args.*, input, user_input, prompt_input)
//   - flag when both appear in a string-formatting / concatenation
//     construct on the same line
//
// We accept some false positives in exchange for catching the visible
// fraction of the bug. Calibration corpus fixtures with
// `expectedAbsent: aiPromptInjectionRisk` capture the false-positive
// shapes worth filtering.
type PromptInjectionDetector struct {
	Root string
}

// promptInjectionScanExts is the language allowlist. The detector is
// pattern-based, so we keep it tight to the languages whose AI codebases
// are visible in the calibration corpus.
var promptInjectionScanExts = map[string]bool{
	".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".go": true,
}

// promptIdentifierPattern is the "this looks prompt-related" half. We
// require the identifier to be assigned, concatenated, or appended to
// — i.e. a write context. Reading a `prompt` var is fine.
var promptIdentifierPattern = regexp.MustCompile(
	`(?i)\b(?:system_?prompt|user_?prompt|prompt|instruction|message[s]?)\s*(?:[+]?=|\.append\(|\.format\()`,
)

// userInputShapes is the "this looks user-controlled" half. Each entry
// is a regex tested against the same line.
var userInputShapes = []*regexp.Regexp{
	regexp.MustCompile(`\brequest\.(?:body|query|params|json|args)\b`),
	regexp.MustCompile(`\breq\.(?:body|query|params|json)\b`),
	regexp.MustCompile(`(?i)\buser_?input\b`),
	regexp.MustCompile(`(?i)\bprompt_?input\b`),
	regexp.MustCompile(`\bargs\.(?:message|prompt|input|query)\b`),
	regexp.MustCompile(`\bparams\.(?:message|prompt|input|query)\b`),
	regexp.MustCompile(`\binput\(\s*\)`),                  // python input()
	regexp.MustCompile(`\bos\.environ\["?USER_INPUT"?\]`), // env-driven user input
}

// fStringPromptPattern catches Python f-string and JS template-literal
// shapes where user input is interpolated into prompt-shaped vars.
// These don't always have an obvious assignment on the same line, so
// they get their own pass.
var fStringPromptPattern = regexp.MustCompile(
	`(?i)(?:f["']|` + "`" + `)[^"'` + "`" + `]*(?:prompt|instruction|system|user)[^"'` + "`" + `]*\{[^}]*(?:input|request|req|args|params|user)[^}]*\}`,
)

// Detect emits SignalAIPromptInjectionRisk per matching line.
func (d *PromptInjectionDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	paths := d.gatherPaths(snap)

	var out []models.Signal
	for _, relPath := range paths {
		abs := filepath.Join(d.Root, relPath)
		hits := scanFileForPromptInjection(abs)
		for _, h := range hits {
			out = append(out, models.Signal{
				Type:        signals.SignalAIPromptInjectionRisk,
				Category:    models.CategoryAI,
				Severity:    models.SeverityHigh,
				Confidence:  0.7,
				Location:    models.SignalLocation{File: relPath, Line: h.Line},
				Explanation: h.Explanation,
				SuggestedAction: "Use a prompt template with explicit user-content boundaries, or run user input through a sanitiser before concatenation.",

				SeverityClauses: []string{"sev-high-003"},
				Actionability:   models.ActionabilityScheduled,
				LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageTestAuthoring},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-102",
				RuleURI:         "docs/rules/ai/prompt-injection-risk.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.7,
					IntervalLow:  0.55,
					IntervalHigh: 0.82,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceModerate,
			})
		}
	}
	return out
}

func (d *PromptInjectionDetector) gatherPaths(snap *models.TestSuiteSnapshot) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !promptInjectionScanExts[strings.ToLower(filepath.Ext(p))] {
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, tf := range snap.TestFiles {
		add(tf.Path)
	}
	for _, sc := range snap.Scenarios {
		add(sc.Path)
	}
	return out
}

type injectionHit struct {
	Line        int
	Explanation string
}

func scanFileForPromptInjection(path string) []injectionHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	const maxLine = 1 << 20
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, maxLine)

	var hits []injectionHit
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		// Skip comment-only lines: documenting an attack pattern in a
		// docstring shouldn't fire the detector.
		if isCommentLine(text) {
			continue
		}
		// Pass 1: prompt-shape with user-input on same line.
		if promptIdentifierPattern.MatchString(text) && hasUserInputShape(text) {
			hits = append(hits, injectionHit{
				Line:        line,
				Explanation: "User-controlled input concatenated into a prompt-shaped variable without visible sanitisation.",
			})
			continue
		}
		// Pass 2: f-string / template literal interpolation pattern.
		if fStringPromptPattern.MatchString(text) {
			hits = append(hits, injectionHit{
				Line:        line,
				Explanation: "Prompt-shaped string literal interpolates user-input-shaped variable; review escaping or boundary tokens.",
			})
			continue
		}
	}
	return hits
}

func hasUserInputShape(text string) bool {
	for _, rx := range userInputShapes {
		if rx.MatchString(text) {
			return true
		}
	}
	return false
}

func isCommentLine(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	switch {
	case strings.HasPrefix(t, "#"),
		strings.HasPrefix(t, "//"),
		strings.HasPrefix(t, "*"),
		strings.HasPrefix(t, `"""`),
		strings.HasPrefix(t, `'''`):
		return true
	}
	return false
}
