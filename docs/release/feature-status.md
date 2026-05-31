# Feature Status — 0.2.0

Single source of truth for what ships in 0.2.0. Every public claim should map back to a status here. Drift is treated as a release blocker.

Each shipping capability has three tags:

- **Pillar** — Understand, Align, or Gate
- **Tier** — 1 (publicly claimable), 2 (experimental), 3 (in development, no public claim)
- **Status** — stable / experimental / planned / deprecated

The pillar tells you which of the three workflow questions this capability answers; the tier tells you whether public copy backs it.

Statuses:

- **stable** — implemented, tested, measured; no known breaking changes planned.
- **experimental** — implemented but rough: behavior may change, measurement is in progress, or surface area is intentionally narrow. Documented but flagged.
- **planned** — described in marketing/design docs as a Terrain capability but not yet implemented in code. Examples that show the future shape are kept in docs only with a `[planned]` tag.
- **deprecated** — was implemented; being removed. Will not appear in this list past one release.

If a feature is **planned**, no detector emits its signals today.

---

## Workflows

| Feature | Pillar | Tier | Status | Notes |
|---|---|---|---|---|
| `terrain analyze` | Understand | 1 | stable | Full snapshot pipeline; `--baseline` flag added in 0.2. Detector panics recovered to a single `detectorPanic` signal. `--fail-on / --timeout / --new-findings-only` ship as Gate-pillar primitives. |
| `terrain init` | cross-cutting | 1 | stable | Bootstrap a `.terrain/` config tree. |
| `terrain report summary / posture / metrics / focus` | Understand | 1 | stable | Aggregation views over signals. |
| `terrain report insights / explain` | Understand | 1 | stable | Read-side queries; reason chains supported. |
| `terrain report impact` | Gate | 1 | stable | Change-scoped selection; `--explain-selection` lands in 0.2.0. |
| `terrain report pr` | Gate | 1 | stable | PR-scoped report + comment template. `--fail-on / --new-findings-only` parity with `analyze`. |
| `terrain report select-tests` | Align | 2 | experimental | Recommended protective test set. |
| `terrain report show <kind> <id>` | Understand | 1 | stable | Drill into test, unit, owner, or finding. |
| `terrain compare` | Understand | 1 | experimental | Snapshots over time. Output format may shift across 0.2.x as adopters report friction; the JSON envelope remains forward-compatible. |
| `terrain migrate <verb>` | Align | 1 | stable | Per-direction tier badges added in 0.2. `terrain convert` retains per-file fall-through. |
| `terrain policy check` | Gate | 1 | stable | Local policy enforcement. `terrain init` emits a starter policy template. |
| `terrain config <verb>` | cross-cutting | 1 | stable | feedback, telemetry. |
| `terrain doctor` | cross-cutting | 2 | stable | Migration-readiness diagnostic; per-pillar maturity view. |
| `terrain debug <verb>` | Understand | 2 | stable | graph / coverage / fanout / duplicates / depgraph. |
| `terrain serve` | Understand | 2 | experimental | Local HTTP report; localhost-only, no auth. |
| `terrain ai list` | Gate (inventory) | 1 | stable | AI surface inventory. |
| `terrain ai doctor` | cross-cutting | 2 | stable | Diagnostic check on AI scenario configuration. |
| `terrain ai run` | Gate | 2 | stable | Captures eval framework output. AI-gate exit code 4 on `actionBlock`. Trust-boundary doc clarifies parses-vs-executes. |
| `terrain ai run` + `terrain ai record` / `terrain ai baseline compare` | Gate | 2 | stable | Regression-aware AI gate: `ai record` snapshots a known-good run; `ai baseline compare` flags regressions. |
| `terrain ai record / baseline / replay` | Gate | 2 | stable | Baseline lifecycle. |
| `terrain portfolio <verb>` | Align | 2 | experimental | Multi-repo workspace. Multi-repo manifest format + per-repo aggregation lands in 0.2; full closure is future work. |
| `terrain explain finding <id>` | Gate | 1 | stable | Resolve a stable finding ID back to its evidence. |
| `terrain suppress <id>` | Gate | 1 | stable | Write a suppression entry. |

## Detectors / signal types

The full inventory is in `internal/signals/manifest.go`. The table below is a curated view of the 0.2 changes.

