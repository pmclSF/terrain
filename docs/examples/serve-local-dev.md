# `terrain serve` — local-dev preview

> **Scope:** local development preview only. `terrain serve` is not
> a team dashboard. It binds to `127.0.0.1` by default, has no
> authentication, and runs read-only by default. Don't expose it
> beyond your machine.

## What it's for

When you're iterating on a piece of code and want a fast,
browser-rendered view of the test-system state — coverage gaps,
duplicate clusters, AI surfaces, change-scoped impact — without
re-running `terrain analyze` and reading terminal output every
time.

## Quickstart

```bash
# In your repo:
terrain serve
# → Listening on http://127.0.0.1:7344
# → Press Ctrl-C to stop.
```

Open `http://127.0.0.1:7344` in your browser. The page renders the
same data shapes as `terrain analyze`, with sticky pillar
navigation, signal cards, and tasteful typography.

## Common flags

```bash
# Custom port
terrain serve --port 8000

# Allow request to mutate state (the default is read-only — most
# adopters want this off for local dev too):
terrain serve --read-only=false

# Use a saved snapshot instead of re-analyzing:
terrain serve --snapshot .terrain/snapshots/latest.json
```

## What's safe

- 127.0.0.1 binding only. The server refuses non-localhost origins.
- Read-only by default — `POST` / mutating handlers return 405.
- Per-request `r.Context()` cancellation: closing the browser tab
  cancels the in-flight analysis.
- Singleflight on concurrent analyses — two browser tabs hitting
  the same endpoint share one analysis pass.

## What this isn't

- Not a team dashboard. There's no auth, no multi-user state, no
  audit log.
- Not a CI surface. For CI, use `terrain analyze --json` or the
  `report pr` workflow.
- Not a live-reload watcher. Each request runs fresh; there's no
  background indexing.

## When it pays off

- During a refactor, when you want to see structural impact
  without breaking flow.
- During PR prep, to preview what `report pr` will say before
  pushing.
- When showing a teammate the test-system state in a meeting,
  one-click instead of asking them to run a CLI command.

## Next steps

- `terrain analyze` — same data, terminal output, no server.
- `terrain report pr --base main` — gate a PR diff.
- `terrain --help` — full command surface, grouped by pillar.
