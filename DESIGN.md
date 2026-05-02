# Terrain — Architecture Overview

Terrain is a signal-first test intelligence platform. It analyzes repository structure, test code, runtime artifacts, and local policy to surface actionable findings about test suite health, quality, migration readiness, and risk.

## Core Principles

- **Signals are the core abstraction.** Every finding is a structured signal with type, severity, evidence, and location.
- **Snapshots are the canonical artifact.** The `TestSuiteSnapshot` is the serialized boundary between engine, reporting, and external integrations.
- **Risk must be explainable.** Risk surfaces are derived from signals with transparent scoring, not opaque scores.
- **Local-first.** Terrain is useful on a single machine, without accounts, SaaS, or network access.
- **Privacy boundary.** Aggregate metrics and benchmark exports never expose raw file paths or source code.

## Architecture

```
Repository scan
  -> Framework detection + test file discovery
    -> Signal detection (quality, health, migration, governance)
      -> Risk scoring (reliability, change, speed)
        -> Snapshot (TestSuiteSnapshot)
          -> Reporting (human-readable, JSON, executive summary)
```

See [docs/architecture.md](docs/architecture.md) for the full layered architecture.

## Key Design Documents

| Document | Purpose |
|----------|---------|
| [docs/architecture.md](docs/architecture.md) | Layered architecture: analysis, signals, risk, reporting |
| [docs/signal-model.md](docs/signal-model.md) | Signal abstraction and schema |
| [docs/signal-catalog.md](docs/signal-catalog.md) | All signal types and categories |
| [docs/cli-spec.md](docs/cli-spec.md) | Full command and flag reference |
| [docs/engineering/detector-architecture.md](docs/engineering/detector-architecture.md) | Registry-based detector plugin system |
| [docs/contributing/writing-a-detector.md](docs/contributing/writing-a-detector.md) | How to add a new signal detector |

## Package Map

```
cmd/terrain/          CLI entry point (10 canonical commands + legacy aliases)
internal/             49 packages — see internal/README.md for the listing
```

See [docs/engineering/detector-architecture.md](docs/engineering/detector-architecture.md) for the detector plugin system architecture.

## Migration Context

Terrain originated as a multi-framework test converter. That migration surface now lives directly in the Go CLI under `internal/convert` and `cmd/terrain`, so migration and analysis ship as one product runtime instead of separate stacks.

The historical converter architecture remains documented in [docs/legacy/converter-architecture-legacy.md](docs/legacy/converter-architecture-legacy.md) for reference, but the supported implementation is now Go-native.

## Extension Architecture

The VS Code extension is intentionally thin. It invokes `terrain analyze --json`, reads the snapshot, and renders views. No domain logic is duplicated in the extension. See [docs/vscode-extension.md](docs/vscode-extension.md).
