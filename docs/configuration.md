# Configuration

Terrain reads repository configuration from `.terrain/terrain.yaml`, falling back to a top-level `terrain.yaml` for older setups.

Minimum file:

```yaml
version: 1
```

Common rule tuning:

```yaml
version: 1
rules:
  coverage/no-tests:
    severity: warning
    max_findings: 25
ignore:
  paths:
    - "vendor/**"
  rules:
    coverage/no-tests:
      - "scripts/**"
```

AI surface markers:

```yaml
version: 1
ai:
  ai_markers:
    - "from internal_llm_sdk"
    - "@acme/llm-client"
```

## Gate behavior — the trust floor

The `--fail-on` gate runs with a **trust floor** by default: a *heuristic* AI finding fails the build only when Terrain can prove a fix for it, so CI never breaks on a low-confidence finding. Failing tests, regressions, security/safety leaks, `policy.yaml` violations, and any Critical always gate regardless. Turn it off (gate every finding on severity) per-repo:

```yaml
version: 1
trust_floor: false
```

Omit the key (or set `true`) to keep the default. The CLI `--no-trust-floor` / `--trust-floor` flags on `terrain analyze`, `terrain test`, and `terrain report pr` / `check-runs` override the config per-invocation.

Run `terrain init` to generate starter Terrain files, or `terrain describe --write` to generate a starter surface declaration. Run `terrain fix` to apply the validated remediations Terrain can prove (dry-run by default; `--apply` writes them).
