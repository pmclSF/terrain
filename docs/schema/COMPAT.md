# Snapshot Schema Compatibility Policy

`TestSuiteSnapshot` (`internal/models/snapshot.go`) is the canonical
serialisation boundary between Terrain and every downstream consumer:
the CLI's `--json` output, on-disk snapshots in `.terrain/`, the VS Code
extension, the future hosted experience, and any third-party tool that
parses the artefact.

This document is the contract. Drift between behaviour and this policy is a
release blocker.

## Versioning

Snapshots carry a `snapshotMeta.schemaVersion` field formatted as
`MAJOR.MINOR.PATCH`. Current value: **`1.1.0`** (bumped in 0.2.0).

| Bump | Meaning | Allowed without major version change |
|---|---|---|
| Patch (`1.1.0` → `1.1.1`) | Documentation, validator messages, JSON Schema clarifications | Always |
| Minor (`1.0.0` → `1.1.0`) | New optional fields. Consumers ignore unknown fields and continue working | Yes |
| Major (`1.x.x` → `2.0.0`) | Removing fields, changing field types, changing field semantics, renaming fields | **No** — requires explicit migration |

### Version history

| Version | Release | What changed |
|---|---|---|
| `1.0.0` | 0.1.2 | Initial locked schema. |
| `1.1.0` | 0.2.0 | Added 9 SignalV2 fields on `models.Signal` (all `omitempty`): `severityClauses`, `actionability`, `lifecycleStages`, `aiRelevance`, `ruleId`, `ruleUri`, `detectorVersion`, `relatedSignals`, `confidenceDetail`. Plus `EvalRunEnvelope`, `EvalRunAggregates` types and the `evalRuns []EvalRunEnvelope` field on the snapshot. Plus `scenarios.description` field on terrain.yaml `ScenarioEntry`. Strictly additive. |

### Independent version namespaces

Three version strings ship in 0.2 and are **independent** — a consumer
that pins one against another will misread:

| String | Where | Current value |
|---|---|---|
| Snapshot schema | `snapshotMeta.schemaVersion` in `--json` output | `1.1.0` |
| Manifest export schema | `schemaVersion` at the top of `docs/signals/manifest.json` | `1.0.0` |
| SARIF format | `version` in `--format=sarif` output | `2.1.0` |

The manifest export schema is independent because it describes the
shape of the manifest *file*, not the snapshot. SARIF tracks the
external standard. Always check the field name, not just the value.

## Reader behaviour

A Terrain binary is willing to read snapshots whose major version is **less
than or equal to** the value of `models.MaxSupportedMajorSchema` (currently
`1`). Snapshots stamped with a higher major are rejected by
`models.ValidateSchemaVersion` with an actionable message:

> snapshot schemaVersion "2.0.0" has major 2 but this Terrain binary
> supports up to major 1; upgrade Terrain or downgrade the snapshot.

Older majors are accepted. Snapshots from before 0.1.2 (which may carry
`schemaVersion = "0.0.0"`) load with sensible defaults; missing fields use
their Go zero values, and validators that previously assumed presence of
fields tolerate absence so long as no invariant is violated.

## What is allowed at minor-version steps

- Adding new optional fields (`omitempty` tags or default-zero acceptable).
- Adding new entries to enum-like string fields (categories, severities,
  signal types) — readers must already tolerate unknown values, which is
  enforced by validator falls-through.
- Adding new top-level sections to `TestSuiteSnapshot`.
- Adding new fields to nested structs, including `Signal`, `RiskSurface`,
  `Measurement`, `CodeSurface`, etc.
- Adding new evidence-source values, signal categories that don't yet
  exist (provided the manifest is updated in lockstep).

## What requires a major bump

- Removing any field that exists in the current schema, even if it's
  `omitempty`.
- Renaming a JSON key (Go struct field rename without a `json:` tag is
  considered breaking).
- Changing a field's type (e.g. `string` → `int`, `[]string` → `string`).
- Changing the meaning of an existing field (e.g., redefining `Confidence`
  from `[0,1]` to `[0,100]`).
- Changing the cardinality of a relationship that consumers depend on
  (e.g., a single field becoming a list).
- Changing severity, category, or other enum values that already shipped.

## What this means for contributors

When you propose a snapshot-shape change, ask:

1. Will any consumer (extension, dashboard, downstream tool) break if it
   parses my new output with code written against the previous shape?
2. Am I adding only? Or am I modifying / removing?

If the answer is "modifying / removing", you need a major version bump,
which is itself a breaking change requiring:

- A migration plan documented under `docs/release/<next>.md`.
- Reader code that supports both the old and new majors for at least one
  release after the bump ships.
- An explicit announcement in `CHANGELOG.md`.
- Coordination with the VS Code extension and any downstream consumers we
  know about.

## What about the JSON Schema files in this directory?

`docs/schema/*.schema.json` are emitted JSON Schema drafts that describe
the shape of specific outputs. In 0.2 these will be auto-generated from
the Go struct tags via `go generate`, with a CI gate that fails on drift.
Today they are hand-maintained; updating them when you change the
corresponding struct is part of the change.

## Snapshot manifest companion

The set of valid signal types is governed by
`internal/signals/manifest.go`, which is the single source of truth for
signal vocabulary. The drift gate `TestManifest_MatchesSignalTypes`
catches any constant added without a manifest entry. New signal types
land under minor bumps when they are additive (consumers ignoring unknown
signal types is part of the contract).

## Forward references

- Locked in: 0.1.2.
- Auto-generated JSON Schemas + zero-diff CI gate: 0.2.
- SignalV2 schema with multi-axis taxonomy: 0.2 (additive, minor bump).
- Migration tooling for moving across majors: 0.3+ (when the first major
  bump is on the table).
