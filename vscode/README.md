# Terrain — VS Code

Surfaces findings from the most recent `terrain analyze` run in the VS Code Problems pane, with click-to-navigate and hover diagnostics.

## Setup

1. Install [Terrain](https://github.com/pmclSF/terrain).
2. Install this extension from the Marketplace.
3. Run `terrain analyze` (or any subcommand that emits findings.json) in your repo. The extension auto-loads `.terrain/findings.json`.

## Features

- **Problems pane integration** — Every Finding shows up with severity (error / warning / notice) and a clickable rule ID linking to the canonical docs.
- **Click-to-navigate** — Click a Finding in Problems and jump to its `primary_loc`.
- **Hover diagnostics** — Hover the underlined source location to see the long message, cause-path chain, and the exact CLI command to reproduce locally.
- **Auto-refresh** — The extension watches `.terrain/findings.json` and refreshes when it changes.

## Configuration

- `terrain.findingsPath` — Path to findings.json relative to the workspace root (default: `.terrain/findings.json`).
- `terrain.docsBaseURL` — Base URL prepended to rule slugs for the Open Rule Docs command (default: `https://github.com/pmclSF/terrain/blob/main/docs/rules/`).

## Commands

- **Terrain: Reload Findings** — Manually re-read findings.json.
- **Terrain: Open Rule Docs** — Pick a finding and open its rule documentation in the browser.

## Build

```bash
cd vscode
npm install
npm run compile
npm run package
```

The packaged `.vsix` can be installed locally or published with `vsce publish`.

