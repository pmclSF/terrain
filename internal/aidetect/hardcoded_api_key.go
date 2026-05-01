package aidetect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// aiAPIKeyPatterns are the regular expressions that identify hard-coded
// API keys. The list is provider-prefix-anchored where possible, falling
// back to a generic high-entropy long-string pattern only for the most
// common cases. Each pattern carries the provider name in a named capture
// group so reports can attribute the find precisely.
//
// The regexes deliberately match the **prefix shape** rather than the
// exact char count for each provider, since providers occasionally shift
// length. False positives are caught by tests/calibration/ fixtures
// labelled `expectedAbsent: aiHardcodedAPIKey` (e.g. literal placeholders
// like `sk-fake-key`).
var aiAPIKeyPatterns = []apiKeyRule{
	{
		Name:    "openai",
		Pattern: regexp.MustCompile(`\bsk-(?:proj-|live-|test-)?[A-Za-z0-9_-]{20,}`),
	},
	{
		Name:    "anthropic",
		Pattern: regexp.MustCompile(`\bsk-ant-[a-z0-9_-]{20,}`),
	},
	{
		Name:    "google",
		Pattern: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),
	},
	{
		Name:    "aws",
		Pattern: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	},
	{
		Name:    "github",
		Pattern: regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b`),
	},
	{
		Name:    "huggingface",
		Pattern: regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`),
	},
	{
		Name:    "slack",
		Pattern: regexp.MustCompile(`\bxox[abps]-[0-9A-Za-z-]{10,}\b`),
	},
	{
		Name:    "stripe",
		Pattern: regexp.MustCompile(`\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{20,}\b`),
	},
}

type apiKeyRule struct {
	Name    string
	Pattern *regexp.Regexp
}

// placeholderMarkers are substrings that, when present in a candidate
// match, downgrade it to "obvious placeholder" and skip emission. This
// keeps documentation and test fixtures from tripping the detector.
//
// Each marker is a phrase a human would deliberately write; we don't
// add common digit runs like "1234567" because real (random) keys can
// legitimately contain them.
var placeholderMarkers = []string{
	"fake", "placeholder", "example", "dummy", "test-",
	"redacted", "your-key-here", "your_key_here",
	"xxxxx", "00000",
}

// configFileExts is the allowlist of file extensions the detector
// scans. Keeping the surface narrow avoids the cost of regex-walking
// every text file in a repo; AI evals/configs live in a small set.
var configFileExts = map[string]bool{
	".yaml": true,
	".yml":  true,
	".json": true,
	".env":  true,
	".toml": true,
	".ini":  true,
	".cfg":  true,
}

// HardcodedAPIKeyDetector identifies API keys embedded in AI configuration
// files (eval configs, agent definitions, prompt YAMLs).
//
// Detection is regex-driven on a hand-curated list of provider prefixes;
// see aiAPIKeyPatterns. Matches that contain placeholder-shaped tokens
// (`fake`, `example`, etc.) are dropped to keep false positives down.
//
// The detector emits SignalAIHardcodedAPIKey with severity Critical and
// SeverityClauses citing sev-critical-001 from docs/severity-rubric.md.
type HardcodedAPIKeyDetector struct {
	// Root is the absolute path of the repo being analysed. The
	// detector reads files under this root; the snapshot only carries
	// relative paths.
	Root string
}

// Detect scans configured AI/eval config files for hard-coded API keys.
// Files outside Root, or with extensions not in configFileExts, are
// ignored. Each finding becomes one Signal at file granularity.
func (d *HardcodedAPIKeyDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	if d == nil || snap == nil {
		return nil
	}

	candidatePaths := d.gatherConfigPaths(snap)
	var out []models.Signal
	for _, relPath := range candidatePaths {
		abs := filepath.Join(d.Root, relPath)
		hits := scanFileForAPIKeys(abs)
		for _, h := range hits {
			out = append(out, models.Signal{
				Type:        signals.SignalAIHardcodedAPIKey,
				Category:    models.CategoryAI,
				Severity:    models.SeverityCritical,
				Confidence:  0.92,
				Location:    models.SignalLocation{File: relPath, Line: h.Line},
				Explanation: "Hard-coded " + h.Provider + " API key detected in configuration.",
				SuggestedAction: "Move the secret to an environment variable or secrets store and reference it through the runner's secret-resolution path.",

				// SignalV2 fields.
				SeverityClauses: []string{"sev-critical-001"},
				Actionability:   models.ActionabilityImmediate,
				LifecycleStages: []models.LifecycleStage{models.StageDesign, models.StageMaintenance},
				AIRelevance:     models.AIRelevanceHigh,
				RuleID:          "TER-AI-103",
				RuleURI:         "docs/rules/ai/hardcoded-api-key.md",
				DetectorVersion: "0.2.0",
				ConfidenceDetail: &models.ConfidenceDetail{
					Value:        0.92,
					IntervalLow:  0.85,
					IntervalHigh: 0.95,
					Quality:      "heuristic",
					Sources:      []models.EvidenceSource{models.SourceStructuralPattern},
				},
				EvidenceSource:   models.SourceStructuralPattern,
				EvidenceStrength: models.EvidenceStrong,
			})
		}
	}
	return out
}

