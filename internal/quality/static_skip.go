package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
)

// SplitMechanismName is the canonical mechanism name that toggles the
// static_skipped_test split (unconditional vs conditional-gate). When
// the mechanism is on, the detector emits the split signal types; when
// off, it emits the legacy "staticSkippedTest" type for back-compat.
const SplitMechanismName = "static_skipped_test_split"

// StaticSkipDetector identifies statically skipped tests from source code patterns.
//
// This detector finds skip markers in test files without requiring runtime artifacts:
//   - JS/TS: it.skip(), test.skip(), describe.skip(), xit(), xdescribe()
//   - Go: t.Skip(), t.Skipf(), t.SkipNow()
//   - Python: @pytest.mark.skip, @unittest.skip, pytest.skip()
//   - Java: @Disabled, @Ignore
//
// This closes the P0 gap where docs promise skip detection from `terrain analyze`
// but the runtime-based SkippedTestDetector requires --runtime artifacts.
type StaticSkipDetector struct {
	// RepoRoot is used to read file contents for the conditional-gate
	// classification when the static_skipped_test_split mechanism is on.
	// Empty defaults to "." for back-compat.
	RepoRoot string
}

// Detect scans test files for static skip patterns.
func (d *StaticSkipDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal
	totalSkipped := 0
	totalTests := 0

	type fileSkip struct {
		path      string
		skips     int
		tests     int
		framework string
	}
	var skippedFiles []fileSkip

	for _, tf := range snap.TestFiles {
		if tf.TestCount == 0 {
			continue
		}
		// Ratio>100% rows are NOT suppressed here because TestCount
		// excludes skipped tests (jsTestPattern matches `it(` but not
		// `it.skip(`), so a legitimate skip-heavy file with 4 skips of
		// 1 running test reports SkipCount=4/TestCount=1 = ratio 400%
		// — a real signal, not a counting bug. Proper fix is to
		// include skipped tests in TestCount; deferred to a later
		// detector rebuild.
		totalTests += tf.TestCount
		if tf.SkipCount > 0 {
			totalSkipped += tf.SkipCount
			skippedFiles = append(skippedFiles, fileSkip{
				path:      tf.Path,
				skips:     tf.SkipCount,
				tests:     tf.TestCount,
				framework: tf.Framework,
			})
		}
	}

	if totalSkipped == 0 || totalTests == 0 {
		return nil
	}

	ratio := float64(totalSkipped) / float64(totalTests)
	sev := staticSkipSeverity(ratio)

	signals = append(signals, models.Signal{
		Type:       "staticSkippedTest",
		Category:   models.CategoryHealth,
		Severity:   sev,
		Confidence: 0.8,
		Location:   models.SignalLocation{Repository: "static"},
		Explanation: fmt.Sprintf(
			"%d of %d tests statically skipped (%.0f%%) via code markers (.skip, xit, @skip, etc.).",
			totalSkipped, totalTests, ratio*100,
		),
		SuggestedAction:  "Review skipped tests — restore, remove, or convert to conditional skips with documented reasons.",
		EvidenceStrength: models.EvidencePartial,
		EvidenceSource:   models.SourceStructuralPattern,
		Metadata: map[string]any{
			"skippedCount": totalSkipped,
			"totalCount":   totalTests,
			"ratio":        ratio,
			"scope":        "repository",
			"detection":    "static",
		},
	})

	// Sort by skip ratio descending for deterministic output.
	sort.Slice(skippedFiles, func(i, j int) bool {
		ri := float64(skippedFiles[i].skips) / float64(skippedFiles[i].tests)
		rj := float64(skippedFiles[j].skips) / float64(skippedFiles[j].tests)
		if ri != rj {
			return ri > rj
		}
		return skippedFiles[i].path < skippedFiles[j].path
	})

	// Route the split decision through the canonical state machine so
	// shadow mode emits would-add events without changing user-visible
	// types. Only state=on actually swaps the emitted Type.
	splitOn := mechanisms.GateAdd(mechanisms.Default(), SplitMechanismName,
		mechanisms.EventContext{RuleID: "staticSkippedTest"},
		func() mechanisms.PredicateResult {
			return mechanisms.PredicateResult{
				Fired:   true,
				Reasons: []string{"emit split signal types (unconditional vs conditional-gate)"},
			}
		})
	for _, sf := range skippedFiles {
		fileRatio := float64(sf.skips) / float64(sf.tests)
		// Per-file Type is "staticSkippedTest" by default; when the
		// static_skipped_test_split mechanism is on, classify the file
		// as unconditional (no gate predicate present) vs
		// conditional-gate (env/feature-flag/platform predicate
		// present somewhere in the file).
		sigType := models.SignalType("staticSkippedTest")
		if splitOn {
			if d.fileHasGatePredicate(sf.path) {
				sigType = "staticSkippedTest-conditional-gate"
			} else {
				sigType = "staticSkippedTest-unconditional"
			}
		}
		signals = append(signals, models.Signal{
			Type:             sigType,
			Category:         models.CategoryHealth,
			Severity:         staticSkipSeverity(fileRatio),
			Confidence:       0.8,
			EvidenceStrength: models.EvidencePartial,
			EvidenceSource:   models.SourceStructuralPattern,
			Location:         models.SignalLocation{File: sf.path},
			Explanation: fmt.Sprintf(
				"%d of %d tests statically skipped (%.0f%%) in %s.",
				sf.skips, sf.tests, fileRatio*100, sf.path,
			),
			SuggestedAction: "Review skipped tests — restore, remove, or document the skip reason.",
			Metadata: map[string]any{
				"skippedCount": sf.skips,
				"totalCount":   sf.tests,
				"ratio":        fileRatio,
				"scope":        "file",
				"detection":    "static",
			},
		})
	}

	return signals
}

