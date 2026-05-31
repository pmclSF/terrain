// Package suppression implements the user-facing suppression model that
// honors `.terrain/suppressions.yaml`. A suppression entry tells Terrain
// to drop a finding (or a class of findings) from gating output, with
// metadata for review:
//
//	schema_version: "1"
//	suppressions:
//	  - finding_id: "weakAssertion@internal/legacy/old.go:TestFoo#a1b2c3d4"
//	    reason: "false positive; sanitized upstream"
//	    expires: "2099-08-01"
//	    owner: "@platform-team"
//
//	  - signal_type: "aiPromptInjectionRisk"
//	    file: "internal/legacy/**"
//	    reason: "rewriting this layer in 0.3"
//	    expires: "2099-09-01"
//
// Each entry matches via one of two paths:
//
//   - exact `finding_id` match — most precise; survives line drift if
//     the underlying signal has a stable symbol (per
//     `internal/identity.BuildFindingID` semantics)
//   - `signal_type` + `file` glob match — coarser; useful for
//     class-wide suppressions (e.g. "ignore prompt-injection findings
//     in the legacy layer until rewrite")
//
// Suppression metadata required for a usable adoption workflow:
//
//   - `reason` — required, free text. Reviewable; printed when a
//     suppressed signal would otherwise have been blocking.
//   - `expires` — optional ISO 8601 date. After the date, the
//     suppression is treated as INVALID and the signal fires again,
//     plus a `suppressionExpired` warning surfaces in the report.
//     Missing `expires` is allowed for permanent waivers but
//     discouraged.
//   - `owner` — optional. Free-text owner pointer for review.
//
// Anti-goal: suppressions are NOT a free-form "ignore everything"
// switch. The schema deliberately rejects entries that match neither
// `finding_id` nor `signal_type` + `file` — broad suppressions need
// to be expressed in policy, not in this file.
package suppression

import (
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/pmclSF/terrain/internal/aliases"
	"github.com/pmclSF/terrain/internal/models"
)

// pathPkgMatch is `path.Match` with Unix slash semantics — used by
// pathMatch after normalizing inputs to forward-slashes. Distinct
// from filepath.Match which is host-OS-aware (and would treat `\`
// as the separator on Windows, breaking forward-slashed patterns).
func pathPkgMatch(pattern, name string) (bool, error) {
	return pathpkg.Match(pattern, name)
}

// Default location for the suppression file, relative to the repo root.
const DefaultPath = ".terrain/suppressions.yaml"

// Scope classifies the breadth of a suppression entry. The scope
// label is documentary on the file (the user declares intent) and
// drives the default-expiry policy in the suppress CLI:
//
//	ScopeInstance  — rule_id + file + content_hash. One specific
//	                 finding. Default expiry: +90 days. The hash
//	                 invalidates on edits to the suppressed line so
//	                 the suppression doesn't outlive its rationale.
//	ScopeFile      — rule_id + file (exact). Every finding of the
//	                 rule in that file is suppressed. Default expiry:
//	                 +180 days.
//	ScopeDirectory — rule_id + file (glob). Every finding of the rule
//	                 across the matched paths is suppressed. Default
//	                 expiry: +180 days.
//	ScopeRepo      — rule_id only (no file). Disable the rule for the
//	                 whole repository. Default expiry: +365 days.
//
// A FindingID-based entry has no explicit scope; its match shape is
// equivalent to ScopeInstance (one specific finding).
type Scope string

const (
	ScopeInstance  Scope = "instance"
	ScopeFile      Scope = "file"
	ScopeDirectory Scope = "directory"
	ScopeRepo      Scope = "repo"
)

