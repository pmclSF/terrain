# Schema field tiers

Every field in Terrain's JSON output sits in one of three stability
tiers. The tier tells adopters whether they can build long-lived
tooling against the field, accept temporary brittleness, or treat
the field as Terrain's internal scratch space.

This is the Track 9.12 deliverable for the 0.2.0 release plan. The
parity-plan rationale: adopters integrating Terrain into CI / IDE /
dashboard tooling need to know which fields are safe to depend on
and which can churn between minor releases.

## The three tiers

### Stable

**Contract:** the field name, JSON type, and semantics will not
change without a major schema version bump. Adopters can build
long-lived tooling against stable fields with confidence.

**Examples:**

- `snapshotMeta.schemaVersion`
- `repository.rootPath`, `repository.language`
- `frameworks[].name`, `frameworks[].type`, `frameworks[].fileCount`
- `testFiles[].path`, `testFiles[].framework`,
  `testFiles[].testCount`, `testFiles[].assertionCount`
- `codeUnits[].name`, `codeUnits[].path`, `codeUnits[].kind`,
  `codeUnits[].exported`, `codeUnits[].unitID`
- `signals[].type`, `signals[].severity`, `signals[].path`
- `findingId` on every signal (Track 4.4)
- `aggregates.successRate` on EvalRunResult

Stable fields are claimed publicly. Removing one is a major-version
breaking change with deprecation lead time.

### Beta

**Contract:** the field name and type are unlikely to change in
the next minor release, but the *semantics* (what value Terrain
puts into the field, when, with what precision) may evolve as
calibration corpora arrive.

**Examples:**

- `signals[].confidence` — will be calibrated against the 0.3
  precision corpus; today's confidence values are detector self-
  reports, not measured against ground truth
- `signals[].evidence[]` — the field shape is stable; the set
  of evidence sources cited may grow per detector
- `aiSubdomain` (Track 5.1) — vocabulary is stable for 0.2; new
  AI signal types may add entries
- `testTypeConfidence`, `testTypeEvidence` — same shape; rule
  set may expand
- `metadata.compatibilityNotes` — populated by the snapshot
  migrator; messages may be reworded

Beta fields are safe to read. Building features that branch on a
specific *value* (e.g. "show this UI when confidence > 0.85")
should track changes between Terrain releases.

### Internal

**Contract:** the field exists for Terrain's own diagnostics or
debugging and may be renamed, restructured, or removed without
notice. Treat as Terrain's scratch space.

**Examples:**

- `metadata.diagnostics.*` (when emitted by `--collect-diagnostics`)
- Per-detector implementation hints embedded in
  `signals[].evidence[]` strings (the user-facing line is beta;
  detector-internal sub-strings are not)
- Anything under `internalDebug.*` (when emitted)
- `_internal*` prefixed fields anywhere

Internal fields are visible in JSON output for debuggability but
are explicitly outside the schema contract.

## How to tell a field's tier

In order of precedence:

1. **Field name pattern.** Anything prefixed `_internal`, named
   `internalDebug`, or under a `metadata.diagnostics.*` path is
   internal. Anything under `metadata.compatibilityNotes` is beta.
2. **JSON Schema annotation.** Where the schema is hand-curated
   (`docs/schema/analysis.schema.json` and
   `docs/schema/conversion.schema.json`), look for the
   `x-terrain-tier` extension keyword on the property. Values:
   `stable` / `beta` / `internal`. Absence defaults to *beta*
   for any field shipped post-0.2.
3. **This page.** When in doubt, the explicit examples above are
   authoritative. Fields not listed default to beta.

## Promotion path

Fields move from internal → beta → stable as evidence accumulates
that the contract is sustainable.

| From → To | Trigger |
|-----------|---------|
| internal → beta | Field is read by at least one external integration; Terrain commits to a name + type for the next minor |
| beta → stable | Calibration evidence + adopter usage demonstrate the semantic contract is durable; field is named in the schema's `required` block when applicable |
| stable → demoted | Never within a major version. A stable field that's wrong stays through the major and changes at the next major bump. |

## What this means for `terrain analyze --json`

A typical adopter integrating against Terrain JSON should:

1. **Pin a `snapshotMeta.schemaVersion` they tested against** —
   not because Terrain breaks compatibility freely, but because
   beta-tier semantic shifts are the most common change shape.
2. **Defensively read beta fields with a fallback** — e.g.
   "if `signals[i].confidence` is below 0.7 OR absent, treat as
   low-confidence."
3. **Never branch on internal fields** — anything not listed
   above as stable or beta. Internal field reads should be
   limited to debugging.

## Related reading

- [`docs/schema/COMPAT.md`](COMPAT.md) — schema versioning policy
- [`docs/schema/analysis.schema.json`](analysis.schema.json) —
  current analyze JSON schema
- [`docs/schema/conversion.schema.json`](conversion.schema.json) —
  current convert JSON schema
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  per-capability tier matrix (different "tier" axis — capability
  publicly-claimable status)
