# Terrain v3.1 Implementation Plan

> **Date:** 2026-03-30
> **Scope:** 13 work items across 3 phases, addressing 5 P1 engineering gaps + 8 product experience gaps
> **Constraint:** Go stdlib only (no new deps). Deterministic. Testable. Conventional commits.

---

## Unified Task List

The 5 stabilization gaps (S1-S5) and 8 product gaps (P1-P8) are merged into 13 concrete work items, ordered by dependency and leverage.

| ID | Task | Phase | Effort | Deps |
|----|------|-------|--------|------|
| W1 | Headline + next actions | 1 | S | None |
| W2 | Actionable recommendations with file targeting | 1 | M | None |
| W3 | Health guidance when runtime absent | 1 | S | None |
| W4 | `--verbose` standardization across all commands | 1 | M | None |
| W5 | Detection determinism tests | 1 | S | None |
| W6 | Progressive disclosure (first-run) | 1 | S | W1 |
| W7 | Confidence calibration from fixtures | 1 | M | W5 |
| W8 | SARIF output for GitHub Code Scanning | 2 | M | W1 |
| W9 | HTML report output | 2 | L | W1, W2 |
| W10 | AI graph node population | 2 | S | W7 |
| W11 | CI annotations + trend snapshots | 2 | M | W8 |
| W12 | Benchmark suite on public repos | 2 | M | None |
| W13 | `terrain init` interactive redesign | 2 | M | W6 |

Phase 3 items (AI eval ingestion, mobile/device farm, `terrain serve`, marketplace) are documented as design specs but not scheduled — they depend on Phase 1-2 adoption feedback.

---

## Parallelization Map

```
PHASE 1 (Weeks 1-3)
  W1: Headline + actions ──────> W6: First-run ──────> (done)
  W2: Actionable recs ─────────────────────────────────> (done)
  W3: Health guidance ─────────────────────────────────> (done)
  W4: --verbose ───────────────────────────────────────> (done)
  W5: Determinism tests ───────> W7: Calibration ─────> (done)

PHASE 2 (Weeks 4-6)
  W8: SARIF ───────> W11: Annotations + trends ───────> (done)
  W9: HTML report ─────────────────────────────────────> (done)
  W10: AI graph nodes ─────────────────────────────────> (done)
  W12: Benchmark suite ────────────────────────────────> (done)
  W13: Init redesign ─────────────────────────────────> (done)
```

W1-W5 can all start in parallel (no file conflicts). W6 waits for W1. W7 waits for W5. Phase 2 items are independent of each other.

---

## W1: Headline + Next Actions

**Goal:** First line of `terrain analyze` answers "what's going on?" in one sentence with 3 things to do about it.

### Files to create

**`internal/analyze/headline.go`**

```go
package analyze

// deriveHeadline produces a single opinionated sentence from the Report.
// Priority order: critical signals > redundancy > fanout > coverage > instability > healthy.
func deriveHeadline(r *Report) string
```

Logic (evaluated in order, first match wins):

| Condition | Template |
|-----------|----------|
| `r.SignalSummary.Critical > 0` | `"Your test suite has %d critical issues requiring immediate attention."` |
| `r.DuplicateClusters.RedundantTestCount > 50` | `"Your test suite has %d redundant tests — consolidation could save ~%d%% of CI time."` |
| `r.HighFanout.FlaggedCount > 0` | `"%d shared fixtures ripple across %d tests — any change is high-risk."` |
| `len(r.WeakCoverageAreas) > 0` with >50% files | `"%d%% of source files have no structural test coverage."` |
| `r.StabilityClusters != nil && r.StabilityClusters.UnstableCount > 0` | `"%d tests are unstable, clustering around %d shared root causes."` |
| default | `"Your test suite looks healthy: %d tests across %d frameworks."` |

All data is already computed in `analyze.Report` — zero new analysis.

**`internal/analyze/actions.go`**

```go
package analyze

// NextAction is a prioritized recommendation with a runnable command.
type NextAction struct {
    Title       string `json:"title"`
    Command     string `json:"command"`
    Explanation string `json:"explanation"`
    Effort      string `json:"effort"`
}

// deriveNextActions returns up to 3 actions, prioritizing data-completeness
// actions first (they unlock more findings), then finding-based actions.
func deriveNextActions(r *Report) []NextAction
```

Action priority (data-completeness first):

| Condition | Action |
|-----------|--------|
| No coverage in `r.DataCompleteness` | `{Title: "Unlock coverage analysis", Command: "terrain analyze --coverage <path>", Effort: "5 min"}` |
| No runtime in `r.DataCompleteness` | `{Title: "Unlock health signals", Command: "terrain analyze --runtime <path>", Effort: "5 min"}` |
| `r.DuplicateClusters.ClusterCount > 0` | `{Title: "Remove %d redundant test clusters", Command: "terrain insights", Effort: "~2 hrs"}` |
| `len(r.WeakCoverageAreas) > 0` | `{Title: "Add tests for %d uncovered files", Command: "terrain analyze --verbose", Effort: "~30 min"}` |
| `r.HighFanout.FlaggedCount > 0` | `{Title: "Reduce fixture fan-out risk", Command: "terrain debug fanout", Effort: "~1 hr"}` |

Return first 3 that match.

### Files to modify

**`internal/analyze/analyze.go`** — Add fields to `Report` struct (after line 96):

```go
Headline    string       `json:"headline"`
NextActions []NextAction `json:"nextActions,omitempty"`
```

At the end of `Build()` (after line 691, before `return r`):

```go
r.Headline = deriveHeadline(r)
r.NextActions = deriveNextActions(r)
```

