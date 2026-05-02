# Changelog

All notable changes to Terrain are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Post-0.2 work tracked separately.

## [0.2.0] — 2026-05-02 — AI parity, calibration gate, CLI compression

The release that turns Terrain's AI story into something testable. Twelve
new AI detectors ship with calibration anchors at 100% precision/recall
on a 27-fixture corpus; the CLI surface compresses 35→11 commands; the
calibration runner becomes a load-bearing regression gate. Per
`docs/release/0.2.md`.

### AI detector batch (12/12 from the round-4 plan)

10 ship `[stable]`, 2 ship `[experimental]`. 11 of 12 carry calibration
anchors at 1.00 precision/recall on the per-detector fixture corpus;
`aiHardcodedAPIKey` ships without a fixture (constructing a non-example
real-shaped key would risk repository secret-scanner alerts — see
`docs/release/0.2-known-gaps.md` for the calibration plan in 0.3).

- **`aiHardcodedAPIKey`** `[stable]` — config files leaking provider API
  keys. *No calibration fixture; tested via unit tests only.*
- **`aiNonDeterministicEval`** `[stable]` — eval configs declaring a model
  without pinning `temperature: 0`. Per-provider scoping (multi-provider
  configs emit one verdict per provider entry, not one for the whole
  file). Accepts `.yaml`, `.yml`, `.json`, `.toml`.
- **`aiModelDeprecationRisk`** `[stable]` — floating model tags
  (`gpt-4`, `claude-3-opus`, etc.) and sunset variants
  (`text-davinci-003`, `code-davinci-001/002`, `claude-2`). Severity by
  category: deprecated → High, floating → Medium. Comment-prefix
  detection covers SQL `--`, INI `;`, HTML `<!--`, Markdown bullet/
  blockquote, RST `..`, VB `'`.
- **`aiPromptInjectionRisk`** `[experimental]` — user-input concatenated
  into prompt-shaped variables without sanitisation. Multi-line
  concatenation supported (3-line window). User-input shapes cover
  Express/Koa, FastAPI typed-parameter constructs, Flask, Django,
  Pyramid, gRPC, and CLI-arg-driven input.
