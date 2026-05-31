# CLI Specification

## Philosophy

The CLI is the primary OSS interface to Terrain.

It must be:
- fast
- trustworthy
- scriptable
- readable
- machine-friendly

## Surface — canonical 14 + legacy aliases

0.2.0 introduced namespace dispatchers (`terrain report`,
`terrain migrate`, `terrain config`, `terrain debug`,
`terrain ai`). The canonical surface is:

| Canonical | What it does |
|---|---|
| `terrain analyze` | Full snapshot pipeline; the headline command. Writes `.terrain/findings.json` after every run. |
| `terrain test` | CI-mode wrapper around analyze: emits JUnit XML and a markdown step-summary (point `--summary` at `$GITHUB_STEP_SUMMARY` for GitHub Actions). |
| `terrain init [path]` | First-run scaffolder — runs analyze + emits a starter `.terrain/` layout including an annotated `policy.yaml.example`. |
| `terrain report <verb>` | Read-side views: `summary`, `insights`, `metrics`, `explain`, `show`, `impact`, `pr`, `posture`, `select-tests` |
| `terrain migrate <verb>` | Framework migration: `run`, `config`, `list`, `detect`, `shorthands`, `estimate`, `status`, `checklist`, `readiness`, `blockers`, `preview` |
| `terrain ai <verb>` | AI inventory + eval orchestration: `list`, `run`, `replay`, `record`, `baseline`, `baseline compare`, `doctor`, `findings` |
| `terrain inject --prompt <path>` | Generate jailbreak-shaped test inputs from a prompt template. Emits pytest / vitest / JSON. |
| `terrain scaffold --schema <path>` | Generate boundary-case mutation tests from a JSON Schema. Emits pytest / vitest / JSON. |
| `terrain plugins <verb>` | Third-party detector plugins: `manifest <path>` validates a plugin manifest (`list`, `add`, `remove` reserved for a future release). |
| `terrain debug <verb>` | Dependency-graph drill-downs: `graph`, `coverage`, `fanout`, `duplicates`, `depgraph` |
| `terrain config <verb>` | Workspace prefs: `feedback`, `telemetry` |
| `terrain doctor [path]` | Diagnostics for current setup. Surfaces registry, alias-registry, gitignore, and per-rule policy-override state. |
| `terrain mcp [--root <dir>]` | Start the [Model Context Protocol](https://modelcontextprotocol.io) server on stdio for AI coding assistants. Reads `.terrain/findings.json` from the last analyze run. |
| `terrain portfolio` | Single-repo (stable) and multi-repo manifest (experimental) portfolio analysis |
| `terrain serve` | Local HTTP server with HTML report + JSON API (default port 8421, 127.0.0.1 only) |
| `terrain version` | Version, commit, build date, snapshot schema version |

Legacy aliases (`terrain summary`, `terrain insights`,
`terrain compare`, `terrain convert <file>`, `terrain focus`,
etc.) continue to route to the same handlers through 0.2.x.
Removal is future work.

The legacy top-level commands documented in this file
(`terrain summary`, `terrain insights`, etc.) continue to work
through 0.2.x as aliases that route to the same runners. Set
`TERRAIN_LEGACY_HINT=1` to see deprecation hints.

### `terrain version`
Purpose:
Print version, commit, build date, and snapshot schema version.

Output: `terrain <version> (commit <sha>, built <date>; snapshot schema <version>)`

Flags:
- `--json` — output machine-readable version metadata, including
  `schemaVersion` so CI tools can pin on the snapshot contract.

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
- `--format FORMAT` — output format: `json`, `text`, `sarif`, `annotation` (default: `text`). SARIF and annotation formats target their respective consumers; HTML output ships from `terrain serve`.
- `--verbose` — show all findings in analyze output
- `--write-snapshot` — persist snapshot to .terrain/snapshots/latest.json
- `--coverage PATH` — ingest coverage data (LCOV, Istanbul JSON)
- `--coverage-run-label LABEL` — coverage run label: unit, integration, or e2e
- `--runtime PATH` — path to runtime artifact (JUnit XML or Jest JSON); comma-separated for multiple
- `--gauntlet PATH` — path to Gauntlet AI eval result artifact (JSON); comma-separated for multiple
- `--slow-threshold MS` — slow test threshold in milliseconds (default: 5000)

Artifacts written:
- `.terrain/findings.json` — canonical Finding artifact (schema version 1), written after every run. Stable shape; downstream consumers (`terrain mcp`, IDE plugins, third-party SARIF uploaders) read from this file. Gitignored by the template `terrain init` writes.
- `.terrain/snapshots/latest.json` — full TestSuiteSnapshot, written only when `--write-snapshot` is set. Used by `terrain compare` for trend tracking.
- `.terrain/shadow-report.jsonl` — append-only log of mechanisms running in shadow state (created only if any mechanism is non-off).

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
AI/eval validation namespace. List detected scenarios, run evals, manage baselines, validate setup, and emit AI eval-gap findings.

Subcommands:
- `terrain ai list` — list detected AI/eval scenarios, prompt surfaces, dataset surfaces, and eval files
- `terrain ai run` — execute eval scenarios and collect results
- `terrain ai replay` — replay and verify a previous eval run artifact
- `terrain ai record` — record eval run results as a baseline snapshot
- `terrain ai baseline` — show the latest baseline snapshot
- `terrain ai baseline compare` — diff the latest baseline against the prior one
- `terrain ai doctor` — validate AI/eval setup: scenarios, prompts, datasets, eval files, graph wiring
- `terrain ai findings` — emit AI eval-gap findings with confidence + severity + evidence

Common flags (all subcommands):
- `--root PATH` — repository root (default: current directory)
- `--json` — output JSON

Additional flags:
- `terrain ai list --verbose` — show detection evidence per surface
- `terrain ai run --base REF` — git base ref for impact-based scenario selection
- `terrain ai run --full` — run all scenarios (skip impact selection)
- `terrain ai run --dry-run` — show what would run without executing
- `terrain ai findings --posture=gate` — gate-tier filter (default: observability)

---

### `terrain inject`
Purpose:
Read a prompt template, match it against a curated set of jailbreak / injection patterns (DAN-style, instruction leak, system-prompt fishing, role confusion, indirect-via-retrieval), and emit a runnable test scaffold so adopters can assert their prompt pipeline degrades safely.

Usage: `terrain inject --prompt <path> [--lang python|typescript|json] [--json] [--list]`

Flags:
- `--prompt PATH` — prompt template to scan (required).
- `--lang LANG` — scaffold language: `python` (default, pytest), `typescript` (vitest), or `json` (raw).
- `--json` — shortcut for `--lang=json`.
- `--list` — only list matched patterns; don't emit a scaffold.

LLM-free: terrain never invokes the model. The assertion is the adopter's.

---

### `terrain scaffold`
Purpose:
Read a JSON Schema describing a prompt's expected input shape and emit a runnable mutation-test scaffold. Each declared property produces a parametrized test exercising boundary cases (empty / whitespace / max-length / unicode-edge / SQL-injection-shaped / XSS-shaped / path-traversal-shaped / null-byte / INT32 bounds / near-double-limits).

Usage: `terrain scaffold --schema <path> [--prompt <path>] [--lang python|typescript|json] [--json]`

Flags:
- `--schema PATH` — JSON Schema describing the prompt's input shape (required).
- `--prompt PATH` — optional path to the prompt under test (printed as a header comment).
- `--lang LANG` — scaffold language: `python` (default, pytest), `typescript` (vitest), or `json` (raw cases).
- `--json` — shortcut for `--lang=json`.

LLM-free; deterministic output.

---

### `terrain plugins`
Purpose:
Validate third-party plugin manifests and (in a future release) install + run plugins.

Subcommands:
- `terrain plugins manifest <path>` — validate a plugin manifest against schema v1.
- `terrain plugins list` — list registered plugins (today: always empty; the runtime ships in a future release).
- `terrain plugins add <plugin-id>` / `terrain plugins remove <plugin-id>` — reserved; returns an explicit "not yet implemented" error.

Flags:
- `--json` — output JSON.

Plugin rules must declare an allowed `mechanism_class` (`structural-ast`, `import-graph`, `receiver-type`, or `manifest-schema`). Literal-string and regex primitives are explicitly forbidden: every rule must clear a class, not a cell.

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

### `terrain serve` *(experimental)*

Purpose:
Run a local HTTP server that renders the analysis report at `/` and exposes
a JSON API at `/api/analyze` and `/api/health`. The HTML view auto-refreshes
every 30 seconds. Intended for single-developer local exploration; a richer
dashboard with embedded charts is reserved for a future release.

Usage: `terrain serve [--root PATH] [--port N] [--host HOST] [--read-only]`

Flags:
- `--root PATH` — repository root to analyze (default: `.`).
- `--port N` — bind port (default: 8421).
- `--host HOST` — bind host (default: `127.0.0.1`). Setting any other
  value emits a stderr warning because the server has no built-in
  authentication.
- `--read-only` — reject any non-GET/HEAD/OPTIONS request with HTTP 405.
  Every handler shipped in 0.2 is GET-only, so this is a contract gate
  for any future state-changing endpoint rather than a behavior change
  for current traffic. Users who set `--read-only=true` get the
  enforcement they ticked the box for.

Security:
- Binds to `127.0.0.1` by default.
- Sets CSP, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  and `Referrer-Policy: no-referrer` on every response.
- Validates `Origin`/`Referer` headers; cross-origin browser requests are
  rejected with 403. Empty headers (curl, server-to-server) are allowed.
- Reads (and serves) the snapshot from disk; never writes.

For multi-developer hosts, use an SSH tunnel rather than binding to a
non-localhost address; first-class authentication is future work.

---

## GitHub Actions

Two workflow templates are provided in `.github/workflows/`:

### `terrain-pr.yml` — Test Selection Gate
Runs on every PR. Analyzes impact, selects relevant tests, runs them, and posts a PR comment with the results.

### `terrain-ai.yml` — AI Risk Review Gate
Runs on every PR. Checks AI surface coverage, runs impact-scoped eval scenario selection, and posts a PR comment summarizing the AI risk review (uncovered surfaces, blocking signals, eval regressions). Blocks the PR if the gate returns "block" (uncovered safety-critical surfaces, accuracy regressions, etc.). Deeper detection mechanisms and labeled-repo precision floors are future work.

Both workflows are opt-in — copy them to your repository's `.github/workflows/` directory to enable.

---

## Language Support

| Layer | JS/TS | Go | Python | Java |
|-------|-------|-----|--------|------|
| Framework detection | ✓ | ✓ | ✓ | ✓ |
| Code unit extraction | ✓ | ✓ | ✓ | ✓ |
| Import graph | ✓ full resolver | ✓ AST | ✓ | ✗ heuristic |
| Fixture detection | ✓ | ✓ | ✓ | ✓ |
| Impact analysis | Full | Full | Full | Heuristic |

Java import resolution is planned for a future release. Impact analysis for Java projects uses structural heuristics (file path matching, framework conventions) rather than dependency tracing.

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
