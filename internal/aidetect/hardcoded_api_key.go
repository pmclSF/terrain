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
// length. Literal placeholders like `sk-fake-key` are suppressed by
// downstream filtering.
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
//
// Coverage includes real-world key-leak surfaces beyond the obvious
// YAML/JSON: .properties (Java configs), .tfvars (Terraform), .sh
// (env-export shell scripts), .config (.NET/generic), and
// .dockerfile/Dockerfile — polyglot AI infra repos commonly stash
// keys in these.
var configFileExts = map[string]bool{
	".yaml":       true,
	".yml":        true,
	".json":       true,
	".env":        true,
	".toml":       true,
	".ini":        true,
	".cfg":        true,
	".properties": true, // Java
	".tfvars":     true, // Terraform
	".sh":         true, // env-export shell scripts
	".config":     true, // .NET / generic
	".dockerfile": true, // explicit dockerfile extension
}

// isTestFixturePath returns true when the path looks like a
// test-mock / recorded-response fixture rather than real config.
// Used to suppress aiHardcodedAPIKey FPs on fixture libraries
// (placebo, vcrpy, cassettes, etc.) that contain mocked SDK
// responses with fake credentials by design.
func isTestFixturePath(relPath string) bool {
	lower := strings.ToLower(filepath.ToSlash(relPath))
	// Test-path prefixes (handled the same way as
	// quality.isToolingPath).
	testishMarkers := []string{
		"/tests/data/", "/tests/fixtures/",
		"/test/data/", "/test/fixtures/",
		"/testdata/", "/__fixtures__/",
		"/placebo/",    // botocore/placebo mock recordings
		"/cassettes/",  // vcrpy/betamax recorded HTTP
		"/recordings/", // various test recorders
	}
	if strings.HasPrefix(lower, "tests/data/") ||
		strings.HasPrefix(lower, "tests/fixtures/") ||
		strings.HasPrefix(lower, "test/data/") ||
		strings.HasPrefix(lower, "test/fixtures/") ||
		strings.HasPrefix(lower, "testdata/") ||
		strings.HasPrefix(lower, "__fixtures__/") {
		return true
	}
	for _, m := range testishMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
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
	// Root is the absolute path of the repo being analyzed. The
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
		// Skip test-fixture paths. SDK-testing fixtures commonly
		// contain example-shaped keys (e.g. AKIA...EXAMPLE) in mocked
		// API responses; a real committed-key incident in test
		// fixtures is rare relative to the per-repo FP load these
		// mocks generate.
		if isTestFixturePath(relPath) {
			continue
		}
		abs := filepath.Join(d.Root, relPath)
		hits := scanFileForAPIKeys(abs)
		for _, h := range hits {
			// Scan-error hits surface as a low-severity diagnostic, not
			// a critical Signal. An earlier revision emitted the
			// synthetic "scan-error: ..." hit with SeverityCritical and
			// it looked like a real secret in the rendered report —
			// confusing users and ranking infra noise as a top-priority
			// finding. Route through the detectorPanic-shaped engine-
			// self-diagnostic channel: SeverityMedium so it surfaces but
			// doesn't dominate the
			// dashboard, and Type stays aiHardcodedAPIKey for catalog
			// roundtripping.
			if h.ScanError {
				out = append(out, models.Signal{
					Type:        signals.SignalAIHardcodedAPIKey,
					Category:    models.CategoryAI,
					Severity:    models.SeverityMedium,
					Confidence:  0.5,
					Location:    models.SignalLocation{File: relPath, Line: h.Line},
					Explanation: "Secret-scan coverage degraded: scanner failed mid-file (" + strings.TrimPrefix(h.Provider, "scan-error:") + "). The remainder of the file was not scanned for hardcoded API keys.",
					SuggestedAction: "Investigate why the file is unreadable (oversized line, encoding issue, truncated upload). Re-run after addressing.",
					SeverityClauses: []string{"sev-medium-005"},
					Actionability:   models.ActionabilityScheduled,
					LifecycleStages: []models.LifecycleStage{models.StageMaintenance},
					AIRelevance:     models.AIRelevanceMedium,
					RuleID:          "terrain/ai/hardcoded-api-key",
					RuleURI:         "docs/rules/ai/hardcoded-api-key.md",
					DetectorVersion: "0.2.0",
					EvidenceSource:   models.SourceStructuralPattern,
					EvidenceStrength: models.EvidenceWeak,
					Metadata: map[string]any{"scanError": true},
				})
				continue
			}
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
				RuleID:          "terrain/ai/hardcoded-api-key",
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
	// ScanError is set for the synthetic "scanner failed mid-file"
	// hit. Callers route these to diagnostics output rather than
	// emitting them as critical-severity Signals — earlier revisions
	// landed scan errors in the same Signal slice as real secrets,
	// which painted a binary blob as a high-entropy key match in the
	// rendered report.
	ScanError bool
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
	// both findings — earlier revisions used a per-line `break` after
	// the first match that swallowed the second key.
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
	// Without checking sc.Err(), a single line longer than 1 MB
	// (minified YAML, embedded blob) would silently drop the rest of
	// the file — secret never detected. Surface scanner errors as a
	// degraded-coverage hit; the caller routes them to diagnostics
	// output rather than emitting them as Signals.
	if err := sc.Err(); err != nil {
		hits = append(hits, keyHit{Provider: "scan-error:" + err.Error(), Line: line, ScanError: true})
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