**`internal/reporting/analyze_report_v2.go`** — Modify `RenderAnalyzeReportV2` to lead with headline. Insert before the "Repository Profile" section (line 19):

```go
fmt.Fprintf(w, "\n  %s\n", r.Headline)
if len(r.NextActions) > 0 {
    fmt.Fprintln(w)
    fmt.Fprintln(w, "What to do next:")
    for i, a := range r.NextActions {
        fmt.Fprintf(w, "  %d. %s\n", i+1, a.Title)
        fmt.Fprintf(w, "     $ %s\n", a.Command)
        fmt.Fprintf(w, "     %s (effort: %s)\n", a.Explanation, a.Effort)
        if i < len(r.NextActions)-1 {
            fmt.Fprintln(w)
        }
    }
}
```

### Tests

**`internal/analyze/headline_test.go`** — Test each headline condition branch with a minimal `Report` struct.

**`internal/analyze/actions_test.go`** — Test: data-completeness actions appear first; max 3 returned; empty report returns at least one action.

### Golden test updates

Run `go test ./cmd/terrain/ -run Golden -update` after changes. All golden files in `cmd/terrain/testdata/` and `internal/testdata/` will need updating.

### Acceptance criteria

- [ ] `terrain analyze` on any repo leads with one headline sentence
- [ ] 1-3 next steps with `$` commands appear before data sections
- [ ] `terrain analyze --json | jq .headline` returns non-empty string
- [ ] `terrain analyze --json | jq '.nextActions | length'` returns 1-3
- [ ] All golden tests updated and passing

---

## W2: Actionable Recommendations with File Targeting

**Goal:** Every `terrain insights` recommendation names specific files to act on and a Terrain command to dig deeper.

### Files to create

**`internal/insights/actions.go`**

```go
package insights

// deriveActions enriches findings into file-targeted recommendations.
func deriveActions(
    findings []Finding,
    snap *models.TestSuiteSnapshot,
    dgCov *depgraph.CoverageResult,
    dgDupes *depgraph.DuplicateResult,
    dgFanout *depgraph.FanoutResult,
) []Recommendation
```

Per-category targeting:

| Finding category | `TargetFiles` source | `Command` |
|-----------------|---------------------|-----------|
| `CategoryDuplication` | Test file paths from top cluster in `dgDupes.Clusters[0].Files` | `terrain show test <path>` |
| `CategoryCoverage` | Source file paths from `dgCov.UncoveredFiles` (top 5) | `terrain explain <path>` |
| `CategoryFanout` | Fixture paths from `dgFanout.TopNodes` (top 3) | `terrain debug fanout` |
| `CategoryHealth` (flaky) | Test file paths from `snap.Signals` where `Type == "flakyTest"` | `terrain show test <path>` |
| `CategoryHealth` (skipped) | Test file paths from `snap.Signals` where `Type == "skippedTest"` | `terrain show test <path>` |

Effort band: `len(targetFiles)` 1-3 = "small", 4-10 = "medium", 10+ = "large".

**`internal/insights/actions_test.go`** — Test with synthetic findings + snapshot. Assert: all recommendations have `TargetFiles`, `EffortBand`, and `Command` populated.

### Files to modify

**`internal/insights/insights.go`** — Add fields to `Recommendation` struct (after line 120):

```go
TargetFiles []string `json:"targetFiles,omitempty"`
TargetUnits []string `json:"targetUnits,omitempty"`
EffortBand  string   `json:"effortBand,omitempty"`
Command     string   `json:"command,omitempty"`
```

In `Build()`, replace or augment the existing recommendation derivation with `deriveActions()`. The existing `Build()` at line 164 already has access to all depgraph results via `BuildInput`.

**`internal/reporting/insights_report_v2.go`** — In the "Recommended Actions" section of `RenderInsightsReport`, render new fields:

```go
if len(rec.TargetFiles) > 0 {
    fmt.Fprintf(w, "     files:  %s\n", strings.Join(rec.TargetFiles[:min(3, len(rec.TargetFiles))], ", "))
    if len(rec.TargetFiles) > 3 {
        fmt.Fprintf(w, "             +%d more\n", len(rec.TargetFiles)-3)
    }
}
if rec.Command != "" {
    fmt.Fprintf(w, "     run:    %s\n", rec.Command)
}
```

### Acceptance criteria

- [ ] Every `terrain insights` recommendation includes file paths
- [ ] `terrain insights --json` includes `targetFiles`, `effortBand`, `command`
- [ ] Every recommendation has a runnable `terrain` command
- [ ] Effort bands are consistent: small/medium/large

---

## W3: Health Guidance When Runtime Absent

**Goal:** When health signals are absent, tell users exactly why and how to generate the artifacts.

### Files to create

**`internal/reporting/guidance.go`**

```go
package reporting

import (
    "fmt"
    "io"
    "github.com/pmclSF/terrain/internal/models"
)

// WriteHealthGuidance prints actionable guidance when runtime data is absent.
// It is a no-op if runtime signals are present or if w is nil.
func WriteHealthGuidance(w io.Writer, snap *models.TestSuiteSnapshot) {
    if hasRuntimeSignals(snap) {
        return
    }
    fmt.Fprintln(w)
    fmt.Fprintln(w, "Health signals (flaky, slow, dead tests) require runtime artifacts.")
    fmt.Fprintln(w, "Generate with:")
    fmt.Fprintln(w, "  Jest:    npx jest --json --outputFile=jest-results.json")
    fmt.Fprintln(w, "  Pytest:  pytest --junitxml=junit.xml")
    fmt.Fprintln(w, "  Go:      go test -json ./... > test-results.json")
    fmt.Fprintln(w, "  JUnit:   mvn test (generates target/surefire-reports/*.xml)")
    fmt.Fprintln(w, "Then re-run with: terrain <command> --runtime <path>")
}

func hasRuntimeSignals(snap *models.TestSuiteSnapshot) bool {
    for _, sig := range snap.Signals {
        switch sig.Type {
        case "slowTest", "flakyTest", "skippedTest", "deadTest", "unstableSuite":
            return true
        }
    }
    return false
}
```

