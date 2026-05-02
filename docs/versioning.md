# Versioning Policy

Terrain follows [Semantic Versioning 2.0.0](https://semver.org/) with
clarifications about what "public API" means for a CLI + library +
schema product.

## Version numbers

A Terrain release version `MAJOR.MINOR.PATCH` means:

- **MAJOR** — Breaking changes to the canonical CLI surface, the
  snapshot schema, the signal manifest, or the JSON output schema.
  Pre-1.0 (current era), MAJOR=0; we instead bump MINOR for any
  breaking change.
- **MINOR** — New canonical CLI commands, new signal types, new
  fields in JSON output, new detectors. May include
  behavior-affecting bugfixes.
- **PATCH** — Bug fixes that don't add capabilities. May tighten
  detector precision (fewer false positives), but should not
  introduce new false negatives.

## What counts as breaking

Treat each of the following as a breaking change requiring a MINOR
bump (until 1.0) or MAJOR bump (after 1.0):

1. **CLI canonical command renamed or removed.** Legacy aliases are
   preserved across at least one MINOR release with a deprecation
   hint before removal.
2. **Existing CLI flag renamed, removed, or changed semantics** on
   a canonical command.
3. **Exit-code value changed** for an existing semantic. New exit
   codes for new semantics are non-breaking.
4. **Existing JSON field renamed, removed, or changed type.**
   Adding a new field is non-breaking.
5. **Snapshot schema field renamed, removed, or changed type.** The
   schema is versioned independently; see
   [`docs/schema/COMPAT.md`](schema/COMPAT.md). Cross-major-snapshot
   reads are explicit (the engine rejects unknown major versions).
6. **Signal-type ID renamed or removed.** Stable signal types
   round-trip indefinitely — adding new ones is fine; renaming an
   existing one breaks consumers parsing JSON output by type.
7. **Severity clause ID renamed.** Clauses are cited by detectors
   and by external policy rules; renaming breaks both.

## What counts as behavior change (NOT breaking)

These move detector outputs around but don't break the contract:

- A detector's confidence range tightening (it still emits in the
  same range, but is more selective)
- A detector's severity escalating from Medium to High when the
  rubric clause justifies it (the JSON shape is unchanged)
- A new detector firing on previously-clean code (consumers should
  filter by signal type, not by aggregate count)
- Performance improvements that don't change output

These are documented in CHANGELOG entries but don't require a MINOR
bump on their own.

## What counts as bug fix

- A detector previously firing on benign code (false positive)
  stops firing on it, given the underlying code didn't change
- A detector previously missing real cases (false negative) starts
  catching them
- An exit code that used to be wrong (e.g., emitting 1 for "entity
  not found" when the design intended 5) is corrected, with a
  CHANGELOG entry naming the affected commands

## Pre-release identifiers

Pre-release tags use the format `MAJOR.MINOR.PATCH-PHASE.N`:

- `-alpha.N` — internal milestones, no contract guarantees
- `-beta.N` — feature-complete; API surface frozen for the release;
  bug fixes only
- `-rc.N` — release candidate; only ship-blocker fixes from here

## Release cadence

- **MAJOR / MINOR**: when the work is ready, not on a fixed cadence
- **PATCH**: as bug fixes accumulate; usually within 2-4 weeks of
  the parent MINOR

## Compatibility windows

| Surface | Window |
|---------|--------|
| Canonical CLI commands | At least one MINOR with deprecation hint before removal |
| Legacy CLI aliases | Removed in the next MAJOR after deprecation |
| Snapshot schema (same MAJOR) | Forward-compatible: 0.1.x reads 0.1.y |
| Snapshot schema (cross MAJOR) | Explicit migration step; old MAJOR rejected |
| Signal manifest | Stable types persist; experimental types may shift |
| Severity rubric | Clauses are immutable IDs; descriptions may evolve |

## Living docs

The current release's stability tier per surface is in
[`docs/release/feature-status.md`](release/feature-status.md). The
honest carryovers from the most recent release are in
`docs/release/<version>-known-gaps.md`.
