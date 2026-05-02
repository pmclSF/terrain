<!--
Thanks for contributing to Terrain.

This template surfaces a few things reviewers always need to know.
Filling it in saves a review round-trip; everything below is optional
but specific.
-->

## Summary

<!-- One or two sentences. What does this PR do, and why? -->

## Type of change

<!-- Check all that apply. -->

- [ ] Bug fix (no behavior change beyond the fix)
- [ ] New feature (additive only — no changes to existing behavior)
- [ ] Breaking change (alters existing behavior, schema, CLI, or output shape)
- [ ] Documentation only
- [ ] Test or tooling only
- [ ] Refactor (behavior-preserving)

## Reviewer checklist

- [ ] Tests cover the new code (or there's a justification in the body
      below for why they don't).
- [ ] `go test ./...` and `npm test` pass locally.
- [ ] If snapshot or signal-catalog shape changes,
      `docs/schema/COMPAT.md` was consulted and the bump rules respected.
- [ ] If a new `Signal*` constant was added, `internal/signals/manifest.go`
      gained a matching entry (drift gate `TestManifest_MatchesSignalTypes`
      will fail otherwise).
- [ ] If the README's example outputs changed, `docs/release/feature-status.md`
      reflects the new state.
- [ ] No file > 5 MB and no binary file extensions (the husky pre-commit
      hook should have caught this; if it didn't, please flag).

## Security / privacy implications

<!--
Mention anything that:
  - reads or writes files outside .terrain/ or the repo
  - shells out to git, npm, pytest, or other external commands
  - touches the AI eval execution path
  - changes telemetry, SARIF, or snapshot output shape
  - changes the signature/integrity verification path
"None" is a fine answer.
-->

## Breaking changes

<!--
If this PR breaks anything (CLI flags, JSON output, snapshot shape,
command exit codes), list each break here with a one-line migration
note. If unsure, see docs/schema/COMPAT.md for the contract.
-->

## Test plan

<!--
- [ ] Smoke-tested locally with: ...
- [ ] Added unit / integration tests for: ...
- [ ] Regression-tested existing behavior for: ...
-->
