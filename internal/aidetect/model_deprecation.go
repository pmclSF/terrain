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

// modelDeprecationList is the curated registry of model identifiers that
// either refer to a deprecated/sunset model OR are floating tags whose
// resolution silently changes over time. Each entry carries:
//
//   - the matched literal (case-insensitive)
//   - a category: "deprecated" or "floating"
//   - a one-line explanation surfaced in the signal
//
// The list is hand-curated and intentionally conservative. We expand it
// as the calibration corpus grows. False positives are tracked under
// `expectedAbsent: aiModelDeprecationRisk` in the corpus.
var modelDeprecationList = []deprecationRule{
	// Floating / undated tags.
	{Match: "gpt-4", Category: "floating", Explanation: "model tag `gpt-4` resolves to whatever the provider currently maps it to; pin a dated variant (e.g. gpt-4-0613)"},
	{Match: "gpt-3.5-turbo", Category: "floating", Explanation: "model tag `gpt-3.5-turbo` is a moving alias; pin a dated variant"},
	{Match: "claude-3-opus", Category: "floating", Explanation: "model tag `claude-3-opus` floats across provider releases; pin claude-3-opus-YYYYMMDD"},
	{Match: "claude-3-sonnet", Category: "floating", Explanation: "model tag `claude-3-sonnet` floats; pin a dated variant"},
	{Match: "claude-3-haiku", Category: "floating", Explanation: "model tag `claude-3-haiku` floats; pin a dated variant"},

	// Sunset / deprecated lineage.
	{Match: "text-davinci-003", Category: "deprecated", Explanation: "OpenAI text-davinci-003 reached EOL in early 2024; switch to gpt-4-* or gpt-3.5-turbo-*"},
	{Match: "text-davinci-002", Category: "deprecated", Explanation: "OpenAI text-davinci-002 is sunset; switch to a current chat model"},
	// code-davinci lineage: pre-0.2.x had a bare `code-davinci` rule, but
	// the trailing boundary class excludes `-`, so neither `code-davinci-001`
	// nor `code-davinci-002` (the actual identifiers in the wild) matched.
	// Enumerate the dated variants explicitly. The bare `code-davinci`
	// stays for the exact-string case.
	{Match: "code-davinci", Category: "deprecated", Explanation: "OpenAI code-davinci-* is sunset; use gpt-4 with code prompts"},
	{Match: "code-davinci-001", Category: "deprecated", Explanation: "OpenAI code-davinci-001 is sunset (Codex deprecation, 2023-03); use gpt-4 with code prompts"},
	{Match: "code-davinci-002", Category: "deprecated", Explanation: "OpenAI code-davinci-002 is sunset (Codex deprecation, 2023-03); use gpt-4 with code prompts"},
	{Match: "code-davinci-edit-001", Category: "deprecated", Explanation: "OpenAI code-davinci-edit-001 is sunset; the edits API itself was deprecated in 2024"},
	{Match: "code-cushman-001", Category: "deprecated", Explanation: "OpenAI code-cushman-001 is sunset (Codex deprecation, 2023-03); use gpt-3.5-turbo or gpt-4"},
	{Match: "claude-2", Category: "deprecated", Explanation: "Anthropic claude-2 lineage is being sunset; migrate to claude-3.x"},
	{Match: "claude-1", Category: "deprecated", Explanation: "Anthropic claude-1 is sunset"},
}

type deprecationRule struct {
	Match       string
	Category    string
	Explanation string
}

// modelMatchPatterns are precompiled boundary-anchored regexes for the
// deprecation list. Built once on package init.
//
// The trailing `(?:[^-0-9A-Za-z_]|$)` is the dated-variant guard: we
// match the literal tag only when the next character ends the token
// (whitespace / quote / punctuation / EOL). A real-world dated variant
// like `gpt-4-0613` has `-0` after `gpt-4`, which fails the guard, so
// it does NOT match the bare `gpt-4` rule. RE2 doesn't support
// lookaround, so the guard consumes the trailing character — which is
// fine because the only consumer (FindString) just checks for any
// non-empty match.
var modelMatchPatterns = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(modelDeprecationList))
	for _, r := range modelDeprecationList {
		// Trailing boundary excludes `.` so dot-versioned variants like
		// `claude-2.1` and `gpt-3.5-turbo-0125` aren't matched by their
		// undated parent (`claude-2`, `gpt-3.5-turbo`). Without this,
		// pinning to a current dated model fires the deprecation
		// detector — guaranteed false positive on any 2024+ model that
		// happens to share a prefix with a deprecated tag.
		anchor := `\b` + regexp.QuoteMeta(r.Match) + `(?:[^-.0-9A-Za-z_]|$)`
		out = append(out, regexp.MustCompile(`(?i)`+anchor))
	}
	return out
}()

// modelScanExts narrows the file scan to text formats where model
// identifiers typically live: configs and source files.
var modelScanExts = map[string]bool{
	".yaml": true, ".yml": true, ".json": true, ".toml": true,
	".env": true, ".ini": true, ".cfg": true,
	".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".go": true, ".java": true, ".rb": true, ".rs": true,
}

// ModelDeprecationDetector flags references to deprecated or floating
// model tags in repository config and source files. Lives in the AI
// domain because the consequence is "your eval / agent silently drifts
// when the provider remaps the tag".
type ModelDeprecationDetector struct {
	// Root is the absolute path of the repo. Snapshot paths are
	// repo-relative.
	Root string
}

