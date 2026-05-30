// Package findinghistory implements per-repo learning: when a
// `(rule_id, file_path)` pair fires repeatedly across PRs without
// being dismissed, Terrain auto-demotes that pair from the inline
// PR comment to the observability footer. The detector keeps firing
// — visibility learns.
//
// Contract: when `(repo, rule_id, file_path)` fires ≥3 times across
// PRs without `/dismiss`, switch that rule+file from inline-comment
// to observability-footer for that file. Detector still fires;
// visibility learns. No LLM. Compounds across weeks of use — the
// strongest single thing that prevents long-tail fatigue.
//
// Storage: a single YAML file at `.terrain/finding-history.yaml` in
// the repo. Schema v1. The pipeline reads on each run and writes
// the updated counts at the end.
//
// Threshold: 3 fires. Tunable via `WithThreshold(n)` but the spec's
// default is 3 — high enough to avoid noise-driven demotion, low
// enough to react within a typical 1-2 week PR cadence.
//
// Dismissal: a `/dismiss` on a finding resets the demoted state for
// that (rule_id, file_path) — the dismissal is the user's signal
// "this is a real finding I'm choosing to suppress with a reason,"
// distinct from visibility-fatigue demotion.
package findinghistory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultThreshold is the fire-count threshold above which a
// (rule_id, file_path) auto-demotes to observability. Adjusted via
// Store.SetThreshold; defaults to 3.
const DefaultThreshold = 3

// DefaultPath is the on-disk location relative to the repo root.
const DefaultPath = ".terrain/finding-history.yaml"

// CurrentSchemaVersion is the schema string new files declare.
const CurrentSchemaVersion = "1"

// Entry is one tracked (rule_id, file_path) pair.
type Entry struct {
	// RuleID is the detector's stable rule_id (e.g. "aiPromptInjectionRisk").
	RuleID string `yaml:"rule_id"`
	// File is the repo-relative file path the rule fired on.
	File string `yaml:"file"`
	// Fires is the number of times the pair fired across PR runs.
	Fires int `yaml:"fires"`
	// LastFire is the ISO date of the most recent fire.
	LastFire string `yaml:"last_fire,omitempty"`
	// LastDismiss is the ISO date of the most recent /dismiss. When
	// LastDismiss >= LastFire, the pair is considered "actively
	// dismissed" and renders inline regardless of fire count.
	LastDismiss string `yaml:"last_dismiss,omitempty"`
}

// File is the YAML envelope.
type File struct {
	SchemaVersion string  `yaml:"schema_version"`
	Entries       []Entry `yaml:"entries"`
}

// Store is the in-memory representation of the on-disk history file
// plus the threshold + clock used for ShouldDemote decisions.
//
// Thread-safety: Increment/Dismiss/Save take a mutex so the pipeline
// can call them from multiple goroutines (though the typical
// invocation is sequential at end-of-pipeline).
type Store struct {
	threshold int
	// now is the clock used to stamp LastFire / LastDismiss.
	// Defaulted to time.Now; tests inject a fixed clock.
	now      func() time.Time
	mu       sync.Mutex
	entries  map[string]*Entry // keyed by ruleID + "::" + file
	loadedAt time.Time         // documentary
}

// New returns an empty Store with the default threshold.
func New() *Store {
	return &Store{
		threshold: DefaultThreshold,
		now:       time.Now,
		entries:   map[string]*Entry{},
		loadedAt:  time.Now(),
	}
}

// Load reads the history file at `path`. Returns (empty Store, nil)
// when the file doesn't exist — that's a legitimate "first PR ever"
// state. Returns a structured error for parse / schema failures.
func Load(path string) (*Store, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return nil, fmt.Errorf("findinghistory: read %q: %w", path, err)
	}
	return LoadFromBytes(body)
}

// LoadFromBytes parses an in-memory history payload. Tests use this
// directly to inject fixture history.
func LoadFromBytes(data []byte) (*Store, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("findinghistory: parse: %w", err)
	}
	switch f.SchemaVersion {
	case "", "1":
		// supported
	default:
		return nil, fmt.Errorf("findinghistory: unsupported schema_version %q (expected %q)", f.SchemaVersion, CurrentSchemaVersion)
	}
	s := New()
	for i, e := range f.Entries {
		if strings.TrimSpace(e.RuleID) == "" {
			return nil, fmt.Errorf("findinghistory: entries[%d]: rule_id required", i)
		}
		if strings.TrimSpace(e.File) == "" {
			return nil, fmt.Errorf("findinghistory: entries[%d]: file required", i)
		}
		entry := e
		s.entries[key(e.RuleID, e.File)] = &entry
	}
	return s, nil
}