**`internal/reporting/guidance_test.go`** — Test: no runtime signals → writes guidance; has runtime signal → writes nothing.

### Files to modify

**`cmd/terrain/cmd_insights.go`** — Add guidance call after each report render (only when `!jsonOutput`):

In `runInsights` (line 198), after `RenderInsightsReport`:
```go
if !jsonOutput {
    reporting.WriteHealthGuidance(os.Stdout, result.Snapshot)
}
```

Same pattern in `runPosture` (line 41), `runMetrics` (line 58). Do NOT add to `runSummary` or `runFocus` (brief commands, avoid spam).

**`internal/reporting/analyze_report_v2.go`** — Enhance the existing Stability section (lines 257-263) to include generation commands from `WriteHealthGuidance`.

**`internal/engine/artifacts.go`** — Enrich `MissingArtifactHints` (line 167) to include framework-specific generation commands in the hint strings.

### Acceptance criteria

- [ ] `terrain analyze` without `--runtime` shows generation commands in Stability section
- [ ] `terrain posture`, `terrain insights`, `terrain metrics` show guidance when runtime absent
- [ ] Guidance disappears when `--runtime` is provided
- [ ] Guidance never appears in `--json` output
- [ ] `terrain summary` and `terrain focus` do NOT show guidance (too brief)

---

## W4: `--verbose` Standardization

**Goal:** `--verbose` works on all report commands with consistent meaning: default = condensed, verbose = detailed evidence.

### Files to create

**`internal/reporting/report_options.go`**

```go
package reporting

// ReportOptions controls rendering behavior across all report functions.
type ReportOptions struct {
    Verbose bool
}
```

### Files to modify

**`cmd/terrain/main.go`** — Add `--verbose` flag to 6 command blocks. For each of `insights` (line 148), `posture` (line 137), `portfolio` (line 126), `summary` (line 196), `focus` (line 205), `metrics` (line 157):

```go
verboseFlag := cmd.Bool("verbose", false, "show detailed evidence and breakdowns")
```

Update function calls to pass `*verboseFlag`.

**`cmd/terrain/cmd_insights.go`** — Update all 6 signatures:

```go
func runInsights(root string, jsonOutput, verbose bool) error  // was (root, jsonOutput)
func runPosture(root string, jsonOutput, verbose bool) error
func runMetrics(root string, jsonOutput, verbose bool) error
func runSummary(root string, jsonOutput, verbose bool) error
func runFocus(root string, jsonOutput, verbose bool) error
func runPortfolio(root string, jsonOutput, verbose bool) error
```

Pass `reporting.ReportOptions{Verbose: verbose}` to each render call.

**`internal/reporting/insights_report_v2.go`** — Update signature:

```go
func RenderInsightsReport(w io.Writer, r *insights.Report, opts ...ReportOptions)
```

When verbose: show per-finding evidence (affected file paths, detection tier, confidence), show per-recommendation rationale detail.

**`internal/reporting/posture_report.go`** — Update signature:

```go
func RenderPostureReport(w io.Writer, snap *models.TestSuiteSnapshot, opts ...ReportOptions)
```

When verbose: show individual measurement values, band thresholds, contributing signal counts.

**`internal/reporting/metrics_report.go`** — Update signature:

```go
func RenderMetricsReport(w io.Writer, ms *metrics.Snapshot, opts ...ReportOptions)
```

When verbose: show per-metric component scores.

**`internal/reporting/summary_report.go`** — Update signature:

```go
func RenderSummaryReport(w io.Writer, snap *models.TestSuiteSnapshot, h *heatmap.Heatmap, opts ...ReportOptions)
```

When verbose: show heatmap detail per test file.

**`internal/reporting/portfolio_report.go`** — Update existing `RenderPortfolioReport` and `RenderPortfolioSection` to accept `opts ...ReportOptions`. When verbose: show per-asset cost/leverage detail.

### Verbose additions per command

| Command | Default | `--verbose` adds |
|---------|---------|-----------------|
| insights | Top findings, severity, title | Per-finding affected files, detection tier, confidence |
| posture | Band labels per dimension | Measurement values, thresholds, contributing signals |
| metrics | Scorecard numbers | Per-metric component breakdown |
| portfolio | Summary stats | Per-asset cost/leverage, redundancy detail |
| summary | Executive overview | Per-file heatmap detail |
| focus | Top 3 actions | Full rationale, dependency chain |

### Constraints

- `--json` + `--verbose`: verbose is ignored (JSON always returns full data)
- Default output is byte-identical to current output when `--verbose` is not passed
- Empty data: verbose blocks guarded with data checks (no empty sections)

### Acceptance criteria

- [ ] All 6 commands accept `--verbose` and show it in `--help`
- [ ] Default output identical to current (no regressions)
- [ ] `--verbose` produces more detail in every command
- [ ] Existing `analyze --verbose`, `explain --verbose`, `ai list --verbose` unchanged

---

## W5: Detection Determinism Tests

**Goal:** Prove that every detection/inference function produces byte-identical output across runs.

