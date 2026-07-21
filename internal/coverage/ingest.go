package coverage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pmclSF/terrain/internal/saferead"
)

// IngestFile reads a coverage artifact file and returns normalized records.
// Supports LCOV and Istanbul JSON formats (auto-detected).
func IngestFile(artifactPath string, runLabel string) (*CoverageArtifact, error) {
	data, err := saferead.ReadFileCap(artifactPath, saferead.DataCap)
	if err != nil {
		return nil, fmt.Errorf("reading coverage artifact: %w", err)
	}

	content := string(data)
	trimmed := strings.TrimSpace(content)

	var records []CoverageRecord
	var format string

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		// Istanbul JSON comes in two shapes. coverage-summary.json (jest
		// --coverageReporters=json-summary, nyc) is aggregate-only
		// ({total:{lines:{pct}}, "<file>":{lines:{...}}}); coverage-final.json is
		// per-statement ({"<file>":{statementMap,s,...}}). They must be parsed
		// differently — feeding a summary to the per-statement parser yields
		// records with 0 lines and a FALSE 0%-coverage gate.
		if isIstanbulSummary(data) {
			records, err = parseIstanbulSummary(data)
			if err != nil {
				return nil, fmt.Errorf("parsing istanbul summary coverage: %w", err)
			}
			format = "istanbul-summary"
		} else {
			records, err = parseIstanbul(data)
			if err != nil {
				return nil, fmt.Errorf("parsing istanbul coverage: %w", err)
			}
			format = "istanbul"
		}
	} else {
		// Try LCOV.
		records, err = parseLCOV(content)
		if err != nil {
			return nil, fmt.Errorf("parsing lcov coverage: %w", err)
		}
		format = "lcov"
	}

	return &CoverageArtifact{
		Records:  records,
		RunLabel: runLabel,
		Provenance: ArtifactProvenance{
			SourceFile: artifactPath,
			Format:     format,
			RunLabel:   runLabel,
		},
	}, nil
}

// IngestDirectory scans a directory for coverage artifacts and ingests them.
// Files are parsed in parallel using a worker pool. Results are merged in
// filesystem order for deterministic output.
func IngestDirectory(dir string, runLabel string) ([]CoverageArtifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Collect candidate file paths.
	var candidatePaths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isCoverageFile(strings.ToLower(e.Name())) {
			candidatePaths = append(candidatePaths, filepath.Join(dir, e.Name()))
		}
	}

	if len(candidatePaths) == 0 {
		return nil, nil
	}

	// Single file fast path.
	if len(candidatePaths) == 1 {
		art, err := IngestFile(candidatePaths[0], runLabel)
		if err != nil {
			return nil, err
		}
		return []CoverageArtifact{*art}, nil
	}

	// Parallel ingestion: each file is independent.
	type indexedResult struct {
		art *CoverageArtifact
		err error
	}
	perFile := make([]indexedResult, len(candidatePaths))

	workers := len(candidatePaths)
	if workers > 8 {
		workers = 8
	}
	indexCh := make(chan int, len(candidatePaths))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				art, err := IngestFile(candidatePaths[idx], runLabel)
				perFile[idx] = indexedResult{art: art, err: err}
			}
		}()
	}
	for i := range candidatePaths {
		indexCh <- i
	}
	close(indexCh)
	wg.Wait()

	// Merge in deterministic (filesystem) order.
	var artifacts []CoverageArtifact
	var warnings []string
	for i, ir := range perFile {
		if ir.err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", filepath.Base(candidatePaths[i]), ir.err))
			continue
		}
		if ir.art != nil {
			artifacts = append(artifacts, *ir.art)
		}
	}

	if len(warnings) > 0 {
		if len(artifacts) == 0 {
			return nil, errors.New("failed to ingest any coverage artifacts: " + strings.Join(warnings, "; "))
		}
		return artifacts, &IngestWarning{Warnings: warnings}
	}
	return artifacts, nil
}

// IngestWarning indicates partial coverage ingestion success.
// Artifacts were ingested successfully, but one or more files were skipped.
type IngestWarning struct {
	Warnings []string
}

func (w *IngestWarning) Error() string {
	if w == nil || len(w.Warnings) == 0 {
		return ""
	}
	limit := 3
	if len(w.Warnings) < limit {
		limit = len(w.Warnings)
	}
	msg := strings.Join(w.Warnings[:limit], "; ")
	if len(w.Warnings) > limit {
		msg += fmt.Sprintf("; +%d more", len(w.Warnings)-limit)
	}
	return "coverage ingest partially succeeded: " + msg
}

func isCoverageFile(name string) bool {
	return name == "lcov.info" ||
		strings.HasSuffix(name, ".lcov") ||
		name == "coverage-final.json" ||
		name == "coverage-summary.json" ||
		(strings.HasPrefix(name, "coverage") && strings.HasSuffix(name, ".json"))
}

// --- LCOV parser ---