// SetThreshold overrides the demote threshold (default 3). Passing
// 0 or negative resets to DefaultThreshold.
func (s *Store) SetThreshold(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		s.threshold = DefaultThreshold
		return
	}
	s.threshold = n
}

// SetClock overrides the time source. Production code never calls
// this; tests inject a fixed time to make ISO-date stamps
// deterministic.
func (s *Store) SetClock(clock func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if clock == nil {
		s.now = time.Now
		return
	}
	s.now = clock
}

// Get returns the entry for a (ruleID, file) pair, plus a presence
// bool. Returns the zero Entry when not tracked.
func (s *Store) Get(ruleID, file string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key(ruleID, file)]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// Increment bumps the fire counter for (ruleID, file) and records
// today as LastFire. Creates the entry on first call.
func (s *Store) Increment(ruleID, file string) {
	if ruleID == "" || file == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(ruleID, file)
	e, ok := s.entries[k]
	if !ok {
		e = &Entry{RuleID: ruleID, File: file}
		s.entries[k] = e
	}
	e.Fires++
	e.LastFire = s.now().Format("2006-01-02")
}

// Dismiss records today as LastDismiss for (ruleID, file). Resets
// the demoted state — a dismissal is the user's signal "I've
// reviewed this; suppress with a reason" and overrides fatigue-
// driven demotion. The next fire that's not followed by another
// dismiss restarts the counter toward the threshold.
//
// Dismiss does NOT delete the history entry — keeping the entry
// preserves the count for analytics ("we've seen this rule fire 7
// times, dismissed twice"), even though demotion is currently
// gated by the date comparison alone.
func (s *Store) Dismiss(ruleID, file string) {
	if ruleID == "" || file == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(ruleID, file)
	e, ok := s.entries[k]
	if !ok {
		e = &Entry{RuleID: ruleID, File: file}
		s.entries[k] = e
	}
	e.LastDismiss = s.now().Format("2006-01-02")
}

// ShouldDemote returns true when (ruleID, file) has fired at or
// above the threshold AND has NOT been dismissed on-or-after its
// most recent fire. Used by the PR-comment renderer to flip a
// finding from inline → observability-footer.
func (s *Store) ShouldDemote(ruleID, file string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key(ruleID, file)]
	if !ok {
		return false
	}
	if e.Fires < s.threshold {
		return false
	}
	// Active dismissal: if the user dismissed on-or-after the most
	// recent fire, surface the next fire inline (don't auto-demote).
	if e.LastDismiss != "" && e.LastFire != "" && e.LastDismiss >= e.LastFire {
		return false
	}
	return true
}

// Save writes the in-memory store back to `path`. Creates the
// `.terrain/` directory if needed. Entries are sorted (rule_id, file)
// so the on-disk file is diff-friendly.
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("findinghistory: mkdir: %w", err)
	}
	ordered := s.snapshotLocked()
	out := File{
		SchemaVersion: CurrentSchemaVersion,
		Entries:       ordered,
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("findinghistory: marshal: %w", err)
	}
	header := "# Terrain finding history — auto-managed by the pipeline.\n" +
		"# Used to demote chronically-firing-but-not-dismissed (rule_id, file)\n" +
		"# pairs from inline PR comments to the observability footer. Safe to\n" +
		"# commit; safe to delete (deleting resets the per-pair counter).\n\n"
	body := append([]byte(header), data...)
	// Atomic write: tmp + rename. Two concurrent analyze runs racing
	// the save (e.g., a CI runner racing a local pre-commit hook)
	// would otherwise truncate-and-overwrite, and a crash between
	// truncate and write would leave the file partially written so
	// the next loader's yaml.Unmarshal silently fails.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".finding-history-*.tmp")
	if err != nil {
		return fmt.Errorf("findinghistory: tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("findinghistory: write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("findinghistory: close tmp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("findinghistory: chmod tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("findinghistory: rename: %w", err)
	}
	return nil
}

// All returns every entry, sorted by (rule_id, file). Used by the
// CLI inspection surface (`terrain debug finding-history`) and by
// tests.
func (s *Store) All() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() []Entry {
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		return out[i].File < out[j].File
	})
	return out
}

func key(ruleID, file string) string {
	return ruleID + "::" + file
}
