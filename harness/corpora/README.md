# Hand-labeled PR corpora

One subdirectory per dogfood repo. Each contains a `labels.yaml` with ≥100 hand-labeled PRs (mix of intended-green and seeded-failure).

## Format

```yaml
version: 1
labels:
  - pr: "PR#42 on terrain-testing-fullstack-rag"
    sha: <head_sha>
    base_sha: <base_sha>
    expected_findings:
      - rule: regression/eval-regression
        eval: summarize_refusal
        ground_truth_cause: "frontend/CommentInput.tsx:42 — input length cap removed"
      - rule: coverage/no-tests
        unit: src/api/handlers/refund.py:RefundHandler.process
    rationale: "single-line documentation of why these findings are expected"

  - pr: "PR#43 on terrain-testing-fullstack-rag"
    sha: <sha>
    base_sha: <sha>
    expected_findings: []  # intended-green
    rationale: "CSS-only change; no AI surfaces or impact"
```

## Labeling effort

Per `docs/HARNESS.md`: ~30 min per PR × 100 PRs × 5 repos ≈ 250 person-hours, single-labeler; ~500 person-hours with multi-labeler consensus via `terrain-corpus vote`.

## Status

| Repo | State |
|---|---|
| terrain-testing-fullstack-rag | TODO: repo built bespoke per Tier 4; PR seeding + labeling per Tier 4 |
| terrain-testing-go-monolith | TODO: license-audit + fork candidate selection (operational §19 #1) → PR seeding + labeling |
| terrain-testing-ai-only | TODO: repo built bespoke per Tier 4 → PR seeding + labeling |
| terrain-testing-polyglot-monorepo | TODO: license-audit + fork → PR seeding + labeling |
| terrain-testing-ml-pipeline | TODO: repo built bespoke per Tier 4 → PR seeding + labeling |
