# Security & Data Handling

> *For security reviewers, compliance teams, and adopters whose organizations require explicit data-flow documentation before adoption.*

This document covers **what data Terrain processes, where that data goes, and what changes when optional features are enabled.** For coordinated-disclosure security policy (how to report a vulnerability), see `SECURITY.md`.

## TL;DR

- **Default configuration:** Terrain operates fully offline. Zero outbound network calls. Verifiable by running `terrain --print-network`.
- **All data Terrain processes stays on the adopter's machine / CI runner unless an LLM tier is explicitly enabled.**
- **No telemetry.** Terrain does not phone home, collect anonymous usage statistics, or report crashes.
- **Optional LLM tiers (Ollama / BYOK / internal endpoint) are opt-in and per-adopter; each documents what data leaves the boundary.**

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

Verifiable: run `terrain --print-network` — for templates-only operation, the output is empty. The command audits Terrain's full configuration and lists every external endpoint that would be contacted given the current `terrain.yaml` settings.

Build-time dependencies (Go module fetching, etc.) are not at runtime; they're at install/build time and follow standard Go toolchain behavior.

## Binary install integrity

Pre-built binaries are downloaded by the `npm install -g mapterrain` and `brew install pmclSF/terrain/mapterrain` paths from GitHub Releases. Each release is signed with [Sigstore + cosign](https://www.sigstore.dev/) keyless signatures.

- **npm path:** the postinstall script verifies each binary against its Sigstore signature before placing it on `PATH`. Install cosign first (`brew install cosign` on macOS/Linux, `scoop install cosign` on Windows). Node 22+ is required for the postinstall — the script uses APIs (`fetch`, top-level `await`, modern stream primitives) that landed in Node 22; CI images on Node 20 LTS should use the Homebrew or `go install` path.
- **Brew path:** Homebrew handles its own bottle signing; Sigstore verification of the underlying binary still applies if you also pre-install cosign.
- **Source path (`go install`):** the Go toolchain validates module checksums via `go.sum`; no Sigstore step.

If cosign isn't installed and you need to proceed:

| Environment variable | Effect | When to use |
|---|---|---|
| `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` | Falls back to SHA-256 checksum verification (still validates the binary hasn't been tampered with in transit) | CI images that can't install cosign |
| `TERRAIN_INSTALLER_SKIP_VERIFY=1` | Skips all verification entirely | Air-gapped environments or local development only — not recommended for production CI |

For air-gapped installations or organizations that mirror their own binaries, download from the [releases page](https://github.com/pmclSF/terrain/releases), verify the cosign signature manually against `terrain_<platform>.tar.gz.sig`, and place on `PATH`.

## What changes when each optional feature is enabled

Terrain has three LLM tiers, all opt-in and configured in `terrain.yaml`. None is enabled by default.

### Templates tier (always-on; this is the default)

- All CI diagnostics use deterministic templated text.
- **Network calls:** none.
- **Data leaving the adopter machine / CI runner:** none.

### One-time descriptions tier (`terrain describe`)

Generates surface descriptions for `terrain.yaml`. Runs on developer machines only (never in CI).

- **When enabled:** invoking `terrain describe` calls the configured LLM provider with prompts containing surface metadata (file paths, code excerpts, eval names).
- **Network calls:** to the configured provider's endpoint. See "LLM provider matrix" below.
- **Data leaving the adopter machine:** source code excerpts and surface names sent to the chosen provider for description generation.
- **Mitigation for security-sensitive orgs:** use the default Ollama provider — entirely local, no data leaves the machine.

### CLI / agent enrichment tier (`terrain explain`, MCP server)

- **When enabled:** invoking `terrain explain` or using the MCP server with Claude Code / Cursor calls the configured LLM provider with the finding's diagnostic content.
- **Network calls:** to the configured provider's endpoint.
- **Data leaving the adopter machine / CI runner:** diagnostic content (cause-path snippets, code excerpts at cause locations, eval names) sent to the provider for narrative composition.
- **Important: this tier is never invoked in CI.** Even if `explain:` is configured in `terrain.yaml`, only the CLI and agent surfaces consume it. The CI surface (`terrain test` in CI mode) is template-only regardless of `explain:` configuration.

## LLM provider matrix

| Provider | Where data goes | Adopter cost | Notes |
|---|---|---|---|
| **Ollama (default)** | Stays on the adopter's machine (local inference) | Adopter's local compute | Recommended for security-sensitive orgs; no data leaves the boundary |
| **Internal endpoint** (vLLM, internal AI gateway) | Adopter's own infrastructure | Adopter's compute | Documented OpenAI-compatible HTTP contract; adopter validates trust |
| **OpenAI** (BYOK) | OpenAI's API endpoint | Adopter's OpenAI account | Diff / prompt / code-excerpt content sent to OpenAI |
| **Anthropic** (via OpenAI-compatible proxy) | Anthropic's API endpoint | Adopter's Anthropic account | Same shape as OpenAI |
| **Other OpenAI-compatible providers** | Provider-specific | Adopter's provider account | Each adopter validates per their provider's data handling policy |

When BYOK external API is selected, Terrain documents the request shape in this section. The data leaving the adopter machine includes:
- The prompt template for description generation or explanation
- Surface / file / eval names from `terrain.yaml`
- Code excerpts at cause-path locations (~50-line windows; configurable)

## What data leaves the adopter machine via the CI provider

Beyond LLM tiers, Terrain emits artifacts that are consumed by the CI platform itself (GitHub Actions / GitLab CI / etc.). The CI provider is part of the trust boundary by virtue of being the runner.

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

**Mitigation for highly-sensitive repos:** the `redact_source: true` config option in `terrain.yaml` redacts source-code excerpts from the artifacts. Diagnostics still emit (rule fired, location, suggested action) but without code-content quotes. Adopters with stringent code-confidentiality requirements use this option.

The CI provider's own data handling (retention, indexing, access control) is the adopter's responsibility to evaluate. Terrain emits the artifacts; the platform stores them.

## What about adopter data?

Adopter usage data is not aggregated, shared, or published. Terrain has no mechanism to collect it (see "no telemetry" above).

## Security review checklist for adopters

If your security team requires a checklist:

- [ ] Templates tier (default) operates fully offline — verified by running `terrain --print-network` and observing zero entries
- [ ] No telemetry — verified by inspecting `go.mod` for known telemetry libraries (none expected) and by `terrain --print-network` showing no opt-in telemetry endpoint
- [ ] Source code stays in the adopter's boundary by default — no LLM tier enabled, `redact_source: true` available for sensitive repos
- [ ] Optional LLM tiers are opt-in and per-`terrain.yaml` — verified by inspecting the `terrain.yaml` `explain:` block
- [ ] Ollama default option keeps data on the machine — verified by inspecting the network calls when `terrain describe` runs against an Ollama endpoint
- [ ] CI-provider artifact emissions are documented — see "What data leaves via the CI provider" above
- [ ] License: Apache 2.0 (Terrain itself), CC-BY 4.0 (corpus) — verified in `LICENSE` and corpus repository
- [ ] No mandatory cloud services — Terrain works fully offline against a local repo

## How to report a vulnerability

See `SECURITY.md` for the coordinated-disclosure policy.

## Updates to this document

This document is updated alongside any change to Terrain's data-handling surface. Each release's `CHANGELOG.md` notes whether `SECURITY-DATA-HANDLING.md` changed; if so, what changed.