### Stable in 0.2

| Signal | Detector | Notes |
|---|---|---|
| `weakAssertion` | `internal/quality/weak_assertion.go` | Regex/density-based; AST upgrade is future work. |
| `mockHeavyTest` | `internal/quality/mock_heavy.go` | |
| `untestedExport` | `internal/quality/untested_export.go` | Layered (import-graph + heuristic). |
| `coverageThresholdBreak` | `internal/quality/coverage_threshold.go` | Severity flips at hard 100%-gap boundary; smoothing is future work. |
| `slowTest` | `internal/health/slow_detector.go` | Requires runtime artifact. |
| `skippedTest` | `internal/health/skipped_detector.go` | Runtime variant. |
| `staticSkippedTest` | `internal/quality/static_skip.go` | Source-pattern variant. |
| `flakyTest` | `internal/health/flaky_detector.go` | Detects retry evidence and same-run mixed outcomes. Statistical >10% failure-rate detection is **planned**. |
| `unstableSuite` | `internal/health/unstable_detector.go` | |
| `deadTest` | `internal/health/dead_detector.go` | |
| (duplicate-cluster analysis) | `internal/depgraph/duplicate.go` | Surfaced via `DuplicateClusters` in analyze reports rather than as a `Signal`. 0.91+ similarity claim is **planned** for the AST-based upgrade. |
| `frameworkMigration` | `internal/migration/detectors.go` | |
| `policyViolation` | `internal/governance/evaluate.go` | |
| `assertionFreeImport` | `internal/structural/assertion_free_import.go` | |
| `aiHardcodedAPIKey` | `internal/aidetect/hardcoded_api_key.go` | **No fixture** (would risk repository secret-scanner alerts on synthetic high-entropy keys). Tested via unit tests only. |
| `aiNonDeterministicEval` | `internal/aidetect/non_deterministic_eval.go` | Per-provider scoping (multi-provider configs emit one verdict per provider entry). Accepts YAML / JSON / TOML. |
| `aiModelDeprecationRisk` | `internal/aidetect/model_deprecation.go` | Severity by category: deprecated → High, floating → Medium. Trailing-boundary class excludes `.` so dot-versioned variants (`claude-2.1`) no longer match their undated parent. Comment-prefix detection covers SQL `--`, INI `;`, HTML, Markdown, RST, VB. Dated `code-davinci-{001,002,edit-001}` + `code-cushman-001` enumerated. |
| `aiToolWithoutSandbox` | `internal/aidetect/tool_without_sandbox.go` | Approval markers checked structurally (key name + truthy value, description fields excluded) — closes the description-bypass loophole. Benign-object whitelist (`delete_cache`, `purge_logs`, etc.) suppresses bounded-blast-radius cases; always-high verbs (`exec`, `eval`, `send_payment`) keep firing regardless. |
| `aiSafetyEvalMissing` | `internal/aidetect/safety_eval_missing.go` | Implicit path-based coverage when `CoveredSurfaceIDs` is empty (the default for auto-derived scenarios) so the detector matches the dominant scenario shape without flooding false positives. |
| `aiHallucinationRate` | `internal/aidetect/hallucination_rate.go` | Denominator restricted to scoreable cases (errored cases excluded). 17 keyword stems including "not in source", "no evidence", "unsupported", "outside scope", "off-topic". |
| `aiCostRegression` | `internal/aidetect/cost_regression.go` | Both relative threshold AND absolute floor (`MinAbsDelta`, default $0.0005/case) must clear. Confidence scales by paired-case count (0.5 at paired=1, plateau at 0.9 from paired≥20). Catastrophic regressions (≥2× cost) escalate to High via `sev-high-008`. |
| `aiRetrievalRegression` | `internal/aidetect/retrieval_regression.go` | Allowlist covers Ragas modern (`context_precision`, `context_recall`, `context_entity_recall`), Ragas legacy, nDCG, faithfulness, LangSmith `relevance_score`. Confidence scales by paired-case count (shared helper with `aiCostRegression`). |
| `aiPromptVersioning` | `internal/aidetect/prompt_versioning.go` | Inline-version pattern requires a non-empty literal (digits/semver/calver/quoted). Placeholder tokens (`TODO`, `TBD`, `???`, `placeholder`, `none`, `unknown`) explicitly rejected. |
| `aiEmbeddingModelChange` | `internal/aidetect/embedding_model_change.go` | Prefers structured RAG surfaces (EvidenceStrong, conf 0.85); falls back to file-scan (EvidenceModerate, conf 0.80). Catches env-var-loaded models via framework constructor patterns. |
| `uncoveredAISurface` | `internal/structural/uncovered_ai_surface.go` | Emits, but coverage attribution depends on scenario declarations. |
| `phantomEvalScenario` | `internal/structural/phantom_eval_scenario.go` | |
| `capabilityValidationGap` | `internal/structural/capability_validation_gap.go` | |
| `untestedPromptFlow` | `internal/structural/untested_prompt_flow.go` | |

