# Legacy Notes

> ⚠️ **[DEPRECATED — DO NOT USE FOR NEW WORK]** This document is historical
> context describing how the current architecture relates to the original
> JavaScript converter codebase. Use [DESIGN.md](../../DESIGN.md),
> [docs/architecture/](../architecture/), and [docs/release/feature-status.md](../release/feature-status.md)
> for current product direction.

This file describes the relationship between the legacy Terrain codebase and the current architecture.

## Intent

The current architecture is a refactor, not a discard-and-rebuild effort.

Existing migration functionality should be preserved where possible and gradually mapped into:

- internal/migration
- internal/signals
- internal/reporting
- cmd/terrain

## Migration strategy

1. Freeze docs and architecture
2. Introduce canonical models and signals
3. Build `terrain analyze`
4. Preserve old behavior via adapters where necessary
5. Move legacy migration logic behind the new architecture gradually

## Rule

Do not delete working code until:
- a replacement exists
- tests cover the replacement
- architectural ownership is clear
