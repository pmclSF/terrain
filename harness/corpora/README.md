# Hand-labeled PR corpora

One subdirectory per validation repo. Each contains a `labels.yaml` with hand-labeled PRs (mix of intended-green and seeded-failure).

## Format

```yaml
version: 1
labels:
  - pr: "PR#42 on <repo>"
    sha: <head_sha>
    base_sha: <base_sha>
    expected_findings:
      - rule: regression/eval-regression
        eval: summarize_refusal
        ground_truth_cause: "frontend/CommentInput.tsx:42 — input length cap removed"
      - rule: coverage/no-tests
        unit: src/api/handlers/refund.py:RefundHandler.process
    rationale: "single-line documentation of why these findings are expected"

  - pr: "PR#43 on <repo>"
    sha: <sha>
    base_sha: <sha>
    expected_findings: []  # intended-green
    rationale: "CSS-only change; no AI surfaces or impact"
```
