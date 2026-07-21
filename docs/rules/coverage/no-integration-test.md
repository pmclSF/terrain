# terrain/coverage/no-integration-test — No Integration Test

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `noIntegrationTest`  
**Domain:** quality  
**Default severity:** medium  
**Lifecycle status:** experimental  
**Gating tier:** observability

## Summary

A code unit reachable from a production entry point (handler / route) has no integration test exercising it through that entry point.

## Remediation

Add an integration test that exercises the handler / route end-to-end. The unit test stays as a fast inner-loop check; the integration test ensures the cross-stack contract holds.

## Evidence sources

- `graph-traversal`

## Confidence range

Confidence interval: 0.80–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A code unit reachable from a production entry point (HTTP handler, route, or RPC method) has no integration test exercising it through that entry point. Unit tests cover the unit in isolation, but the cross-stack path is unguarded.

## 2. Status

Experimental — off by default; enable in terrain.yaml.

## 3. What this catches

- A handler `handle_refund` with unit tests for the inner logic but no test that POSTs to `/api/refund`
- A gRPC method whose handler is unit-tested but no end-to-end RPC test exists
- A code unit that's only invoked via a tRPC procedure with no procedure-level test

## 4. Why this matters

Unit tests are fast and catch logic errors; integration tests catch wiring errors. The two failure modes are different — `handle_refund` may pass every unit test but be unreachable because middleware rejects the request, or because the route is registered under the wrong path. Integration tests guard the cross-stack contract.

## 5. Detection mechanism

- **Approach:** graph traversal. Find SurfaceHandler / SurfaceRoute nodes; for each, walk reachable code units (the handler's logic); check whether any test reaches both the entry point AND the unit.
- **Inputs:** ImpactGraph edges + integration-test classification.
- **Scope:** flagged when no edge exists from any integration test to the entry-point surface.

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

## 11. Related rules

- `terrain/coverage/no-tests` — same shape but for code units in general
- `terrain/coverage/blind-spot` — graph-shape complement
