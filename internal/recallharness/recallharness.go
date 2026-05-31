// Package recallharness measures per-mechanism + union recall for
// rules that have been split (or are about to be split) across multiple
// detection mechanisms. The contract:
//
//	"When a rule R is split into mechanisms M1..Mn, the union recall
//	 over the golden TP set must not drop more than the configured
//	 threshold from the pre-split recall."
//
// The shape of the problem: a single rule (e.g. untestedExport) starts
// as one regex/AST traversal. Over time it gets split into multiple
// mechanisms (barrel-export resolution, scope-classifier,
// def-following). Each mechanism is a separate code path with its own
// precision/recall tradeoffs. When a mechanism is added or removed,
// the harness reports:
//
//   - Per-mechanism recall: what fraction of golden TPs does this
//     mechanism alone catch?
//   - Union recall: what fraction of golden TPs do all mechanisms
//     together catch?
//   - Overlap: which TPs are caught by multiple mechanisms
//     (informational — high overlap is wasted work; zero overlap is
//     fragile).
package recallharness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Harness is the parsed contents of one per-rule recall-harness YAML.
type Harness struct {
	SchemaVersion int `yaml:"schema_version"`

	// RuleID is the canonical rule this harness measures.
	RuleID string `yaml:"rule_id"`

	// Mechanisms is the list of detection mechanisms that contribute to this
	// rule. Each entry has an id (used to match against Finding.Mechanism)
	// and a documentary description.
	Mechanisms []Mechanism `yaml:"mechanisms"`

	// GoldenTPs is the curated set of true positives this harness uses as
	// the recall denominator. Adding a TP requires a fresh PR + reviewer
	// sign-off — never silently grow the set to "fix" recall.
	GoldenTPs []GoldenTP `yaml:"golden_tps"`

	// UnionMinRecall is the floor for sum-of-mechanisms recall. If the union
	// recall drops below this on a CI run, the build fails. Default 0.0
	// means no floor (use during initial bring-up).
	UnionMinRecall float64 `yaml:"union_min_recall"`
}

// Mechanism is one detection code path that contributes to a rule's findings.
type Mechanism struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description,omitempty"`

	// MinRecall is the floor for this mechanism's individual recall (over
	// the union of TPs labeled as belonging to it via MechanismHint). 0.0
	// means no per-mechanism floor — only the union floor enforces.
	MinRecall float64 `yaml:"min_recall,omitempty"`
}

// GoldenTP is one labeled true positive. MechanismHint, when set, names the
// mechanism this TP is expected to be caught by (used for per-mechanism
// recall denominators). When empty, the TP counts toward the union recall
// only — useful for TPs that genuinely need multi-mechanism coverage.
type GoldenTP struct {
	Repo           string `yaml:"repo"`
	File           string `yaml:"file"`
	Line           int    `yaml:"line,omitempty"`
	MechanismHint  string `yaml:"mechanism_hint,omitempty"`
	Note           string `yaml:"note,omitempty"`
}

// Finding is the runtime shape the harness compares against. Callers convert
// their domain-specific finding type into this struct, including which
// mechanism produced it.
type Finding struct {
	RuleID    string
	Mechanism string
	Repo      string
	File      string
	Line      int
}

// Report is the outcome of running a Harness against a set of findings.
type Report struct {
	RuleID       string
	TotalGolden  int
	UnionCaught  int
	UnionRecall  float64
	UnionMinFail bool

	// PerMechanism gives the recall data for each declared mechanism. Order
	// matches the YAML declaration so reports are stable.
	PerMechanism []MechanismReport
}

// MechanismReport carries one mechanism's recall slice.
type MechanismReport struct {
	ID            string
	HintedGolden  int     // TPs with this mechanism as their MechanismHint
	HintedCaught  int     // of those, how many this mechanism caught
	HintedRecall  float64 // HintedCaught / HintedGolden (0 when no hints)
	TotalCaught   int     // count of all golden TPs this mechanism caught (incl. unhinted/cross)
	MinRecallFail bool
}

// LoadHarness parses a harness YAML from path.
func LoadHarness(path string) (*Harness, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read harness %s: %w", path, err)
	}
	return ParseHarness(data, filepath.Base(path))
}

// ParseHarness parses harness YAML from in-memory bytes.
func ParseHarness(data []byte, label string) (*Harness, error) {
	h := &Harness{}
	if err := yaml.Unmarshal(data, h); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	if err := h.validate(label); err != nil {
		return nil, err
	}
	return h, nil
}

// LoadAll loads every *.yaml file in dir as a Harness, keyed by RuleID.
func LoadAll(dir string) (map[string]*Harness, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read harness dir %s: %w", dir, err)
	}
	out := map[string]*Harness{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		h, err := LoadHarness(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if _, dup := out[h.RuleID]; dup {
			return nil, fmt.Errorf("duplicate rule %q in harness dir %s", h.RuleID, dir)
		}
		out[h.RuleID] = h
	}
	return out, nil
}

