# E2E-to-code attribution

How Terrain links e2e tests back to the source code they exercise —
and the explicit limit of that linking in 0.2.

## The problem

Unit and integration tests *import* the code they exercise. Their
imports become edges in Terrain's import graph, and from those edges
Terrain can answer: "if I change `src/auth/login.ts`, which tests
should I run?" The graph traversal is sound; the imports are
ground truth; the attribution is precise.

E2E tests don't work that way. A Playwright spec navigates to
`http://localhost:3000/login`, types into a form, clicks a button,
and asserts on rendered DOM. Nothing in that file imports
`src/auth/login.ts`. The test exercises the same code path that
unit tests exercise — but the link between the two is not in the
import graph.

This is a real limit, not a Terrain shortcoming. Every static-
analysis tool faces it: e2e tests are deliberately decoupled from
implementation so they survive refactors, and that decoupling
removes the import-graph signal Terrain relies on for unit /
integration attribution.

## What 0.2 does

Terrain attributes e2e tests to code units only via **structural**
signals. Adopters should read these as best-effort heuristics, not
as ground truth.

### 1. Path co-location

If `e2e/login.spec.ts` lives next to a feature directory whose
sibling unit tests link to specific code units, e2e attribution
borrows those links transitively. Confidence: **medium**. Common
case: a monorepo packages directory where each feature owns both
its unit tests and its e2e specs.

### 2. Declared associations in framework configs

Playwright and Cypress configs sometimes declare which routes /
features each spec exercises (via `testMatch` patterns or
`describe()` titles). When parseable, Terrain folds those
declarations into the link set. Confidence: **medium-high** when
the config is explicit, **low** when only the test name carries
the signal.

### 3. Shared fixture paths

If an e2e spec imports a fixture file (page-object, test-data,
auth-helper), and that fixture is imported by other tests with
known code-unit links, Terrain transitively links the e2e spec to
those code units. Confidence: **low** — fixtures are often shared
across many features and the transitive link can be loose.

### 4. Convention-based mapping (last resort)

For repos without explicit configs or co-location structure,
Terrain falls back to convention: the `e2e/auth/` directory is
assumed to exercise `src/auth/`. Confidence: **low**, marked as
`structural-only` in evidence.

## What 0.2 explicitly does NOT do

These are out of scope and remain so until 0.3 or later. We
document them up front so adopters don't infer guarantees that
aren't there.

### Runtime trace ingestion

A natural way to close the gap is to run the e2e suite once with
coverage instrumentation, capture which source lines each spec
hits, and use that as ground truth. We don't do this in 0.2. It
requires running tests, which Terrain explicitly does not do (see
[`docs/product/ai-trust-boundary.md`](ai-trust-boundary.md)).
Adopters who want coverage-grade e2e attribution should run their
e2e suite with `--coverage` and feed the resulting LCOV / Istanbul
artifact through Terrain's coverage ingestion path. Terrain will
read it. Terrain will not produce it.

### URL-to-route mapping

A web app's e2e spec navigates to `/login`. The route handler is
in `src/routes/auth.ts:loginHandler`. Linking the two requires
parsing the framework's router config (Express, Next.js, FastAPI,
Rails) and matching URL patterns to handler functions. We don't
do this in 0.2. The router-parsing surface area is large
(every framework has a different shape) and the precision /
recall trade-offs are not yet measured.

### DOM-selector to component mapping

A Playwright spec interacts with `page.locator('[data-testid="login-button"]')`.
The component that renders that button is `src/components/Login.tsx`.
Linking the two requires parsing the test for selectors, parsing
the source for component definitions, and matching them. We don't
do this in 0.2. Test selectors aren't standardized — `data-testid`,
`aria-label`, `id`, role-based, text-based — and matching them
correctly across rebuilt components is research-grade work.

### Cross-language e2e attribution

A Playwright spec written in TypeScript exercising a Go backend
service via HTTP. The backend handlers are in
`internal/handlers/users.go`. Cross-language linking is not in 0.2.
The import-graph crosses ecosystems imperfectly even for unit
tests; for e2e, where the link goes through a network boundary,
we explicitly do not attempt it.

## How this surfaces in output

When Terrain reports impact analysis on a change to source code:

```
terrain report impact --base main
```

The output now distinguishes attribution confidence per test:

```
Recommended Tests
  src/auth/__tests__/login.test.ts [exact] (import-graph)
    Covers: src/auth/login.ts:loginUser
  e2e/auth/login.spec.ts [structural-only] (path co-location)
    Covers: src/auth/login.ts (file-level, not symbol-level)
  e2e/checkout.spec.ts [convention] (low confidence)
    Reason: directory mapping suggests this exercises src/checkout/
```

The `[exact]` / `[structural-only]` / `[convention]` tag is the
attribution-confidence signal. `--explain-selection` (added in
Track 3.2) renders the full reason chain — important for e2e
because the reasons are looser than for unit tests and adopters
need to inspect them.

When `terrain report posture` is invoked, the analysis-completeness
signal flags repos where e2e attribution is the only source of
coverage for a code unit:

```
Posture
  coverage_diversity:  ELEVATED
    e2e/checkout.spec.ts is the only test linked to src/checkout/cart.ts
    — but the link is structural-only. Treat this as suggestive,
    not as proof of coverage.
```

## Why we ship structural-only attribution at all

The alternative — emitting *no* link for e2e specs — is worse than
shipping low-confidence links that adopters can inspect. With no
link, `terrain report impact` would silently exclude e2e specs
from the recommended-tests set whenever a source file changed,
even when the e2e spec is the only test exercising that path. With
low-confidence links plus an honest `[structural-only]` tag,
adopters see the link and can decide: trust it for now, or run all
e2e specs as a precaution, or invest in coverage instrumentation.

The same principle drives every limit on this page: visible
imperfection beats invisible omission.

## 0.3 roadmap

Order of likely investment:

1. **Coverage-artifact ingestion for e2e** — read LCOV / Istanbul
   produced by `playwright test --coverage` and use it as ground
   truth, replacing the structural fallback whenever present.
2. **Router config parsing** — Express, Next.js, FastAPI, Rails.
   Map URLs in test specs to handler functions in source.
3. **Selector-to-component mapping** — opt-in per repo via a
   `.terrain/e2e-config.yaml` that declares the selector
   convention used (e.g. `data-testid` only).

None of these are 0.2 work. None will block 0.2. The honest carve-
out documented here is the 0.2 contract.

## Related reading

- [`docs/product/test-type-classification.md`](test-type-classification.md)
  — how Terrain decides a test is e2e in the first place
- [`docs/product/impact-analysis-model.md`](impact-analysis-model.md)
  — full impact-analysis architecture
- [`docs/architecture/04-deterministic-test-identity.md`](../architecture/04-deterministic-test-identity.md)
  — test-identity model that makes attribution stable across runs
