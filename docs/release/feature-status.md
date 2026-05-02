# Feature Status — 0.2.0

Single source of truth for what ships in 0.2.0. Every claim in `README.md`,
`docs/product/vision.md`, marketing material, and example outputs should
map back to a status here. Drift is treated as a release blocker.

Each shipping capability has three tags:

- **Pillar** — Understand, Align, or Gate (per `docs/product/vision.md`)
- **Tier** — 1 (publicly claimable, parity floor ≥ 4), 2 (experimental, floor ≥ 3), 3 (in development, no public claim)
- **Status** — stable / experimental / planned / deprecated

The pillar + tier columns are new in 0.2.0. The pillar tells you which
of the three workflow questions this capability answers; the tier tells
you whether marketing claims back it.

Statuses:

- **stable** — implemented, tested, calibrated; no known breaking changes planned.
- **experimental** — implemented but rough: behavior may change, calibration is in progress, or surface area is intentionally narrow. Documented but flagged.
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
| `terrain report impact` | Gate | 1 | stable | Change-scoped selection; `--explain-selection` lands in 0.2.0 (Track 3.2). |
| `terrain report pr` | Gate | 1 | stable | PR-scoped report + comment template. `--fail-on / --new-findings-only` parity with `analyze` (Track 3.1). |
| `terrain report select-tests` | Align | 2 | experimental | Recommended protective test set. |
| `terrain report show <kind> <id>` | Understand | 1 | stable | Drill into test, unit, owner, or finding. |
| `terrain compare` | Understand | 1 | stable | Snapshots over time. |
| `terrain migrate <verb>` | Align | 1 | stable | Per-direction tier badges added in 0.2 (Track 6.6). `terrain convert` retains per-file fall-through. |
| `terrain policy check` | Gate | 1 | stable | Local policy enforcement. `terrain init` emits a starter policy template (Track 7.6). |
| `terrain config <verb>` | cross-cutting | 1 | stable | feedback, telemetry. |
| `terrain doctor` | cross-cutting | 2 | stable | Migration-readiness diagnostic; per-pillar maturity view (Track 2.5). |
| `terrain debug <verb>` | Understand | 2 | stable | graph / coverage / fanout / duplicates / depgraph. |
| `terrain serve` | Understand | 2 | experimental | Local HTTP report; localhost-only, no auth. PR #132 wires request context + singleflight (Track 4.1). |
| `terrain ai list` | Gate (inventory) | 1 | stable | AI surface inventory. |
| `terrain ai doctor` | cross-cutting | 2 | stable | Diagnostic check on AI scenario configuration. |
| `terrain ai run` | Gate | 2 | stable | Captures eval framework output. AI-gate exit code 4 on `actionBlock`. Trust-boundary doc (Track 7.3) clarifies parses-vs-executes. |
| `terrain ai run --baseline` | Gate | 2 | stable | Regression-aware AI gate. |
| `terrain ai record / baseline / replay` | Gate | 2 | stable | Baseline lifecycle. |
| `terrain ai compare` | Gate | 3 | planned | Prompt-pair regression detection. 0.3. |
| `terrain ai gate` | Gate | 3 | planned | Standalone CI gate command. Today, gating goes through `terrain ai run` + `--baseline`. 0.3. |
| `terrain portfolio <verb>` | Align | 2 | experimental | Multi-repo workspace. Multi-repo manifest format + per-repo aggregation lands in 0.2 (Track 6.1–6.3); full closure in 0.2.x. |
| `terrain explain finding <id>` | Gate | 1 | stable (0.2) | Resolve a stable finding ID back to its evidence. Track 4.6. |
| `terrain suppress <id>` | Gate | 1 | stable (0.2) | Write a suppression entry. Track 4.7. |

## Detectors / signal types

The full inventory is at `docs/signals/manifest.json` (regenerated from
`internal/signals/manifest.go`). The table below is a curated view of
the 0.2 changes; the manifest is authoritative.

### Stable in 0.2

| Signal | Detector | Notes |
|---|---|---|
| `weakAssertion` | `internal/quality/weak_assertion.go` | Regex/density-based; AST upgrade in 0.3. |
| `mockHeavyTest` | `internal/quality/mock_heavy.go` | |
| `untestedExport` | `internal/quality/untested_export.go` | Layered (import-graph + heuristic). |
| `coverageThresholdBreak` | `internal/quality/coverage_threshold.go` | Severity flips at hard 100%-gap boundary; smoothing in 0.3. |
| `slowTest` | `internal/health/slow_detector.go` | Requires runtime artifact. |
| `skippedTest` | `internal/health/skipped_detector.go` | Runtime variant. |
| `staticSkippedTest` | `internal/quality/static_skip.go` | Source-pattern variant. |
| `flakyTest` | `internal/health/flaky_detector.go` | Detects retry evidence and same-run mixed outcomes. Statistical >10% failure-rate detection is **planned** for 0.3. |
| `unstableSuite` | `internal/health/unstable_detector.go` | |
| `deadTest` | `internal/health/dead_detector.go` | |
| `duplicateCluster` | `internal/depgraph/duplicate.go` | 0.91+ similarity claim is **planned** for the AST-based upgrade in 0.3. |
| `frameworkMigration` | `internal/migration/detectors.go` | |
| `policyViolation` | `internal/governance/evaluate.go` | |
| `assertionFreeImport` | `internal/structural/assertion_free_import.go` | |
| `aiHardcodedAPIKey` | `internal/aidetect/hardcoded_api_key.go` | **No calibration fixture** (would risk repository secret-scanner alerts on synthetic high-entropy keys). Tested via unit tests only. |
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

