# CLI Specification

## Philosophy

The CLI is the primary OSS interface to Terrain.

It must be:
- fast
- trustworthy
- scriptable
- readable
- machine-friendly

### `terrain version`
Purpose:
Print version, commit, and build date.

Output: `terrain <version> (commit <sha>, built <date>)`

Flags:
- `--json` — output machine-readable version metadata

---

## Core commands

### `terrain init`
Purpose:
Inspect a repository for common coverage/runtime artifacts and print a
ready-to-run `terrain analyze` command with detected paths.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root to inspect (default: current directory)
- `--json` — output JSON init result

### `terrain analyze`
Primary command.

Purpose:
Generate a snapshot and summary of:
- frameworks
- health signals
- quality signals
- migration signals
- risk surfaces

Must support:
- human-readable output
- JSON output
- snapshot persistence

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON snapshot
- `--format FORMAT` — output format: json, text, sarif, annotation, html (default: text)
- `--verbose` — show all findings in analyze output
- `--write-snapshot` — persist snapshot to .terrain/snapshots/latest.json
- `--coverage PATH` — ingest coverage data (LCOV, Istanbul JSON)
- `--coverage-run-label LABEL` — coverage run label: unit, integration, or e2e
- `--runtime PATH` — path to runtime artifact (JUnit XML or Jest JSON); comma-separated for multiple
- `--gauntlet PATH` — path to Gauntlet AI eval result artifact (JSON); comma-separated for multiple
- `--slow-threshold MS` — slow test threshold in milliseconds (default: 5000)