- **`aiToolWithoutSandbox`** `[stable]` — destructive agent tools without
  an approval gate, sandbox flag, or dry-run path. Structural
  key-name + truthy-value check (description fields excluded so
  adversarial bypass via prose doesn't suppress the finding).
  Benign-object whitelist (`delete_cache`, `purge_logs`, etc.) suppresses
  the bounded-blast-radius cases; always-high verbs (`exec`, `eval`,
  `send_payment`) keep firing regardless.
- **`aiSafetyEvalMissing`** `[stable]` — safety-critical AI surfaces
  (prompt / agent / tool / context) with no safety-shaped scenario
  coverage. Implicit path-based coverage when `CoveredSurfaceIDs` is
  empty (the default for auto-derived scenarios) so the detector
  doesn't flood false positives on the dominant scenario shape.
- **`aiHallucinationRate`** `[stable]` — eval runs with
  hallucination-shaped failure rate above the configured threshold.
  Denominator excludes errored cases (provider crash / timeout) via
  `caseIsScoreable` so infra noise doesn't dilute the rate.
  Keyword set covers 17 stems including "not in source", "no
  evidence", "unsupported", "outside scope", "off-topic".
- **`aiCostRegression`** `[stable]` — paired-case avg cost-per-case rising
  more than the configured threshold versus a baseline snapshot. Both
  relative AND absolute deltas must clear (default `MinAbsDelta` =
  $0.0005/case) so $0.0001 → $0.0002 noise doesn't fire. Confidence
  scales by paired-case count (0.5 at paired=1, plateau at 0.9 from
  paired≥20). Catastrophic regressions (≥2× cost) escalate to High
  via `sev-high-008`.
- **`aiRetrievalRegression`** `[stable]` — retrieval-quality named scores
  dropping versus baseline. Allowlist covers Ragas modern
  (`context_precision`, `context_recall`, `context_entity_recall`),
  Ragas legacy (`context_relevance`), `nDCG`, `coverage`, `faithfulness`,
  `answer_relevancy`, and LangSmith `relevance_score`. Confidence
  scales by paired-case count (shared helper with `aiCostRegression`).
- **`aiPromptVersioning`** `[stable]` — prompt-kind surfaces shipping
  without a recognisable version marker. Placeholder tokens
  (`version: TODO`, `version: TBD`, `version: ???`,
  `version: placeholder`, `version: none`, `version: unknown`) do NOT
  satisfy the requirement.
- **`aiFewShotContamination`** `[experimental]` — prompt few-shot examples
  overlapping verbatim with the inputs of eval scenarios that cover them.
  Implicit path-based coverage matches the dominant auto-derived
  scenario shape (empty `CoveredSurfaceIDs`).
- **`aiEmbeddingModelChange`** `[stable]` — repos referencing an embedding
  model in source without a retrieval-shaped eval scenario. Prefers
  structured RAG surfaces (EvidenceStrong) when present; falls back to
  file-scan (EvidenceModerate). Catches env-var-loaded models via
  framework constructor patterns (`OpenAIEmbeddings`,
  `SentenceTransformer`, `langchaingo.NewEmbeddings`, etc.).

### Calibration corpus + load-bearing gate

- **27 fixtures × 33 distinct AI/quality/health/migration/structural/
  runtime signal types fire on real-shaped fixtures.** *The gate is a
  recall gate*: every labeled signal must still fire after a detector
  change. Extra signals emitted but not labeled are silent (counted
  neither as TP nor FP). The precision-floor companion gate (≥90%
  precision against a labeled-repo corpus) slipped to 0.3 — see
  `0.2-known-gaps.md` "Calibration corpus follow-ups".
- **Calibration gate is now load-bearing.** `t.Errorf` on any
  unmatched expected label. Empty-corpus bypass closed: `t.Skipf` →
  `t.Fatalf` with `minFixtures=25` assertion. Deletion no longer
  skips the gate.
- **Match-key precision improved.** Matcher key now includes `Symbol`
  in addition to `(Type, File)` so multi-symbol fixtures distinguish
  "fired per-symbol" from "fired once on the same line."
  `ExpectedAbsent` path matching uses the same normalization as the
  positive-match path, fixing eval-data detectors that stamp absolute
  paths.
- **Known gaps deferred to 0.3**: `aiHardcodedAPIKey` has no fixture
  (constructing a real-shaped key risks repo secret-scanner alerts);
  no DeepEval or Ragas-shaped fixtures (only Promptfoo); no near-
  threshold fixtures for cost/retrieval/coverage detectors so a
  comparator-flip regression could survive.
- **Eval-data fixture authoring.** Calibration runner auto-discovers
  per-fixture `eval-runs/{promptfoo,deepeval,ragas}.json` and
  `baseline.json`. Synthesises baseline snapshots from
  `baseline/eval-runs/` so regression-shaped fixtures are authored as two
  pairs of framework JSON files, not hand-written snapshot blobs.
- **`terrain.yaml` `scenarios.description` field.** Propagates onto
  `models.Scenario.Description` for detectors that compare scenario
  inputs to prompt content.

### CLI restructure — phase A (canonical 11 + 33 legacy aliases)

The canonical 11-command surface ships as non-breaking namespace
dispatchers (`terrain report`, `terrain migrate`, `terrain config`)
alongside the historical 32 top-level commands. The binary today
accepts ~43 top-level entries; the 11-command shape is the
*recommended* surface, not the only-reachable surface, and `terrain
--help` still lists the legacy commands. Legacy commands remain
through 0.2; in-band deprecation warnings are deferred to 0.2.x;
removal targets 0.3.

```
1.  terrain init
2.  terrain analyze
3.  terrain report <verb>     # 9 read-side verbs (summary, insights,
                              #   metrics, explain, show, impact, pr,
                              #   posture, select-tests)
4.  terrain migrate <verb>    # 11 verbs (run/config/list/detect/
                              #   shorthands/estimate/status/checklist/
                              #   readiness/blockers/preview)
5.  terrain ai <verb>
6.  terrain portfolio <verb>
7.  terrain config <verb>     # feedback, telemetry
8.  terrain doctor
9.  terrain debug <verb>
10. terrain serve
11. terrain version
```

`terrain convert <file> --to <framework>` continues to work via the
per-file converter — the `convert` namespace dispatcher falls through
to `runConvertCLI` (single-file mode) for non-verb args, distinct from
the `migrate` namespace's directory-mode fall-through. Phase B (folding
`policy`/`compare` into `analyze` flags) and the `--focus`/`--output`
flag-collapse from former top-level `focus`/`export` are deferred — see
"Deferred to 0.3."

### Eval framework adapters

- **Promptfoo.** `internal/airun.ParsePromptfooJSON` reads `--output` JSON
  (v3 nested + v4 flat shapes). Wired through `--promptfoo-results` flag.
- **DeepEval.** `--deepeval-results` flag, same envelope shape.
- **Ragas.** `--ragas-results` flag, same envelope shape.
- **Baseline-snapshot mechanism.** `--baseline <path>` loads a previous
  `TestSuiteSnapshot` and attaches it to the current run for
  regression-aware detectors.

### RAG structured parser — Go + Java added

