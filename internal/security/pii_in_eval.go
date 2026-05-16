package security

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

// DetectPIIInEval scans eval-directory files for PII patterns
// (emails, phone numbers, SSNs, credit cards). Implements
// terrain/security/pii-in-eval.
//
// 0.2.0 ships the Go-native default with regex-based detection. The
// task description (#17) calls for a Microsoft Presidio opt-in path
// for richer named-entity recognition; that integration is followup
// work that runs through a separate adapter package. Until it lands,
// the Go-native default covers the common cases.
//
// Coverage scope:
//   - eval-directory files (eval/, evals/, evaluations/, __evals__/)
//   - .yaml, .yml, .json, .jsonl, .csv, .txt files (no AST needed)
//   - .py and .md files included since adopters frequently embed
//     example payloads inline
//
// The detector emits one Signal per file with at least one match;
// the Metadata.matches field lists the PII kinds found.
func DetectPIIInEval(path string) []models.Signal {
	if !looksLikeEvalPath(path) {
		return nil
	}
	if !looksLikePIIScanCandidate(path) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	matches := scanPIIPatterns(data)
	if len(matches) == 0 {
		return nil
	}

	kinds := make([]string, 0, len(matches))
	for k := range matches {
		kinds = append(kinds, k)
	}

	return []models.Signal{{
		Type:             signals.SignalPIIInEval,
		Category:         models.CategoryAI,
		Severity:         models.SeverityCritical,
		Confidence:       confidenceForPIIMatches(matches),
		EvidenceStrength: models.EvidenceStrong,
		EvidenceSource:   models.SourceStructuralPattern,
		Location:         models.SignalLocation{File: path},
		Explanation: fmt.Sprintf(
			"Eval-directory file %s contains PII-shaped values (%s). Eval datasets that retain production PII expose customer data to anyone with repo access; redaction or synthetic data is required.",
			path, strings.Join(kinds, ", "),
		),
		SuggestedAction: "Replace PII in the eval dataset with synthetic equivalents (Faker / Mimesis / mockaroo) or apply a redaction pass before committing.",
		RuleID:          "terrain/security/pii-in-eval",
		RuleURI:         "docs/rules/security/pii-in-eval.md",
		DetectorVersion: "0.2.0",
		Metadata: map[string]any{
			"piiKinds": kinds,
		},
	}}
}

func looksLikeEvalPath(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range []string{"/eval/", "/evals/", "/evaluations/", "/__evals__/"} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func looksLikePIIScanCandidate(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range []string{".yaml", ".yml", ".json", ".jsonl", ".csv", ".txt", ".py", ".md", ".tsv"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// piiPattern is a single PII regex.
type piiPattern struct {
	name    string
	pattern *regexp.Regexp
}

// piiPatterns is the Go-native default PII vocabulary. Patterns are
// conservative to keep FP rate down; richer NER comes via Presidio.
var piiPatterns = []piiPattern{
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
	// US SSN: 3-2-4 digits separated by - or space, with leading digit not 9
	{"ssn", regexp.MustCompile(`\b[0-8][0-9]{2}[-\s]?[0-9]{2}[-\s]?[0-9]{4}\b`)},
	// US phone: NPA-NXX-XXXX with optional +1 country code
	{"phone-us", regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?[2-9][0-9]{2}\)?[-.\s]?[2-9][0-9]{2}[-.\s]?[0-9]{4}\b`)},
	// IPv4 (in many adopter datasets a session identifier).
	{"ipv4", regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)},
	// Credit card (loose 13-19 digit run with optional separators).
	{"credit-card", regexp.MustCompile(`\b[3456][0-9]{3}[-\s]?[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}\b`)},
}

func scanPIIPatterns(data []byte) map[string]int {
	out := map[string]int{}
	// Read line by line so any single line that includes a synthetic
	// header is still scanned without pulling all of the file into
	// memory.
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		for _, p := range piiPatterns {
			if loc := p.pattern.FindIndex(line); loc != nil {
				out[p.name]++
			}
		}
	}
	return out
}

func confidenceForPIIMatches(matches map[string]int) float64 {
	// Confidence rises with the variety of PII shapes (single-kind
	// matches may be false positives like SSN-shaped synthetic IDs).
	switch len(matches) {
	case 0:
		return 0
	case 1:
		return 0.75
	case 2:
		return 0.88
	}
	return 0.95
}