### Files to create

**`internal/testdata/determinism_detection_test.go`**

Follow the exact pattern from existing `determinism_test.go`: run function N times, JSON-marshal output, compare all runs to run 0.

```go
package testdata_test

// Each test runs 10 iterations, JSON-marshals output, asserts all match run 0.

func TestDeterminism_ParsePromptAST_JS(t *testing.T)
func TestDeterminism_ParsePromptAST_Python(t *testing.T)
func TestDeterminism_ParseEmbeddedPrompts_JS(t *testing.T)
func TestDeterminism_ParseEmbeddedPrompts_Python(t *testing.T)
func TestDeterminism_InferCodeSurfaces(t *testing.T)
func TestDeterminism_InferAIContextSurfaces(t *testing.T)
func TestDeterminism_FullPipeline_AIFixture(t *testing.T)
```

### Test fixtures

For `ParsePromptAST` and `ParseEmbeddedPrompts`: inline source strings as `const` in the test file. Each string contains enough complexity to exercise multiple regex paths and map operations.

For `InferCodeSurfaces` and `InferAIContextSurfaces`: use `t.TempDir()` with 5-10 small files written in `TestMain` or `beforeEach`. These functions take `(root string, testFiles []models.TestFile, sourceFiles []string)`.

For full pipeline: use `tests/fixtures/ai-prompt-only` (smallest AI fixture).

### Template

```go
func TestDeterminism_ParsePromptAST_JS(t *testing.T) {
    t.Parallel()
    const src = `const messages = [
        { role: "system", content: "You are a helpful assistant" },
        { role: "user", content: userInput },
    ];
    const response = await openai.chat.completions.create({ model: "gpt-4", messages });`

    results := make([]string, 10)
    for i := 0; i < 10; i++ {
        surfaces := analysis.ParsePromptAST("src/chat.js", src, "js")
        data, _ := json.Marshal(surfaces)
        results[i] = string(data)
    }
    for i := 1; i < 10; i++ {
        if results[i] != results[0] {
            t.Errorf("run %d differs from run 0:\n  got:  %s\n  want: %s", i, results[i], results[0])
        }
    }
}
```

### If a test fails

The failure indicates non-determinism (map iteration, goroutine ordering, etc.). Fix by adding `sort.Slice` to the offending function's output before returning. This is the point of the tests — they catch drift.

### Acceptance criteria

- [ ] `go test ./internal/testdata/ -run Determinism_Detection` passes
- [ ] Each detection function (ParsePromptAST, ParseEmbeddedPrompts, InferCodeSurfaces, InferAIContextSurfaces) has at least one test
- [ ] One full-pipeline determinism test exists
- [ ] Tests run in < 30s total

---

## W6: Progressive Disclosure (First-Run)

**Goal:** First `terrain analyze` shows ~30 lines of focused output. Subsequent runs show the full report.

**Depends on:** W1 (headline + next actions)

### Files to create

**`internal/engine/firstrun.go`**

```go
package engine

import (
    "os"
    "path/filepath"
)

// IsFirstRun returns true if .terrain/.initialized does not exist in root.
func IsFirstRun(root string) bool {
    _, err := os.Stat(filepath.Join(root, ".terrain", ".initialized"))
    return os.IsNotExist(err)
}

// MarkFirstRunComplete creates .terrain/.initialized as a marker.
func MarkFirstRunComplete(root string) error {
    dir := filepath.Join(root, ".terrain")
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, ".initialized"), []byte("1\n"), 0644)
}
```

**`internal/engine/firstrun_test.go`** — Test: fresh dir returns true; after mark returns false; idempotent.

**`internal/reporting/analyze_report_compact.go`**

```go
package reporting

// RenderAnalyzeReportCompact renders a focused ~30-line report for first-time users.
// Shows: headline, next actions, test inventory, top 3 findings, data completeness.
func RenderAnalyzeReportCompact(w io.Writer, r *analyze.Report)
```

Sections:
1. Header (`Terrain -- Test Suite Analysis`)
2. Headline (from W1)
3. Next actions (from W1)
4. Test inventory: `{N} test files, {M} frameworks ({list})`
5. Top 3 key findings (from `r.KeyFindings[:3]`)
6. Data completeness checklist
7. Footer: `Run terrain analyze --verbose for the full report.`

### Files to modify

**`cmd/terrain/cmd_analyze.go`** — After pipeline runs and report is built (before rendering), check first-run:

```go
if !jsonOutput && engine.IsFirstRun(root) {
    reporting.RenderAnalyzeReportCompact(os.Stdout, report)
    _ = engine.MarkFirstRunComplete(root)
    return nil
}
```

The `--verbose` flag always bypasses compact mode (users explicitly asking for detail get it).

### Acceptance criteria

- [ ] First `terrain analyze` on a fresh repo: ~30 lines
- [ ] Second `terrain analyze`: full report
- [ ] `--verbose` always shows full report regardless of first-run
- [ ] `--json` always returns full JSON regardless of first-run
- [ ] `.terrain/.initialized` created after first successful run

---

## W7: Confidence Calibration from Fixtures

**Goal:** Replace heuristic confidence scores with ground-truth-calibrated values where fixture data supports it.

**Depends on:** W5 (determinism tests must pass first to ensure calibration inputs are stable)

### Files to create

**`internal/truthcheck/calibrate_cmd.go`**

```go
package truthcheck

// RunCalibration runs the analysis pipeline against each fixture directory,
// loads its terrain_truth.yaml, and computes calibration metrics.
func RunCalibration(fixtureDirs []string) (*CalibrationResult, error)
```