// Entry is one suppression rule. Either FindingID (exact match) or
// SignalType (+ optional File / ContentHash) must be set; the loader
// rejects entries that satisfy neither.
//
// Schema v1 (legacy): `finding_id` OR (`signal_type` AND `file`).
// Schema v2 (current): adds `scope` (documentary; default-expiry
// hint) and `content_hash` (SHA-256 of the 5-line normalized context
// window — when set, the matcher recomputes the current hash and
// requires equality, so the suppression invalidates when the line
// changes).
//
// Loading is backwards-compatible: a v1 file loads as v2 entries with
// empty `Scope` and `ContentHash`. The match-time semantics are
// unchanged for the unset case.
type Entry struct {
	FindingID   string    `yaml:"finding_id,omitempty"`
	SignalType  string    `yaml:"signal_type,omitempty"`
	File        string    `yaml:"file,omitempty"`         // glob pattern
	Scope       Scope     `yaml:"scope,omitempty"`        // optional; documentary + drives default-expiry
	ContentHash string    `yaml:"content_hash,omitempty"` // SHA-256 of 5-line normalized context
	Reason      string    `yaml:"reason"`
	Expires     string    `yaml:"expires,omitempty"` // ISO 8601 date
	Owner       string    `yaml:"owner,omitempty"`
	expiresAt   time.Time // parsed during load; zero for "no expiry"
}

// File is the YAML envelope. SchemaVersion is "1" (legacy) or "2"
// (current); both load through the same path.
type File struct {
	SchemaVersion string  `yaml:"schema_version"`
	Suppressions  []Entry `yaml:"suppressions"`
}

// CurrentSchemaVersion is what new files should declare. Reader
// accepts "1" and "2" (and empty, treated as "2").
const CurrentSchemaVersion = "2"

// DefaultExpiryForScope returns the recommended default expiry
// duration for a scope. The suppress CLI uses this when the user
// doesn't pass an explicit --expires. The schema does not enforce
// these — adopters can override per-entry.
//
//	instance  → 90 days
//	file      → 180 days
//	directory → 180 days
//	repo      → 365 days
//	(unknown) → 90 days (conservative)
func DefaultExpiryForScope(s Scope) time.Duration {
	switch s {
	case ScopeRepo:
		return 365 * 24 * time.Hour
	case ScopeFile, ScopeDirectory:
		return 180 * 24 * time.Hour
	case ScopeInstance:
		return 90 * 24 * time.Hour
	}
	return 90 * 24 * time.Hour
}

// LoadResult is what callers actually use: validated entries + per-
// entry parse errors that didn't prevent loading.
type LoadResult struct {
	Entries  []Entry
	Warnings []string // non-fatal issues (e.g. unparseable expiry date)
}

// Load reads the suppression file at `path`. Returns nil + nil error
// when the file doesn't exist (no suppressions = legitimate state).
// Returns a structured error for parse / schema failures.
func Load(path string) (*LoadResult, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &LoadResult{}, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	var raw File
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	switch raw.SchemaVersion {
	case "", "1", "2":
		// supported
	default:
		return nil, fmt.Errorf("unsupported suppressions schema_version %q (expected \"1\" or \"2\")", raw.SchemaVersion)
	}

	result := &LoadResult{}
	for i, e := range raw.Suppressions {
		// Validate entry shape: exactly one matching mode must be set.
		hasID := strings.TrimSpace(e.FindingID) != ""
		hasType := strings.TrimSpace(e.SignalType) != ""
		hasFile := strings.TrimSpace(e.File) != ""
		isRepoScope := e.Scope == ScopeRepo
		switch {
		case hasID && (hasType || hasFile):
			return nil, fmt.Errorf("suppressions[%d]: cannot combine finding_id with signal_type/file — use one or the other", i)
		case !hasID && !hasType:
			return nil, fmt.Errorf("suppressions[%d]: must set either finding_id, or signal_type (plus file unless scope=repo)", i)
		case !hasID && hasType && !hasFile && !isRepoScope:
			return nil, fmt.Errorf("suppressions[%d]: signal_type entries require either a file pattern or scope: repo", i)
		}
		// Validate Scope value (when provided).
		if e.Scope != "" {
			switch e.Scope {
			case ScopeInstance, ScopeFile, ScopeDirectory, ScopeRepo:
				// ok
			default:
				return nil, fmt.Errorf("suppressions[%d]: unknown scope %q (expected: instance, file, directory, repo)", i, e.Scope)
			}
		}
		// ContentHash validation: requires SignalType + File.
		if strings.TrimSpace(e.ContentHash) != "" {
			if !hasType || !hasFile {
				return nil, fmt.Errorf("suppressions[%d]: content_hash requires signal_type and file", i)
			}
		}

		// reason is required — every suppression must justify itself.
		if strings.TrimSpace(e.Reason) == "" {
			return nil, fmt.Errorf("suppressions[%d]: reason is required", i)
		}

		// Parse expires; warn (not error) on unparseable so a single
		// bad date doesn't reject the whole file.
		if strings.TrimSpace(e.Expires) != "" {
			t, err := time.Parse("2006-01-02", e.Expires)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("suppressions[%d]: unparseable expires %q (expected YYYY-MM-DD); treating as no expiry", i, e.Expires))
			} else {
				e.expiresAt = t
			}
		}

		result.Entries = append(result.Entries, e)
	}

	return result, nil
}

