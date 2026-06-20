# Security & Data Handling

> *For security reviewers, compliance teams, and adopters whose organizations require explicit data-flow documentation before adoption.*

This document covers **what data Terrain processes, where that data goes, and what changes when optional features are enabled.** For coordinated-disclosure security policy (how to report a vulnerability), see `SECURITY.md`.

## TL;DR

- **Default configuration:** Terrain operates fully offline. Zero outbound network calls. Verifiable by running `terrain --print-network`.
- **All data Terrain processes stays on the adopter's machine / CI runner unless the adopter publishes CI artifacts through their CI provider.**
- **No remote telemetry.** Terrain does not phone home or report crashes. Optional local-only telemetry can be enabled with `terrain config telemetry --on`; it writes JSONL to `~/.terrain/telemetry.jsonl` and Terrain never sends it anywhere.
- **No LLM provider is contacted in 0.3.0.** The `explain:` provider block is parsed for forward compatibility, but it is not consumed by `terrain explain`, `terrain mcp`, or CI.

## What Terrain does

Terrain reads source code, configuration files, eval definitions, and (where present) registry / pipeline metadata in the adopter's repository. It produces structured findings — failing test cases with cause-path attribution — emitted as JUnit XML, GitHub check-run annotations, Step Summary markdown, SARIF (for `security/*` rules), and a structured `findings.json` artifact.

## What data Terrain processes

