# Terrain 0.2.0 — Release notes (long form)

> *This file is the long-form release notes for 0.2.0. The user-facing changelog summary is in [`CHANGELOG.md`](../../CHANGELOG.md). This document captures the parity-gate methodology and the three-pillar breakdown.*

## Headline

0.2.0 is the first release where pre-flight checks for AI/ML systems land end-to-end as a static analyzer that runs locally and in CI, with **no LLM API key required**, ever.

## Parity-gate methodology

0.2.0 is the first release shipped under the parity gate: every functional area must clear its pillar floor (Gate ≥ 3, Understand ≥ 3, Align ≥ 3 soft) before the tag cuts. Gate floor=3 reflects recall-anchored synthetic calibration in 0.2.0; real-repo precision-floor work continues in subsequent releases.

Per-capability status with pillar + tier: [`feature-status.md`](feature-status.md).

## Three pillars

The release groups deliverables by three pillars — Understand, Align, Gate. Every capability in 0.2.0 maps to one of these and clears its tier floor.

- **Understand** (Tier 1): full snapshot pipeline; `report summary/posture/metrics/insights/explain`; AI surface inventory; cross-repo views.
- **Align** (Tier 1): framework migration with per-file confidence; alignment-first docs; multi-repo manifest format.
- **Gate** (Tier 1): `report pr / impact` with `--fail-on / --new-findings-only / --timeout`; suppressions (`.terrain/suppressions.yaml`); stable finding IDs; `terrain explain finding <id>`; one recommended GitHub Action template.

## Verdict engine

Twelve new AI detectors ship with recall-anchor calibration fixtures (a regression gate that prevents silent recall loss when detectors change shape). On top of that, the verdict engine lands — a typed-evidence pipeline with cross-file context, per-cohort calibration, production-context training gating, and a precision floor on the calibrated panel:

- **16.83% precision on the app-shape cohort** — 4–6× the path-only baseline; Wilson 95% CI lower bound 11%.
- Reach it from `terrain ai findings`.

## CLI compression

The CLI surface compresses 35 → 11 canonical commands while keeping every legacy alias working. The calibration runner becomes a load-bearing regression gate (any drop below the 100%-recall anchor blocks the build).

## What's stable in 0.2

See [`CHANGELOG.md`](../../CHANGELOG.md) "What's stable in 0.2" section for the full per-capability table.

## Strategic framing

This release lands the substrate for AI-feature pre-flight in CI. Real-repo precision and detector-roster expansion continue in subsequent releases.
