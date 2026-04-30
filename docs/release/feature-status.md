# Feature Status — 0.1.2

Single source of truth for what ships in 0.1.2. Every claim in `README.md`,
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
| `terrain analyze` | stable | Full snapshot pipeline. |
| `terrain insights` | stable | Health grade computed via `internal/insights/insights.go`; rubric documented in `docs/health-grade-rubric.md` (lands in 0.1.2 Stage D). |
| `terrain impact` | stable | One-hop traversal in 0.1.2; transitive walk is **planned** for 0.3 (round 2/3 finding). Confidence bands are point estimates today; intervals are 0.2. |
| `terrain explain` | stable | Reason chains supported; per-detector evidence quality varies. |
| `terrain serve` | experimental | HTTP API on 127.0.0.1 only; the dashboard "embedded charts" claim from earlier docs is **planned** for 0.2/0.3, not shipped. Stage F in 0.1.2 hardens flags + headers. |
| `terrain ai run` | experimental | Currently shells out to detected eval framework; full metric ingestion + regression detection is 0.2. |
| `terrain ai compare` | planned | Prompt regression detection is 0.3. |
| `terrain ai gate` | planned | Standalone CI gate command is 0.2/0.3. Today, gating goes through `terrain policy check`. |
| `terrain convert` | mixed | 25 conversion directions catalogued; Cypress→Playwright is the most complete; the rest range from B-grade to experimental. Per-direction status appears via `terrain convert list` in 0.1.2 (Stage A2). |

## Detectors / signal types

The full inventory will live in `internal/signals/manifest.go` after 0.1.2 Stage A3
ships. Until then, the table below is authoritative for what 0.1.2 actually emits
and what is aspirational in marketing.

### Stable in 0.1.2

| Signal | Detector | Notes |
|---|---|---|
| `weakAssertion` | `internal/quality/weak_assertion.go` | Regex/density-based; AST upgrade in 0.3. |
| `mockHeavyTest` | `internal/quality/mock_heavy.go` | Count-based; boundary/internal classification in 0.3. |
| `untestedExport` | `internal/quality/untested_export.go` | Heuristic + import-graph fallback. |
| `coverageThresholdBreak` | `internal/quality/coverage_threshold.go` | Severity flips at hard 100%-gap boundary; smoothing in 0.3. |
| `slowTest` | `internal/health/slow_detector.go` | Requires runtime artifact. |
| `skippedTest` | `internal/health/static_skip*.go` | |
| `flakyTest` | `internal/health/flaky_detector.go` | **Detects retry evidence and same-run mixed outcomes — not a statistical failure-rate threshold.** Marketing claims like ">10% failure rate" are aspirational; see `aiHallucinationRate`-style statistical signals in the planned list. |
| `duplicateCluster` | `internal/depgraph/duplicate.go` | Threshold is `0.60` weighted similarity (Jaccard + LCS over fingerprints). README example output cites `0.91+`; that figure is **planned** for the AST-based detector upgrade in 0.3. |
| `frameworkMigration` | `internal/migration/*` | |
| `policyViolation` | `internal/governance/evaluate.go` | |

### Experimental in 0.1.2

| Signal | Notes |
|---|---|
| AI surface inference (`SurfacePrompt`, `SurfaceContext`, `SurfaceDataset`, `SurfaceToolDef`, `SurfaceRetrieval`, etc.) | Detection works but precision/recall are uncalibrated and many adversarial cases miss (round 3 audit ~55% recall, ~75% precision). Calibration corpus arrives in 0.2. |
| `uncoveredAISurface` | Emits, but coverage attribution depends on `.terrain/terrain.yaml` scenario declarations. |
| `phantomEvalScenario` | Active; semantics will tighten as taxonomy v2 lands. |
| Risk band + Health Grade thresholds | Code is deterministic but thresholds (4 / 9 / 16; A/B/C/D cutoffs) are uncalibrated against a labeled corpus. Calibrated v3 scoring is 0.3. |

### Planned (referenced in docs but not yet implemented)

| Signal | Status | Earliest |
|---|---|---|
| `xfailAccumulation` (age-based) | No detector exists. README example output of "34 xfail markers older than 180 days" is aspirational. | 0.2/0.3 |
| Statistical flaky-test rate (e.g. ">10% failure rate over rolling window") | Today's detector is retry-based only; statistical rate needs runtime history not yet ingested. | 0.3 |
| `aiSafetyEvalMissing` | 0.2 |
| `aiPromptVersioning` | 0.2 |
| `aiPromptInjectionRisk` | 0.2 |
| `aiHardcodedAPIKey` | 0.2 |
| `aiToolWithoutSandbox` | 0.2 |
| `aiNonDeterministicEval` | 0.2 |
| `aiModelDeprecationRisk` | 0.2 |
| `aiCostRegression` | 0.2 |
| `aiHallucinationRate` | 0.2 |
| `aiFewShotContamination` | 0.2 |
| `aiEmbeddingModelChange` | 0.2 |
| `aiRetrievalRegression` | 0.2 |

## Performance claims

| Claim | Reality in 0.1.2 |
|---|---|
| "Understand your test system in 30 seconds" | Holds on small/medium repos (≤ ~1,000 test files) on a recent laptop. Larger monorepos (10k+ files) take longer; benchmark suite + budgets land in 0.2. |
| "Useful on a single machine, without accounts, SaaS, or network access" | True. Telemetry is opt-in and local-only. |

## Distribution claims

| Claim | Reality in 0.1.2 |
|---|---|
| Homebrew install on macOS | The 0.1.2 release pipeline builds darwin amd64 + arm64 (Stage B). Homebrew tap publish runs post-release. |
| `npm install -g mapterrain` | Works on darwin/linux/windows × amd64/arm64 from 0.1.2 onward. |
| Signed binaries | Cosign signatures attached to archives, SBOMs, and checksums (Stage B). The npm postinstall verifier is **warn-only** in 0.1.2; **hard-fail** in 0.2. |

## "AI-native era" claim

The README headline "Built for the AI-native era" is **aspirational** in 0.1.2.
Current AI capability:

- AI surface inference (prompts, contexts, datasets, tool definitions, RAG components) — experimental
- `terrain ai list` / `terrain ai doctor` / `terrain ai run` (shell-out execution) — experimental
- 12 dedicated AI signal detectors — **planned for 0.2** (none ship in 0.1.2)
- Multi-model A/B comparison, prompt regression detection, cost/latency regression — **planned for 0.3**
- LSP/MCP server for IDE/agent integration — **planned for 0.4**

The 0.2 release is what makes the "AI-native era" framing credible. Until then,
treat AI workflows as the most actively-evolving area of the product.

---

## How to use this document

- Adding a feature to the README? Add it to this table first; if it's not yet
  in code, mark it `planned`.
- Adding a detector? Add it to "stable" or "experimental" and link the file.
- Removing a feature? Move it to `deprecated` for one release before deletion.
- Drift between this document and code is a release blocker — `make docs-verify`
  starts checking for it in 0.2 once the manifest pipeline is wired.

Owner: `@PMCLSF`. Review on every release tag.