### Experimental in 0.2

| Signal | Notes |
|---|---|
| `aiPromptInjectionRisk` | Pattern-based; assignment-anchored (`==` equality excluded); 3-line concatenation window so Black/Prettier-formatted multi-line concatenation is caught. User-input shapes cover Express/Koa, FastAPI typed params, Flask, Django, Pyramid, gRPC, CLI args. AST-precise taint-flow analysis is 0.3. |
| `aiFewShotContamination` | Substring overlap with 40-character minimum chunk + 5-distinct-word guard; implicit path-based coverage matches auto-derived scenarios. Full n-gram overlap is 0.3. |
| AI surface inference (`SurfacePrompt`, `SurfaceContext`, `SurfaceDataset`, `SurfaceToolDef`, `SurfaceRetrieval`) | Detection works; precision/recall uncalibrated. |
| Risk band + Health Grade thresholds | Code is deterministic but thresholds (4 / 9 / 16; A/B/C/D cutoffs) are uncalibrated against a labeled corpus. Calibrated v2 scoring is 0.3. |
| `terrain compare` snapshot diff | Implemented; output format may shift. |

### Planned (referenced in docs but not yet implemented)

| Signal | Earliest |
|---|---|
| `xfailAccumulation` (age-based) | 0.3 |
| Statistical flaky-test rate (rolling-window failure-rate) | 0.3 |
| `accuracyRegression`, `evalFailure`, `evalRegression`, `schemaParseFailure`, `safetyFailure`, `aiPolicyViolation`, `toolGuardrailViolation` | 0.3 — manifest entries marked planned; promotion plans were "lands in 0.2" pre-update; slipped explicitly. |
| `chunkingRegression`, `rerankerRegression`, `topKRegression`, `contextOverflowRisk` | 0.3 |
| `latencyRegression` | 0.3 |
| Other entries | See `docs/signals/manifest.json` for canonical status. |

## Performance claims

| Claim | Reality in 0.2 |
|---|---|
| "Understand your test system in 30 seconds" | Holds on small/medium repos (≤ ~1,000 test files). Self-analyze on the terrain repo (~2,000 source files): ~99s (down from ~213s after the skip-dir consolidation in 0.2). Benchmark suite ships; broader workload gating tracked in 0.2-known-gaps.md. |
| "Useful on a single machine, without accounts, SaaS, or network access" | True. Telemetry is opt-in and local-only. |

## Distribution claims

| Claim | Reality in 0.2 |
|---|---|
| Homebrew install on macOS | Goreleaser pipeline builds darwin amd64 + arm64; brew tap publish runs post-release. |
| `npm install -g mapterrain` | Works on darwin/linux/windows × amd64/arm64. |
| Signed binaries | Cosign signatures attached to archives, SBOMs, and checksums. SLSA L2 build provenance attestation per archive. |
| Cosign verification on npm install | **Mandatory** by default. Missing cosign aborts the install with a clear remediation block. Escapes: `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` (degrade to checksum-only) and `TERRAIN_INSTALLER_SKIP_VERIFY=1` (skip entirely). Installer redirect chain capped at 5 hops. |

## "AI-native era" claim

The README headline "Built for the AI-native era" is now substantiated:

- AI surface inference (prompts, contexts, datasets, tool definitions, RAG components) — stable
- `terrain ai list` / `terrain ai doctor` / `terrain ai run` — stable
- 12 dedicated AI signal detectors — **stable** (10) + **experimental** (2) — see calibration corpus at `tests/calibration/`
- Multi-model A/B comparison (`terrain ai compare`) — **planned for 0.3**
- LSP/MCP server for IDE/agent integration — **planned for 0.4**

## CLI restructure

The 0.2 release introduces three new namespace dispatchers (`report`,
`migrate`, `config`) alongside the historical 32 top-level commands.
The canonical 11-command shape is the recommended surface; legacy
commands keep working through 0.2 with a deprecation note in 0.2.x and
removal in 0.3.

| Phase | When | What |
|---|---|---|
| A | 0.2 | Namespace dispatchers ship; both shapes work. |
| A.x | 0.2.x | In-band deprecation warnings on legacy invocations (`Hint: 'terrain summary' becomes 'terrain report summary' in 0.3`). |
| B | 0.3 | Legacy top-level commands removed. `policy` folds into `analyze --policy=<file>`; `compare` folds into `analyze --against=<ref>`. Universal flag schema (`--root`, `--format`, `--output`, `--quiet`, etc.) lands. |

---

## How to use this document

- Adding a feature to the README? Add it to this table first; if it's not yet
  in code, mark it `planned`.
- Adding a detector? Add it to "stable" or "experimental" and link the file.
- Removing a feature? Move it to `deprecated` for one release before deletion.
- Drift between this document and code is a release blocker — `make docs-verify`
  checks the manifest in 0.2; bringing this file under that gate is on
  the 0.2.x list.

Owner: `@PMCLSF`. Review on every release tag.