_…plus the long-standing structural / quality / migration / governance signals carried over from 0.1.x — see per-detector docs at `docs/rules/`._

### Experimental in 0.2

| Signal | Notes |
|---|---|
| `aiPromptInjectionRisk` | Pattern-based; assignment-anchored; 3-line concatenation window so multi-line concatenation is caught. User-input shapes cover Express/Koa, FastAPI typed params, Flask, Django, Pyramid, gRPC, CLI args. AST-precise taint-flow analysis is future work. |
| `aiFewShotContamination` | Substring overlap with minimum-chunk + distinct-word guards; implicit path-based coverage matches auto-derived scenarios. Full n-gram overlap is future work. |
| AI surface inference (`SurfacePrompt`, `SurfaceContext`, `SurfaceDataset`, `SurfaceToolDef`, `SurfaceRetrieval`) | Detection works; precision/recall not yet measured. |
| Risk band + Health Grade thresholds | Code is deterministic but thresholds are author-set. A future scoring revision will re-anchor them against representative repository data. |
| `terrain compare` snapshot diff | Implemented; output format may shift. |

### Planned (referenced in docs but not yet implemented)

Several detector entries (xfail accumulation, statistical flaky-test rate, additional regression and quality detectors) are referenced in the design but not implemented at 0.2.0. They will appear in future releases.

## Performance claims

| Claim | Reality in 0.2 |
|---|---|
| "Understand your test system in 30 seconds" | Holds on small/medium repos (≤ ~1,000 test files). Self-analyze on the terrain repo (~2,000 source files): ~99s. Benchmark suite ships; broader workload gating is future work. |
| "Useful on a single machine, without accounts, SaaS, or network access" | True. Telemetry is opt-in and local-only. |

## Distribution claims

| Claim | Reality in 0.2 |
|---|---|
| Homebrew install on macOS | Goreleaser pipeline builds darwin amd64 + arm64; brew tap publish runs post-release. |
| `npm install -g mapterrain` | Works on darwin/linux/windows × amd64/arm64. |
| Signed binaries | Cosign signatures attached to archives, SBOMs, and checksums. SLSA L2 build provenance attestation per archive. |
| Cosign verification on npm install | **Mandatory** by default. Missing cosign aborts the install with a clear remediation block. Escapes: `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` (degrade to checksum-only) and `TERRAIN_INSTALLER_SKIP_VERIFY=1` (skip entirely). Installer redirect chain capped at 5 hops. |

## AI capability substrate

- AI surface inference (prompts, contexts, datasets, tool definitions, RAG components) — stable
- `terrain ai list` / `terrain ai doctor` / `terrain ai run` — stable
- AI signal detectors (mix of **stable** and **experimental**) — gated by the bundled recall-regression fixture set
- Additional AI capabilities (multi-model comparison, full LSP/MCP IDE integration) are future work

## CLI restructure

The 0.2 release introduces three new namespace dispatchers (`report`, `migrate`, `config`) alongside the historical 32 top-level commands. The canonical 11-command shape is the recommended surface; legacy commands keep working through 0.2 with deprecation hints. Removal is future work.

---

## How to use this document

- Adding a feature to the README? Add it to this table first; if it's not yet in code, mark it `planned`.
- Adding a detector? Add it to "stable" or "experimental" and link the file.
- Removing a feature? Move it to `deprecated` for one release before deletion.
- Drift between this document and code is a release blocker — `make docs-verify` checks the manifest.

Owner: `@PMCLSF`. Review on every release tag.
