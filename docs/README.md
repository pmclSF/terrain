# Terrain Documentation

**Pre-flight checks for AI/ML systems and the tests around them.**

## Get started

- [Quickstart](quickstart.md) — first report in five minutes
- [Demo](demo.md) — try Terrain in a few minutes
- [Product Reference](PRODUCT.md) — mission, principles, scope, stability commitments
- [Overview](OVERVIEW.md) — evaluator-focused summary

## Use Terrain

- [CLI Specification](cli-spec.md) — full command and flag reference
- [VS Code Extension](vscode-extension.md) — extension architecture and views
- [Telemetry](telemetry.md) — opt-in local telemetry

## Understand findings

- [Severity](severity-rubric.md) — severity labels and how to configure them
- [Signal Catalog](signal-catalog.md) — signal type reference

## Reference

- [JSON Schema](json-schema.md) — JSON output structure
- [Glossary](glossary.md) — Terrain-specific vocabulary
- [Versioning](versioning.md) — semantic-versioning contract
- [Compatibility](compatibility.md) — platforms, frameworks, languages
- [Limitations](LIMITATIONS.md) — what 0.2.0 does not do

## Integrations

- [Promptfoo](integrations/promptfoo.md)
- [DeepEval](integrations/deepeval.md)
- [Ragas](integrations/ragas.md)
- [Gauntlet](integrations/gauntlet.md)
- [MCP server](integrations/mcp.md)

## Contribute

- [Contributing — overview](CONTRIBUTING.md) — RFC process, governance, rule lifecycle, issue triage
- [Writing a Detector](contributing/writing-a-detector.md)
- [Adding a Measurement](contributing/adding-a-measurement.md)
- [Testing and Quality](contributing/testing-and-quality.md)

## Engineering reference

- [Coverage Ingestion](engineering/coverage-ingestion.md) — LCOV / Istanbul ingestion
- [Determinism](engineering/determinism.md) — deterministic output contract
- [Measurement Explainability](engineering/measurement-explainability.md) — how `terrain explain` is computed
- [Change Risk Posture](engineering/change-risk-posture.md)

## Per-rule documentation

Per-rule docs live under `docs/rules/<category>/<rule-name>.md`. Each shipped rule has its own page covering what it catches, why it matters, the detection mechanism, configuration, false-positive patterns, and stability commitments.

## Release

- [Release Process](releasing.md) — versioning and release workflow
- [Feature Status](release/feature-status.md) — per-capability stable / experimental / preview status
- [Release Notes — 0.2.0](release/RELEASE-NOTES-0.2.0.md) — long-form release notes
- [Supply-chain provenance](release/supply-chain.md) — release artifacts and signing

## Schema reference

- [Schema compatibility](schema/COMPAT.md) — schema versioning contract
- [Eval-adapter contract](schema/eval-adapters.md)
- [Explain output](schema/explain.md)
- [Migration output](schema/migration.md)
- [Portfolio output](schema/portfolio.md)
- [PR-analysis output](schema/pr-analysis.md)

## Security

- [Dependencies](security/dependencies.md)

## Legacy

Historical documentation for retired components:

- [Legacy Notes](legacy/legacy-notes.md)
- [Converter Architecture (legacy)](legacy/converter-architecture-legacy.md)
- [Getting Started (legacy)](legacy/getting-started-legacy.md)
- [Migration Guide (legacy)](legacy/migration-guide-legacy.md)
- [CLI Reference (legacy)](legacy/cli-reference-legacy.md)
- [Configuration (legacy)](legacy/configuration-legacy.md)
- [Conversion Process (legacy)](legacy/conversion-process-legacy.md)
- [Quarantine & Suppression (legacy)](legacy/quarantine-and-suppression-legacy.md)
- [Jest ESM Strategy ADR](adr/004-jest-esm-strategy.md)
