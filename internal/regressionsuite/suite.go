// Package regressionsuite is the frozen-TP regression machinery that gates
// shared-infrastructure module ships. Per the binding rules:
//
//   - When a shared module (A7 barrel resolver, A3 scope classifier, ASCG,
//     EHR, FvS, SurfaceLiteralPresenceGate, etc.) ships in Phase 2, it must
//     ship with a frozen suite of TPs from each consumer detector.
//   - The symmetric ≥10% rule (or ≥5 TPs) gates the ship: if a module change
//     drops more than max_tp_loss frozen TPs, the module's PR is blocked.
//
// Workflow:
//
//  1. Author of a shared-infrastructure module collects the TPs each
//     consumer detector currently fires on (from v2 validation data or
//     equivalent).
//  2. The TPs are written into a per-module YAML file under
//     harness/regression-suites/<module>.yaml.
//  3. CI runs LoadSuite + Check against the head SHA's findings; a regression
//     past max_tp_loss fails the build.
//
// Phase 1 baseline: this package + empty placeholder YAMLs. Suites populate
// as Phase 2 modules land.
package regressionsuite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Suite is the parsed contents of one regression-suite YAML file.
type Suite struct {
	// SchemaVersion is the format version of this YAML. Only 1 is supported
	// today; mismatches return an error so a schema bump can be noticed.
	SchemaVersion int `yaml:"schema_version"`

	// Module is the human-readable name of the shared module this suite
	// gates (e.g. "A7-barrel-resolver", "SurfaceLiteralPresenceGate").
	Module string `yaml:"module"`

	// MaxTPLoss is the maximum number of frozen TPs that can be missing
	// before the suite reports a failure. 10 is the canonical floor per the
	// symmetric ≥10%/≥5-TP rule.
	MaxTPLoss int `yaml:"max_tp_loss"`

	// ConsumerDetectors lists the rule_ids whose recall this suite gates.
	// Documentary today; used by the doctor surface to explain which detectors
	// are protected by which suite.
	ConsumerDetectors []string `yaml:"consumer_detectors,omitempty"`

	// FrozenTPs is the canonical list of true positives that must continue
	// to fire after the module change. Each entry pins (rule_id, repo, file,
	// line) — line is optional when the rule fires at file scope.
	FrozenTPs []FrozenTP `yaml:"frozen_tps"`
}

// FrozenTP is one (rule_id, location) pair that a future run must reproduce.
type FrozenTP struct {
	RuleID string `yaml:"rule_id"`
	Repo   string `yaml:"repo"`
	File   string `yaml:"file"`
	Line   int    `yaml:"line,omitempty"`
	Note   string `yaml:"note,omitempty"`
}

// LoadSuite parses a suite YAML from path.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite %s: %w", path, err)
	}
	return ParseSuite(data, filepath.Base(path))
}

// ParseSuite parses suite YAML from in-memory bytes. The label is used in
// error messages to identify the suite when the YAML didn't come from disk.
func ParseSuite(data []byte, label string) (*Suite, error) {
	s := &Suite{}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	if err := s.validate(label); err != nil {
		return nil, err
	}
	return s, nil
}

// LoadAll loads every *.yaml file in the given directory as a Suite.
// Returns the suites keyed by Module name.
func LoadAll(dir string) (map[string]*Suite, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read suite dir %s: %w", dir, err)
	}
	out := map[string]*Suite{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			// Skip README-shaped or hidden files.
			continue
		}
		s, err := LoadSuite(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if _, dup := out[s.Module]; dup {
			return nil, fmt.Errorf("duplicate module %q in suite dir %s", s.Module, dir)
		}
		out[s.Module] = s
	}
	return out, nil
}

// Finding is the runtime shape the suite compares against. Callers convert
// their domain-specific finding type into this struct before calling Check.
type Finding struct {
	RuleID string
	Repo   string
	File   string
	Line   int
}

// Report is the outcome of running Check against a set of findings.
type Report struct {
	Module          string
	TotalFrozen     int
	MissingTPs      []FrozenTP
	UnexpectedFires []Finding // findings that exceed the frozen set (informational only)
	MaxAllowedLoss  int
	Failed          bool
}

// Check compares findings against the frozen suite. A FrozenTP counts as
// present when there's a matching (RuleID, Repo, File) in findings; Line is
// matched when both sides provide it. Returns a Report describing missing
// TPs and whether the failure threshold was crossed.
func (s *Suite) Check(findings []Finding) *Report {
	report := &Report{
		Module:         s.Module,
		TotalFrozen:    len(s.FrozenTPs),
		MaxAllowedLoss: s.MaxTPLoss,
	}
	for _, frozen := range s.FrozenTPs {
		if !findingsContain(findings, frozen) {
			report.MissingTPs = append(report.MissingTPs, frozen)
		}
	}
	report.Failed = len(report.MissingTPs) > s.MaxTPLoss
	return report
}

// validate checks the suite is internally consistent.
func (s *Suite) validate(label string) error {
	if s.SchemaVersion != 1 {
		return fmt.Errorf("%s: unsupported schema_version %d (expected 1)", label, s.SchemaVersion)
	}
	if s.Module == "" {
		return fmt.Errorf("%s: missing module field", label)
	}
	if s.MaxTPLoss < 0 {
		return fmt.Errorf("%s: max_tp_loss must be non-negative", label)
	}
	seen := map[string]bool{}
	for i, tp := range s.FrozenTPs {
		if tp.RuleID == "" || tp.File == "" {
			return fmt.Errorf("%s: frozen_tps[%d] missing rule_id or file", label, i)
		}
		key := frozenKey(tp)
		if seen[key] {
			return fmt.Errorf("%s: frozen_tps[%d] duplicates an earlier entry (%s)", label, i, key)
		}
		seen[key] = true
	}
	// Sort for deterministic Report.MissingTPs ordering.
	sort.Slice(s.FrozenTPs, func(i, j int) bool {
		return frozenKey(s.FrozenTPs[i]) < frozenKey(s.FrozenTPs[j])
	})
	return nil
}

func findingsContain(findings []Finding, frozen FrozenTP) bool {
	for _, f := range findings {
		if f.RuleID != frozen.RuleID {
			continue
		}
		if f.Repo != "" && frozen.Repo != "" && f.Repo != frozen.Repo {
			continue
		}
		if f.File != frozen.File {
			continue
		}
		if frozen.Line != 0 && f.Line != 0 && frozen.Line != f.Line {
			continue
		}
		return true
	}
	return false
}

func frozenKey(tp FrozenTP) string {
	return fmt.Sprintf("%s|%s|%s|%d", tp.RuleID, tp.Repo, tp.File, tp.Line)
}
