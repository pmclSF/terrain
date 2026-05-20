# Internal docs — read with appropriate skepticism

Files in this directory are **internal planning artifacts**, not
user-facing documentation. They describe roadmaps, vision, audit
trails, business framing, and other working material that hasn't
been (and may never be) ratified as product truth.

## What lives here

| File / theme | Purpose |
|---|---|
| `MASTER_PLAN.md`, `roadmap.md` | Multi-quarter planning artifacts |
| `vision.md`, `moat.md`, `paid-product.md` | Business / positioning framing |
| Audit & review docs | Snapshots of internal review passes (lovable-release-audit, etc.) |
| Per-release working notes | Deltas between what was planned and what shipped |

## What does NOT live here

User-facing docs (quickstart, CLI spec, signal model, integration
guides, release notes, supply-chain) live one level up under
`docs/`. The split is intentional: external readers should be able
to ignore this directory entirely without missing anything they
need to use the product.

## If you're a contributor

If you're contributing for the first time, **start with**:

- The repo root [`README.md`](../../README.md)
- [`docs/quickstart.md`](../quickstart.md)
- [`CONTRIBUTING.md`](../../CONTRIBUTING.md)

The internal docs help you understand context for big-picture
decisions, but they aren't a substitute for reading the code or the
shipping documentation.

## Currency disclaimer

Files in this directory may be wildly out of date. Documents here
are NOT subject to the `make docs-verify` gate that protects
user-facing docs from drift. Treat dates and version references
inside as snapshots in time, not authoritative claims about the
current product.