// gatePredicateRe matches the shapes that indicate a skip is wrapped
// by a runtime gate condition (environment, feature flag, platform).
var gatePredicateRe = regexp.MustCompile(
	`(?i)` +
		`process\.env\.[A-Z_]+|` + // JS: process.env.FOO
		`os\.environ\b|os\.getenv\(|` + // Python: os.environ / os.getenv
		`@\s*pytest\.mark\.skipif\b|` + // Pytest: skipif
		`@\s*unittest\.skipIf\b|` + // Python unittest: skipIf
		`@\s*Skip[A-Z]\w*\b|` + // JUnit/etc: @SkipOnX
		`if\s+__name__\b|` + // common platform/CI gate
		`platform\.(system|machine|python_implementation)\b|` +
		`feature[_]?flag\b|featureFlag\b|` +
		`os\.Getenv\(`)

func (d *StaticSkipDetector) fileHasGatePredicate(relPath string) bool {
	root := d.RepoRoot
	if root == "" {
		root = "."
	}
	abs := filepath.Join(root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		// Unreadable file — fail open to the more conservative
		// "unconditional" classification, since we can't confirm a
		// gate exists.
		return false
	}
	text := stripLineComments(string(data))
	return gatePredicateRe.MatchString(text)
}

// stripLineComments removes line-comment content from the source text
// before regex matching. Handles the three dialects this detector
// targets:
//   - C-style `// rest of line`
//   - Python / shell / YAML `# rest of line` (Python files are the
//     common case for static-skip false-positives, where a comment
//     like `# os.getenv("FOO")` would otherwise count as a gate
//     predicate and mis-classify an unconditional skip).
//   - Block `/* ... */` comments collapsed to a single space.
//
// String-literal handling is NOT implemented: tokens that look like
// comments inside quoted strings (e.g. a JS `"# in string"` literal
// or a `#privateField` identifier) are still stripped. The
// classification this feeds is conservative on false negatives — when
// in doubt, the rule mis-classifies as "unconditional" rather than
// silently passing — so the fail direction is safe.
//
// On an unterminated block comment, the remainder of the file is
// preserved verbatim (rather than truncated) so a malformed source
// file doesn't accidentally clear the gate-predicate check.
func stripLineComments(text string) string {
	if !strings.ContainsAny(text, "/#") {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	i := 0
	for i < len(text) {
		// Block comment.
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '*' {
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				// Unterminated /* … */: preserve the rest of the file
				// verbatim so the gate-predicate check still has the
				// remaining source to work with.
				b.WriteString(text[i:])
				return b.String()
			}
			b.WriteByte(' ')
			i += end + 4
			continue
		}
		// Line comment (// or #).
		if (i+1 < len(text) && text[i] == '/' && text[i+1] == '/') || text[i] == '#' {
			nl := strings.IndexByte(text[i:], '\n')
			if nl < 0 {
				// Last line, no newline. Drop the comment tail.
				return b.String()
			}
			i += nl // keep the newline so line numbers don't shift
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

func staticSkipSeverity(ratio float64) models.SignalSeverity {
	if ratio > 0.5 {
		return models.SeverityHigh
	}
	if ratio > 0.2 {
		return models.SeverityMedium
	}
	return models.SeverityLow
}