### `terrain impact`
Purpose:
Impact analysis for changed code. Shows which code units, test gaps,
tests, and owners are affected by a git diff.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--base REF` — git base ref for diff (default: HEAD~1)
- `--json` — output JSON impact result
- `--show VIEW` — drill-down: units, gaps, tests, owners, graph, selected
- `--owner NAME` — filter results by owner

### `terrain posture`
Purpose:
Detailed posture breakdown with measurement evidence by dimension.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON posture snapshot
- `--verbose` — show measurement values and thresholds

### `terrain migration readiness`
Purpose:
Assess migration readiness with framework inventory, blocker taxonomy,
quality factors that compound migration risk, area-by-area safety
classification, and coverage guidance.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON readiness summary

### `terrain migration blockers`
Purpose:
List migration blockers by type and highest-risk areas. Focused view
for teams actively planning a framework migration.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON blockers summary

### `terrain migration preview`
Purpose:
Preview migration for a single file or directory scope. Shows source
framework, suggested target, blockers, safe patterns, and difficulty
assessment. Honest about limitations when preview is not possible.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON preview result
- `--file PATH` — preview a single file (relative to root)
- `--scope DIR` — preview all files in a directory scope

### `terrain compare`
Purpose:
Compare two snapshots and show trend changes.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root (default: current directory)
- `--from PATH` — baseline snapshot JSON (default: second-latest in .terrain/snapshots/)
- `--to PATH` — current snapshot JSON (default: latest in .terrain/snapshots/)
- `--json` — output JSON comparison

Behavior:
- If --from and --to are not specified, uses the two most recent timestamped snapshots
- If fewer than two snapshots exist, returns a clear error message

### `terrain policy check`
Purpose:
Evaluate current repository state against local Terrain policy.

Must support:
- human-readable output (default)
- JSON output (`--json`)
- CI-friendly exit codes

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON result

Exit codes:
- `0` — no policy file found, or policy exists with no violations
- `1` — policy file malformed or evaluation/runtime error
- `2` — policy violations found (CI gate signal)

Policy file:
- Loaded from `.terrain/policy.yaml` in the analyzed repository root
- Missing policy file is not an error (exit 0, informational message)
- Malformed policy file produces an actionable error (exit 1)

Supported policy rules:
- `disallow_skipped_tests` — flag skipped tests as violations
- `disallow_frameworks` — list of framework names not permitted
- `max_test_runtime_ms` — maximum average test runtime
- `minimum_coverage_percent` — minimum coverage threshold
- `max_weak_assertions` — maximum allowed weakAssertion signals
- `max_mock_heavy_tests` — maximum allowed mockHeavyTest signals

### `terrain summary`
Purpose:
Executive summary — leadership-oriented risk, trend, and benchmark readiness report.

Must support:
- human-readable output (default)
- JSON output (`--json`)

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — output JSON executive summary (ExecutiveSummary model)
- `--verbose` — show detailed heatmap breakdown

Output includes:
- Overall posture by dimension (reliability, change, speed, governance)
- Key numbers (test files, frameworks, total signals, critical findings, high-risk areas)
- Top risk areas with risk type and band
- Trend highlights (if prior snapshots exist in .terrain/snapshots/)
- Dominant signal drivers
- Evidence-based recommended focus
- Benchmark readiness (ready dimensions, limited dimensions, segmentation)

Behavior:
- Automatically attempts to load prior snapshots for trend comparison
- Gracefully degrades when no snapshot history exists
- Benchmark readiness section describes what is measurable, not how it ranks

### `terrain export benchmark`
Purpose:
Output a benchmark-safe JSON artifact for future anonymous comparison.

Flags:
- `--root PATH` — repository root to analyze (default: current directory)
- `--json` — accepted for explicit machine-readable invocation (output is always JSON)

Output is always JSON (no human-readable mode — this is a machine artifact).

Export includes:
- Schema version for compatibility
- Segmentation tags (primary language, primary framework, test file bucket, framework count, coverage/runtime/policy presence)
- Full aggregate metrics (same as `terrain metrics --json`)

Privacy boundary:
- No raw file paths, symbol names, source code, or user identity
- Only aggregate counts, ratios, qualitative bands, and segmentation tags
- Safe for anonymous aggregation and cross-repo comparison

### `terrain insights`
Purpose:
Prioritized improvement actions with rationale. Shows what to fix first
and why, based on signal analysis.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON insights
- `--verbose` — show per-finding evidence and file details

### `terrain explain`
Purpose:
Evidence chain for a specific entity — test file, code unit, owner, scenario,
or finding. Answers "Why did Terrain make this decision?"

Usage: `terrain explain <test-path|test-id|code-unit|owner|scenario-id|selection>`

Flags:
- `--root PATH` — repository root (default: current directory)
- `--base REF` — git base ref for diff (used when explaining impact-related decisions)
- `--json` — output JSON explanation
- `--verbose` — show detection evidence, tiers, and confidence details

Note: Flags can appear before or after the target argument.

### `terrain focus`
Purpose:
Focus summary — where to concentrate testing effort based on risk,
coverage gaps, and recent changes.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON focus summary
- `--verbose` — show full rationale, dependency chains, and blind spots

### `terrain portfolio`
Purpose:
Portfolio view of the test suite — treats the test suite as a portfolio
of investments, showing coverage breadth, test type distribution, and
risk allocation across the codebase.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON portfolio snapshot
- `--verbose` — show per-asset details

### `terrain metrics`
Purpose:
Output aggregate, benchmark-ready metrics scorecard.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON metrics snapshot
- `--verbose` — show detailed metric breakdowns

### `terrain select-tests`
Purpose:
Protective test selection for changed code. Given a git diff, returns the
minimal set of tests that should run to cover affected code paths.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--base REF` — git base ref for diff (default: HEAD~1)
- `--json` — output JSON protective test set

