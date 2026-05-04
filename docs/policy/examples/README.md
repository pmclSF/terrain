# Terrain policy examples

Three starter policies for `.terrain/policy.yaml`. Pick the closest
match for your repo's adoption stage and copy it over the file
`terrain init` generates.

| File | Use when | Blocks on |
|------|----------|-----------|
| [`minimal.yaml`](minimal.yaml) | First-time adoption / inherited debt | Nothing — every rule warn-only |
| [`balanced.yaml`](balanced.yaml) | Most teams, after a couple of weeks of polish | Critical AI regressions + safety gaps + skipped tests |
| [`strict.yaml`](strict.yaml) | Mature repos on enforced-quality branches | Every high-or-above finding + zero accuracy regression |

## How to use

`terrain init` writes a commented policy file to
`.terrain/policy.yaml`. To start with one of these examples
instead:

```bash
# Pick the policy that matches your adoption stage:
cp docs/policy/examples/minimal.yaml .terrain/policy.yaml

# Then in CI:
terrain policy check
```

## Adoption ramp

1. **Start with `minimal.yaml`** on a fresh adoption. Every rule
   warns; nothing blocks the build. Watch what fires for a week.
2. **Promote to `balanced.yaml`** once the warning list is
   calibrated. Pair with `terrain analyze --fail-on critical
   --new-findings-only --baseline <path>` so existing debt is
   grandfathered in but every new finding must clear the bar.
3. **Promote to `strict.yaml`** for mature repos on enforced-
   quality branches. Pair with the suppression workflow
   (`terrain suppress <finding-id> --reason "..." --expires
   YYYY-MM-DD`) so legitimate waivers don't accumulate silently.

## What policy.yaml does NOT cover

- **Severity gates** are a CLI flag (`--fail-on critical`), not a
  policy rule. The recommended GitHub Action template combines
  both.
- **Suppressions** live in `.terrain/suppressions.yaml`, not the
  policy file. Suppressions wave specific findings; policy rules
  set repo-wide thresholds.
- **Per-team overrides** are not yet supported. The policy file
  is repo-wide. Per-team / per-directory policies are tracked for
  0.3.

## Related docs

- [`docs/product/vision.md`](../../product/vision.md) — overall
  product narrative
- [`CONTRIBUTING.md`](../../../CONTRIBUTING.md#parity-gate-lifting-maturity-uniformly) —
  parity gate semantics for contributors
- [`docs/release/feature-status.md`](../../release/feature-status.md) —
  per-capability tier status