// gatherConfigPaths returns every config-extension file we should scan.
// Combines two sources:
//
//   1. files already in the snapshot (TestFiles, Scenarios)
//   2. a fresh walk of d.Root for files matching the extension allowlist
//
// Source #2 is what catches eval YAMLs / agent JSONs that aren't tests
// per se and so don't appear in TestFiles. Without it, a repo with no
// JS/Go test runner would never have its eval configs scanned.
func (d *HardcodedAPIKeyDetector) gatherConfigPaths(snap *models.TestSuiteSnapshot) []string {
	fromSnap := snapshotPaths(snap)
	fromWalk := walkRepoForConfigs(d.Root, scanOpts{
		extensions: configFileExts,
	})
	merged := uniquePaths(fromSnap, fromWalk)

	out := make([]string, 0, len(merged))
	for _, p := range merged {
		if configFileExts[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
	}
	return out
}

// keyHit is one match in one file.
type keyHit struct {
	Provider string
	Line     int
}

// scanFileForAPIKeys streams the file and returns every API-key match
// that survives placeholder filtering. Returns no error: a file that
// can't be opened is silently skipped — gathering errors here would
// drown the user in I/O noise on partial checkouts and node_modules.
func scanFileForAPIKeys(path string) []keyHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hits []keyHit
	sc := bufio.NewScanner(f)
	// Allow long YAML lines (default scanner buffer is 64 KB).
	const maxLine = 1 << 20
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, maxLine)

	line := 0
	// Track which (line, provider) we've already emitted so a config
	// that lists `openai_key=... aws_key=...` on a single line emits
	// both findings — pre-0.2.x the per-line `break` after the first
	// match swallowed the second key.
	emitted := map[string]bool{}
	for sc.Scan() {
		line++
		text := sc.Text()
		for _, rule := range aiAPIKeyPatterns {
			match := rule.Pattern.FindString(text)
			if match == "" {
				continue
			}
			if isPlaceholder(match) {
				continue
			}
			key := fmt.Sprintf("%d:%s", line, rule.Name)
			if emitted[key] {
				continue
			}
			emitted[key] = true
			hits = append(hits, keyHit{Provider: rule.Name, Line: line})
		}
	}
	// Pre-0.2.x sc.Err() was never checked, so a single line longer
	// than 1 MB (minified YAML, embedded blob) would silently drop
	// the rest of the file — secret never detected. Surface scanner
	// errors as a degraded-coverage hit so the caller knows the file
	// wasn't fully read.
	if err := sc.Err(); err != nil {
		hits = append(hits, keyHit{Provider: "scan-error:" + err.Error(), Line: line})
	}
	return hits
}

// isPlaceholder is a cheap "is this a literal example, not a real key"
// check. Returns true when the match contains any placeholder marker
// substring or is composed almost entirely of repeated characters.
func isPlaceholder(match string) bool {
	low := strings.ToLower(match)
	for _, m := range placeholderMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	// Detect "all the same character / mostly zeros" patterns common
	// in docs (e.g. AKIAXXXXXXXXXXXXXXXX).
	if hasLowEntropy(match) {
		return true
	}
	return false
}

// hasLowEntropy returns true when the string is dominated by a single
// repeated character (e.g. "AKIAXXXXXXXXXXXXXXXX"). Real keys are
// pseudo-random and never look like this.
func hasLowEntropy(s string) bool {
	if len(s) < 12 {
		return false
	}
	counts := map[byte]int{}
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	for _, c := range counts {
		if c*2 > len(s) {
			return true
		}
	}
	return false
}