func parseLCOV(content string) ([]CoverageRecord, error) {
	var records []CoverageRecord
	var current *CoverageRecord

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "SF:") {
			filePath := strings.TrimPrefix(line, "SF:")
			filePath = normalizeCoveragePath(filePath)
			current = &CoverageRecord{
				FilePath:     filePath,
				LineHits:     map[int]int{},
				BranchHits:   map[string]int{},
				FunctionHits: map[string]int{},
			}
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "DA:"):
			// DA:line_number,hit_count
			parts := strings.SplitN(strings.TrimPrefix(line, "DA:"), ",", 2)
			if len(parts) == 2 {
				lineNum, _ := strconv.Atoi(parts[0])
				hits, _ := strconv.Atoi(parts[1])
				if lineNum > 0 {
					current.LineHits[lineNum] = hits
				}
			}

		case strings.HasPrefix(line, "FN:"):
			// FN:line_number,function_name
			parts := strings.SplitN(strings.TrimPrefix(line, "FN:"), ",", 2)
			if len(parts) == 2 {
				fnName := parts[1]
				if _, ok := current.FunctionHits[fnName]; !ok {
					current.FunctionHits[fnName] = 0
				}
			}

		case strings.HasPrefix(line, "FNDA:"):
			// FNDA:hit_count,function_name
			parts := strings.SplitN(strings.TrimPrefix(line, "FNDA:"), ",", 2)
			if len(parts) == 2 {
				hits, err := strconv.Atoi(parts[0])
				if err != nil {
					continue // skip malformed FNDA line
				}
				fnName := parts[1]
				current.FunctionHits[fnName] = hits
			}

		case strings.HasPrefix(line, "BRDA:"):
			// BRDA:line,block,branch,hits
			parts := strings.SplitN(strings.TrimPrefix(line, "BRDA:"), ",", 4)
			if len(parts) == 4 {
				key := parts[0] + ":" + parts[1] + ":" + parts[2]
				hits := 0
				if parts[3] != "-" {
					var err error
					hits, err = strconv.Atoi(parts[3])
					if err != nil {
						continue // skip malformed BRDA line
					}
				}
				current.BranchHits[key] = hits
			}

		case line == "end_of_record":
			if current != nil {
				recomputeCounts(current)
				records = append(records, *current)
				current = nil
			}
		}
	}

	// Handle missing end_of_record.
	if current != nil {
		recomputeCounts(current)
		records = append(records, *current)
	}

	return records, nil
}

// --- Istanbul JSON parser ---

// istanbulEntry represents one file's coverage in Istanbul JSON format.
type istanbulEntry struct {
	Path         string                    `json:"path"`
	StatementMap map[string]istanbulRange  `json:"statementMap"`
	S            map[string]int            `json:"s"`
	FnMap        map[string]istanbulFn     `json:"fnMap"`
	F            map[string]int            `json:"f"`
	BranchMap    map[string]istanbulBranch `json:"branchMap"`
	B            map[string][]int          `json:"b"`
}

type istanbulRange struct {
	Start istanbulPos `json:"start"`
	End   istanbulPos `json:"end"`
}

type istanbulPos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type istanbulFn struct {
	Name string        `json:"name"`
	Loc  istanbulRange `json:"loc"`
}

type istanbulBranch struct {
	Type      string          `json:"type"`
	Locations []istanbulRange `json:"locations"`
}

func parseIstanbul(data []byte) ([]CoverageRecord, error) {
	var entries map[string]istanbulEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	// Sort keys for deterministic output order.
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var records []CoverageRecord
	for _, k := range keys {
		entry := entries[k]
		rec := CoverageRecord{
			FilePath:     normalizeCoveragePath(entry.Path),
			LineHits:     map[int]int{},
			BranchHits:   map[string]int{},
			FunctionHits: map[string]int{},
		}

		// Statement coverage → line hits.
		for stmtID, hits := range entry.S {
			if loc, ok := entry.StatementMap[stmtID]; ok {
				rec.LineHits[loc.Start.Line] = hits
			}
		}

		// Function coverage.
		for fnID, hits := range entry.F {
			if fn, ok := entry.FnMap[fnID]; ok {
				name := fn.Name
				if name == "" {
					name = fmt.Sprintf("anonymous_%s", fnID)
				}
				rec.FunctionHits[name] = hits
			}
		}

		// Branch coverage.
		for brID, hits := range entry.B {
			for i, h := range hits {
				key := fmt.Sprintf("%s:%d", brID, i)
				rec.BranchHits[key] = h
			}
		}

		recomputeCounts(&rec)
		records = append(records, rec)
	}

	return records, nil
}

// istanbulSummaryMetric is one metric block (lines/statements/functions/branches)
// in a coverage-summary.json entry.
type istanbulSummaryMetric struct {
	Total   int `json:"total"`
	Covered int `json:"covered"`
}

