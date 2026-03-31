# Terrain Test Intelligence -- VS Code Extension

Thin VS Code / Cursor extension that renders Terrain analysis as sidebar views.
All intelligence lives in the CLI -- the extension is a read-only lens.

See [docs/vscode-extension.md](../../docs/vscode-extension.md) for the design.

## Architecture

```
terrain analyze --json
terrain insights --json
terrain migration readiness --json
        --> report bundle --> TreeDataProviders --> Sidebar Views
```

The extension invokes the CLI, parses the report bundle, and renders views.
No business logic is duplicated in TypeScript.

## Sidebar Views

| View | Content |
|------|---------|
| **Overview** | Repository name, framework count, test file count, signal count, headline, risk posture, top findings |
| **Health** | Reliability findings from `terrain insights`, skipped-test burden, runtime-readiness guidance |
| **Quality** | Coverage and architecture findings from `terrain insights` |
| **Migration** | Framework summary, readiness explanation, blocker count, area assessments, coverage guidance |
| **Review** | Medium+ findings grouped by category, severity, and directory/scope |

## Commands

| Command | Description |
|---------|-------------|
| `Terrain: Refresh Analysis` | Re-run `terrain analyze --json` and refresh all views |
| `Terrain: Open Executive Summary` | Open terminal with `terrain summary` |
| `Terrain: Show Migration Blockers` | Open terminal with `terrain migration blockers` |
| `Terrain: Reveal File` | Open the file associated with a finding |

## States

All views handle these states:

- **Empty**: No analysis run yet -- shows prompt to refresh
- **Loading**: Analysis in progress -- shows spinner
- **Error**: CLI failed -- shows error message and install guidance
- **Loaded**: Real data rendered from snapshot

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `terrain.binaryPath` | `terrain` | Path to the terrain CLI binary |
| `terrain.autoRefresh` | `false` | Auto-refresh the sidebar after relevant workspace file changes |

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
| `src/extension.ts` | Extension entry point, TreeDataProviders, commands, CLI integration |
| `src/types.ts` | TypeScript types aligned with analyze/insights/migration JSON contracts |
| `src/signal_renderer.ts` | Grouping/filtering helpers for report findings |
| `src/views.ts` | View data builders for the extension report bundle |
