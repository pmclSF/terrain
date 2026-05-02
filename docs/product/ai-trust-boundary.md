# AI Trust Boundary

What Terrain executes vs. what it parses, where AI processing
happens, and what it doesn't do. This page exists because the
launch-readiness review flagged the boundary as insufficiently
documented — adopters were unsure whether `terrain ai run` invokes
LLMs, ships secrets, or sandboxes anything.

## TL;DR

- **Terrain does not execute LLMs.** Eval frameworks (Promptfoo /
  DeepEval / Ragas) execute prompts against models. Terrain reads
  the artifacts they produce.
- **Terrain does not ship secrets.** API keys live where they
  always lived — your shell, your CI secrets, your `.env`. Terrain
  doesn't read them, doesn't pass them anywhere, doesn't log them.
- **Terrain does not phone home.** Local-first. The only network
  I/O happens during installation (downloading signed binaries
  from GitHub Releases) — analysis itself is offline.
- **Terrain does not sandbox eval execution.** Sandboxing is 0.3
  work. If the eval framework has tools that touch your system,
  that surface is the framework's responsibility in 0.2.

## Per-command boundary

### `terrain analyze`

**What it does:** static analysis of repo source + ingestion of
artifacts (coverage / runtime / eval results when present).

**What it doesn't do:** invoke any LLM. Read secrets. Make network
requests for analysis purposes.

**Filesystem access:** read-only walk of the repo root passed in
(or `.` by default). No writes outside `.terrain/snapshots/`
(only when `--write-snapshot` is set explicitly).

### `terrain ai list` / `terrain ai doctor`

**What it does:** scans the repo for AI surface declarations
(prompts, tools, contexts, eval scenarios), reports on inventory
+ coverage.

**What it doesn't do:** invoke any LLM. Make network requests.
Touch the eval framework binaries.

### `terrain ai run`

**Important — this is the highest-trust-surface command in 0.2.**

**What `terrain ai run` does:**
1. Loads scenarios from your `terrain.yaml` config.
2. Optionally invokes the eval framework binary (Promptfoo /
   DeepEval / Ragas) as a child process when scenarios point at
   one.
3. Reads the framework's output JSON.
4. Computes a baseline-aware decision (`pass` / `warn` / `block`)
   and writes the result to `.terrain/artifacts/`.

**Where the LLM call actually happens:** *inside* the eval
framework, not inside Terrain. The framework decides how to call
the model, what API key to use, what timeouts apply, whether to
sandbox tool calls. Terrain is one layer above.

**What this means for adopters:**
- Your existing API key handling stays as-is. If Promptfoo
  picked up `OPENAI_API_KEY` from your shell before, it still
  does. Terrain doesn't proxy or wrap the key.
- Your existing rate limiting / retry behavior stays as-is —
  it's the framework's policy.
- If the framework has tool-execution capabilities (file write,
  shell command), the framework decides what's allowed. Terrain
  does not add a sandbox layer in 0.2.

### `terrain ai run --baseline <path>` / `--ingest-only`

`--ingest-only` skips the framework invocation and only reads
existing eval-output JSON files (via `--promptfoo-results`,
`--deepeval-results`, `--ragas-results`). In this mode Terrain is
fully passive — no child processes, no LLM calls.

## Sandboxing roadmap

| Capability | 0.2 | 0.3 |
|------------|-----|-----|
| Read-only filesystem access in `terrain analyze` | yes | yes |
| Sandbox `terrain ai run` child processes | no | yes |
| Tool-call allowlist for AI tool invocations | no (framework's job) | terrain-side allowlist |
| Network egress controls during eval execution | no (framework's job) | optional terrain-side network policy |

If you need 0.3-grade sandboxing in 0.2, run `terrain ai run` with
`--ingest-only` and execute the eval framework yourself in your
preferred sandbox (Docker, gVisor, Firecracker, etc.). Terrain
will read the framework's output without invoking it.

## Detector boundary (pure-static side)

The 12 AI risk detectors that ship in 0.2 (`aiPromptInjectionRisk`,
`aiHardcodedAPIKey`, `aiToolWithoutSandbox`, etc.) are all **pure
static analysis** — they read source code on disk, never invoke an
LLM. False positives are heuristic; AST-grade taint analysis lands
in 0.3.

## What Terrain doesn't promise about AI risk in 0.2

The audit doc spells these out explicitly. Repeating here so
nothing in this trust-boundary doc reads as a guarantee:

- Terrain does NOT judge model truthfulness (`aiHallucinationRate`
  reads the eval framework's hallucination metadata, not Terrain
  judging hallucinations directly).
- Terrain does NOT promise public-grade precision floors. Recall is
  calibration-anchored on a 27-fixture corpus; precision against
  labeled real-repo corpora is 0.3 work.
- Terrain does NOT replace dedicated AI safety services. Lakera /
  Guardrails / similar runtime guards solve a different problem
  (request-time protection); Terrain solves the structural /
  pre-deploy / inventory side.

## Related reading

- [`docs/product/vision.md`](vision.md) — full product narrative
- [`docs/product/trust-ladder.md`](trust-ladder.md) — adoption ramp
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  per-capability tier (which AI surfaces are publicly claimable)
- [`docs/release/0.2-known-gaps.md`](../release/0.2-known-gaps.md) —
  honest carryovers, including AI-specific ones
- [`docs/integrations/{promptfoo,deepeval,ragas}.md`](../integrations/) —
  per-framework integration notes
