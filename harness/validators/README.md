# Validators

One validator per quality bar. Each is a Go program/library that takes runner output and produces a per-rule pass/fail with evidence.

| File | What it checks |
|---|---|
| `schema_check.go` | Every emitted Finding validates against `schemas/finding.v1.json`. Doc page exists at `docs/rules/<rule-id>.md`. Worked example matches a real Finding from runner output. |
| `silence_check.go` | Across the intended-green corpus, zero PR comments / annotations / notifications. |
| `reproduction_parity.go` | CI-mode and CLI-mode Findings are byte-equivalent (modulo timestamps). |
| `fp_rate.go` | Per-rule false-positive rate ≤ 5% on intended-green corpus. Min-sample-size floor N≥10. |
| `recall.go` | Per-rule recall ≥ 90% on seeded-failure corpus. |
| `junit_renderer_conformance.go` | JUnit XML renders cleanly in dorny/test-reporter, mikepenz/action-junit-report, GitLab native. |
| `runtime_budget.go` | Per-phase runtime within budget (graph ≤20s, rules ≤30s, render ≤5s, total ≤60s on 50k-file repo). |
| `fail_mode_probe.go` | Synthetic panic / OOM / I/O error → fail-closed by default. |
| `bidirectional_attribution.go` | FE→AI and prompt→FE scripted PRs both produce expected Findings with expected cause-paths. |