// Check computes per-mechanism + union recall against the supplied findings
// and returns a Report. The harness's RuleID is the implicit filter — only
// findings with a matching RuleID participate.
func (h *Harness) Check(findings []Finding) *Report {
	relevant := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.RuleID == h.RuleID {
			relevant = append(relevant, f)
		}
	}

	// Index findings by (mechanism, location) for cheap lookup.
	caughtAny := make([]bool, len(h.GoldenTPs))
	mechCaught := make([]map[int]bool, len(h.Mechanisms))
	for i := range mechCaught {
		mechCaught[i] = map[int]bool{}
	}
	mechIndex := map[string]int{}
	for i, m := range h.Mechanisms {
		mechIndex[m.ID] = i
	}

	for _, f := range relevant {
		mi, ok := mechIndex[f.Mechanism]
		if !ok {
			// Finding from a mechanism not declared in this harness.
			// Skip — declaring a mechanism is required to count it.
			continue
		}
		for gi, tp := range h.GoldenTPs {
			if !goldenMatches(tp, f) {
				continue
			}
			caughtAny[gi] = true
			mechCaught[mi][gi] = true
		}
	}

	report := &Report{
		RuleID:      h.RuleID,
		TotalGolden: len(h.GoldenTPs),
	}
	for _, c := range caughtAny {
		if c {
			report.UnionCaught++
		}
	}
	if report.TotalGolden > 0 {
		report.UnionRecall = float64(report.UnionCaught) / float64(report.TotalGolden)
	}
	report.UnionMinFail = h.UnionMinRecall > 0 && report.UnionRecall < h.UnionMinRecall

	for i, m := range h.Mechanisms {
		mr := MechanismReport{ID: m.ID}
		// Build the per-mechanism denominator: golden TPs that hint at this mechanism.
		for gi, tp := range h.GoldenTPs {
			if tp.MechanismHint == m.ID {
				mr.HintedGolden++
				if mechCaught[i][gi] {
					mr.HintedCaught++
				}
			}
			if mechCaught[i][gi] {
				mr.TotalCaught++
			}
		}
		if mr.HintedGolden > 0 {
			mr.HintedRecall = float64(mr.HintedCaught) / float64(mr.HintedGolden)
		}
		mr.MinRecallFail = m.MinRecall > 0 && mr.HintedGolden > 0 && mr.HintedRecall < m.MinRecall
		report.PerMechanism = append(report.PerMechanism, mr)
	}
	return report
}

// AnyFail returns true when any floor (union or per-mechanism) was breached.
func (r *Report) AnyFail() bool {
	if r.UnionMinFail {
		return true
	}
	for _, mr := range r.PerMechanism {
		if mr.MinRecallFail {
			return true
		}
	}
	return false
}

func (h *Harness) validate(label string) error {
	if h.SchemaVersion != 1 {
		return fmt.Errorf("%s: unsupported schema_version %d (expected 1)", label, h.SchemaVersion)
	}
	if h.RuleID == "" {
		return fmt.Errorf("%s: missing rule_id", label)
	}
	if h.UnionMinRecall < 0 || h.UnionMinRecall > 1 {
		return fmt.Errorf("%s: union_min_recall must be in [0,1]", label)
	}
	mechIDs := map[string]bool{}
	for i, m := range h.Mechanisms {
		if m.ID == "" {
			return fmt.Errorf("%s: mechanisms[%d] missing id", label, i)
		}
		if mechIDs[m.ID] {
			return fmt.Errorf("%s: mechanisms[%d] duplicates id %q", label, i, m.ID)
		}
		if m.MinRecall < 0 || m.MinRecall > 1 {
			return fmt.Errorf("%s: mechanisms[%d].min_recall must be in [0,1]", label, i)
		}
		mechIDs[m.ID] = true
	}
	seen := map[string]bool{}
	for i, tp := range h.GoldenTPs {
		if tp.File == "" {
			return fmt.Errorf("%s: golden_tps[%d] missing file", label, i)
		}
		if tp.MechanismHint != "" && !mechIDs[tp.MechanismHint] {
			return fmt.Errorf("%s: golden_tps[%d] hints undeclared mechanism %q", label, i, tp.MechanismHint)
		}
		key := goldenKey(tp)
		if seen[key] {
			return fmt.Errorf("%s: golden_tps[%d] duplicates an earlier entry (%s)", label, i, key)
		}
		seen[key] = true
	}
	// Deterministic mechanism order in reports.
	sort.SliceStable(h.Mechanisms, func(i, j int) bool {
		return h.Mechanisms[i].ID < h.Mechanisms[j].ID
	})
	return nil
}

func goldenMatches(tp GoldenTP, f Finding) bool {
	// Tightened repo match: require exact match on both sides. The
	// previous "empty-side wildcards" behavior allowed cross-repo
	// findings to count toward recall, inflating the numerator.
	if tp.Repo != f.Repo {
		return false
	}
	if tp.File != f.File {
		return false
	}
	// File-scope match is allowed when the golden TP omits Line. When
	// the golden TP specifies a line, the finding must match exactly.
	if tp.Line != 0 && f.Line != tp.Line {
		return false
	}
	return true
}

func goldenKey(tp GoldenTP) string {
	return fmt.Sprintf("%s|%s|%d", tp.Repo, tp.File, tp.Line)
}
