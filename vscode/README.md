# Terrain — legacy VS Code prototype

This directory contains an older VS Code diagnostics prototype kept for reference. The shipped extension alpha lives in [`extension/vscode/`](../extension/vscode/), and its public behavior is documented in [`docs/vscode-extension.md`](../docs/vscode-extension.md).

This prototype is not published to the VS Code Marketplace, not part of `make extension-verify`, and not the release extension.

## Setup

1. Install [Terrain](https://github.com/pmclSF/terrain).
2. Build or package this prototype locally if you need to inspect the older diagnostics experiment.
3. Run `terrain analyze` (or any subcommand that emits findings.json) in your repo. The extension auto-loads `.terrain/findings.json`.

## Prototype capabilities

- **Diagnostic rendering experiment** — Findings render with severity and a clickable rule ID linking to the canonical docs.
- **Click-to-navigate** — Click a Finding and jump to its `primary_loc`.
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

The packaged `.vsix` can be installed locally for prototype testing. The current release does not publish this package.
