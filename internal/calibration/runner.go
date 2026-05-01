package calibration

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
)

// wilson95 is the Wilson score 95% confidence interval for a
// proportion. Inlined here (rather than importing internal/airun) so
// the calibration package stays a leaf dependency. Mirrors
// airun.WilsonInterval95; tested by airun's confidence_test.go.
func wilson95(successes, total int) (float64, float64) {
	if total <= 0 {
		return 0, 1
	}
	if successes < 0 {
		successes = 0
	}
	if successes > total {
		successes = total
	}
	const z = 1.959964
	n := float64(total)
	pHat := float64(successes) / n
	z2 := z * z

	denom := 1 + z2/n
	center := (pHat + z2/(2*n)) / denom
	margin := z * math.Sqrt(pHat*(1-pHat)/n+z2/(4*n*n)) / denom

	lo := center - margin
	hi := center + margin
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// Match outcomes for a single (Type, File) pair across emitted vs
// expected signals.
const (
	OutcomeTruePositive  = "TP"
	OutcomeFalsePositive = "FP"
	OutcomeFalseNegative = "FN"
)

// Match is one expected/emitted comparison.
type Match struct {
	Type    models.SignalType
	File    string
	Outcome string // TP / FP / FN
	Notes   string // from labels.yaml when available
}

// FixtureResult is the per-fixture outcome of a calibration run.
type FixtureResult struct {
	Fixture string
	Path    string
	Matches []Match
}

// CountByOutcome groups matches by TP/FP/FN for the fixture.
func (r FixtureResult) CountByOutcome() map[string]int {
	out := map[string]int{
		OutcomeTruePositive:  0,
		OutcomeFalsePositive: 0,
		OutcomeFalseNegative: 0,
	}
	for _, m := range r.Matches {
		out[m.Outcome]++
	}
	return out
}

// CorpusResult aggregates fixture results into per-detector and overall
// precision/recall. PrecisionByType / RecallByType are 0..1; they are
// not defined when a detector has zero positives in the denominator.
type CorpusResult struct {
	Fixtures []FixtureResult

	// Per-detector counts, summed across the corpus.
	TP map[models.SignalType]int
	FP map[models.SignalType]int
	FN map[models.SignalType]int
}

// PrecisionByType returns precision for each detector type that has at
// least one TP+FP. Detectors with no positives at all are omitted.
func (c CorpusResult) PrecisionByType() map[models.SignalType]float64 {
	out := map[models.SignalType]float64{}
	for typ, tp := range c.TP {
		denom := tp + c.FP[typ]
		if denom == 0 {
			continue
		}
		out[typ] = float64(tp) / float64(denom)
	}
	return out
}

// RecallByType returns recall for each detector type that has at least
// one TP+FN. Detectors with no expected fires are omitted.
func (c CorpusResult) RecallByType() map[models.SignalType]float64 {
	out := map[models.SignalType]float64{}
	for typ, tp := range c.TP {
		denom := tp + c.FN[typ]
		if denom == 0 {
			continue
		}
		out[typ] = float64(tp) / float64(denom)
	}
	return out
}

// MetricInterval pairs a point estimate with a Wilson 95% interval.
// Used by PrecisionByTypeInterval / RecallByTypeInterval so detector
// metrics carry uncertainty alongside the headline number.
type MetricInterval struct {
	Value        float64
	IntervalLow  float64
	IntervalHigh float64
	Successes    int
	Total        int
}

// PrecisionByTypeInterval returns Wilson 95% intervals for per-detector
// precision. Detectors with zero TP+FP are omitted (no data to bracket).
//
// The interval narrows as the corpus grows: with 1 fixture per detector
// the bounds will be wide, near-uninformative; at 25-50 fixtures (the
// 0.2 corpus target) bounds become useful.
func (c CorpusResult) PrecisionByTypeInterval() map[models.SignalType]MetricInterval {
	out := map[models.SignalType]MetricInterval{}
	for typ, tp := range c.TP {
		fp := c.FP[typ]
		total := tp + fp
		if total == 0 {
			continue
		}
		lo, hi := wilson95(tp, total)
		out[typ] = MetricInterval{
			Value:        float64(tp) / float64(total),
			IntervalLow:  lo,
			IntervalHigh: hi,
			Successes:    tp,
			Total:        total,
		}
	}
	return out
}

// RecallByTypeInterval returns Wilson 95% intervals for per-detector
// recall. See PrecisionByTypeInterval for semantics.
func (c CorpusResult) RecallByTypeInterval() map[models.SignalType]MetricInterval {
	out := map[models.SignalType]MetricInterval{}
	for typ, tp := range c.TP {
		fn := c.FN[typ]
		total := tp + fn
		if total == 0 {
			continue
		}
		lo, hi := wilson95(tp, total)
		out[typ] = MetricInterval{
			Value:        float64(tp) / float64(total),
			IntervalLow:  lo,
			IntervalHigh: hi,
			Successes:    tp,
			Total:        total,
		}
	}
	return out
}

// SortedDetectorTypes returns every detector mentioned anywhere in the
// corpus result, in stable alphabetical order. Useful for deterministic
// report rendering.
func (c CorpusResult) SortedDetectorTypes() []models.SignalType {
	seen := map[models.SignalType]bool{}
	for typ := range c.TP {
		seen[typ] = true
	}
	for typ := range c.FP {
		seen[typ] = true
	}
	for typ := range c.FN {
		seen[typ] = true
	}
	out := make([]models.SignalType, 0, len(seen))
	for typ := range seen {
		out = append(out, typ)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// AnalyzerFunc runs Terrain's analyse pipeline against a fixture path
// and returns the emitted Signals. Injected by callers so the package
// is decoupled from the engine import (avoids cycles).
type AnalyzerFunc func(fixturePath string) ([]models.Signal, error)

// FindFixtures walks a directory tree and returns every directory that
// contains a `labels.yaml`. Sorted for determinism.
func FindFixtures(corpusRoot string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(corpusRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "labels.yaml")); statErr == nil {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

// Run executes the calibration runner against every fixture under
// corpusRoot and returns aggregated precision/recall.
//
// Matching is on (Type, File). Line/Symbol from the label are not used
// for matching today — they are advisory and shown in mismatch reports.
// This trades label maintainability (line numbers shift on edits) for
// recall accuracy on noisy-line-number detectors.
func Run(corpusRoot string, analyse AnalyzerFunc) (CorpusResult, error) {
	dirs, err := FindFixtures(corpusRoot)
	if err != nil {
		return CorpusResult{}, fmt.Errorf("find fixtures under %s: %w", corpusRoot, err)
	}

	result := CorpusResult{
		TP: map[models.SignalType]int{},
		FP: map[models.SignalType]int{},
		FN: map[models.SignalType]int{},
	}

	for _, fixtureDir := range dirs {
		labels, err := LoadLabels(fixtureDir)
		if err != nil {
			return result, err
		}

		signals, err := analyse(fixtureDir)
		if err != nil {
			return result, fmt.Errorf("analyse %s: %w", fixtureDir, err)
		}

		fr := matchFixture(*labels, signals, fixtureDir)
		result.Fixtures = append(result.Fixtures, fr)
		for _, m := range fr.Matches {
			switch m.Outcome {
			case OutcomeTruePositive:
				result.TP[m.Type]++
			case OutcomeFalsePositive:
				result.FP[m.Type]++
			case OutcomeFalseNegative:
				result.FN[m.Type]++
			}
		}
	}

	return result, nil
}

// matchFixture is the (Type, File) matching algorithm.
//
// For each emitted signal:
//   - if a label.Expected entry has same Type AND File: TP, consume the label
//   - if an ExpectedAbsent entry has same Type AND File: FP (false positive)
//   - otherwise: silent (out-of-scope detection — corpus doesn't claim either way)
//
// Each unconsumed Expected entry then counts as FN.
func matchFixture(labels FixtureLabels, signals []models.Signal, fixtureDir string) FixtureResult {
	out := FixtureResult{
		Fixture: labels.Fixture,
		Path:    fixtureDir,
	}

	consumed := make([]bool, len(labels.Expected))
	expectedKey := func(e ExpectedSignal) string {
		return string(e.Type) + "\x00" + e.File
	}

	// Detectors that ingest external artifacts (eval framework outputs)
	// stamp the absolute path of the artifact into Signal.Location.File.
	// Labels list paths relative to the fixture directory, so we strip
	// the fixture prefix before matching.
	relSignalFile := func(sigFile string) string {
		if sigFile == "" {
			return ""
		}
		if rel, err := filepath.Rel(fixtureDir, sigFile); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
		return sigFile
	}

	for _, sig := range signals {
		// Try to match against an expected (positive) label.
		matched := false
		sigFile := relSignalFile(sig.Location.File)
		for i, exp := range labels.Expected {
			if consumed[i] {
				continue
			}
			if expectedKey(exp) == string(sig.Type)+"\x00"+sigFile {
				consumed[i] = true
				out.Matches = append(out.Matches, Match{
					Type:    sig.Type,
					File:    sigFile,
					Outcome: OutcomeTruePositive,
					Notes:   exp.Notes,
				})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Check for explicit false-positive guard.
		for _, abs := range labels.ExpectedAbsent {
			if expectedKey(abs) == string(sig.Type)+"\x00"+sig.Location.File {
				out.Matches = append(out.Matches, Match{
					Type:    sig.Type,
					File:    sig.Location.File,
					Outcome: OutcomeFalsePositive,
					Notes:   abs.Notes,
				})
				break
			}
		}
		// Otherwise: out-of-scope — corpus doesn't claim either way.
	}

	// Unconsumed expected entries are false negatives.
	for i, exp := range labels.Expected {
		if !consumed[i] {
			out.Matches = append(out.Matches, Match{
				Type:    exp.Type,
				File:    exp.File,
				Outcome: OutcomeFalseNegative,
				Notes:   exp.Notes,
			})
		}
	}

	return out
}