// istanbulSummaryEntry is one file's (or the "total" rollup's) aggregate
// coverage in coverage-summary.json.
type istanbulSummaryEntry struct {
	Lines     istanbulSummaryMetric `json:"lines"`
	Functions istanbulSummaryMetric `json:"functions"`
	Branches  istanbulSummaryMetric `json:"branches"`
}

// isIstanbulSummary reports whether the JSON is a coverage-summary.json (a
// top-level "total" rollup whose value carries a "lines" metric) rather than a
// per-statement coverage-final.json. coverage-final.json is keyed by file path
// and never has a "total" key with a "lines" sub-object.
func isIstanbulSummary(data []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	total, ok := probe["total"]
	if !ok {
		return false
	}
	var metric map[string]json.RawMessage
	if err := json.Unmarshal(total, &metric); err != nil {
		return false
	}
	_, hasLines := metric["lines"]
	return hasLines
}

// parseIstanbulSummary reads coverage-summary.json. It has only aggregate
// counts per file, so it synthesizes hit maps (the first `covered` entries hit,
// the rest not) that recomputeCounts derives the correct totals/percentages
// from — the same shape the rest of the pipeline expects.
func parseIstanbulSummary(data []byte) ([]CoverageRecord, error) {
	var entries map[string]istanbulSummaryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		if k == "total" {
			continue // the "total" rollup is not a file
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	records := make([]CoverageRecord, 0, len(keys))
	for _, k := range keys {
		e := entries[k]
		lc := clampCovered(e.Lines.Total, e.Lines.Covered)
		fc := clampCovered(e.Functions.Total, e.Functions.Covered)
		bc := clampCovered(e.Branches.Total, e.Branches.Covered)
		// Set BOTH the synthesized hit maps AND the aggregate counts: the hit
		// maps keep recomputeCounts consistent if these records are later merged,
		// while the aggregate counts are what the coverage summary reads directly
		// for a single (unmerged) file.
		records = append(records, CoverageRecord{
			FilePath:             normalizeCoveragePath(k),
			LineHits:             synthLineHits(e.Lines.Total, lc),
			FunctionHits:         synthNamedHits(e.Functions.Total, fc),
			BranchHits:           synthNamedHits(e.Branches.Total, bc),
			LineTotalCount:       maxZero(e.Lines.Total),
			LineCoveredCount:     lc,
			FunctionTotalCount:   maxZero(e.Functions.Total),
			FunctionCoveredCount: fc,
			BranchTotalCount:     maxZero(e.Branches.Total),
			BranchCoveredCount:   bc,
		})
	}
	return records, nil
}

func clampCovered(total, covered int) int {
	if covered < 0 {
		return 0
	}
	if covered > total {
		return maxZero(total)
	}
	return covered
}

func maxZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// maxSynthHits bounds the per-file synthesized hit map size. A real source
// file has far fewer lines/functions/branches than this; the cap only
// prevents a malformed or hostile coverage-summary.json (whose totals are
// untrusted ints) from driving an unbounded allocation.
const maxSynthHits = 1_000_000

// synthLineHits builds a per-line hit map with `total` lines, the first
// `covered` of them marked hit — enough for recomputeCounts to recover the
// aggregate line coverage from a summary that carries no per-line data.
func synthLineHits(total, covered int) map[int]int {
	if total < 0 {
		total = 0
	}
	if total > maxSynthHits {
		total = maxSynthHits
	}
	if covered > total {
		covered = total
	}
	m := make(map[int]int, total)
	for i := 1; i <= total; i++ {
		if i <= covered {
			m[i] = 1
		} else {
			m[i] = 0
		}
	}
	return m
}

// synthNamedHits is synthLineHits for string-keyed metrics (functions, branches).
func synthNamedHits(total, covered int) map[string]int {
	if total < 0 {
		total = 0
	}
	if total > maxSynthHits {
		total = maxSynthHits
	}
	if covered > total {
		covered = total
	}
	m := make(map[string]int, total)
	for i := 0; i < total; i++ {
		if i < covered {
			m[strconv.Itoa(i)] = 1
		} else {
			m[strconv.Itoa(i)] = 0
		}
	}
	return m
}

// normalizeCoveragePath normalizes a file path from coverage artifacts.
// Strips absolute path prefixes and normalizes separators.
func normalizeCoveragePath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.ReplaceAll(p, "\\", "/")

	// Strip common absolute path prefixes.
	// Look for typical project root markers, matching the most
	// package-qualifying marker first: in a monorepo layout like
	// /repo/packages/<name>/lib/index.js, anchoring on /lib/ would
	// collapse packages/foo/lib/index.js and packages/bar/lib/index.js
	// to the same "lib/index.js" key and misattribute coverage. Anchor
	// on /packages/ first so the package-qualified sub-path is
	// preserved and distinct files stay distinct.
	markers := []string{"/packages/", "/src/", "/lib/", "/app/"}
	for _, m := range markers {
		if idx := strings.Index(p, m); idx > 0 {
			return p[idx+1:]
		}
	}

	// Strip leading / if still absolute.
	p = strings.TrimPrefix(p, "/")
	return p
}