For each directory: load `tests/truth/terrain_truth.yaml` via `LoadTruthSpec`, run `engine.RunPipeline(dir)`, build `CalibrationInput{Name, Snapshot, Truth}`, collect all inputs, call `CalibrateFromFixtures(inputs)`.

**`internal/truthcheck/calibrate_cmd_test.go`** — Integration test against 4 fixture dirs (`ai-prompt-only`, `ai-mixed-traditional`, `ai-rag-pipeline`, `terrain-world`). Assert `FixtureCount == 4`, `TotalSurfaces > 0`, known kinds appear in `ByKind`.

**`internal/truthcheck/calibrated_scores.go`** — Generated lookup table:

```go
package truthcheck

import "github.com/pmclSF/terrain/internal/models"

// CalibratedScores maps surface kinds to calibrated confidence values.
// Generated by: terrain-truthcheck --calibrate
// Source fixtures: ai-prompt-only, ai-mixed-traditional, ai-rag-pipeline, terrain-world
type KindScore struct {
    Confidence float64
    Basis      string // "calibrated" or "heuristic"
}

var CalibratedScores = map[models.CodeSurfaceKind]KindScore{
    // Populated after running calibration against fixtures.
    // Kinds with >=5 data points get Basis: "calibrated".
    // Kinds with <5 data points retain Basis: "heuristic" with current values.
}
```

### Files to modify

**`internal/analysis/code_surface.go`** — In `assignInferenceMetadata` (lines 94-170), before the switch/case, check `CalibratedScores`:

```go
if cs, ok := truthcheck.CalibratedScores[s.Kind]; ok && cs.Basis == "calibrated" {
    s.Confidence = cs.Confidence
    s.ConfidenceBasis = models.ConfidenceBasisCalibrated
    return
}
// ... existing switch/case for heuristic fallback
```

**`internal/analysis/prompt_parser.go`** — Same pattern for inline confidence values (0.82, 0.78, 0.92, etc.). Check calibration table first.

**`internal/analysis/prompt_ast_parser.go`** — Same pattern for AST-tier confidence values.

**`cmd/terrain-truthcheck/main.go`** — Add `--calibrate` flag:

```go
if calibrateFlag {
    fixtureDirs := []string{
        "tests/fixtures/ai-prompt-only",
        "tests/fixtures/ai-mixed-traditional",
        "tests/fixtures/ai-rag-pipeline",
        "tests/fixtures/terrain-world",
    }
    result, err := truthcheck.RunCalibration(fixtureDirs)
    // ... print FormatCalibrationReport(result)
}
```

### Architectural decision

Calibration table is a **Go source file** committed to the repo. Rationale: no runtime I/O, version-controlled, reviewable in diffs. Regenerated by running `terrain-truthcheck --calibrate` and copying output.

### Risks

- **Circular dependency**: changing confidence could alter which surfaces are detected, changing calibration. Mitigation: one-shot process. Run calibration, update scores, verify truthcheck still passes.
- **Small samples**: some kinds will have <5 data points. They stay "heuristic" — the `Basis` field tracks this honestly.

### Acceptance criteria

- [ ] `terrain-truthcheck --calibrate` produces a CalibrationResult with `FixtureCount == 4`
- [ ] Kinds with >=5 data points have `ConfidenceBasis: "calibrated"` in output
- [ ] All truthcheck tests still pass after score adjustment
- [ ] No confidence score changes by more than 0.15 from current heuristic values
- [ ] `calibrated_scores.go` committed with generation metadata in comments

---

## W8: SARIF Output for GitHub Code Scanning

**Goal:** `terrain analyze --format sarif` produces SARIF 2.1.0 JSON that GitHub Code Scanning can ingest.

### Files to create

**`internal/sarif/sarif.go`** — SARIF 2.1.0 struct types (stdlib `encoding/json` only):

```go
package sarif

type Log struct {
    Schema  string `json:"$schema"`
    Version string `json:"version"`
    Runs    []Run  `json:"runs"`
}

type Run struct {
    Tool    Tool     `json:"tool"`
    Results []Result `json:"results"`
}

type Tool struct {
    Driver ToolComponent `json:"driver"`
}

type ToolComponent struct {
    Name           string `json:"name"`
    Version        string `json:"version"`
    InformationURI string `json:"informationUri,omitempty"`
    Rules          []Rule `json:"rules,omitempty"`
}

type Rule struct {
    ID               string     `json:"id"`
    ShortDescription Message    `json:"shortDescription"`
    DefaultConfig    RuleConfig `json:"defaultConfiguration,omitempty"`
}

type RuleConfig struct {
    Level string `json:"level"`
}

type Result struct {
    RuleID    string     `json:"ruleId"`
    Level     string     `json:"level"`
    Message   Message    `json:"message"`
    Locations []Location `json:"locations,omitempty"`
}

type Message struct {
    Text string `json:"text"`
}

type Location struct {
    PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
    ArtifactLocation ArtifactLocation `json:"artifactLocation"`
    Region           *Region          `json:"region,omitempty"`
}

type ArtifactLocation struct {
    URI string `json:"uri"`
}

type Region struct {
    StartLine int `json:"startLine,omitempty"`
}
```

**`internal/sarif/convert.go`**

```go
package sarif

// FromAnalyzeReport converts an analyze.Report into a SARIF log.
func FromAnalyzeReport(r *analyze.Report, version string) *Log
```

Mapping:

| Terrain | SARIF |
|---------|-------|
| `KeyFinding` severity critical/high | Result level `"error"` |
| `KeyFinding` severity medium | Result level `"warning"` |
| `KeyFinding` severity low | Result level `"note"` |
| `WeakCoverageAreas[].Path` | `PhysicalLocation.ArtifactLocation.URI` |
| Finding category | Rule ID: `terrain/weak-coverage`, `terrain/duplicate-tests`, `terrain/high-fanout`, `terrain/flaky-tests`, `terrain/skipped-tests` |

**`internal/sarif/convert_test.go`** — Build a minimal `analyze.Report` with known findings, convert, assert SARIF structure: schema is `"https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"`, version is `"2.1.0"`, result count matches finding count, levels map correctly.

### Files to modify

**`cmd/terrain/cmd_analyze.go`** — Extend the format switch (line 135):

```go
case "sarif":
    sarifLog := sarif.FromAnalyzeReport(report, version)
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(sarifLog)
```

**`cmd/terrain/main.go`** — Update format flag help text to include `sarif`.

### Acceptance criteria

- [ ] `terrain analyze --format sarif` produces valid SARIF 2.1.0 JSON
- [ ] `terrain analyze --format sarif | jq '.version'` returns `"2.1.0"`
- [ ] Each KeyFinding maps to exactly one SARIF Result
- [ ] Severity-to-level mapping is correct
- [ ] SARIF output is valid when uploaded via `codeql-action/upload-sarif@v3`

---

## W9: HTML Report Output

**Goal:** `terrain analyze --format html` produces a self-contained, print-friendly HTML page.

**Depends on:** W1 (headline), W2 (actionable recommendations)

### Files to create

**`internal/reporting/html_template.go`** — HTML + CSS as a Go `const` string. Self-contained (no CDN, no external JS).

Key design decisions:
- CSS Grid layout, `<details>` for collapsible sections (no JavaScript)
- CSS `conic-gradient` for donut charts, percentage `width` for bars
- `@media print` styles for clean PDF export
- Color palette: critical=#dc3545, high=#fd7e14, medium=#ffc107, low=#0d6efd, good=#198754

Template sections:
1. Header with health grade badge
2. Headline + next actions as styled cards
3. Repository profile as key-value grid
4. Coverage confidence as stacked bar
5. Risk posture as 5 horizontal bars
6. Key findings as severity-colored cards
7. Signal summary as donut
8. Data completeness as checklist
9. Detailed findings in `<details>` blocks

**`internal/reporting/html.go`**

```go
package reporting

import (
    "html/template"
    "io"
    "github.com/pmclSF/terrain/internal/analyze"
)

// RenderAnalyzeHTML writes a self-contained HTML report to w.
func RenderAnalyzeHTML(w io.Writer, r *analyze.Report) error {
    tmpl, err := template.New("report").Funcs(htmlFuncs).Parse(htmlTemplate)
    if err != nil {
        return fmt.Errorf("parse html template: %w", err)
    }
    return tmpl.Execute(w, r)
}

var htmlFuncs = template.FuncMap{
    "pct":           func(n, total int) int { ... },
    "severityColor": func(sev string) string { ... },
    "gradeColor":    func(grade string) string { ... },
}
```

**`internal/reporting/html_test.go`** — Render a report, assert: output starts with `<!DOCTYPE html>`, contains no `<script>` tags, contains `</html>`, file size < 500KB.

### Files to modify

**`cmd/terrain/cmd_analyze.go`** — Add to format switch:

```go
case "html":
    if jsonOutput {
        return fmt.Errorf("cannot combine --format html with --json")
    }
    return reporting.RenderAnalyzeHTML(os.Stdout, report)
```

**`cmd/terrain/main.go`** — Update format help: `--format (json|text|html|sarif)`.

### File output behavior

When stdout is a terminal and format is `html`: write to `.terrain/reports/analyze-{YYYYMMDD-HHMMSS}.html` and print the path. When piped: write to stdout. This enables both `terrain analyze --format html` (auto-file) and `terrain analyze --format html > report.html` (pipe).

### Acceptance criteria

- [ ] `terrain analyze --format html` produces a self-contained HTML file
- [ ] File opens correctly in Chrome, Firefox, Safari
- [ ] No `<script>` tags in output (CSS-only charts)
- [ ] Print to PDF produces clean document
- [ ] File size < 500KB
- [ ] `--format html --json` returns clear error

---

## W10: AI Graph Node Population

**Goal:** Wire AI CodeSurfaces into the dependency graph as first-class nodes with source-file edges.

**Depends on:** W7 (calibrated confidence values for edge weights)

### Discovery

`buildAISurfaceNodes` at `internal/depgraph/build.go:620` already creates AI nodes and scenario-to-AI edges. The actual gaps are:
1. No edges from AI nodes back to containing source files
2. Edge confidence hardcoded at 0.8 instead of using surface detection confidence

### Files to modify

**`internal/depgraph/edge.go`** — Add edge type (after line 77):

```go
EdgeAIDefinedInFile = "ai_defined_in_file"
```

**`internal/depgraph/build.go`** — In `buildAISurfaceNodes` (line 620), after creating the AI node:

```go
// Link AI node to its containing source file.
sourceFileID := "file:" + cs.Path
if g.Node(sourceFileID) != nil {
    g.AddEdge(&Edge{
        From:         nodeID,
        To:           sourceFileID,
        Type:         EdgeAIDefinedInFile,
        Confidence:   1.0,
        EvidenceType: EvidenceStaticAnalysis,
    })
}
```

Replace hardcoded 0.8 confidence on scenario-to-AI edges (line 667) with `cs.Confidence`:

```go
Confidence: cs.Confidence, // was 0.8
```

### Files to create