`ParseRAGStructured` was JS+Python only in 0.1; 0.2 adds langchaingo
(Go) and langchain4j (Java) parsers. Same six component kinds across
all four languages: embedding model, vector store, text splitter,
retriever, document loader, reranker. Config extraction
(`ChunkSize`, `TopK`, `ModelName`) maps to `RAGComponentConfig` so
the embedding-model-change detector gets the high-confidence
structured-surface path on Go and Java codebases too.

### SignalV2 schema

Nine new fields on `models.Signal`, all `omitempty`:
`SeverityClauses`, `Actionability`, `LifecycleStages`, `AIRelevance`,
`RuleID`, `RuleURI`, `DetectorVersion`, `RelatedSignals`,
`ConfidenceDetail`. **TestSuiteSnapshot schema** bumped from 1.0.0 →
1.1.0; **manifest export schema** stays at 1.0.0 (the two version
strings are independent — see `docs/schema/COMPAT.md`).

### Severity rubric

17 stable clauses (`sev-{critical,high,medium,low,info}-NNN`) named in
`internal/severity/rubric.go`, rendered to `docs/severity-rubric.md`
via `cmd/terrain-docs-gen`. Each detector quotes the clauses it
exercises in its emitted signals.

### Auto-generated rule docs

`docs/rules/<domain>/<rule>.md` auto-generated for all 68 rules by
`cmd/terrain-docs-gen`. Hand-authored prose below the
`<!-- docs-gen: end stub -->` marker is preserved across regenerations.
Drift fails `make docs-verify` (CI gate).

### Other infrastructure

- **Generated signal manifest export.** `docs/signals/manifest.json` is
  regenerated from `internal/signals.allSignalManifest` via
  `cmd/terrain-docs-gen`. `make docs-gen` writes; `make docs-verify` diffs.
- **CI hard-fail gate** on `make docs-verify` (extended ubuntu runner).
- **Performance regression gate.** `make bench-gate` +
  `terrain-bench-gate` fail PRs that regress benchmarks >10%.
- **SLSA L2 build provenance.** `actions/attest-build-provenance@v3`
  emits a signed in-toto attestation per release archive.
- **Tree-sitter parser pool.** `sync.Pool` reuses parsers across calls.
- **Pytest fixture dependency graph.** `@pytest.fixture` parameter
  extraction feeds the import graph.
- **JUnit 5 `@Nested` + `@DisplayName` extraction.** Hierarchical test
  identification matches the framework's reporting model.
- **Hierarchical Go `t.Run` extraction.** Sub-test stack tracking.
- **Vitest in-source tests.** `if (import.meta.vitest)` blocks discovered
  alongside conventional spec files.
- **TSConfig path resolution.** `extends` chain + multi-target +
  `jsconfig.json` fallback.
- **`.terrain/conversion-history/` audit trail.** Every conversion writes
  a JSONL line.
- **Per-file conversion confidence.** Per-file scores expose where the
  converter was uncertain.
- **`terrain convert --preview`.** LCS-based unified diff.
- **AI surface detection expansion.** Datasets, pgvector cursor calls,
  MCP tool definitions, in-memory FAISS indexes.
- **Capability validation gap detector.** Pairs AI capabilities with
  eval scenarios; flags capabilities without validation.
- **`terrain ai run` captures eval framework output** to
  `.terrain/artifacts/`.
- **Cosign keyless signing + npm provenance + SLSA attestations** on
  every release archive. *Caveat*: the npm postinstaller verifier
  (`bin/terrain-installer.js`) **degrades to checksum-only** when
  `cosign` is not installed on the host (returns
  `verified: false, reason: 'cosign-missing'`) rather than aborting.
  The hard-fail framing in `docs/release/0.2.md` overstates the
  current behavior. Promoting to mandatory cosign verification
  (with `TERRAIN_INSTALLER_SKIP_VERIFY=1` as the documented
  escape) is on the 0.2.x list.

### Changed

- `package.json`, `extension/vscode/package.json`, `package-lock.json`
  at 0.2.0.

### Fixed

- Race detector failure on Ubuntu CI from `os.Stdout`-touching parallel
  tests; `runCaptured` wraps the previously-unprotected callers.
- `TestParallelForEachIndexCtx_CancelMidway` flaky on Ubuntu race
  runners; per-item sleep makes cancellation propagation visible.
- Calibration coverage fixture wasn't tracked
  (`.gitignore` filtered `coverage/`); exception added.
- `docs-verify.sh` lacked the executable bit in the git index.
- `aiModelDeprecationRisk` regex matched dot-versioned variants like
  `claude-2.1` and `gpt-3.5-turbo-0125` against their undated parents
  (`claude-2`, `gpt-3.5-turbo`) — guaranteed false positive on current
  pinned models. Trailing-boundary class now excludes `.`.
- `aiRetrievalRegression` allowlist missed Ragas's modern
  `context_precision`/`context_recall`/`context_entity_recall` keys;
  detector silently fired zero signals on real Ragas runs. Added.
