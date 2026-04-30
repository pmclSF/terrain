# internal/

Go engine packages. See [docs/architecture.md](../docs/architecture.md) for
the layer model and [docs/architecture/](../docs/architecture/) for
deeper-dive design notes per area.

This README was last regenerated for **0.1.2**. The package listing is
the canonical source of truth for the public claim "Terrain consists of
N internal packages"; if the count changes here, update DESIGN.md and
the marketing copy in lockstep. The drift gate
(`go test ./internal/signals/... -run TestManifest`) catches signal
vocabulary drift; a similar gate for package layout is queued for 0.2.

## Packages

Grouped by layer. Each row links the directory; the layer column tells
you where the package sits in the dependency stack documented at
`docs/architecture.md`.

| Package | Layer | Purpose |
|---|---|---|
| [`models`](models) | Core | Canonical types: `TestSuiteSnapshot`, `Framework`, `TestFile`, `CodeUnit`, `Signal`, `RiskSurface`, schema-version contract. |
| [`signals`](signals) | Core | Signal vocabulary: type constants, `manifest.go` (single source of truth), detector interfaces, registry. |
| [`engine`](engine) | Pipeline | The 10-step `RunPipeline` orchestrator that scans → extracts → detects → ranks → produces a snapshot. |
| [`analysis`](analysis) | Static Analysis | Repository scanning, framework detection (with `.gitignore` honour and bounded file-cache), import-graph construction, AI surface inference. |
| [`testcase`](testcase) | Static Analysis | Per-language test-case extraction (Go, JS/TS, Python, Java) with tree-sitter ASTs. |
| [`testtype`](testtype) | Static Analysis | Test-type classification (unit / integration / e2e / contract). |
| [`coverage`](coverage) | Static Analysis | LCOV / Istanbul coverage ingestion. |
| [`runtime`](runtime) | Runtime Ingestion | JUnit XML / Jest JSON parsing for runtime-backed health signals. |
| [`gauntlet`](gauntlet) | Runtime Ingestion | AI eval artifact ingestion (Promptfoo / DeepEval / Ragas — full integration is 0.2). |
| [`aidetect`](aidetect) | Inference | AI framework detection (LangChain, LlamaIndex, OpenAI, etc.) and scenario derivation. |
| [`airun`](airun) | Inference | Eval execution and replay artifact bookkeeping. |
| [`depgraph`](depgraph) | Graph | Dependency-graph analysis: duplicates, fanout, redundancy. |
| [`graph`](graph) | Graph | Lower-level graph data structures consumed by depgraph and impact. |
| [`structural`](structural) | Inference | Graph-powered structural detectors (blast radius, fixture fragility, AI surfaces). |
| [`changescope`](changescope) | Impact | Change-scope construction from git diff. |
| [`impact`](impact) | Impact | `terrain impact` analysis: which tests matter for a change. |
| [`identity`](identity) | Impact | Test-case ID stability across runs. |
| [`lifecycle`](lifecycle) | Impact | Test-lifecycle hooks (skip/xfail age tracking). |
| [`stability`](stability) | Impact | Stability cluster detection. |
| [`quality`](quality) | Detectors | Quality detectors: weak assertions, mock-heavy, untested exports, snapshot-heavy. |
| [`health`](health) | Detectors | Health detectors: slow, flaky, skipped, dead, unstable. |
| [`migration`](migration) | Detectors | Migration detectors: blockers, deprecated patterns, dynamic generation, custom matchers. |
| [`governance`](governance) | Detectors | Governance detectors: policy violations, legacy usage, runtime budget. |
| [`policy`](policy) | Risk Engine | Policy model and local-policy evaluation. |
| [`scoring`](scoring) | Risk Engine | Risk-band assignment: reliability, change, speed, governance dimensions (see [scoring rubric](../docs/scoring-rubric.md)). |
| [`measurement`](measurement) | Risk Engine | 18 measurements across 5 posture dimensions. |
| [`metrics`](metrics) | Risk Engine | Aggregate metrics scorecard. |
| [`portfolio`](portfolio) | Risk Engine | Portfolio intelligence (cost, breadth, leverage, redundancy). |
| [`ownership`](ownership) | Risk Engine | Code-ownership resolution from CODEOWNERS / .terrain config. |
| [`heatmap`](heatmap) | Reporting | Risk-heatmap data for the summary view. |
| [`insights`](insights) | Reporting | `terrain insights` recommendations and Health Grade derivation (see [health-grade rubric](../docs/health-grade-rubric.md)). |
| [`explain`](explain) | Reporting | `terrain explain` reasoning chains. |
| [`analyze`](analyze) | Reporting | `terrain analyze` report assembly. |
| [`summary`](summary) | Reporting | Executive summary rendering. |
| [`comparison`](comparison) | Reporting | Snapshot diffing for `terrain compare`. |
| [`reporting`](reporting) | Reporting | Output rendering: CLI text, JSON, GitHub annotations. |
| [`sarif`](sarif) | Reporting | SARIF 2.1.0 emission with optional path redaction. |
| [`matrix`](matrix) | Reporting | Environment / device matrix surfaces. |
| [`server`](server) | CLI | `terrain serve` HTTP server with security middleware. |
| [`logging`](logging) | Shared | slog-based structured logging with quiet/info/debug levels. |
| [`telemetry`](telemetry) | Shared | Opt-in local telemetry; 0o600-locked event log. |
| [`benchmark`](benchmark) | Shared | Privacy-safe benchmark export. |
| [`skipstats`](skipstats) | Shared | Skip statistics summarisation. |
| [`convert`](convert) | Convert | Test-framework conversion (Cypress↔Playwright, Jest↔Vitest, etc. — see [feature status](../docs/release/feature-status.md) for per-direction grade). |
| [`testdata`](testdata) | Test infrastructure | Fixture builders for adversarial / property tests. The directory carries a 0.2 TODO to migrate into `*_test.go` files. |
| [`truthcheck`](truthcheck) | Tooling | Standalone `terrain-truthcheck` binary supporting consumer-side validation. |

**Count: 46 packages.**

## Status

`internal/plugin/` was deleted in 0.1.2 (round 1 review confirmed it was
dead code — no callers in `cmd/` or `internal/`). The current detector
extension model is documented at
[docs/engineering/detector-architecture.md](../docs/engineering/detector-architecture.md);
a runtime plugin loader is a possible 0.3+ direction, not a current
capability.

For tests: 199 `*_test.go` files across 48 test packages (a couple of
fixture directories add their own packages). The full release verification
suite runs via `make release-verify`.
