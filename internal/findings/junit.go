package findings

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// JUnit XML emission.
//
// JUnit output must render cleanly in the three dominant CI consumers:
// dorny/test-reporter, mikepenz/action-junit-report, and GitLab native.
// The shape below is the intersection of what those three accept;
// deviations from the JUnit XML schema that any of them reject are
// not used.
//
// Mapping rules:
//   - One <testsuite> per rule_id. Suite name is the rule_id.
//   - One <testcase> per finding. Case name is primary_loc (path:line).
//   - Severity → JUnit element:
//     - error    → <failure>
//     - warning  → not emitted in JUnit (PR-comment + Step Summary only)
//     - notice   → not emitted in JUnit
//   - <failure message="<short_message>"> body carries <long_message>,
//     cause-path, reproduction command, docs URL.
//   - Tests count = error count; failures = error count; skipped = 0.

// JUnitOptions controls JUnit XML emission.
type JUnitOptions struct {
	// EmitWarnings includes severity=warning findings as testcase
	// entries (without <failure>). Default is to omit warnings —
	// JUnit is the CI gate, warnings live in Step Summary only.
	EmitWarnings bool

	// HostName is the optional system name carried on the testsuites
	// element. Empty omits the attribute.
	HostName string

	// Timestamp is the optional run timestamp (RFC3339). Empty omits.
	Timestamp string
}

// WriteJUnit emits the artifact as JUnit XML. The output passes the
// validators used by dorny/test-reporter and mikepenz/action-junit-report.
func (a *Artifact) WriteJUnit(w io.Writer, opts JUnitOptions) error {
	suites := groupBySuite(a.Findings, opts.EmitWarnings)

	root := junitTestSuites{
		Time:     0,
		Tests:    countCases(suites),
		Failures: countFailures(suites),
		Errors:   0,
		Suites:   suites,
	}
	if opts.HostName != "" {
		root.Hostname = opts.HostName
	}
	if opts.Timestamp != "" {
		root.Timestamp = opts.Timestamp
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("findings: encode junit: %w", err)
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}

func groupBySuite(findings []Finding, includeWarnings bool) []junitTestSuite {
	byRule := map[string][]Finding{}
	for _, f := range findings {
		if f.Severity != SeverityError && !(includeWarnings && f.Severity == SeverityWarning) {
			continue
		}
		byRule[f.RuleID] = append(byRule[f.RuleID], f)
	}
	ruleIDs := make([]string, 0, len(byRule))
	for k := range byRule {
		ruleIDs = append(ruleIDs, k)
	}
	sort.Strings(ruleIDs)

	out := make([]junitTestSuite, 0, len(ruleIDs))
	for _, rule := range ruleIDs {
		fs := byRule[rule]
		suite := junitTestSuite{
			Name:     rule,
			Tests:    len(fs),
			Failures: countFailureSeverity(fs),
			Cases:    make([]junitTestCase, 0, len(fs)),
		}
		for _, f := range fs {
			suite.Cases = append(suite.Cases, findingToCase(f))
		}
		out = append(out, suite)
	}
	return out
}

func findingToCase(f Finding) junitTestCase {
	caseName := f.PrimaryLoc.Path
	if f.PrimaryLoc.Line > 0 {
		caseName = fmt.Sprintf("%s:%d", caseName, f.PrimaryLoc.Line)
	}
	tc := junitTestCase{
		Name:      caseName,
		ClassName: f.RuleID,
	}
	if f.Severity == SeverityError {
		tc.Failure = &junitFailure{
			Message: f.ShortMessage,
			Type:    f.RuleID,
			Body:    failureBody(f),
		}
	}
	return tc
}

// failureBody composes the JUnit failure body: long_message, cause
// path, reproduction command, docs URL.
func failureBody(f Finding) string {
	var b strings.Builder
	if f.LongMessage != "" {
		b.WriteString(f.LongMessage)
		b.WriteString("\n\n")
	}
	if len(f.CausePath) > 0 {
		b.WriteString("Cause path:\n")
		for i, loc := range f.CausePath {
			fmt.Fprintf(&b, "  %d. %s", i+1, loc.Path)
			if loc.Line > 0 {
				fmt.Fprintf(&b, ":%d", loc.Line)
			}
			if loc.NodeKind != "" {
				fmt.Fprintf(&b, " (%s)", loc.NodeKind)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if f.Reproduction != "" {
		b.WriteString("Reproduce locally:\n")
		b.WriteString("  ")
		b.WriteString(f.Reproduction)
		b.WriteString("\n\n")
	}
	if f.DocsURL != "" {
		b.WriteString("Docs: ")
		b.WriteString(f.DocsURL)
		b.WriteString("\n")
	}
	return b.String()
}

func countCases(suites []junitTestSuite) int {
	n := 0
	for _, s := range suites {
		n += s.Tests
	}
	return n
}

func countFailures(suites []junitTestSuite) int {
	n := 0
	for _, s := range suites {
		n += s.Failures
	}
	return n
}

func countFailureSeverity(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == SeverityError {
			n++
		}
	}
	return n
}

// --- JUnit XML shape ---

type junitTestSuites struct {
	XMLName   xml.Name         `xml:"testsuites"`
	Time      float64          `xml:"time,attr,omitempty"`
	Tests     int              `xml:"tests,attr"`
	Failures  int              `xml:"failures,attr"`
	Errors    int              `xml:"errors,attr"`
	Hostname  string           `xml:"hostname,attr,omitempty"`
	Timestamp string           `xml:"timestamp,attr,omitempty"`
	Suites    []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Cases    []junitTestCase  `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr,omitempty"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Body    string `xml:",chardata"`
}