- `terrain convert <file> --to <framework>` regressed during the CLI
  fold-in (routed to project-wide migrate runner). Restored by giving
  `convert` its own namespace dispatcher with `runConvertCLI` as the
  fall-through.

### Polish (release-prep adversarial review fixes)

Beyond the headline detector + CLI work, two parallel adversarial-
review passes (`/gambit:parallel-agents` × 7 domains, ~245 findings
after dedup) closed the verified P0/P1 subset before tag:

- **Release infra**: `npm-release` job adds `setup-go` (would have
  crashed at first publish via `prepublishOnly → verify-pack.js → go
  build`); `supply-chain.md` drops a phantom `windows/arm64` artifact
  goreleaser doesn't build; SLSA L2 build-provenance via
  `actions/attest-build-provenance@v3` is documented; new
  `release-smoke` job downloads + verifies the published archive
  reports the tag's version.
- **Engine self-diagnostic**: `detectorPanic` added to
  `models.SignalCatalog` + manifest. Pre-fix `safeDetect`'s panic-
  recovery emitted a sentinel that `ValidateSnapshot` then rejected as
  unknown, dropping the whole snapshot — defeating the graceful-
  degradation promise. `RequiresGraph` mismatch now surfaces a
  detectorPanic-shaped diagnostic instead of silently dropping the
  registration.
- **Eval adapters**: Promptfoo errors-bucket wired through the row-
  derived stats fallback so provider-crash rows land in
  `Aggregates.Errors` (not `Failures`); per-case cost falls back to
  top-level `cost` field when `r.response.tokenUsage.cost` is zero;
  `createdAt` magnitude check (seconds vs millis) handles v4 CLI
  variants. DeepEval gains `runId` fallback (newer 1.x shape) and
  metric-name whitespace normalization. Ragas accepts
  `evaluation_results` (modern ≥0.1.0) and `scores` (DataFrame export)
  shapes alongside legacy `results`. Envelope `SourcePath` now
  repo-relative (forward-slash normalized) so SARIF output doesn't
  leak developer home directories.
- **CLI**: 14 legacy commands gain `legacyDeprecationNotice` calls so
  `TERRAIN_LEGACY_HINT=1` produces uniform migration prompts;
  `--read-only` on `terrain serve` promoted from no-op to actual HTTP
  405 enforcement; `terrain version --json` includes
  `schemaVersion`; `terrain show`/`explain` use a dedicated `exit 5
  (not found)` so CI scripts can branch on missing-entity vs analysis
  failure. `runDepgraph` routed through `AnalyzeContext` for Ctrl-C
  unwind.
- **Determinism**: `sortSignals` adds `Symbol` as a tiebreaker after
  `Line` and switches to `sort.SliceStable` so byte-identical snapshot
  output under `SOURCE_DATE_EPOCH` survives signals on the same
  (Type, File, Line) but different symbols.
- **Supply chain hardening**: every PR-triggered workflow gains
  `concurrency` + `cancel-in-progress` so force-pushes don't pile up
  runs; `timeout-minutes` on every job (15-45min); CodeQL Python
  matrix dropped (no production Python under analysis);
  `COSIGN_EXPERIMENTAL=1` removed from cosign 2.x invocations;
  installer redirect chain capped at 5; goreleaser archives ship
  `LICENSE` + `README.md`.