**`internal/depgraph/build_ai_test.go`**

Build a `TestSuiteSnapshot` with known CodeSurfaces:
- 2x `SurfacePrompt` (different paths)
- 1x `SurfaceDataset`
- 1x `SurfaceEvalDef`
- Source files for each path
- 1 Scenario that covers 2 of the surfaces

Call `Build(snap)`. Assert:
- `g.NodesByType(NodePrompt)` has 2 entries
- `g.NodesByType(NodeDataset)` has 1 entry
- `g.NodesByType(NodeEvalMetric)` has 1 entry
- Edges of type `EdgeAIDefinedInFile` exist from AI nodes to source files
- Edge confidence on scenario-to-AI edges matches surface confidence (not 0.8)

### Acceptance criteria

- [ ] `Build()` creates NodePrompt/NodeDataset/NodeModel/NodeEvalMetric from CodeSurfaces
- [ ] AI nodes link to their source files via `EdgeAIDefinedInFile`
- [ ] Scenario-to-AI edge confidence uses surface detection confidence
- [ ] `terrain debug depgraph --root tests/fixtures/ai-prompt-only` shows AI nodes in stats

---

## W11: CI Annotations + Trend Snapshots

**Goal:** GitHub Actions PRs get inline annotations from Terrain findings. Snapshots are auto-collected for trend tracking.

**Depends on:** W8 (SARIF)

### Files to create

**`internal/reporting/annotations.go`**

```go
package reporting

// RenderGitHubAnnotations writes ::error and ::warning workflow commands
// for each KeyFinding in the analyze report.
func RenderGitHubAnnotations(w io.Writer, r *analyze.Report)
```

Format per finding:
```
::warning file={path},line=1,title=Terrain: {title}::{description}
```

### Files to modify

**`.github/actions/terrain-impact/action.yml`** — Add steps:

```yaml
- name: Generate SARIF
  if: inputs.upload-sarif == 'true'
  shell: bash
  run: |
    ${{ steps.build.outputs.binary }} analyze --root "${{ inputs.root }}" --format sarif > .terrain/artifacts/terrain-analyze.sarif

- name: Upload SARIF to Code Scanning
  if: inputs.upload-sarif == 'true'
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: .terrain/artifacts/terrain-analyze.sarif
    category: terrain

- name: Save trend snapshot
  if: inputs.save-snapshot == 'true'
  shell: bash
  run: |
    ${{ steps.build.outputs.binary }} analyze --root "${{ inputs.root }}" --write-snapshot

- name: Upload trend snapshot
  if: inputs.save-snapshot == 'true'
  uses: actions/upload-artifact@v4
  with:
    name: terrain-snapshot-${{ github.sha }}
    path: .terrain/snapshots/
    retention-days: 90
```

Add new inputs: `upload-sarif` (default: `'false'`), `save-snapshot` (default: `'false'`).

### Acceptance criteria

- [ ] `terrain analyze --format annotation` emits `::warning`/`::error` lines
- [ ] SARIF upload step in GitHub Action works with `codeql-action/upload-sarif`
- [ ] Trend snapshots uploaded as build artifacts with 90-day retention
- [ ] All new action inputs default to `'false'` (backward compatible)

---

## W12: Benchmark Suite on Public Repos

**Goal:** Demonstrate Terrain's analysis on well-known open-source repos. Publish results as trust evidence.

### Files to create

**`cmd/terrain-bench/repos.go`**

```go
package main

type BenchmarkRepo struct {
    Owner     string
    Name      string
    Ref       string // pinned commit SHA for reproducibility
    Language  string
    Framework string
    Expected  BenchmarkExpectation
}

type BenchmarkExpectation struct {
    MinTestFiles  int
    MinFrameworks int
    MinFindings   int
}

var BenchmarkRepos = []BenchmarkRepo{
    {Owner: "facebook", Name: "react", Ref: "...", Language: "js", Framework: "jest",
        Expected: BenchmarkExpectation{MinTestFiles: 500, MinFrameworks: 1, MinFindings: 3}},
    {Owner: "pallets", Name: "flask", Ref: "...", Language: "python", Framework: "pytest",
        Expected: BenchmarkExpectation{MinTestFiles: 30, MinFrameworks: 1, MinFindings: 2}},
    {Owner: "golang", Name: "go", Ref: "...", Language: "go", Framework: "testing",
        Expected: BenchmarkExpectation{MinTestFiles: 1000, MinFrameworks: 1, MinFindings: 3}},
    {Owner: "vuejs", Name: "core", Ref: "...", Language: "ts", Framework: "vitest",
        Expected: BenchmarkExpectation{MinTestFiles: 50, MinFrameworks: 1, MinFindings: 2}},
    {Owner: "pandas-dev", Name: "pandas", Ref: "...", Language: "python", Framework: "pytest",
        Expected: BenchmarkExpectation{MinTestFiles: 200, MinFrameworks: 1, MinFindings: 3}},
}
```

**`docs/benchmarks/results.md`** — Generated markdown table:

```markdown
| Repository | Tests | Frameworks | Findings | Runtime |
|-----------|-------|-----------|----------|---------|
| facebook/react | 2,847 | jest | 12 | 3.2s |
| pallets/flask | 412 | pytest | 7 | 0.8s |
| ... | ... | ... | ... | ... |
```

### Files to modify

**`Makefile`** — Add target:

```makefile
benchmark-public:
    go run ./cmd/terrain-bench --repos
```

### Acceptance criteria