- **Source code** in the adopter's repository (read-only; never written by Terrain)
- **Configuration files** the adopter declares: `terrain.yaml`, `pyproject.toml`, `package.json`, `go.mod`, eval-framework configs (promptfoo / deepeval / ragas / Great Expectations), registry configs (MLflow / W&B)
- **Eval outputs** when adopters wire eval frameworks: the outputs are read as JSON/YAML artifacts, not re-run by Terrain (except as documented per-rule, e.g., `regression/eval-regression`'s base/head comparison)
- **Git history** for the PR base ↔ head diff
- **Optional: snapshots from `.terrain/baselines/`** if the adopter has enabled regression rules with baseline storage

## What network calls Terrain makes by default

**Zero outbound network calls in default configuration.**

Verifiable: run `terrain --print-network` — for 0.3.0 operation, the active endpoint audit lists `(none)`. If a future-facing provider setting is present in `terrain.yaml`, the command reports it as configured-but-inactive rather than as an endpoint Terrain contacts.

Build-time dependencies (Go module fetching, etc.) are not at runtime; they're at install/build time and follow standard Go toolchain behavior.

## Binary install integrity

Pre-built binary archives are attached to GitHub Releases and signed with [Sigstore + cosign](https://www.sigstore.dev/) keyless signatures. The install path determines which integrity layer runs automatically.

- **npm path:** the postinstall script downloads the matching GitHub Release archive and verifies it against its Sigstore signature before placing it on `PATH`. The npm binary matrix is macOS/Linux amd64+arm64 and Windows amd64. Install cosign first (`brew install cosign` on macOS/Linux, `scoop install cosign` on Windows). Node 22+ is required for the postinstall — the script uses APIs (`fetch`, top-level `await`, modern stream primitives) that landed in Node 22; CI images on Node 20 LTS should use the Homebrew or `go install` path.
- **Brew path:** the tap formula builds from the tagged GitHub source tarball and Homebrew verifies the formula checksum / bottle metadata it manages. To verify a pre-built release archive with cosign, download the archive from GitHub Releases and follow the manual verification steps below.
- **Source path (`go install`):** the Go toolchain validates module checksums via `go.sum`; no Sigstore step.

If cosign isn't installed and you need to proceed:

| Environment variable | Effect | When to use |
|---|---|---|
| `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` | Falls back to SHA-256 checksum verification (still validates the binary hasn't been tampered with in transit) | CI images that can't install cosign |
| `TERRAIN_INSTALLER_SKIP_VERIFY=1` | Skips all verification entirely | Air-gapped environments or local development only — not recommended for production CI |

For air-gapped installations or organizations that mirror their own binaries, download from the [releases page](https://github.com/pmclSF/terrain/releases), verify the cosign signature manually against `terrain_<platform>.tar.gz.sig`, and place on `PATH`.

## What changes when each optional feature is enabled

Terrain has deterministic templates by default. LLM provider configuration exists as a reserved schema surface in 0.3.0, but no shipped command contacts an LLM provider.

### Templates tier (always-on; this is the default)

- All CI diagnostics use deterministic templated text.
- **Network calls:** none.
- **Data leaving the adopter machine / CI runner:** none.

### Local surface declaration generator (`terrain describe`)

Generates starter surface declarations for `terrain.yaml`. Runs on developer machines only (never in CI).

- **When enabled:** invoking `terrain describe` scans source structure and writes a starter `terrain.yaml` only when `--write` is passed.
- **Network calls:** none.
- **Data leaving the adopter machine:** none.
- **Security note:** generated descriptions are deterministic starter labels; adopters edit them by hand if they want richer narrative descriptions.

### Reserved LLM provider config (`explain:`)

- **0.3.0 behavior:** `explain.provider` parses and validates, but no shipped command calls it. `terrain explain`, `terrain mcp`, and `terrain test` remain template-only.
- **Network calls:** none in 0.3.0.
- **Data leaving the adopter machine / CI runner:** none through this config in 0.3.0.
- **Why it exists:** the schema reserves provider names (`ollama`, `openai`, `anthropic`, `custom`, `none`) so future LLM enrichment can land without a config-schema migration.

## Reserved LLM provider matrix

| Provider | Where data goes | Adopter cost | Notes |
|---|---|---|---|
| **Ollama (default)** | Stays on the adopter's machine (local inference) | Adopter's local compute | Recommended for security-sensitive orgs; no data leaves the boundary |
| **Internal endpoint** (vLLM, internal AI gateway) | Adopter's own infrastructure | Adopter's compute | Documented OpenAI-compatible HTTP contract; adopter validates trust |
| **OpenAI** (BYOK) | OpenAI's API endpoint | Adopter's OpenAI account | Diff / prompt / code-excerpt content sent to OpenAI |
| **Anthropic** (via OpenAI-compatible proxy) | Anthropic's API endpoint | Adopter's Anthropic account | Same shape as OpenAI |
| **Other OpenAI-compatible providers** | Provider-specific | Adopter's provider account | Each adopter validates per their provider's data handling policy |

The matrix above describes the reserved provider families, not active 0.3.0 data flow. When a future LLM consumer is wired, the data leaving the adopter machine will be documented here before the feature is marked shipped.

## What data leaves the adopter machine via the CI provider

Beyond reserved LLM provider config, Terrain emits artifacts that are consumed by the CI platform itself (GitHub Actions / GitLab CI / etc.). The CI provider is part of the trust boundary by virtue of being the runner.

**Artifacts emitted to the CI runner:**
- JUnit XML with diagnostic body (includes source-code excerpts in `<failure>` content)
- GitHub check-run annotations (path / line / message; the `raw_details` field includes the diagnostic body)
- SARIF 2.1.0 (for `security/*` rules; uploaded to GitHub Security tab via `github/codeql-action/upload-sarif`)
- Step Summary markdown
- `findings.json` artifact

**Data inside these artifacts:**
- File paths (the adopter's repo paths)
- Line numbers
- Diagnostic short messages (templated; deterministic)
- Code excerpts at cause-path locations (sourced from the adopter's repo at the head SHA)
- Eval names, metric deltas, before/after IO examples (for regression rules)

**Mitigation for highly-sensitive repos:** source-content redaction is not active in 0.3.0. The `redact_source: true` config option parses successfully for forward compatibility, but no emission path consumes it yet. Adopters with stringent code-confidentiality requirements should avoid publishing diagnostic artifacts that include source excerpts until redaction wiring lands.

The CI provider's own data handling (retention, indexing, access control) is the adopter's responsibility to evaluate. Terrain emits the artifacts; the platform stores them.

## What about adopter data?

Adopter usage data is not aggregated, shared, or published. Terrain has no mechanism to collect remote usage data; optional local telemetry stays on the adopter's machine unless they explicitly share it.

## Security review checklist for adopters

If your security team requires a checklist:

- [ ] Templates tier (default) operates fully offline — verified by running `terrain --print-network` and observing zero entries
- [ ] No remote telemetry — verified by inspecting `go.mod` for known telemetry libraries (none expected) and by `terrain --print-network` showing no telemetry endpoint
- [ ] Source code stays in the adopter's boundary by default — no LLM provider contacted; source-content redaction is reserved but inactive in 0.3.0
- [ ] `explain:` provider config is parsed but inactive in 0.3.0 — verified by `terrain --print-network` showing no active endpoints and, when configured, a configured-but-inactive entry
- [ ] CI-provider artifact emissions are documented — see "What data leaves via the CI provider" above
- [ ] License: Apache 2.0 (Terrain itself), CC-BY 4.0 (corpus) — verified in `LICENSE` and corpus repository
- [ ] No mandatory cloud services — Terrain works fully offline against a local repo

## How to report a vulnerability

See `SECURITY.md` for the coordinated-disclosure policy.

## Updates to this document

This document is updated alongside any change to Terrain's data-handling surface. Each release's `CHANGELOG.md` notes whether `SECURITY-DATA-HANDLING.md` changed; if so, what changed.
