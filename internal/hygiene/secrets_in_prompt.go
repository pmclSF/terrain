package hygiene

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/saferead"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectSecretsInPrompt scans prompt-classified files (CodeSurfaces
// of Kind=SurfacePrompt) for embedded credentials and emits a Signal
// per match. Implements terrain/hygiene/secrets-in-prompt.
//
// Uses a Go-native regex set covering the high-severity credential
// shapes below:
//
//   - OpenAI keys (sk-... prefix)
//   - Anthropic keys (sk-ant-... prefix)
//   - GitHub tokens (ghp_ / ghs_ / gho_ / ghu_ / ghr_ prefixes)
//   - Slack tokens (xoxb-... xoxa-... xoxp-...)
//   - AWS access keys (AKIA... + 16 hex)
//   - JWT-shaped strings (header.payload.signature)
//   - Bearer tokens embedded as authorization headers
//   - Generic 40+ char base64-looking secrets (high FP rate; off by
//     default at 0.2.0)
func DetectSecretsInPrompt(promptFilePaths []string) []models.Signal {
	var out []models.Signal
	for _, path := range promptFilePaths {
		data, err := saferead.ReadFile(path)
		if err != nil {
			continue
		}
		matches := scanSecretPatterns(data)
		if len(matches) == 0 {
			continue
		}
		kinds := make([]string, 0, len(matches))
		for k := range matches {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		out = append(out, models.Signal{
			Type:             signals.SignalSecretsInPrompt,
			Category:         models.CategoryAI,
			Severity:         models.SeverityCritical,
			Confidence:       0.95,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceStructuralPattern,
			Location:         models.SignalLocation{File: path},
			Explanation: fmt.Sprintf(
				"Prompt file %s contains embedded credential(s): %s. Anyone with read access to the prompt has access to the credential; rotating the credential after exposure is the only mitigation.",
				path, strings.Join(kinds, ", "),
			),
			SuggestedAction: "Rotate the leaked credential immediately, then move it to an environment variable or secret manager. Reference {{ env.OPENAI_API_KEY }} (or your prompt-template equivalent) in the prompt instead of inlining.",
			RuleID:          "terrain/hygiene/secrets-in-prompt",
			RuleURI:         "docs/rules/hygiene/secrets-in-prompt.md",
			DetectorVersion: "0.2.0",
			Metadata: map[string]any{
				"secretKinds": kinds,
			},
		})
	}
	return out
}

// secretPatterns are the high-confidence credential shapes. Patterns
// with high FP rate (generic base64, generic hex) are intentionally
// excluded — they're noise without a richer entropy/context model.
var secretPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"openai-api-key", regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
	{"anthropic-api-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{30,}`)},
	{"github-token", regexp.MustCompile(`\b(?:ghp|ghs|gho|ghu|ghr)_[A-Za-z0-9]{36,}\b`)},
	{"slack-bot-token", regexp.MustCompile(`\bxox[bapr]-[A-Za-z0-9\-]{10,}\b`)},
	{"aws-access-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`)},
	{"bearer-token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9_\-]{20,}\b`)},
}

// secretPlaceholderMarkers mark a matched credential as a documentation /
// example / test value rather than a live secret. This mirrors the placeholder
// defense the sibling hardcoded-api-key detector has always had and this
// Critical-severity detector previously lacked. Only deliberate human-written
// phrases are listed — never common digit/hex runs, since a real (random) key
// can legitimately contain them (that would cause false negatives).
var secretPlaceholderMarkers = []string{
	"fake", "placeholder", "example", "dummy", "test-", "test_", "sample",
	"redacted", "your-key", "your_key", "your-token", "your_token",
	"xxxxx", "00000",
}

func isSecretPlaceholder(match string) bool {
	low := strings.ToLower(match)
	for _, m := range secretPlaceholderMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return hasLowSecretEntropy(match)
}

// hasLowSecretEntropy returns true when the string is dominated by a single
// repeated character — the shape of a doc placeholder, never a real key.
func hasLowSecretEntropy(s string) bool {
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

func scanSecretPatterns(data []byte) map[string]int {
	out := map[string]int{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		for _, p := range secretPatterns {
			for _, loc := range p.pattern.FindAllIndex(line, -1) {
				if isSecretPlaceholder(string(line[loc[0]:loc[1]])) {
					continue // documentation / example / placeholder credential
				}
				out[p.name]++
				break // one count per line per kind
			}
		}
	}
	return out
}
