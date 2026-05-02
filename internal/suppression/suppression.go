// Package suppression implements the user-facing suppression model that
// honors `.terrain/suppressions.yaml`. A suppression entry tells Terrain
// to drop a finding (or a class of findings) from gating output, with
// metadata for review:
//
//	schema_version: "1"
//	suppressions:
//	  - finding_id: "weakAssertion@internal/legacy/old.go:TestFoo#a1b2c3d4"
//	    reason: "false positive; sanitized upstream"
//	    expires: "2026-08-01"
//	    owner: "@platform-team"
//
//	  - signal_type: "aiPromptInjectionRisk"
//	    file: "internal/legacy/**"
//	    reason: "rewriting this layer in 0.3"
//	    expires: "2026-09-01"
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
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/pmclSF/terrain/internal/models"
)

// Default location for the suppression file, relative to the repo root.
const DefaultPath = ".terrain/suppressions.yaml"

// Entry is one suppression rule. Either FindingID (exact match) or
// SignalType + File (class match) must be set; the loader rejects
// entries that satisfy neither.
type Entry struct {
	FindingID  string    `yaml:"finding_id,omitempty"`
	SignalType string    `yaml:"signal_type,omitempty"`
	File       string    `yaml:"file,omitempty"` // glob pattern
	Reason     string    `yaml:"reason"`
	Expires    string    `yaml:"expires,omitempty"` // ISO 8601 date
	Owner      string    `yaml:"owner,omitempty"`
	expiresAt  time.Time // parsed during load; zero for "no expiry"
}

// File is the YAML envelope.
type File struct {
	SchemaVersion string  `yaml:"schema_version"`
	Suppressions  []Entry `yaml:"suppressions"`
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
	if raw.SchemaVersion != "" && raw.SchemaVersion != "1" {
		return nil, fmt.Errorf("unsupported suppressions schema_version %q (expected \"1\")", raw.SchemaVersion)
	}

	result := &LoadResult{}
	for i, e := range raw.Suppressions {
		// Validate entry shape: exactly one matching mode must be set.
		hasID := strings.TrimSpace(e.FindingID) != ""
		hasType := strings.TrimSpace(e.SignalType) != ""
		hasFile := strings.TrimSpace(e.File) != ""
		switch {
		case hasID && (hasType || hasFile):
			return nil, fmt.Errorf("suppressions[%d]: cannot combine finding_id with signal_type/file — use one or the other", i)
		case !hasID && !(hasType && hasFile):
			return nil, fmt.Errorf("suppressions[%d]: must set either finding_id, or both signal_type and file", i)
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
func Apply(snapshot *models.TestSuiteSnapshot, entries []Entry, now time.Time) (matched []Entry, expired []Entry) {
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

	matchedIdx := make(map[int]bool, len(active))

	snapshot.Signals = filterSignals(snapshot.Signals, active, matchedIdx)
	for fi := range snapshot.TestFiles {
		tf := &snapshot.TestFiles[fi]
		tf.Signals = filterSignals(tf.Signals, active, matchedIdx)
	}

	for i, e := range active {
		if matchedIdx[i] {
			matched = append(matched, e)
		}
	}
	return matched, expired
}

func filterSignals(signals []models.Signal, active []Entry, matchedIdx map[int]bool) []models.Signal {
	if len(signals) == 0 {
		return signals
	}
	kept := signals[:0]
	for _, s := range signals {
		hitIdx := -1
		for i, e := range active {
			if matches(e, s) {
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
func matches(e Entry, s models.Signal) bool {
	if e.FindingID != "" {
		return e.FindingID == s.FindingID
	}
	// signal_type + file path match.
	if string(s.Type) != e.SignalType {
		return false
	}
	if e.File == "" {
		return false
	}
	matched, err := pathMatch(e.File, s.Location.File)
	if err != nil {
		return false
	}
	return matched
}

// pathMatch is a glob match that supports `**` (recursive) on top of
// the standard `filepath.Match` semantics. Patterns are matched
// against the signal's file path verbatim — callers should normalize
// paths before storing them.
func pathMatch(pattern, path string) (bool, error) {
	// Forward-slashes are canonical in Terrain paths.
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, path)
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
			ok, err := filepath.Match(pat[pi], path[ti])
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
