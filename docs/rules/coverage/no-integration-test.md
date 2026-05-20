# `terrain/coverage/no-integration-test`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A code unit reachable from a production entry point (HTTP handler, route, or RPC method) has no integration test exercising it through that entry point. Unit tests cover the unit in isolation, but the cross-stack path is unguarded.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0

## 3. What this catches

- A handler `handle_refund` with unit tests for the inner logic but no test that POSTs to `/api/refund`
- A gRPC method whose handler is unit-tested but no end-to-end RPC test exists
- A code unit that's only invoked via a tRPC procedure with no procedure-level test

## 4. Why this matters

Unit tests are fast and catch logic errors; integration tests catch wiring errors. The two failure modes are different — `handle_refund` may pass every unit test but be unreachable because middleware rejects the request, or because the route is registered under the wrong path. Integration tests guard the cross-stack contract.

## 5. Detection mechanism

- **Approach:** graph traversal. Find SurfaceHandler / SurfaceRoute nodes; for each, walk reachable code units (the handler's logic); check whether any test reaches both the entry point AND the unit.
- **Inputs:** ImpactGraph edges + TestFile classification (`testtype.IsIntegration`).
- **0.2.0 scope:** flagged when no edge exists from any integration test to the entry-point surface.

## 6. Worked example

```
warning[terrain/coverage/no-integration-test]: handler "/api/refund" has no integration test
  --> backend/handlers/refund.go:42
   = help: Add an integration test that POSTs to /api/refund.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/no-integration-test
```

## 7. Configuration

```yaml
rules:
  coverage/no-integration-test:
    severity: low
ignore:
  rules:
    coverage/no-integration-test:
      - "internal/admin/**"
```

## 9. Reproducibility

```bash
terrain test --selector coverage/no-integration-test
```

## 10. Stability commitment

Rule ID, severity, and the entry-point surface set are stable from v0.2.0.

## 11. Related rules

- `terrain/coverage/no-tests` — same shape but for code units in general
- `terrain/coverage/blind-spot` — graph-shape complement
