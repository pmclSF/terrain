# Terrain severity rubric

> **Generated from `internal/severity/rubric.go`. Edits go in code, then `make docs-gen`.**

Every signal Terrain emits assigns a severity (Critical / High / Medium / Low / Info).
This rubric is the source of truth for what each level means.

Detectors cite one or more clause IDs in the `severityClauses` field of every
`Signal` they emit (SignalV2, schema 1.1.0+). The IDs are stable forever — once
published, a number is never reused. Retired clauses are marked, not removed.

Severity ≠ actionability. A Critical-severity finding in a deprecated module may
still be Advisory; a Medium finding blocking a release may be Immediate. The
`actionability` field on Signal handles that axis separately.

## Clause table

### Critical

#### `sev-critical-001` — Secret leak with production reach

Code, fixture, or eval config contains a credential that grants production access (API key, signing key, DB DSN with creds, OAuth client secret).

**Applies when:**

- OPENAI_API_KEY=sk-... committed to a YAML eval file
- hardcoded AWS access key in a test fixture under tests/
- `postgres://user:password@prod-host:5432/db` in a pytest conftest

**Does not apply when:**

- placeholder strings like "sk-fake-key" or "password123"
- keys clearly scoped to a sandbox / staging / mock service

#### `sev-critical-002` — Destructive AI tool without approval gate

An LLM agent or tool definition can perform an irreversible operation (delete, drop, exec) without an explicit approval gate, sandbox, or dry-run mode.

**Applies when:**

- agent definition includes a `run_shell` tool with no allowlist
- `tools/delete_user.py` registered as an MCP tool with no confirmation

#### `sev-critical-003` — CI gate disabled in main

A required pre-merge gate (lint, type-check, test suite) has been silently disabled in the configuration on the default branch.

**Applies when:**

- `continue-on-error: true` added to the only test job
- `if: false` block around the entire suite invocation

**Does not apply when:**

- a single flaky test marked .skip with a tracking ticket
- non-blocking informational job (e.g. coverage upload)

### High

#### `sev-high-001` — Weak coverage on changed surface

A symbol or path that just changed has no test coverage AND no nearby test files; releases ship blind.

**Applies when:**

- new exported function added in src/auth/ with no test under test/auth/
- file modified in this diff has zero LinkedCodeUnits matches

#### `sev-high-002` — Flaky test failing >10% in last 50 runs

Test fails intermittently at a rate that signals a real reliability issue, not transient noise.

**Applies when:**

- 5+ failures over 50 most-recent CI runs of the same test
- the test has a documented .retry() or @flaky decorator

**Does not apply when:**

- single observed failure with no historical context
- test failed once in a release-blocking pipeline that was reverted

#### `sev-high-003` — Prompt-injection-shaped concatenation

User-controlled input is concatenated into a prompt without escaping, system-prompt boundaries, or structured input boundaries.

**Applies when:**

- f"You are an assistant. The user said: {user_input}"
- `prompt += request.body.message` with no validation

#### `sev-high-004` — Missing safety eval on agent surface

An LLM agent or autonomous workflow has no eval scenario covering the documented safety category (jailbreak, harm, leak).

**Applies when:**

- agent.yaml references `tools.execute_code` with no eval covering misuse
- deployed prompt has no scenario tagged `category: safety`

### Medium

#### `sev-medium-001` — Weak assertion (semantically loose)

Test uses an assertion shape that passes for many incorrect values (`toBeTruthy`, `assert response`, `assertNotNull`) where a precise match is feasible.

**Applies when:**

- `expect(result).toBeTruthy()` checking a string value
- `assertNotNull(user)` instead of `assertEquals("alice", user.name)`

#### `sev-medium-002` — Mock-heavy test (>3 mocks)

Test relies on more than three mocks, creating a tight coupling to implementation that breaks under refactoring.

**Applies when:**

- a unit test that mocks DB, cache, queue, and HTTP client

#### `sev-medium-003` — Non-deterministic eval configuration

An LLM eval runs without temperature pinned to 0 or a deterministic seed, so re-runs produce noisy comparisons.

**Applies when:**

- promptfoo config with no `temperature: 0` or `seed:`
- eval scenario uses a model variant with stochastic decoding by default

#### `sev-medium-004` — Duplicate test cluster

Two or more tests share ≥0.60 similarity on test name and assertions, indicating likely copy-paste reduction opportunity.

**Applies when:**

- three tests named `test_login_*` differing only in inputs

**Does not apply when:**

- intentional parametrize / table-driven cases with shared scaffold

#### `sev-medium-005` — Floating model tag

An LLM call references a model name that resolves to whatever the provider currently maps it to (e.g. `gpt-4`), so behaviour silently drifts.

**Applies when:**

- `model: "claude-3-opus"` without a version date suffix
- `gpt-4` instead of `gpt-4-0613`

### Low

#### `sev-low-001` — Skipped test without ticket reference

A `.skip` / `@pytest.mark.skip` / `@Disabled` annotation has no comment or annotation linking to a tracking ticket.

**Applies when:**

- `it.skip("flaky")` with no follow-up ticket

#### `sev-low-002` — Deprecated test pattern in legacy area

Older test idiom (sinon, enzyme, JUnit 4 Hamcrest) used in code outside the active migration scope; correct but inconsistent.

#### `sev-low-003` — Slow test (>5s)

Single test runtime exceeds 5 seconds without a documented justification (integration test, container startup).

**Does not apply when:**

- test annotated as @slow / @integration with policy exemption

### Info

#### `sev-info-001` — Untested export, low blast radius

Exported symbol has no direct test, but is internal-only or has zero callers in the repo's import graph.

#### `sev-info-002` — Non-canonical assertion style

Assertion style differs from the project's prevailing convention (e.g. `expect.toBe` in a project that uses `assert.equal`).

## How to cite

In a detector that emits a `Signal`, set `SeverityClauses` to the IDs that justify
the chosen severity:

```go
models.Signal{
    Type:            "weakAssertion",
    Severity:        models.SeverityMedium,
    SeverityClauses: []string{"sev-medium-001"},
    // ... rest of signal
}
```

`internal/severity.ValidateClauseIDs` returns the set of unknown IDs from a list,
which detectors and tests use to fail loudly on typos.

## Calibration ladder

Clauses are heuristic in 0.2 — author-set based on the rule's structure and the
examples above. The 0.2 calibration corpus (50 labeled repos) measures per-clause
precision/recall and re-anchors borderline severities. Calibrated clauses gain a
`Quality: "calibrated"` field on the corresponding `ConfidenceDetail`.
