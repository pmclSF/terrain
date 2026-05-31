package hygiene

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectSecretsInPrompt scans prompt-classified files (CodeSurfaces
// of Kind=SurfacePrompt) for embedded credentials and emits a Signal
// per match. Implements terrain/hygiene/secrets-in-prompt.
//
// 0.2.0 ships a Go-native regex-based default. The task description
// (#17) calls for gitleaks library integration; that's the documented
// followup. The Go-native default covers the high-severity cases at
// 0.2.0:
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
		data, err := os.ReadFile(path)
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

func scanSecretPatterns(data []byte) map[string]int {
	out := map[string]int{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		for _, p := range secretPatterns {
			if p.pattern.Find(line) != nil {
				out[p.name]++
			}
		}
	}
	return out
}