// Apply removes signals that match an active (unexpired) suppression
// entry from the snapshot. Returns the list of suppressions that
// matched at least one signal (so the report can show them) plus the
// list of expired entries that need a warning surface.
//
// Apply mutates the snapshot in place. Suppressed signals are removed
// from both `snapshot.Signals` and per-test-file `TestFile.Signals`.
//
// Order of evaluation:
//  1. Drop expired entries (and surface them as warnings).
//  2. For each remaining entry, walk every signal; if the entry
//     matches, mark the signal for removal and record the match.
//  3. Rewrite the signal slices without matched signals.
//
// `now` is injected so tests can drive expiry deterministically.
//
// Calls ApplyWithAliases with a nil registry — `signal_type` matches
// require literal-string equality. Callers that want a renamed-rule
// suppression to also suppress the new ID during the deprecation
// window should use ApplyWithAliases.
func Apply(snapshot *models.TestSuiteSnapshot, entries []Entry, now time.Time) (matched []Entry, expired []Entry) {
	return ApplyWithAliases(snapshot, entries, nil, now)
}

// ApplyWithAliases is the alias-aware form of Apply. When `reg` is
// non-nil, `signal_type` entries match any signal whose Type is in
// `reg.ExpandOldID(e.SignalType)`. The expansion is one-way: a
// suppression on the OLD rule_id continues to suppress findings
// emitted under the NEW rule_id(s) during the deprecation window.
// The reverse (suppression on new ID auto-suppresses findings still
// labeled old) is deliberately NOT supported — adopters who write
// suppressions against new IDs are post-migration; the OLD ID will
// stop firing before the alias entry is removed.
func ApplyWithAliases(snapshot *models.TestSuiteSnapshot, entries []Entry, reg *aliases.Registry, now time.Time) (matched []Entry, expired []Entry) {
	if snapshot == nil {
		return nil, nil
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Partition into active vs expired.
	active := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			expired = append(expired, e)
			continue
		}
		active = append(active, e)
	}

	if len(active) == 0 {
		return nil, expired
	}

	// Pre-expand each entry's SignalType into a set so the inner loop
	// is O(1) per signal-type compare instead of O(n_aliases).
	expanded := make([]map[string]bool, len(active))
	for i, e := range active {
		if e.SignalType == "" {
			continue
		}
		set := map[string]bool{e.SignalType: true}
		if reg != nil {
			for _, id := range reg.ExpandOldID(e.SignalType) {
				set[id] = true
			}
		}
		expanded[i] = set
	}

	matchedIdx := make(map[int]bool, len(active))

	snapshot.Signals = filterSignals(snapshot.Signals, active, expanded, matchedIdx)
	for fi := range snapshot.TestFiles {
		tf := &snapshot.TestFiles[fi]
		tf.Signals = filterSignals(tf.Signals, active, expanded, matchedIdx)
	}

	for i, e := range active {
		if matchedIdx[i] {
			matched = append(matched, e)
		}
	}
	return matched, expired
}

// hashCache memoizes ContextHash computations across a single
// filterSignals invocation. Signals with the same (file, line) share
// a single hash compute. Built per-call so it doesn't leak across
// snapshots or test runs.
type hashCache struct {
	cache map[string]string
}

func newHashCache() *hashCache { return &hashCache{cache: map[string]string{}} }

// Get returns the cached ContextHash for (file, line), computing once
// on the first request. Errors propagate from the underlying
// ContextHash call (file I/O errors); a non-existent file returns the
// sentinel empty string with nil error.
func (c *hashCache) Get(file string, line int) (string, error) {
	if file == "" || line <= 0 {
		return "", nil
	}
	key := fmt.Sprintf("%s:%d", file, line)
	if h, ok := c.cache[key]; ok {
		return h, nil
	}
	h, err := ContextHash(file, line)
	if err != nil {
		return "", err
	}
	c.cache[key] = h
	return h, nil
}

