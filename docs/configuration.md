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

Run `terrain init` to generate starter Terrain files, or `terrain describe --write` to generate a starter surface declaration.