// Detect emits SignalAIModelDeprecationRisk for each (file, line) where
// a deprecated or floating tag appears. One signal per line; multiple
// matches on the same line are deduplicated.
func (d *ModelDeprecationDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}
	paths := d.gatherScanPaths(snap)

	var out []models.Signal
	for _, relPath := range paths {
		abs := filepath.Join(d.Root, relPath)
		hits := scanFileForModelTags(abs)
		for _, h := range hits {
			// 0.2.0 final-polish: severity now tracks the category.
			// "deprecated" tags (text-davinci-003, code-davinci-002,
			// claude-1) are sunset and the next API call WILL break;
			// these are High. "floating" tags (gpt-4, claude-3-opus)
			// merely drift over time as the provider remaps the alias;
			// these stay Medium. Pre-fix every category was Medium,
			// which under-prioritized the genuinely-broken cases.
			severity := models.SeverityMedium
			if h.Rule.Category == "deprecated" {
				severity = models.SeverityHigh
			}
			out = append(out, models.Signal{
				Type:        signals.SignalAIModelDeprecationRisk,
				Category:    models.CategoryAI,
				Severity:    severity,
				Confidence:  0.88,
				Location:    models.SignalLocation{File: relPath, Line: h.Line},
				Explanation: h.Rule.Explanation,
				SuggestedAction: "Pin to a dated model variant or upgrade to a supported tier.",

				SeverityClauses: []string{"sev-medium-005"},
				Actionability:   models.ActionabilityScheduled,
				LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageMaintenance},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-106",
				RuleURI:         "docs/rules/ai/model-deprecation-risk.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.88,
					IntervalLow:  0.78,
					IntervalHigh: 0.94,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceModerate,
				Metadata: map[string]any{
					"category": h.Rule.Category,
					"match":    h.Rule.Match,
				},
			})
		}
	}
	return out
}

// gatherScanPaths returns files to scan. Combines snapshot files with
// a repo walk so model identifiers in non-test source still get
// flagged. The extension filter is applied to both sources.
func (d *ModelDeprecationDetector) gatherScanPaths(snap *models.TestSuiteSnapshot) []string {
	fromSnap := snapshotPaths(snap)
	fromWalk := walkRepoForConfigs(d.Root, scanOpts{
		extensions: modelScanExts,
	})
	merged := uniquePaths(fromSnap, fromWalk)

	var out []string
	for _, p := range merged {
		if modelScanExts[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
	}
	return out
}

// modelHit is one match in one file.
type modelHit struct {
	Line int
	Rule deprecationRule
}

// scanFileForModelTags streams the file and emits modelHit per matched
// pattern, deduplicating multiple hits on the same line for the same
// rule. Files that fail to open are silently skipped.
func scanFileForModelTags(path string) []modelHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hits []modelHit
	sc := bufio.NewScanner(f)
	const maxLine = 1 << 20
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, maxLine)

	type lineRule struct {
		line  int
		match string
	}
	emitted := map[lineRule]bool{}
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		// Skip comment-only lines in source — our patterns can hit
		// changelog entries documenting deprecations.
		if commentLooksLikeChangeLog(text) {
			continue
		}
		for i, rx := range modelMatchPatterns {
			if !rx.MatchString(text) {
				continue
			}
			rule := modelDeprecationList[i]
			key := lineRule{line: line, match: rule.Match}
			if emitted[key] {
				continue
			}
			emitted[key] = true
			hits = append(hits, modelHit{Line: line, Rule: rule})
		}
	}
	return hits
}

// commentLooksLikeChangeLog returns true if a line is overwhelmingly
// likely to be a changelog or docs comment about a deprecation, where
// the whole point is to mention the deprecated tag — flagging that as
// a finding would be inverted.
//
// Comment-prefix coverage: pre-0.2.x this only recognized `#`, `//`,
// and `*` (block-comment continuation), missing the styles used by SQL
// (`--`), Lua/Haskell (`--`), config (`;`), shell-doc (`#:`), Lisp
// (`;;`), HTML/Markdown (`<!--`), reStructuredText (`..`), and
// markdown bullet/header lines that prose-document deprecations
// (`-`, `*`, `1.`, `>`, `#`). Source files quoting deprecated model
// names inside CHANGELOG-shaped lines were producing false positives.
func commentLooksLikeChangeLog(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	hasCommentPrefix := false
	for _, prefix := range commentLikePrefixes {
		if strings.HasPrefix(t, prefix) {
			hasCommentPrefix = true
			break
		}
	}
	if !hasCommentPrefix {
		return false
	}
	low := strings.ToLower(t)
	for _, marker := range []string{"deprecat", "sunset", "removed", "eol", "changelog", "switch to", "migrate"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

// commentLikePrefixes is the union of comment / prose-line markers we
// treat as "this line is documentation, not source." Order doesn't
// matter — we test all of them with HasPrefix. Multi-character prefixes
// (`<!--`, `--`, `;;`) intentionally precede their single-character
// substrings in the slice so that future readers see them grouped, but
// HasPrefix is order-independent.
var commentLikePrefixes = []string{
	"<!--", // HTML / Markdown
	"-->",  // closing marker, occasionally on own line
	"//",   // C / Go / JS
	"/*",   // C / Java block-comment open
	"*/",   // close
	"--",   // SQL / Lua / Haskell
	";;",   // Lisp double semicolon
	";",    // INI / Lisp
	"#",    // Python / Ruby / Shell / YAML / Markdown header
	"%",    // Erlang / Prolog / TeX
	".. ",  // reStructuredText comment marker
	"> ",   // Markdown blockquote (often used in CHANGELOG snippets)
	"* ",   // block-comment continuation OR markdown bullet
	"- ",   // markdown bullet
	"+ ",   // markdown bullet (alt)
	"' ",   // VB / older BASIC dialects (require trailing space to avoid Python single-quoted strings at column 0)
}