- [ ] `make benchmark-public` clones 5 repos and runs terrain analyze on each
- [ ] All 5 repos produce non-zero test file counts
- [ ] Runtime < 10s per repo
- [ ] Results published in `docs/benchmarks/results.md`
- [ ] Repo commits are pinned for reproducibility

---

## W13: `terrain init` Interactive Redesign

**Goal:** `terrain init --interactive` generates a `.terrain/terrain.yaml` tailored to the team's testing philosophy.

**Depends on:** W6 (first-run detection)

### Files to modify

**`internal/engine/initconfig.go`** — Add `RunInitInteractive`:

```go
// RunInitInteractive prompts the user for testing preferences and generates
// a tailored terrain.yaml configuration.
func RunInitInteractive(root string, reader io.Reader) (*InitResult, error)
```

Prompts (read from `reader` for testability):

1. **Testing philosophy** — `"What's your CI priority? (1) thoroughness (2) speed (3) balanced"` → sets coverage thresholds (80%/50%/65%)
2. **Coverage data** — `"Path to coverage data? (leave empty to skip)"` → auto-detect if empty
3. **Runtime data** — `"Path to test results? (leave empty to skip)"` → auto-detect if empty
4. **AI features** — `"Does this project have AI/ML features? (y/n)"` → enable/disable AI detection config

Generates `.terrain/terrain.yaml` with policy thresholds, artifact paths, and feature flags matching answers.

**`cmd/terrain/cmd_analyze.go`** — In `runInit`, check for `--interactive` flag:

```go
if interactive {
    return engine.RunInitInteractive(root, os.Stdin)
}
return engine.RunInit(root)
```

**`cmd/terrain/main.go`** — Add `--interactive` flag to `init` command.

### Tests

**`internal/engine/initconfig_test.go`** — Test `RunInitInteractive` with a `strings.Reader` providing canned answers. Assert generated YAML contains expected thresholds.

### Acceptance criteria

- [ ] `terrain init` works as before (non-interactive)
- [ ] `terrain init --interactive` prompts 4 questions and generates config
- [ ] Generated `.terrain/terrain.yaml` reflects answers
- [ ] Passing an `io.Reader` with answers makes the function testable without stdin

---

## Phase 3: Design Specs (Not Scheduled)

These are documented for future implementation after Phase 1-2 adoption feedback.

### P3-A: AI Eval Ingestion

Add `EvalResult` to `TestSuiteSnapshot`. Ingest eval artifacts (Gauntlet JSON, generic JSON/CSV with accuracy/latency/cost). `terrain ai baseline compare` computes deltas. Prompt versioning via `git diff` on prompt surfaces.

**Key files:** `internal/aieval/ingest.go`, `internal/aieval/comparison.go`, `internal/models/snapshot.go`, `cmd/terrain/cmd_ai.go`.

### P3-B: Mobile/Device Farm Detection

Auto-detect BrowserStack (`.browserstack.yml`), Sauce Labs (`.sauce/config.yml`), Firebase Test Lab (`.firebaserc`), WebdriverIO capabilities. Feed into `matrix.Analyze()` without manual YAML.

**Key files:** `internal/devicefarm/detect.go`, `internal/matrix/ci_infer.go`, `internal/analysis/analyzer.go`.

### P3-C: `terrain serve`

Local HTTP server (`net/http` stdlib) serving HTML report at `/` and JSON API at `/api/analyze`, `/api/insights`, `/api/impact`. Auto-refresh via polling. Default port 8421.

**Key files:** `cmd/terrain/cmd_serve.go`, `internal/server/server.go`, `internal/server/handlers.go`.

### P3-D: GitHub Action Marketplace Listing

Add `branding` to action.yml (icon: shield, color: green). Create README.md for the action with usage examples. Publish to GitHub Marketplace.

**Key files:** `.github/actions/terrain-impact/action.yml`, `.github/actions/terrain-impact/README.md`.

---

## Testing Strategy Summary

| Work item | Test files | Test type |
|-----------|-----------|-----------|
| W1 | `internal/analyze/headline_test.go`, `actions_test.go` | Unit |
| W2 | `internal/insights/actions_test.go` | Unit |
| W3 | `internal/reporting/guidance_test.go` | Unit |
| W4 | Each `*_report_test.go` in reporting/ | Unit (verbose vs default) |
| W5 | `internal/testdata/determinism_detection_test.go` | Determinism |
| W6 | `internal/engine/firstrun_test.go` | Unit |
| W7 | `internal/truthcheck/calibrate_cmd_test.go` | Integration |
| W8 | `internal/sarif/convert_test.go` | Unit |
| W9 | `internal/reporting/html_test.go` | Unit (structure) |
| W10 | `internal/depgraph/build_ai_test.go` | Unit |
| W11 | `internal/reporting/annotations_test.go` | Unit |
| W12 | CI benchmark run | Smoke |
| W13 | `internal/engine/initconfig_test.go` (extended) | Unit |

All tests follow project conventions: no mocks, real instances, `beforeEach` for fresh state, `expect()` assertions.

### Golden test impact

W1, W3, W4, and W6 modify CLI output. After each: run `go test ./cmd/terrain/ -run Golden -update` and `go test ./internal/testdata/ -run Golden -update`, review diffs, commit updated golden files.

---

## Effort Summary

| Phase | Items | Total effort |
|-------|-------|-------------|
| Phase 1 | W1-W7 (7 items) | ~2-3 weeks |
| Phase 2 | W8-W13 (6 items) | ~3-4 weeks |
| Phase 3 | P3-A through P3-D | Future (design only) |

Phase 1 transforms the first-run experience. Phase 2 integrates into CI and builds trust. Phase 3 deepens per-persona value based on adoption feedback.