func filterSignals(signals []models.Signal, active []Entry, expanded []map[string]bool, matchedIdx map[int]bool) []models.Signal {
	if len(signals) == 0 {
		return signals
	}
	cache := newHashCache()
	kept := signals[:0]
	for _, s := range signals {
		hitIdx := -1
		for i, e := range active {
			if matches(e, s, expanded[i], cache) {
				hitIdx = i
				break
			}
		}
		if hitIdx >= 0 {
			matchedIdx[hitIdx] = true
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// matches returns true if entry e suppresses signal s.
//
//   - `expandedTypes` is the pre-computed set of signal-type strings
//     that count as a hit for e (the literal e.SignalType plus any
//     new IDs from the alias registry). Nil/empty set falls back to
//     literal-equality on e.SignalType.
//   - `cache` memoizes ContextHash computations across the
//     filterSignals invocation. Required only when one or more
//     entries set ContentHash; passed unconditionally to keep the
//     signature stable.
//
// When the entry sets ContentHash, the matcher recomputes the
// current context hash at (s.Location.File, s.Location.Line) and
// requires equality. A non-existent file silently fails the hash
// check (returns "" sentinel) so the suppression doesn't fire on a
// finding for a file that no longer exists.
func matches(e Entry, s models.Signal, expandedTypes map[string]bool, cache *hashCache) bool {
	if e.FindingID != "" {
		return e.FindingID == s.FindingID
	}
	// SignalType match (with alias expansion if provided).
	if expandedTypes != nil {
		if !expandedTypes[string(s.Type)] {
			return false
		}
	} else if string(s.Type) != e.SignalType {
		return false
	}
	// File match: required unless scope is repo-wide.
	if e.File == "" {
		// scope=repo: rule_id only, applies everywhere. Schema v1
		// rejected this shape; v2 allows it via explicit scope.
		if e.Scope != ScopeRepo {
			return false
		}
	} else {
		matched, err := pathMatch(e.File, s.Location.File)
		if err != nil || !matched {
			return false
		}
	}
	// Content-hash check (when the entry pins one).
	if e.ContentHash != "" {
		if cache == nil {
			return false
		}
		current, err := cache.Get(s.Location.File, s.Location.Line)
		if err != nil || current == "" {
			return false
		}
		if current != e.ContentHash {
			return false
		}
	}
	return true
}

// pathMatch is a glob match that supports `**` (recursive) on top of
// the standard `path.Match` semantics. Patterns are matched against
// the signal's file path verbatim — callers should normalize paths
// before storing them.
//
// We use the `path` package (Unix semantics) rather than `filepath`
// (host-OS-aware) because Terrain canonicalizes every path to
// forward-slashes. filepath.Match on Windows treats `\` as the
// separator, breaking patterns like `*.go` that should not cross
// path boundaries when input is forward-slashed.
func pathMatch(pattern, path string) (bool, error) {
	// Forward-slashes are canonical in Terrain paths.
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	if !strings.Contains(pattern, "**") {
		return pathPkgMatch(pattern, path)
	}

	// Translate `**` into a regex-equivalent walk: any sequence of path
	// segments. We compile to a series of segment-by-segment matches.
	patSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")
	return matchSegments(patSegs, pathSegs), nil
}

func matchSegments(pat, path []string) bool {
	pi, ti := 0, 0
	starStarPi := -1
	starStarTi := 0
	for ti < len(path) {
		if pi < len(pat) {
			if pat[pi] == "**" {
				starStarPi = pi
				starStarTi = ti
				pi++
				continue
			}
			ok, err := pathPkgMatch(pat[pi], path[ti])
			if err == nil && ok {
				pi++
				ti++
				continue
			}
		}
		if starStarPi >= 0 {
			pi = starStarPi + 1
			starStarTi++
			ti = starStarTi
			continue
		}
		return false
	}
	for pi < len(pat) {
		if pat[pi] != "**" {
			return false
		}
		pi++
	}
	return true
}
