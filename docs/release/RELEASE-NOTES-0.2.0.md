# Terrain 0.2.0 — Release notes (long form)

> *This file is the long-form release notes for 0.2.0. The user-facing changelog summary is in [`CHANGELOG.md`](../../CHANGELOG.md).*

## Headline

0.2.0 is the first release where pre-flight checks for AI/ML systems land end-to-end as a static analyzer that runs locally and in CI, with **no LLM API key required**, ever.

## Release scope

0.2.0 ships under a parity gate: every functional area must clear its pillar floor (Gate, Understand, Align) before the tag cuts.

Per-capability status with pillar + tier: [`feature-status.md`](feature-status.md).

## Three pillars

The release groups deliverables by three pillars — Understand, Align, Gate. Every capability in 0.2.0 maps to one of these and clears its tier floor.

- **Understand** (Tier 1): full snapshot pipeline; `report summary/posture/metrics/insights/explain`; AI surface inventory; cross-repo views.
- **Align** (Tier 1): framework migration with per-file confidence; alignment-first docs; multi-repo manifest format.
- **Gate** (Tier 1): `report pr / impact` with `--fail-on / --new-findings-only / --timeout`; suppressions (`.terrain/suppressions.yaml`); stable finding IDs; `terrain explain finding <id>`; one recommended GitHub Action template.

## Verdict engine

Twelve new AI detectors ship with recall-anchor regression fixtures (a regression gate that prevents silent recall loss when detectors change shape). On top of that, the verdict engine lands — a typed-evidence pipeline with cross-file context, per-cohort weighting, and production-context training gating:

- Substantial precision lift over a path-based baseline on representative app-shape repositories.
- Reach it from `terrain ai findings`.

## CLI compression

The CLI surface compresses 35 → 11 canonical commands while keeping every legacy alias working. The recall-regression runner becomes a load-bearing release gate (any drop below the bundled-fixture recall anchor blocks the build).

## What's stable in 0.2

See [`CHANGELOG.md`](../../CHANGELOG.md) "What's stable in 0.2" section for the full per-capability table.

## Strategic framing

This release lands the substrate for AI-feature pre-flight in CI. Coverage expansion continues in subsequent releases.
