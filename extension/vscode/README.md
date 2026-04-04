# Terrain Test Intelligence

Understand your test system without leaving your editor. Terrain analyzes your repository's test structure, coverage, and quality — then surfaces findings directly in the VS Code sidebar.

## Features

**Sidebar Views** — five panels organized by concern:

- **Overview** — repository profile, framework inventory, headline finding, risk posture
- **Health** — skip burden, flaky/slow test signals, runtime readiness guidance
- **Quality** — coverage gaps, weak assertions, mock-heavy tests, duplicate clusters
- **Migration** — framework readiness, blockers, area assessments
- **Review** — all medium+ findings grouped by category and severity

**Zero Configuration** — the extension invokes the `terrain` CLI on your workspace and renders the results. No setup files, no config, no test execution required.

**Click to Navigate** — findings link directly to the source files they reference. Click any finding to jump to the relevant code.

## Prerequisites

Install the Terrain CLI:

```bash
brew install pmclSF/terrain/mapterrain
# or
npm install -g mapterrain
# or
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

The extension invokes `terrain analyze --json` and `terrain insights --json` in your workspace. All intelligence lives in the CLI — the extension is a lightweight rendering layer.

## Commands

| Command | Description |
|---------|-------------|
| `Terrain: Refresh Analysis` | Re-run analysis and refresh all views |
| `Terrain: Open Executive Summary` | Open terminal with `terrain summary` |
| `Terrain: Show Migration Blockers` | Open terminal with `terrain migration blockers` |
| `Terrain: Reveal File` | Open the file associated with a finding |

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `terrain.binaryPath` | `terrain` | Path to the terrain CLI binary |
| `terrain.autoRefresh` | `false` | Auto-refresh when workspace files change |

## How It Works

```
terrain analyze --json  ─┐
terrain insights --json  ├─→ report bundle ─→ sidebar views
terrain migration readiness --json ─┘
```

The extension calls the CLI, parses JSON output, and renders tree views. No business logic is duplicated in the extension — the CLI is the single source of truth.

## Development

```bash
cd extension/vscode
npm ci
npm run compile
npm test
# Press F5 in VS Code to launch Extension Development Host
```

## Source Files

| File | Purpose |
|------|---------|
| `src/extension.ts` | Entry point, TreeDataProviders, commands, CLI integration |
| `src/types.ts` | TypeScript types matching CLI JSON contracts |
| `src/signal_renderer.ts` | Finding grouping and filtering helpers |
| `src/views.ts` | View data builders for the report bundle |

## Links

- [Terrain CLI](https://github.com/pmclSF/terrain) — the analysis engine
- [Quickstart Guide](https://github.com/pmclSF/terrain/blob/main/docs/quickstart.md) — get started in 5 minutes
- [Signal Catalog](https://github.com/pmclSF/terrain/blob/main/docs/signal-catalog.md) — all 22+ signal types
