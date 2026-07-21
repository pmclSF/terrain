package security

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

// DetectPIIInEval scans eval-directory files for PII patterns
// (emails, phone numbers, SSNs, credit cards). Implements
// terrain/security/pii-in-eval.
//
// Detection is regex-based over eval-directory files. Patterns are
// conservative to keep the false-positive rate low, since this
// detector emits at Critical severity.
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
	data, err := saferead.ReadFile(path)
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
	sort.Strings(kinds)

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
// conservative to keep the false-positive rate down.
var piiPatterns = []piiPattern{
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
	// US SSN: 3-2-4 digits separated by - or space, with leading digit not 9
	{"ssn", regexp.MustCompile(`\b[0-8][0-9]{2}[-\s]?[0-9]{2}[-\s]?[0-9]{4}\b`)},
	// US phone: NPA-NXX-XXXX with optional +1 country code
	{"phone-us", regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?[2-9][0-9]{2}\)?[-.\s]?[2-9][0-9]{2}[-.\s]?[0-9]{4}\b`)},
	// Credit card (loose 13-19 digit run with optional separators).
	{"credit-card", regexp.MustCompile(`\b[3456][0-9]{3}[-\s]?[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}\b`)},
	// NOTE: an IPv4 pattern was intentionally REMOVED. `1.2.3.4` is
	// indistinguishable from a version string / timeout / config value, an IP
	// is at best marginal PII, and the FP rate is far too high for a
	// Critical-severity gate. Re-add only behind a real IP-context signal.
}

// reservedEmailDomains are documentation/test domains (RFC 2606 + common test
// TLDs) that never carry real customer PII.
var reservedEmailFragments = []string{
	"@example.", "@example", "@test.", "@invalid.", "@localhost", "@domain.",
	"@email.com", ".example", ".test", ".invalid", ".local",
}

// knownTestCards are the vendor-published test card numbers (Visa, Stripe,
// Mastercard, Discover, JCB) that appear in fixtures and are never real PANs.
// Only 16-digit PANs are listed because the credit-card pattern matches a
// 16-digit run; shorter formats (15-digit Amex, 14-digit Diners) are not
// scanned, so their test cards would be unreachable suppression entries.
var knownTestCards = map[string]bool{
	"4111111111111111": true, "4242424242424242": true, "4012888888881881": true,
	"5555555555554444": true, "5105105105105100": true, "2223003122003222": true,
	"6011111111111117": true, "6011000990139424": true, "3566002020360505": true,
}

// isSyntheticPII reports whether a matched value is a documentation / test /
// placeholder value rather than real PII — a structural class (reserved
// domains, vendor test cards, all-same/sequential digit runs, invalid SSN
// area numbers) that must not fire a Critical finding.
func isSyntheticPII(kind, val string) bool {
	switch kind {
	case "email":
		v := strings.ToLower(val)
		for _, frag := range reservedEmailFragments {
			if strings.Contains(v, frag) {
				return true
			}
		}
	case "credit-card":
		d := onlyDigits(val)
		return knownTestCards[d] || lowDigitEntropy(d)
	case "ssn":
		d := onlyDigits(val)
		// invalid SSN area numbers (000, 666) are never assigned; low-entropy
		// runs (000000000, 123456789) are placeholders.
		return lowDigitEntropy(d) || strings.HasPrefix(d, "000") || strings.HasPrefix(d, "666")
	}
	return false
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// lowDigitEntropy reports whether a digit run is all-identical or strictly
// sequential (ascending/descending) — the shape of a placeholder, not real PII.
func lowDigitEntropy(d string) bool {
	if len(d) < 4 {
		return false
	}
	allSame, asc, desc := true, true, true
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			allSame = false
		}
		if d[i] != d[i-1]+1 {
			asc = false
		}
		if d[i] != d[i-1]-1 {
			desc = false
		}
	}
	return allSame || asc || desc
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
			for _, loc := range p.pattern.FindAllIndex(line, -1) {
				if isSyntheticPII(p.name, string(line[loc[0]:loc[1]])) {
					continue // documentation / test / placeholder value — not real PII
				}
				out[p.name]++
				break // one count per line per kind
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