- **Documentation**: `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1);
  three issue templates (bug-report, false-positive, feature-request);
  new `docs/glossary.md`, `docs/versioning.md`,
  `docs/compatibility.md`; per-framework integration guides under
  `docs/integrations/{promptfoo,deepeval,ragas}.md`;
  `docs/internal/README.md` disclaimer so the public docs tree
  doesn't mix planning artifacts with shipping documentation.
- **CLI visual polish** (PR #130): dropped a stray `file:` loader-
  prefix in `terrain insights` source paths; replaced `n thing(s)`
  pluralisation notation with proper plural forms across analyze /
  insights / summary / reporting (~19 sites); switched dimension
  display labels to sentence case (`Coverage Depth` →
  `Coverage depth`) for inline use; added polarity-aware band
  rendering so risk-shaped dimensions read naturally
  (`Structural risk: Strong` → `Structural risk: Low`); replaced
  band-only posture lines with concrete totals
  (`Health: Strong  (28 / 772 skipped)`) and dropped zero-valued
  measurements so the line shows what moved the band; added
  `debug <verb>` verb list to top-level help for parity with the
  other namespace dispatchers; `terrain export benchmark` now
  accepts `--json` (no-op; output is always JSON) for flag parity.

### Deferred to 0.3

Items called out in `docs/release/0.2.md` that didn't ship and are
explicitly deferred:

- **Scoring v2 band re-anchoring** — needs a corpus of labeled
  *repositories* (not just per-detector calibration fixtures) to derive
  percentile-based band thresholds. The 50-labeled-repo corpus
  promised as 0.2 critical-path item #4 also slips here.
- **Conversion top-3 fixture corpora to A-grade with 95% post-conversion
  pass rate** — was a Tier-2 release gate in `docs/release/0.2.md`;
  reclassified to deferred. Bulk content authoring (~50 fixtures × 3
  directions).
- **CLI restructure phase B** — fold `policy` into
  `analyze --policy=<file>` and `compare` into `analyze --against=<ref>`.
  Different exit-code semantics; deserves its own review.
- **Universal flag schema + `--detail 1/2/3`** — Phase A landed only
  the namespace dispatchers; flag parity across legacy and namespace
  paths is still inconsistent (`--root` vs `-root`, `--json` vs
  `--format json`).
- **Plugin architecture skeleton** (`internal/airun/plugin.go` interface
  for community adapters) — promised in `docs/release/0.2.md`, not
  shipped.
- **Confidence intervals in `terrain explain` output** — the
  `ConfidenceDetail` struct ships in SignalV2, but the renderer doesn't
  surface `IntervalLow`/`IntervalHigh`. Most intervals are author-
  guessed (`Quality: "heuristic"`) rather than measured.
- **In-band deprecation warnings on legacy commands** — the
  0.2 → 0.2.x → 0.3 runway has no mechanism in 0.2; users running
  `terrain summary` get no hint to switch to `terrain report summary`.
  Targeted for 0.2.x.
- **Manifest entries promoted to ship in 0.2 that didn't promote**:
  `evalFailure`, `evalRegression`, `accuracyRegression`,
  `schemaParseFailure`, `safetyFailure`, `aiPolicyViolation`,
  `toolGuardrailViolation`. Promotion plans updated.
- **`terrain doctor` ↔ `terrain ai doctor` consolidation** — slipped
  from 0.1.2 → 0.2 → now 0.3.
- **`terrain ai gate`** — feature-status promised 0.2/0.3 timeline; not
  shipped.

See `docs/release/0.2-known-gaps.md` (added with this release) for the
full backlog including review-flagged detector improvements (multi-
provider non-determinism scoping, `safety_eval_missing` over-firing on
auto-derived scenarios, `tool_without_sandbox` substring suppression
bypass, cost-regression `MinAbsDelta` floor, etc.).

## [0.1.2] — Truth-up & foundation

The deliberate "boring" release. No new headline features; instead, every
gap between what Terrain marketed and what the code actually delivered is
either closed or explicitly tagged. Schemas, signal vocabulary, and
distribution surfaces are locked so 0.2 can ship features against a stable
foundation. Per `docs/release/0.1.2.md`.

### Honest about what ships

- New: `docs/release/feature-status.md` is the canonical inventory of
  stable / experimental / planned features. Drift between marketing and
  code becomes a release blocker starting in 0.2.
- README: example CLI outputs are now framed explicitly as illustrative
  shape, not literal output. Three signals shown (`xfailAccumulation`,
  statistical ">10% failure rate" flaky detection, `0.91+` duplicate
  similarity) are explicitly tagged `[experimental]` or `[planned]`
  because the underlying detectors don't ship in 0.1.2.
- README: the "30 seconds" claim is now scoped to small-to-medium repos
  with realistic numbers for larger workspaces.
- `docs/legacy/`: every file now carries a strong **DEPRECATED — DO NOT
  USE FOR NEW WORK** banner pointing at current docs.
- `internal/convert/catalog.go`: 10 conversion directions tagged
  `GoNativeStateExperimental` per round 3 audit (Java, Python,
  TestCafe, Selenium families). `terrain convert` warns to stderr when
  invoked on an experimental direction.

### Distribution

- Goreleaser now builds five platforms instead of one: darwin/amd64,
  darwin/arm64, linux/amd64, linux/arm64, windows/amd64. Each is built
  on a matching CI runner because go-tree-sitter requires CGO and
  cannot cross-compile cleanly.
- Release archives, SBOMs, and checksums are signed via Sigstore
  keyless cosign. Signatures and certificates are uploaded with each
  artifact.
- npm postinstall (`bin/terrain-installer.js`) gains a best-effort
  cosign verifier: in 0.1.2 it warns on missing cosign, missing
  signature artifacts, or verification failure but does not block
  install. 0.2 makes this hard-fail unless
  `TERRAIN_INSTALLER_SKIP_VERIFY=1` is set.
- `.github/dependabot.yml`: gomod, github-actions, and the VS Code
  extension package are now tracked alongside the existing root-npm
  ecosystem. Tree-sitter grammar updates surface as PRs automatically.

### Schema & signal vocabulary

- `internal/signals/manifest.go` (new): single source of truth for all
  56 signal types. Status (stable / experimental / planned), default
  severity, confidence range, evidence sources, RuleID, RuleURI, and
  promotion plan are recorded for every entry.
  `TestManifest_MatchesSignalTypes` makes constant↔manifest drift a
  build failure.
- `internal/models/MaxSupportedMajorSchema = 1`. Snapshot reads now
  reject majors above the current binary's understanding via
  `ValidateSchemaVersion`.
- `docs/schema/COMPAT.md` (new): the public compatibility contract.
  Documents what is allowed at minor steps, what requires a major bump,
  and how the manifest's drift gates fit in.
- `docs/scoring-rubric.md` and `docs/health-grade-rubric.md` (new):
  every magic number behind risk-band assignment and Health Grade
  derivation is now extracted to a named constant and explained.

### Correctness & durability fixes

- `.gitignore` is now honoured during repository scanning. Vendored
  trees and generated artefacts the user has explicitly excluded are
  no longer walked.
- File cache is bounded: per-file 8 MB, total 256 MB. Files past the
  cap stream from disk on every read instead of failing the process.
- Worker-pool sizing capped at `min(GOMAXPROCS, 16)`.
- Framework detection probe size raised from 64 KB to 256 KB.
- `internal/metrics/metrics.go:Derive`, `internal/analyze/analyze.go:Build`,
  and `internal/insights/insights.go:Build` are now nil-safe; the
  adversarial test that previously swallowed their panics with
  `t.Logf("acceptable")` is now a strict contract test that fails on
  panic.

### CLI ergonomics

- `NO_COLOR`, `TERM=dumb`, and every common CI provider
  (GitHub Actions, GitLab, CircleCI, Buildkite, Jenkins, Azure
  Pipelines) now suppress progress output. Logs no longer get
  carriage-return garbage in CI.
- Did-you-mean suggestions on unknown commands. Levenshtein distance
  ≤2 gets you up to three suggestions; in-tree implementation, no new
  dependency.
- Exit codes documented as a 5-level scheme. `exitPolicyViolation`
  remains 2 for back-compat in 0.1.2; 0.2 splits it cleanly.
- `terrain doctor` and `terrain ai doctor` consolidation deferred to
  0.2 (the larger CLI restructure).

### Security & privacy

- `--base` git refs are validated against an allow-listed regex
  before being passed to `git diff`. Shell-injection payloads,
  reflog selectors (`@{-1}`), `--upload-pack=evil`, and whitespace
  are all rejected.
- Telemetry config and event log now ship 0o600; the parent
  `~/.terrain` directory ships 0o700.
- SARIF emission gains `--redact-paths`; absolute paths inside the
  repo are rewritten relative, paths outside collapse to bare
  basenames.
- `terrain serve` ships a security middleware: CSP, X-Frame-Options
  DENY, X-Content-Type-Options nosniff, Referrer-Policy no-referrer
  on every response. Origin/Referer validation rejects browser-driven
  cross-origin attacks against localhost. New `--host` flag warns
  when bound to a non-localhost address.

### CI & governance

- Multi-OS test matrix: ubuntu-latest, macos-latest, windows-latest.
  ubuntu remains the canonical runner with the race detector and full
  fixture suite; macos and windows run unit tests to catch
  platform-specific regressions before binaries ship.
- Determinism gate (`make test-determinism`) now runs in CI on every
  PR.
- New: `.github/CODEOWNERS`, `.github/pull_request_template.md`,
  `.husky/pre-commit` (blocks files >5 MB and binary-only extensions).
- `.nvmrc` strict-pinned to `22.11.0`.

### Removed

- `internal/plugin/` package (extension-point interfaces that were
  never wired into the engine). The only adopters were tests in the
  package itself. Detector contributors should read
  `docs/engineering/detector-architecture.md` for the actual in-tree
  registry pattern.

### Versioning

- npm package, `extension/vscode/package.json`, and
  `package-lock.json` all bumped to `0.1.2`. Git-tag/package.json
  drift is now a release-gate failure.

## 0.1.0 — Test System Intelligence Platform (2026-04-06)

Terrain 0.1.0 is the first public release of the Terrain test intelligence
platform. A ground-up rewrite of the analysis engine in Go, the legacy
JavaScript converter becomes one subsystem within a signal-first intelligence
platform that maps test suites, surfaces risk, and drives CI optimization —
all from a single statically-linked binary with zero runtime dependencies.

**83k lines of Go across 47 internal packages. 210 test files. 48 test
packages, all passing. Zero `go vet` warnings. Zero `gofmt` issues.**

### Core Analysis Pipeline

- 10-step deterministic pipeline: scan, policy, signals, ownership, runtime, risk, coverage, measurement, portfolio, snapshot
- Repository scanning with framework detection (17 frameworks across Go, JS/TS, Python, Java)
- Test file discovery, code unit extraction, import graph construction
- Signal-first architecture: every finding is a structured Signal with type, severity, confidence, evidence, and location
- Code surface inference: prompts, contexts, datasets, tool definitions, retrieval/RAG, agents, eval definitions
- Behavior surface derivation from API routes, event handlers, and state transitions
- Environment/device matrix analysis from CI configs and framework settings

### 18 Measurements Across 5 Posture Dimensions

| Dimension | Measurements |
|-----------|-------------|
| Health | flaky share, skip density, dead test share, slow test share |
| Coverage Depth | uncovered exports, weak assertion share, coverage breach share |
| Coverage Diversity | mock-heavy share, framework fragmentation, E2E concentration, E2E-only units, unit test coverage |
| Structural Risk | migration blocker density, deprecated pattern share, dynamic generation share |
| Operational Risk | policy violation density, legacy framework share, runtime budget breach share |

### Signal Detectors

- **Quality**: weak assertions, mock-heavy tests, untested exports, assertion-free tests, orphaned tests
- **Health**: slow tests, flaky tests, skipped tests, dead tests, unstable suites (runtime-backed)
- **Migration**: deprecated patterns, dynamic test generation, custom matchers, unsupported setup, framework fragmentation
- **Governance**: policy violations, legacy framework usage, runtime budget exceeded, AI safety
- **Structural**: phantom eval scenarios, blast-radius hotspots, coverage gap clusters

### CLI Commands (30+)

**Primary commands:**
- `terrain analyze` — full test system analysis with key findings, repo profile, risk posture
- `terrain insights` — prioritized health report with categorized findings and recommendations
- `terrain impact` — change-scope analysis: impacted units, tests, protection gaps, owners
- `terrain explain` — structured reasoning chains for any entity (test, unit, owner, scenario, selection)

**Supporting commands:**
- `terrain init` — detect data files and generate recommended analyze command
- `terrain summary` — executive summary with risk, trends, benchmark readiness
- `terrain focus` — prioritized next actions with top risk areas
- `terrain posture` — detailed posture breakdown with measurement evidence
- `terrain portfolio` — portfolio intelligence: cost, breadth, leverage, redundancy
- `terrain metrics` — aggregate metrics scorecard
- `terrain compare` — snapshot-to-snapshot trend tracking
- `terrain select-tests` — protective test set for a change
- `terrain pr` — PR/change-scoped analysis (markdown, comment, annotation output)
- `terrain show <entity> <id>` — drill into test, unit, owner, or finding
- `terrain migration <sub>` — readiness, blockers, or preview
- `terrain policy check` — evaluate local policy rules (exit 0/1/2 for CI)
- `terrain export benchmark` — privacy-safe JSON export
- `terrain serve` — local HTTP server with HTML report and JSON API

**AI / eval:**
- `terrain ai list` — list detected scenarios, prompts, datasets, eval files
- `terrain ai run` — execute eval scenarios with impact-based selection
- `terrain ai replay` — replay and verify a previous run artifact
- `terrain ai record` — save eval results as baseline
- `terrain ai baseline` — manage eval baselines (show, compare)
- `terrain ai doctor` — validate AI/eval setup

**Conversion / migration:**
- `terrain convert` — Go-native source test conversion (25 directions)
- `terrain convert-config` — framework config file conversion
- `terrain migrate` — project-wide migration with state tracking
- `terrain estimate` — migration complexity estimation
- `terrain status` / `terrain checklist` / `terrain doctor` / `terrain reset` — migration workflow
- `terrain list-conversions` / `terrain shorthands` / `terrain detect` — catalog and detection
- 50 shorthand aliases (e.g., `terrain cy2pw`, `terrain jest2vt`)

**Debug:**
- `terrain debug graph|coverage|fanout|duplicates|depgraph` — internal analysis inspection

### AI / Regular Test Parity

AI surfaces receive the same CI treatment as regular tests:

- Discovery: prompts, contexts, datasets, tool definitions, RAG pipelines, agents, eval definitions
- Impact selection: `terrain ai run --base main` selects only impacted eval scenarios
- Protection gaps: changed AI surfaces without eval coverage appear in `terrain impact` and `terrain pr`
- Policy enforcement: 7 AI-specific policy rules (`block_on_safety_failure`, `block_on_uncovered_context`, etc.)
- PR comments: AI Validation section in `terrain pr` output (markdown + text)
- GitHub Action: `terrain-ai.yml` template for AI CI gates
- Health insights: uncovered AI surfaces appear in `terrain insights`

### Structural Intelligence

Three features that use the dependency graph and surface model to produce recommendations no individual tool can generate:

- **"What to test next"**: ranks untested source files by import graph dependency count — files with more dependents create larger blind spots for change-scoped test selection
- **AI behavior impact chains**: detects files with multiple AI surface types where some are covered and others aren't — a change to the untested surface can alter downstream AI behavior undetected
- **Capability gap detection**: identifies AI capabilities with only positive/accuracy scenarios but no adversarial, safety, or robustness scenarios

### Impact Analysis

- Change-scope analysis against git diff with structural dependency tracing
- Protective test set selection with confidence scoring and reason chains
- Edge-case policy: fallback strategies, confidence adjustments, risk elevation
- Drill-down views: units, gaps, tests, owners, graph, selected
- Manual coverage overlay for untestable paths
- PR-scoped output: markdown, CI comment, GitHub annotations
- AI protection gaps: changed AI surfaces without eval coverage

### Dependency Graph Engine

- 5 reasoning engines: coverage, duplicates, fanout, redundancy, profile
- Edge-case detection (14 types) with policy recommendations
- Stability clustering for shared root-cause detection
- Environment/device matrix coverage analysis
- Language-aware fanout threshold (25, calibrated across Go/Python/JS/Java)

### Go-Native Conversion Runtime

- 25 conversion directions across 4 categories (E2E, unit JS, unit Java, unit Python)
- AST-based converters using tree-sitter for structural accuracy
- Semantic validation of converted output
- Config file conversion (Jest, Vitest, Cypress, Playwright, WebdriverIO, Mocha)
- Project-wide migration with dependency ordering, state tracking, resume/retry
- Confidence scoring per converted file

### Artifact Ingestion

- Runtime: JUnit XML and Jest JSON parsers with file-level metric aggregation
- Coverage: LCOV and Istanbul JSON parsers with code unit attribution
- Coverage by type: unit, integration, e2e run labeling
- Per-test coverage mapping
- Gauntlet AI eval artifact ingestion
- Auto-discovery of common artifact paths

### Reporting

- 14 report renderers (analyze, impact, insights, posture, metrics, portfolio, summary, focus, migration, policy, comparison, explain, impact drilldown, executive)
- HTML report with embedded charts
- SARIF output for IDE integration
- GitHub annotation output for CI
- Markdown PR comment output

### Snapshot and Comparison

- `terrain analyze --write-snapshot` — persist snapshots for trend tracking
- `terrain compare` — snapshot-to-snapshot comparison with signal deltas and risk band changes
- Automatic trend loading in summary and insights commands

### Ownership

- CODEOWNERS file parsing with glob pattern matching
- terrain.yaml ownership configuration
- Git history-based ownership inference
- Owner-scoped health and quality summaries
- Owner-filtered impact analysis

### Policy and Governance

- `.terrain/policy.yaml` rule definitions
- AI policy: block on safety failure, block on signal types
- Framework allowlists and denylists
- Runtime budget enforcement
- CI-friendly exit codes (0 = pass, 2 = violations)

### Packaging

- goreleaser config for multi-platform binaries (macOS, Linux, Windows; amd64, arm64)
- SBOM generation (CycloneDX, SPDX)
- Sigstore signing
- Homebrew tap (`pmclSF/terrain/mapterrain`)
- npm package (`mapterrain`) with platform-specific binary installation
- VS Code extension with sidebar views and commands
- Opt-in privacy-respecting telemetry (local only, no network)

---

## 0.0.1 — Signal-First Foundation (2026-04-03)

Internal milestone. Initial Go-native analysis engine with signal-first
architecture, replacing the V2 JavaScript converter.

### Core Analysis
- Repository scanning with framework detection (17 JS/TS/Java/Python/Go frameworks)
- Test file discovery and code unit extraction
- Signal-first architecture: every finding is a structured Signal with type, severity, evidence, and location
- Evidence model with strength (strong/moderate/weak), source, and confidence scoring

### Signal Detectors
- **Quality**: weak assertions, mock-heavy tests, untested exports, coverage threshold breaks
- **Migration**: deprecated patterns, dynamic test generation, custom matchers, unsupported setup, framework fragmentation
- **Governance**: policy violations, legacy framework usage, runtime budget exceeded
- **Health**: slow tests, flaky tests, skipped tests (runtime-backed)

### Risk Modeling
- Explainable risk engine with reliability, change, and speed dimensions
- Risk surfaces by file, directory, owner, and repository scope
- Heatmap model with directory and owner hotspots

### Migration Intelligence
- `terrain migration readiness` — readiness assessment with quality factors and area assessments
- `terrain migration blockers` — blockers by type and area with representative examples
- `terrain migration preview` — file-level and scope-level migration difficulty preview

### VS Code Extension
- Sidebar views: Overview, Health, Quality, Migration, Review
- TreeDataProvider implementations over CLI JSON output

### Packaging
- goreleaser config for multi-platform binaries
- `terrain version` with build metadata
