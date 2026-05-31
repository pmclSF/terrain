# CLI Specification

The complete reference for every command and flag the `terrain` binary accepts. Adopters who only want the canonical workflow should start with the [quickstart](quickstart.md); this doc is the lookup table.

Repository-scoped commands accept `--root PATH` (defaults to current directory). Machine-readable commands accept `--json`. Run `terrain <command> --help` for the live flag list.

## Surface — canonical 14 + legacy aliases

Canonical commands route through namespace dispatchers (`terrain report`, `terrain migrate`, `terrain config`, `terrain debug`, `terrain ai`):

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
| `terrain plugins <verb>` | Third-party detector plugins: `manifest <path>` validates a plugin manifest. `list`, `add`, `remove` are stub commands; the runtime loader ships in a future release. |
| `terrain debug <verb>` | Dependency-graph drill-downs: `graph`, `coverage`, `fanout`, `duplicates`, `depgraph` |
| `terrain config <verb>` | Workspace prefs: `feedback`, `telemetry` |
| `terrain doctor [path]` | Diagnostics for current setup. Surfaces registry, alias-registry, gitignore, and per-rule policy-override state. |
| `terrain mcp [--root <dir>]` | Start the [Model Context Protocol](https://modelcontextprotocol.io) server on stdio for AI coding assistants. Reads `.terrain/findings.json` from the last analyze run. |
| `terrain portfolio` | Single-repo (stable) and multi-repo manifest (experimental) portfolio analysis |
| `terrain serve` | Local HTTP server with HTML report + JSON API (default port 8421, 127.0.0.1 only) |
| `terrain version` | Version, commit, build date, snapshot schema version |

Legacy top-level commands (`terrain summary`, `terrain insights`, `terrain compare`, `terrain convert <file>`, `terrain focus`, etc.) continue to work as aliases of their namespaced counterparts. Set `TERRAIN_LEGACY_HINT=1` to surface a one-line deprecation hint when a legacy form is invoked.

### `terrain version`

Prints `terrain <version> (commit <sha>, built <date>; snapshot schema <version>)`.

Flags:
- `--json` — machine-readable metadata including `schemaVersion` so CI tools can pin on the snapshot contract.

---

## Core commands

### `terrain init`

Inspects a repository for coverage / runtime artifacts and prints a ready-to-run `terrain analyze` invocation with detected paths. Writes `.terrain/policy.yaml` (commented starter) and `.terrain/policy.yaml.example` (annotated reference covering every supported policy field).

Flags: `--json` for machine-readable output.

### `terrain analyze`

The headline command. Builds the snapshot, runs detectors, and renders the report.

Flags:
- `--format FORMAT` — `json`, `text`, `sarif`, `annotation` (default: `text`). HTML ships from `terrain serve`.
- `--json` — shortcut for `--format=json`.
- `--verbose` — show every finding in the report (not just key ones).
- `--write-snapshot` — persist the full snapshot to `.terrain/snapshots/latest.json` (used by `terrain compare`).
- `--coverage PATH` — ingest coverage data (LCOV, Istanbul JSON).
- `--coverage-run-label LABEL` — `unit`, `integration`, or `e2e`.
- `--runtime PATH` — runtime artifact (JUnit XML or Jest JSON); comma-separated for multiple.
- `--gauntlet PATH` — Gauntlet AI eval result artifact (JSON); comma-separated for multiple.
- `--slow-threshold MS` — slow-test threshold in milliseconds (default: 5000).
- `--fail-on=LEVEL` — gate the build; exits code 6 when any finding meets the threshold. `LEVEL` ∈ `low | medium | high | critical`.
- `--baseline PATH` + `--new-findings-only` — filter to regressions only (onboarding pattern for repos with existing debt).

Artifacts written:
- `.terrain/findings.json` — canonical Finding artifact (schema version 1), written after every run. Stable shape; downstream consumers (`terrain mcp`, IDE plugins, third-party SARIF uploaders) read from this file. Gitignored by the template `terrain init` writes.
- `.terrain/snapshots/latest.json` — full TestSuiteSnapshot, written only when `--write-snapshot` is set. Used by `terrain compare` for trend tracking.
- `.terrain/shadow-report.jsonl` — append-only log of mechanisms running in shadow state (created only if any mechanism is non-off).

### `terrain impact`

Which code units, test gaps, tests, and owners a git diff affects.

Flags:
- `--base REF` — git base ref (default: `HEAD~1`).
- `--show VIEW` — drill-down: `units`, `gaps`, `tests`, `owners`, `graph`, `selected`.
- `--owner NAME` — filter by owner.

### `terrain posture`

Five-dimension posture breakdown with measurement evidence. `--verbose` shows measurement values and thresholds.

### `terrain migration readiness`

Migration readiness with framework inventory, blocker taxonomy, quality factors that compound risk, per-area safety classification, and coverage guidance.

### `terrain migration blockers`

Migration blockers by type and the highest-risk areas. Focused view for teams planning a framework migration.

### `terrain migration preview`

Source framework + suggested target + blockers + safe patterns + difficulty for a file or directory.

Flags: `--file PATH` (single file), `--scope DIR` (directory).

### `terrain compare`

Compare two snapshots and show trend deltas. Defaults to the two most recent in `.terrain/snapshots/`; falls back to a clear error when fewer than two exist.

Flags: `--from PATH`, `--to PATH`.

### `terrain policy check`

Evaluate the repo against `.terrain/policy.yaml`. Exit codes: `0` pass (or no policy file), `1` malformed policy, `2` violations.

Supported rules:
- `disallow_skipped_tests`
- `disallow_frameworks` — list of disallowed framework names
- `max_test_runtime_ms`
- `minimum_coverage_percent`
- `max_weak_assertions`
- `max_mock_heavy_tests`

### `terrain summary`

Executive summary: posture by dimension (reliability, change, speed, governance), key numbers, top risk areas, trend highlights when prior snapshots exist, dominant signal drivers, recommended focus, and benchmark-readiness segmentation. Loads prior snapshots for trend comparison automatically; degrades cleanly when none exist.

`--verbose` shows the heatmap breakdown.

### `terrain export benchmark`

Privacy-safe JSON artifact for anonymous cross-repo comparison. No file paths, symbol names, source code, or identifiers — only aggregate counts, ratios, qualitative bands, and segmentation tags (primary language, primary framework, test-file bucket, framework count, presence of coverage / runtime / policy). Output is always JSON.

### `terrain insights`

Prioritized improvement actions with rationale. `--verbose` adds per-finding evidence and file details.

### `terrain explain`

Evidence chain for any entity — test file, code unit, owner, scenario, or finding.

```
terrain explain <test-path|test-id|code-unit|owner|scenario-id|selection> [--base REF]
```

`--verbose` adds detection evidence, tiers, and confidence. Flags can appear before or after the target.

### `terrain focus`

Where to concentrate testing effort, ranked by risk + coverage gaps + recent change. `--verbose` adds rationale, dependency chains, and blind spots.

### `terrain portfolio`

Treats the test suite as a portfolio of investments: coverage breadth, test-type distribution, risk allocation. `--verbose` adds per-asset detail.

### `terrain metrics`

Aggregate benchmark-ready scorecard. `--verbose` adds metric breakdowns.

### `terrain select-tests`

Minimal protective test set for a git diff. `--base REF` (default `HEAD~1`).

### `terrain pr`

Combined impact + selection + risk for PR review.

Flags:
- `--base REF` (default `HEAD~1`)
- `--format FORMAT` — `markdown`, `comment`, `annotation`

### `terrain show`

Drill into a specific entity.

```
terrain show <test|unit|codeunit|owner|finding> <id-or-path>
```

### `terrain debug`

Developer inspection of internal analysis state.

```
terrain debug <graph|coverage|fanout|duplicates|depgraph> [--changed FILES]
```

### `terrain depgraph`

Full dependency-graph inspection.

Flags: `--show VIEW` (`stats`, `coverage`, `duplicates`, `fanout`, `impact`, `profile`), `--changed FILES`.

---

### `terrain ai`

AI / eval namespace.

| Subcommand | Purpose |
|---|---|
| `terrain ai list` | List detected AI/eval scenarios, prompt surfaces, dataset surfaces, and eval files |
| `terrain ai run` | Execute eval scenarios and collect results |
| `terrain ai replay` | Replay and verify a previous eval run artifact |
| `terrain ai record` | Record eval run results as a baseline snapshot |
| `terrain ai baseline` | Show the latest baseline snapshot |
| `terrain ai baseline compare` | Diff the latest baseline against the prior one |
| `terrain ai doctor` | Validate AI/eval setup (scenarios, prompts, datasets, eval files, graph wiring) |
| `terrain ai findings` | Emit AI eval-gap findings with confidence + severity + evidence |

Notable flags:
- `terrain ai list --verbose` — detection evidence per surface
- `terrain ai run --base REF` — impact-based scenario selection
- `terrain ai run --full` — skip impact selection, run everything
- `terrain ai run --dry-run` — preview without executing
- `terrain ai findings --posture=gate` — gate-tier filter (default: observability)

---

### `terrain inject`

Match a prompt template against a curated set of jailbreak / injection patterns (DAN-style, instruction leak, system-prompt fishing, role confusion, indirect-via-retrieval) and emit a runnable test scaffold. LLM-free — Terrain never invokes the model; the assertion is the adopter's.

```
terrain inject --prompt <path> [--lang python|typescript|json] [--list]
```

Flags:
- `--prompt PATH` — required.
- `--lang LANG` — `python` (default, pytest), `typescript` (vitest), `json` (raw).
- `--json` — shortcut for `--lang=json`.
- `--list` — list matched patterns; don't emit the scaffold.

---

### `terrain scaffold`

Generate a runnable mutation-test scaffold from a JSON Schema. Each declared property produces a parametrized test exercising boundary cases (empty / whitespace / max-length / unicode-edge / SQL-injection / XSS / path-traversal / null-byte / INT32 bounds / near-double-limits). Deterministic; LLM-free.

```
terrain scaffold --schema <path> [--prompt <path>] [--lang python|typescript|json]
```

Flags:
- `--schema PATH` — required.
- `--prompt PATH` — optional; printed as a header comment in the output.
- `--lang LANG` — `python` (default), `typescript`, `json`.

---

### `terrain plugins`

Validate third-party plugin manifests. The runtime that loads and executes plugins is reserved for a future release; the manifest contract is stable today.

| Subcommand | Purpose |
|---|---|
| `terrain plugins manifest <path>` | Validate a plugin manifest against schema v1 |
| `terrain plugins list` | List registered plugins (always empty until the runtime ships) |
| `terrain plugins add / remove` | Reserved; returns "not yet implemented" |

Plugin rules must declare an allowed `mechanism_class` — `structural-ast`, `import-graph`, `receiver-type`, or `manifest-schema`. Literal-string and regex primitives are explicitly forbidden so every rule clears a class, not a cell.

---

## Conversion / migration commands

### `terrain convert`

Go-native test source conversion across 25 directions: E2E (Cypress, Playwright, Selenium, WebdriverIO, Puppeteer, TestCafe), JS unit (Jest, Vitest, Mocha, Jasmine), Java (JUnit 4/5, TestNG), Python (pytest, unittest, nose2).

```
terrain convert <source> --from <framework> --to <framework> [flags]
```

Flags:
- `--from, -f` / `--to, -t` — source / target framework.
- `--output, -o` — output path.
- `--auto-detect` — detect source framework automatically.
- `--validate` / `--strict-validate` — output validation (validate is on by default).
- `--on-error` — `skip` | `fail` | `best-effort`.
- `--plan` — show conversion plan; don't write.
- `--dry-run` — preview without writing.
- `--batch-size N` / `--concurrency N` — defaults 5 / 4.

### `terrain convert-config`

```
terrain convert-config <source> --to <framework>
```

### `terrain migrate`

Project-wide migration with state tracking, resume, and retry.

```
terrain migrate <dir> --from <framework> --to <framework> [--continue] [--retry-failed]
```

### `terrain estimate`

Estimate migration complexity without writing files.

```
terrain estimate <dir> --from <framework> --to <framework>
```

### Supporting commands

- `terrain status [--dir PATH]` — migration progress.
- `terrain checklist [--dir PATH]` — generate migration checklist.
- `terrain doctor [path]` — migration diagnostics.
- `terrain reset [--dir PATH] --yes` — clear migration state.
- `terrain list-conversions` — list all 25 supported directions.
- `terrain shorthands` — list shorthand aliases (e.g. `cy2pw`, `jest2vt`).
- `terrain detect <file-or-dir>` — detect dominant framework.

### `terrain serve` *(experimental)*

Local HTTP server: HTML report at `/`, JSON API at `/api/analyze` and `/api/health`. The HTML view auto-refreshes every 30 seconds. Intended for single-developer local exploration; a richer dashboard with embedded charts is reserved for a future release.

```
terrain serve [--port N] [--host HOST] [--read-only]
```

Flags:
- `--port N` (default 8421), `--host HOST` (default `127.0.0.1`; setting any other value emits a stderr warning — the server has no built-in auth).
- `--read-only` — reject non-GET/HEAD/OPTIONS with HTTP 405. Every handler today is GET-only, so this gates against any future state-changing endpoint.

Security defaults: binds to localhost; CSP + `X-Frame-Options: DENY` + `X-Content-Type-Options: nosniff` + `Referrer-Policy: no-referrer` on every response; cross-origin browser requests rejected with 403 (empty `Origin`/`Referer` headers from curl / server-to-server are allowed). Reads from disk; never writes.

For multi-developer hosts, use an SSH tunnel rather than binding to a non-localhost address. First-class authentication is reserved for a future release.

---

## GitHub Actions templates

Two opt-in workflow templates ship in [`.github/workflows/`](../.github/workflows/):

- **`terrain-pr.yml`** — runs on every PR; analyzes impact, selects relevant tests, runs them, posts a unified PR comment.
- **`terrain-ai.yml`** — runs on every PR; checks AI surface coverage, runs impact-scoped eval scenario selection, posts a PR comment summarizing the AI risk review. Blocks the PR when the gate returns "block" (uncovered safety-critical surfaces, accuracy regressions, etc.).

Copy whichever fits to your repo's `.github/workflows/` to enable.

---

## Language support

| Layer | JS/TS | Go | Python | Java |
|---|---|---|---|---|
| Framework detection | ✓ | ✓ | ✓ | ✓ |
| Code unit extraction | ✓ | ✓ | ✓ | ✓ |
| Import graph | ✓ full resolver | ✓ AST | ✓ | heuristic |
| Fixture detection | ✓ | ✓ | ✓ | ✓ |
| Impact analysis | full | full | full | heuristic |

Java impact analysis uses structural heuristics (file-path matching, framework conventions) rather than dependency tracing; full import resolution is reserved for a future release.
