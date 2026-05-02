package aidetect

import (
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
//
// Pre-0.2.x the assignment branch matched `[+]?=`, which also matched
// `==` (equality) — `if prompt == user_input:` tripped a
// High-severity false positive. The branch now uses negative lookahead
// `=(?!=)` so equality (`==`, `===`), `!==`, `>=`, `<=` are excluded;
// `+=` and assignment (`=`) are retained.
var promptIdentifierPattern = regexp.MustCompile(
	`(?i)\b(?:system_?prompt|user_?prompt|prompt|instruction|message[s]?)\s*(?:\+=|=(?:[^=]|$)|\.append\(|\.format\()`,
)

// userInputShapes is the "this looks user-controlled" half. Each entry
// is a regex tested against the same line OR the next 1–2 lines (see
// scanFileForPromptInjection). The 0.2.0 final-polish pass added
// FastAPI / Flask / Django / Pyramid / gRPC shapes that the original
// list missed — production codebases routinely route user input
// through these framework constructs, so a list anchored on
// `request.body`/`req.json` only saw a small slice of real-world
// prompt-injection patterns.
var userInputShapes = []*regexp.Regexp{
	// Express.js / Koa / generic Node web frameworks.
	regexp.MustCompile(`\brequest\.(?:body|query|params|json|args|form|files|cookies|headers)\b`),
	regexp.MustCompile(`\breq\.(?:body|query|params|json|form|files|cookies|headers)\b`),
	// FastAPI typed-parameter constructs (`= Body(...)`, `= Query(...)`,
	// `= Form(...)`, `= File(...)`, `= Header(...)`, `= Cookie(...)`).
	regexp.MustCompile(`=\s*(?:Body|Query|Form|File|Header|Cookie|Path)\s*\(`),
	// Flask / Pyramid / Django request shapes.
	regexp.MustCompile(`\brequest\.(?:GET|POST|FILES|COOKIES|META|json|values|form)\b`),
	// gRPC: `request.<field>` is too generic, but explicit `request.message`
	// and `request.payload` are the common shapes.
	regexp.MustCompile(`\brequest\.(?:message|payload|prompt|input|query|content)\b`),
	// Generic identifier shapes that consistently denote user content.
	regexp.MustCompile(`(?i)\buser_?input\b`),
	regexp.MustCompile(`(?i)\bprompt_?input\b`),
	regexp.MustCompile(`\bargs\.(?:message|prompt|input|query)\b`),
	regexp.MustCompile(`\bparams\.(?:message|prompt|input|query)\b`),
	regexp.MustCompile(`\binput\(\s*\)`),                  // python input()
	regexp.MustCompile(`\bos\.environ\["?USER_INPUT"?\]`), // env-driven user input
	regexp.MustCompile(`\bsys\.(?:stdin|argv)\b`),         // CLI-arg-driven user input
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
	fromSnap := snapshotPaths(snap)
	fromWalk := walkRepoForConfigs(d.Root, scanOpts{
		extensions: promptInjectionScanExts,
	})
	merged := uniquePaths(fromSnap, fromWalk)

	var out []string
	for _, p := range merged {
		if promptInjectionScanExts[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
	}
	return out
}

type injectionHit struct {
	Line        int
	Explanation string
}

func scanFileForPromptInjection(path string) []injectionHit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil
	}

	// 0.2.0 final-polish: real-world code routinely splits prompt
	// concatenation across multiple lines (`prompt += \n  user.input`),
	// because Black / Prettier wrap long expressions. The pre-fix
	// scanner only saw the prompt-write line, missed the user-input
	// line, and emitted zero findings on the most common shape.
	//
	// New approach: when the prompt-identifier pattern matches a line,
	// the user-input scan looks at that line PLUS the next 2 lines
	// (the typical wrap window). Same-line matches are still preferred
	// for the explanation; multi-line matches carry a slightly weaker
	// confidence in their explanation text.
	var hits []injectionHit
	for i, text := range lines {
		// Skip comment-only lines.
		if isCommentLine(text) {
			continue
		}
		// Pass 1: prompt-shape with user-input on same line OR within
		// the next 2 lines.
		if promptIdentifierPattern.MatchString(text) {
			window := text
			for j := 1; j <= 2 && i+j < len(lines); j++ {
				if isCommentLine(lines[i+j]) {
					break
				}
				window += "\n" + lines[i+j]
			}
			if hasUserInputShape(window) {
				explanation := "User-controlled input concatenated into a prompt-shaped variable without visible sanitisation."
				if !hasUserInputShape(text) {
					explanation = "Prompt-shaped variable on this line is followed by user-controlled input on the next line(s); review concatenation for escape boundaries."
				}
				hits = append(hits, injectionHit{
					Line:        i + 1,
					Explanation: explanation,
				})
				continue
			}
		}
		// Pass 2: f-string / template literal interpolation pattern.
		if fStringPromptPattern.MatchString(text) {
			hits = append(hits, injectionHit{
				Line:        i + 1,
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
