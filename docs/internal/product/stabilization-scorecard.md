# Stabilization Scorecard

> **Date:** 2026-03-30
> **Baseline:** Post-v3 improvements (merged PR #100, CI green on `53220b2`)
> **Method:** Full re-audit across architecture, UX, inference, determinism, maintainability, security, and release readiness.
> **Test suite:** 41 Go packages, 0 failures. JS: 732 test suites, 2274 tests, 0 failures.
> **Historical snapshot:** This scorecard reflects the repository state at the commit above. Counts and blanket assertions in this document can drift after later changes and should not be treated as a live source of truth.

---

## Scoring Key

| Rating | Meaning |
|--------|---------|
| **A** | Production-ready. Tested, deterministic, documented. |
| **B** | Works correctly. Minor gaps remain. |
| **C** | Functional but degraded. Known caveats. |
| **D** | Scaffolded. Not usable without workaround. |

---

## Category Scores

### 1. Architecture (A-)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Package structure | A | 40 internal packages, clean separation, no production circular imports |
| Dependency management | A | 1 Go dep (yaml.v3), 5 pinned npm deps, 0 CVEs |
| File size distribution | B+ | 93% of files < 500 lines; `cmd_ai.go` (1099 lines) is the main outlier |
| Test-to-code ratio | A | 0.75 (42K test lines / 57K source lines) |
| Dead code | B | ~15-20 potentially unused exports; not blocking |

**Key improvement since last audit:** CLI refactored from 3400-line monolith into 9 focused files. Structured logging via `log/slog`. FileCache eliminates redundant I/O.

**Remaining gaps:**
- P2: `cmd_ai.go` should be split further (~1100 lines)
- P2: `analysis` package is 21K lines across 25 files — monitor growth

---

### 2. UX / Product Clarity (A-)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Help text | A | 25 commands documented with examples and typical flow |
| Error messages | A | All include context (flag name, value, entity type) |
| JSON output consistency | A | Identical `json.NewEncoder` pattern across all commands |
| Flag consistency | B | `--verbose` only on 3 of 10+ report commands; `--base` missing from insights/posture |
| Exit codes | B | Exit 2 only used for policy violations; input errors use exit 1 |
| Progress reporting | A | TTY-aware, disabled in `--json` mode, slog in debug mode |

**Key improvement:** Key Findings section in analyze output (replaces single TopInsight). Missing artifact hints guide users to unlock more signals.

**Remaining gaps:**
- P1: `--verbose` should be consistent across all report commands
- P2: `ai replay` omitted from inline usage string (but documented elsewhere)
- P2: Exit code 2 should distinguish input validation from runtime errors

---

### 3. AI / Inference Quality (B+)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Tier system | A | 4 tiers (structural > semantic > pattern > content) with documented confidence ranges |
| Multi-framework detection | A | 12+ frameworks (LangChain, Vercel AI SDK, DSPy, Mirascope, etc.) |
| False positive mitigation | B+ | 2+ marker corroboration for content tier; role-field-alone is weaker |
| Confidence calibration | B | Only Go exports marked "calibrated"; all others are heuristic estimates |
| Capability inference | A | 9 canonical capabilities mapped from surface taxonomy |
| RAG pipeline detection | A | 9-component structured parser with config extraction |

**Key improvement:** AST-enhanced prompt detection with multi-framework patterns. ConfidenceBasis distinguishes calibrated vs heuristic. CalibrateFromFixtures enables ground-truth measurement.

**Remaining gaps:**
- P1: FP rate claim ("< 1% for Tier 1") is stated but not measured against real codebases
- P1: Confidence scores in 0.85-0.95 range are expert estimates, not ground-truth calibrated
- P2: `role:"system"` pattern alone can match non-AI code (database schemas, game data)
- P2: 60-char template literal threshold is arbitrary and undocumented

---

### 4. Determinism (A-)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Computation determinism | A | 8 determinism tests across metrics, measurements, heatmap, scoring, impact, comparison, portfolio |
| Output determinism | A | 9 golden test files validate analyze, insights, impact, portfolio, summary, posture, compare, export |
| Signal ordering | A | Deterministic sort by tier, confidence, detector ID |
| Detection determinism | B | No determinism tests for inference/detection phases (ParsePromptAST, InferCodeSurfaces) |

**Key improvement:** Deterministic map iteration enforced in graph construction. Golden tests validate all primary reports.

**Remaining gaps:**
- P1: No determinism tests for AI detection outputs — results could vary if regex or map iteration is involved
- P2: No golden tests for detection output (only for post-analysis reports)

---

### 5. Maintainability (A-)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Test coverage | A | 41 packages pass, 100% pass rate, test-to-code ratio 0.75 |
| Code documentation | A | All public types in models/ have godoc comments |
| CI gates | A | Format, lint, test (Node 22+24), Go race detector, golden, fixture matrix, benchmarks |
| Conventional commits | A | Enforced by commitlint via husky |
| TODO/FIXME inventory | C | Repository still contains active TODO/FIXME markers; track and age them explicitly instead of claiming zero |

**Key improvement:** Registry initialization no longer panics (returns errors). Extraction logic deduplicated (4 pairs consolidated). Context cancellation throughout analysis layer.

**Remaining gaps:**
- P2: `terrain-bench` and `terrain-truthcheck` utilities have no tests (288 and 122 lines)
- P2: Reporting package has 16 internal imports — could benefit from subpackaging

---

### 6. Security / Release Readiness (A)

| Dimension | Score | Evidence |
|-----------|-------|----------|
| Dependencies | A | Minimal (1 Go, 5 npm), 0 CVEs, npm audit clean |
| Secrets | A | No hardcoded credentials; GitHub Secrets used correctly |
| CI/CD gates | A | 3 hard gates + race detector + golden + fixture matrix + benchmark smoke |
| SBOM | A | CycloneDX + SPDX per release archive via syft |
| Signing | A | Sigstore keyless cosign via GitHub OIDC |
| npm reproducibility | A | Exact pins, `.npmrc` save-exact, CI lockfile verification |
| CodeQL | A | Go + JS/TS + Python scanned on push, PR, and weekly |
| Input validation | B+ | Path validation present; symlink rejection recommended for defense-in-depth |

**Key improvement:** SBOM generation and Sigstore signing added. Production npm deps pinned to exact versions. Lockfile integrity enforced in CI.

**Remaining gaps:**
- P2: Symlink rejection in `validateCommandInputs()` (defense-in-depth, not exploitable)
- P2: npm token rotation schedule not documented

---

## Overall Score

| Category | Score | Weight | Weighted |
|----------|-------|--------|----------|
| Architecture | A- (3.7) | 20% | 0.74 |
| UX | A- (3.7) | 20% | 0.74 |
| AI/Inference | B+ (3.3) | 20% | 0.66 |
| Determinism | A- (3.7) | 10% | 0.37 |
| Maintainability | A- (3.7) | 15% | 0.56 |
| Security/Release | A (4.0) | 15% | 0.60 |
| **Composite** | | | **3.67 (A-)** |

---

## Priority Issues

### P0 (None)

No blocking issues. CI is green. All commands functional. No panics, no data loss risks.

### P1 (Address before next release)

| # | Issue | Category | Impact |
|---|-------|----------|--------|
| 1 | Confidence scores are heuristic, not calibrated — FP rate claims unsubstantiated | Inference | Overstates trustworthiness to enterprise users |
| 2 | No determinism tests for detection/inference outputs | Determinism | Detection ordering could drift unnoticed |
| 3 | `--verbose` inconsistent across report commands (only analyze, explain, ai list) | UX | Users discover missing features through trial-and-error |
| 4 | AI graph nodes (NodePrompt, NodeDataset, etc.) defined but never populated | Inference | AI/ML persona gets no graph-level reasoning |
| 5 | Health signals require `--runtime` but docs imply static availability | Product | First-run user sees no health data without understanding why |

### P2 (Next quarter)

| # | Issue | Category |
|---|-------|----------|
| 6 | `cmd_ai.go` at 1099 lines needs further split | Architecture |
| 7 | `role:"system"` pattern can match non-AI code | Inference |
| 8 | No eval-specific signals (eval_safety_failure, eval_accuracy_regression) | Inference |
| 9 | Environment matrix requires manual YAML config | UX |
| 10 | No visual regression or API contract detection | Product |
| 11 | `terrain-bench` / `terrain-truthcheck` have no tests | Maintainability |
| 12 | Exit codes don't distinguish input vs runtime errors | UX |

---

## Improvements Achieved (This Cycle)

| Change | Category | Impact |
|--------|----------|--------|
| CLI split: 3400-line main.go into 9 files | Architecture | Reviewability, testability |
| Structured logging (log/slog) | Maintainability | Debug mode, CI diagnostics |
| FileCache + context cancellation | Performance | Large-repo support, timeout safety |
| 4-tier detection model formalized | Inference | Explicit confidence ranges and basis |
| Multi-framework AI detection (12+ frameworks) | Inference | Reduces LangChain bias |
| 9-capability inference from surfaces | Inference | Behavior-centric reasoning |
| Safe registry initialization (no panics) | Maintainability | Startup robustness |
| Extraction dedup (4 pairs consolidated) | Maintainability | Single source of truth |
| SBOM + Sigstore signing | Security | Supply-chain compliance |
| npm exact pinning + lockfile enforcement | Security | Reproducible builds |
| Key Findings in analyze output | UX | Faster time-to-value |
| Artifact auto-discovery (29 paths) | UX | Zero-config coverage/runtime |
| Fixture, environment, symbol detection | Inference | Deeper test understanding |
| Confidence calibration infrastructure | Inference | Path to ground-truth measurement |

---

## Recommended Next Milestone

### v3.1: Confidence & Determinism Hardening

**Goal:** Make every confidence score defensible and every output provably deterministic.

1. **Calibrate confidence scores** against the 3 AI benchmark fixtures + terrain-world — measure precision/recall per tier and adjust ranges from measured data, not estimates
2. **Add detection determinism tests** — run `InferCodeSurfaces()` and `ParsePromptAST()` 10x on each fixture and assert byte-identical output
3. **Populate AI graph nodes** — wire `CodeSurface` kinds (prompt, dataset, model) into `Build()` stage so `depgraph` renders AI topology
4. **Standardize `--verbose`** across all report commands
5. **Document health signal prerequisites** — add clear "requires `--runtime`" callout in analyze output when health data is absent

**Why this milestone:** Enterprise buyers audit confidence claims. If Terrain says "0.92 confidence" and can't show how that number was derived, trust erodes. Hardening confidence and determinism is the highest-leverage work before GA.
