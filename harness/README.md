# Validation harness

This tree holds the data + scripts used to verify Terrain releases.

## Directory structure

```
harness/
├── README.md
├── corpora/             — hand-labeled PR corpora per dogfood repo
├── runner/              — entry points that invoke Terrain on each labeled PR
├── validators/          — automated checks (FP rate, recall, schema, runtime, etc.)
├── readiness/           — per-rule readiness cards per release
├── canary/              — sealed canary set for PR-scoped UFPP measurement
├── recall-harnesses/    — per-mechanism recall accounting
├── regression-suites/   — frozen TP suites per shared module
└── reports/             — raw runner outputs (gitignored except the most recent)
```

## Running locally

```bash
go test ./internal/recallharness/... ./internal/regressionsuite/...
```
