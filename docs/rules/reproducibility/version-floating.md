# `terrain/reproducibility/version-floating`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A declared dependency has no exact version pin, so subsequent installs may resolve to a different version and produce different test or eval results.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium (high for unpinned runtime deps; one severity step lower for dev / build / optional deps)
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — see §7

## 3. What this catches

- `requests` in `requirements.txt` with no version specifier — pip resolves the latest matching version at install time
- `^1.2.3` in `package.json` — minor and patch releases are admitted
- `git+https://github.com/foo/bar.git@main` in `pyproject.toml` — `main` is a moving branch reference
- `>=1.0,<2.0` in `pyproject.toml` `[project] dependencies` — any 1.x release qualifies
- `https://example.com/foo.tgz` direct-URL reference — the URL may be re-uploaded with different contents

## 4. Why this matters

The most common source of "works on my machine but not in CI" is two engineers installing the same nominal dependency at different times and getting different transitive trees. The non-determinism is invisible until it isn't — a transitive minor bump in `numpy`, `openai`, or `langchain` regresses an eval, and the bisect points at code that didn't change. Version pinning collapses the search space: same lockfile → same install → reproducible eval and test runs.

The rule is deliberately broad. It fires on any non-exact specifier — not just unpinned. Patch-level drift is rare to break, but minor drift is common (the openai Python SDK changed default model behavior across patch versions in 2024, for instance). Adopters with a deliberate "tolerate patch drift" policy downgrade the severity in `terrain.yaml` rather than disable the rule outright.

## 5. Detection mechanism

The rule consumes parsed dependency manifests from `internal/manifest/` and reads each dependency's `Pinning` classification.

- **Approach:** manifest-parse-and-classify (no source AST walk required)
- **Languages / ecosystems supported:** Python (pyproject.toml PEP-621 + Poetry, requirements.txt PEP-508), Node (package.json dependencies / devDependencies / peerDependencies / optionalDependencies)
- **Inputs consumed:** the manifest list produced by `internal/manifest/Detect`
- **Pinning ladder fired against:**
  - `PinningUnpinned` → high severity
  - `PinningRange` → medium severity
  - `PinningGit` without commit SHA → medium severity
  - `PinningGit` with 7+ hex commit SHA → suppressed (reproducible)
  - `PinningURL` → medium severity
  - `PinningPath` → low severity (reproducible within the same checkout)
  - `PinningExact` → suppressed
- **Section step-down:** runtime deps fire at the severity listed above; dev / build / optional deps fire one step lower.
- **Edge cases handled:** `#egg=` URL fragments preserved as locator metadata; PEP-508 environment markers (`; python_version >= '3.10'`) noted on the dependency but don't change classification
- **Edge cases NOT handled at 0.2.0:** lockfile-aware suppression (when `package-lock.json` / `poetry.lock` is present, range pins are effectively pinned). Lockfile-aware suppression is future work.

## 6. Worked example

```
warning[terrain/reproducibility/version-floating]: dependency "openai" in requirements.txt has a range version specifier (>=1.20)
  --> requirements.txt:7
   = pinning:     range
   = section:     runtime
   = help:        Pin to an exact version (e.g., "openai==<version>" in requirements.txt or a fixed semver in package.json), or commit a lockfile that records the resolved set.
   = docs:        https://github.com/pmclSF/terrain/blob/main/docs/rules/reproducibility/version-floating
```

**Before:**

```
# requirements.txt
openai>=1.20
```

**After:**

```
# requirements.txt
openai==1.42.0
```

## 7. Configuration

**Downgrade severity for range pins** (adopter who relies on a lockfile):

```yaml
rules:
  reproducibility/version-floating:
    severity: low
```

**Ignore specific paths** (vendored manifests, third-party submodules):

```yaml
ignore:
  rules:
    reproducibility/version-floating:
      - "vendor/**"
      - "third_party/**"
```

**Allow specific dependencies** (regulated library that publishes only via range):

```yaml
rules:
  reproducibility/version-floating:
    allow:
      - "some-internal-lib"
```

## 8. False-positive characterization

- **Dependencies with a lockfile** — the most common pattern. A range pin in `package.json` is effectively pinned when `package-lock.json` is committed. 0.2.0 doesn't read lockfiles; lockfile-aware suppression is future work. Mitigation today: downgrade severity to `low` and rely on the lockfile for actual reproducibility.
- **Git-tag pins** — `git+https://github.com/foo/bar.git@v1.0.0` looks like a moving reference but tags are usually immutable in practice. The rule fires; mitigation is to use a commit SHA, or accept the medium severity.
- **Editable installs of in-repo packages** — `-e ./path/to/local-pkg` flags as PinningPath and fires at low severity. Generally accurate (the local checkout is what changes), but adopters can ignore via path.
- **Measured FP rate at last validation:** see the per-rule readiness card published with the release tag.

## 9. Reproducibility

```bash
terrain test --selector reproducibility/version-floating
```

To scope to a specific path:

```bash
terrain test --selector reproducibility/version-floating --path requirements.txt
```

## 10. Stability commitment

This rule's ID, default severity, pinning ladder, and section step-down behavior are stable from v0.2.0. Per `docs/PRODUCT.md` §14 (Versioning):

- **Default severity changes** — breaking; one-cycle deprecation.
- **Pinning-ladder changes** (e.g., promoting PinningPath from low to medium) — breaking; deprecation-cycled.
- **New ecosystems added** (go.mod, Cargo.toml, Gemfile) — additive; documented in `CHANGELOG.md` but not deprecation-cycled. New ecosystems land as part of `internal/manifest/` expansion.
- **Lockfile-aware suppression** (future work) — additive; reduces fire count without changing existing behavior.

## 11. Related rules

- `terrain/reproducibility/no-seed` — same family; flags missing seeds in stochastic code (np.random, torch.manual_seed)
- `terrain/reproducibility/missing-env-pinning` — same family; flags reliance on unpinned environment variables (MODEL=$MODEL without a default)
- `terrain/hygiene/model-fixture-unpinned` — saved model artifacts not pinned to a content-addressed reference
- `terrain/coverage/missing-baseline` — eval ran without a baseline; sibling concern for reproducibility of comparison