### `terrain pr`
Purpose:
PR analysis — combined impact, test selection, and risk summary formatted
for pull request review workflows.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--base REF` — git base ref for diff (default: HEAD~1)
- `--json` — output JSON PR analysis
- `--format FORMAT` — output format: markdown, comment, annotation

### `terrain show`
Purpose:
Drill into a specific entity — show details for a test, code unit, owner,
or finding by ID or path.

Usage: `terrain show <test|unit|codeunit|owner|finding> <id-or-path>`

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON

### `terrain debug`
Purpose:
Developer debugging commands for inspecting internal analysis state.

Usage: `terrain debug <graph|coverage|fanout|duplicates|depgraph> [flags]`

Subcommands:
- `graph` — dependency graph statistics
- `coverage` — coverage attribution details
- `fanout` — import fan-out analysis
- `duplicates` — test identity collision detection

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON
- `--changed FILES` — comma-separated changed files for impact context

### `terrain depgraph`
Purpose:
Full dependency graph inspection with multiple sub-views.

Flags:
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON
- `--show VIEW` — sub-view: stats, coverage, duplicates, fanout, impact, profile
- `--changed FILES` — comma-separated changed files for impact analysis

---

### `terrain ai`
Purpose:
AI/eval validation namespace. List detected scenarios, run evals, manage baselines, and validate setup.

Subcommands:
- `terrain ai list` — list detected AI/eval scenarios, prompt surfaces, dataset surfaces, and eval files
- `terrain ai run` — execute eval scenarios and collect results
- `terrain ai replay` — replay and verify a previous eval run artifact
- `terrain ai record` — record eval run results as a baseline snapshot
- `terrain ai baseline` — manage eval baselines: show, compare
- `terrain ai doctor` — validate AI/eval setup: scenarios, prompts, datasets, eval files, graph wiring

Common flags (all subcommands):
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON

Additional flags:
- `terrain ai list --verbose` — show detection evidence per surface
- `terrain ai run --base REF` — git base ref for impact-based scenario selection
- `terrain ai run --full` — run all scenarios (skip impact selection)
- `terrain ai run --dry-run` — show what would run without executing

---

## Conversion / migration commands

### `terrain convert`
Purpose:
Go-native test source conversion. 25 supported directions across E2E (Cypress, Playwright, Selenium, WebdriverIO, Puppeteer, TestCafe), JS unit (Jest, Vitest, Mocha, Jasmine), Java (JUnit 4/5, TestNG), and Python (pytest, unittest, nose2).

Usage: `terrain convert <source> --from <framework> --to <framework> [flags]`

Flags:
- `--from, -f` — source framework
- `--to, -t` — target framework
- `--output, -o` — output path
- `--auto-detect` — detect source framework automatically
- `--validate` — validate converted output (default: true)
- `--strict-validate` — force strict validation
- `--on-error` — skip|fail|best-effort
- `--plan` — show conversion plan
- `--dry-run` — preview without writing
- `--batch-size` — files per batch (default: 5)
- `--concurrency` — parallel workers (default: 4)
- `--json` — machine-readable output

### `terrain convert-config`
Purpose:
Convert framework configuration files.

Usage: `terrain convert-config <source> --to <framework> [flags]`

### `terrain migrate`
Purpose:
Project-wide migration with state tracking, resume, and retry.

Usage: `terrain migrate <dir> --from <framework> --to <framework> [flags]`

Additional flags:
- `--continue` — resume a previously started migration
- `--retry-failed` — retry only failed files

### `terrain estimate`
Purpose:
Estimate migration complexity without writing files.

Usage: `terrain estimate <dir> --from <framework> --to <framework> [flags]`

### Supporting commands

- `terrain status [--dir PATH]` — show migration progress
- `terrain checklist [--dir PATH]` — generate migration checklist
- `terrain doctor [path]` — run migration diagnostics
- `terrain reset [--dir PATH] --yes` — clear migration state
- `terrain list-conversions [--json]` — list all 25 supported directions
- `terrain shorthands [--json]` — list all 50 shorthand aliases
- `terrain detect <file-or-dir> [--json]` — detect dominant framework

---

## Output rules

### Human-readable output
Should emphasize:
- top findings
- representative examples
- next actions

### JSON output
Should be stable enough to support:
- extension rendering
- CI integration
- snapshot persistence
- third-party tool integration
