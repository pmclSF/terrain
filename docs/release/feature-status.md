# Feature Status — 0.2.0

Single source of truth for what ships in 0.2.0. Every claim in `README.md`,
`DESIGN.md`, marketing material, and example outputs should map back to a
status here. Drift is treated as a release blocker.

Statuses:

- **stable** — implemented, tested, calibrated; no known breaking changes planned.
- **experimental** — implemented but rough: behavior may change, calibration is in progress, or surface area is intentionally narrow. Documented but flagged.
- **planned** — described in marketing/design docs as a Terrain capability but not yet implemented in code. Examples that show the future shape are kept in docs only with a `[planned]` tag.
- **deprecated** — was implemented; being removed. Will not appear in this list past one release.

If a feature is **planned**, no detector emits its signals today.

---

## Workflows

| Feature | Status | Notes |
|---|---|---|
| `terrain analyze` | stable | Full snapshot pipeline; `--baseline` flag added in 0.2. Now panics in detectors are recovered to a single `detectorPanic` signal — the rest of the pipeline isolates. |
| `terrain init` | stable | Bootstrap a `.terrain/` config tree. |
| `terrain report <verb>` | stable | New 0.2 namespace: 9 read-side verbs (summary, insights, metrics, explain, show, impact, pr, posture, select-tests). Routes to the same runners as the legacy top-level commands; output is byte-identical for the cases reviewed. |
| `terrain migrate <verb>` | stable | New 0.2 namespace: 11 verbs covering the merged convert + migrate + migration commands. `terrain convert` retains its per-file fall-through (`terrain convert spec.cy.ts --to playwright` works). |
| `terrain config <verb>` | stable | New 0.2 namespace: feedback, telemetry. |
| `terrain doctor` | stable | Migration-readiness diagnostic. Test-file count is now restricted to direct test-dir parents (was over-counting fixture trees by ~4×). |
| `terrain explain` | stable | Reason chains supported; per-detector evidence quality varies. Now also reachable as `terrain report explain`. |
| `terrain debug <verb>` | stable | graph / coverage / fanout / duplicates / depgraph. |
| `terrain serve` | experimental | HTTP API on 127.0.0.1 only; the dashboard "embedded charts" claim from earlier docs is **planned**, not shipped. |
| `terrain ai list` | stable | Surface inventory + scenario detection. |
| `terrain ai doctor` | stable | Diagnostic check on AI scenario configuration. |
| `terrain ai run` | stable | Captures eval framework output to `.terrain/artifacts/`. AI-gate exit code is now `4` (was `1`) on `actionBlock` per the documented exit-code scheme. |
| `terrain ai record` | stable | Record current scenario state as a baseline. |
| `terrain ai baseline` | stable | List recorded baselines. |
| `terrain ai replay` | stable | Replay a saved artifact. |
| `terrain ai compare` | planned | Prompt-pair regression detection. | 0.3 |
| `terrain ai gate` | planned | Standalone CI gate command. Today, gating goes through `terrain ai run` + `--baseline`. | 0.3 |
| `terrain portfolio <verb>` | experimental | Multi-repo workspace. |

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
| Risk band + Health Grade thresholds | Code is deterministic but thresholds (4 / 9 / 16; A/B/C/D cutoffs) are uncalibrated against a labelled corpus. Calibrated v2 scoring is 0.3. |
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
