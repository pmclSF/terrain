# Test-type classification

How Terrain decides whether a test is a unit test, an integration
test, or an end-to-end test — and the explicit limits of that
classification in 0.2.

## Why this matters

The pitch claims Terrain "maps how your unit, integration, e2e, and
AI tests actually relate to your code." That promise depends on
classifying test files into those four categories accurately. The
launch-readiness review flagged classification as the weakest link
in that promise: the path/suite/framework heuristics worked well for
repos that organize tests by directory, but missed the common case
of integration tests living alongside unit tests in a flat layout
and identifying themselves only through HTTP-testing imports.

Track 3.3 of the 0.2 release plan addressed this gap. This page
documents what now ships and what remains explicitly out of scope
for 0.2.

## What 0.2 detects

Terrain runs three classification passes on each test file, in
order, and merges the results.

### Pass 1 — path / framework / suite name (metadata)

The original heuristic, retained without change:

- Path components: `e2e/`, `integration/`, `unit/`, `__tests__/`,
  `smoke/`, `component/`
- File name patterns: `.e2e.`, `.integration.`, `.cy.{js,ts}`
  (Cypress), `.spec.{js,ts}` (ambiguous, low-confidence unit)
- Framework hints: `playwright`, `cypress`, `puppeteer`,
  `webdriverio`, `testcafe` → e2e; `jest`, `vitest`, `mocha`,
  `pytest`, `junit*` → unit (low confidence — these run integration
  tests too)
- Suite hierarchy names containing "Integration", "E2E", "Smoke"

Confidence ranges from 0.4 (ambiguous `.spec` extension) to 0.9
(explicit e2e framework).

### Pass 2 — content-based integration libraries (new in 0.2)

Terrain reads each test file once and checks for explicit imports
of HTTP-testing or contract-testing libraries that strongly signal
integration testing:

| Ecosystem | Libraries detected | Confidence |
|-----------|-------------------|------------|
| JS / TS   | `supertest`, `nock`, `msw`, `pactum`, `testcontainers` | 0.85–0.9 |
| Go        | `net/http/httptest` | 0.9 |
| Python    | `requests` (call sites), `httpx`, `responses`, `pact` | 0.85–0.9 |
| Java      | `MockMvc`, `RestAssured` | 0.9 |
| Ruby      | `rack/test`, `webmock` | 0.85–0.9 |
| Tooling   | `dredd`, `testcontainers` | 0.85–0.9 |

A match promotes the test from whatever Pass 1 said to
`integration` with the matched library cited in evidence. When
Pass 1 disagrees (e.g. path says `unit/`), the content-based signal
wins because explicit imports are harder to fake than directory
naming — but the conflict is preserved in evidence so downstream
consumers can see it.

The pattern allowlist lives in
`internal/testtype/integration_imports.go`. Adding a library is the
documented extension point.

### Pass 3 — e2e attribution (structural, see limits below)

E2E tests don't normally import the source units they exercise —
they hit a running browser or HTTP boundary. Terrain attributes e2e
tests to code units only via *structural* signals: shared fixture
paths, file-co-location heuristics, declared associations in
playwright/cypress configs. This is honestly weaker than the
import-graph attribution unit and integration tests get. See
[`docs/product/e2e-attribution.md`](e2e-attribution.md) for the
full carve-out.

## What 0.2 does NOT classify

These cases are deliberately out of scope; document them up front
so adopters don't infer a guarantee that isn't there.

### Property-based tests as a separate category

Property-based tests (`fast-check`, `hypothesis`, `quickcheck`) are
classified as `unit` today even though they're a meaningfully
different shape. Adding `property-based` as a first-class type is
0.3 work — the framework type enum already has the slot
(`FrameworkTypePropertyBased`); the inference rules don't.

### Contract tests vs. integration tests

Pact and Dredd both surface as `integration` today even though
contract testing is a distinct discipline. Splitting them would
require reading the specific contract artifact (pact JSON,
OpenAPI spec) — 0.3 work.

### Mutation tests

`stryker`, `mutmut`, `mutpy` outputs are not classified at all
today. They aren't really tests in the same shape — they exercise
existing tests against mutated source. Out of scope for 0.2's
classification model.

### AI evals as a fourth pillar in classification

AI eval scenarios are tracked separately via the AI surface
inventory and eval ingestion path (`terrain ai list`,
`terrain ai run`); they aren't merged into the unit / integration /
e2e classification because they exercise a different surface
(prompts and tools, not code units). The PR comment surfaces both
sides as adjacent stanzas — see
[`docs/product/unified-pr-comment.md`](unified-pr-comment.md).

## Confidence and conflict reporting

Every classified test case carries:

- `testType` — `unit` / `integration` / `e2e` / `component` /
  `smoke` / `unknown`
- `testTypeConfidence` — `[0.0, 1.0]`; values below `0.5` indicate
  the inference disagrees with itself or has only weak signals
- `testTypeEvidence` — list of strings citing what fired

When `--explain-selection` (or `terrain explain`) is invoked, this
evidence is rendered alongside the test in the reason chain. False
positives in classification are visible by inspection rather than
hidden inside a black-box label.

## Known false positives

These are the cases adopters are most likely to hit. They're not
bugs we plan to fix in 0.2 — they're the trade-offs of conservative
heuristics. Suppress per-file with a `.terrain/suppressions.yaml`
entry if needed.

- **Unit tests that import `nock` to mock outbound HTTP** — Terrain
  classifies as integration. The import alone signals "this test
  cares about HTTP," which is the integration-shaped concern; if
  the test is conceptually unit, the path/suite name overrides the
  content signal only when path/suite are highly confident.
- **Python unit tests that import `requests` for type hints only** —
  Pattern requires a call-site (`requests.get(`, etc.), not a bare
  import, so this should not over-fire. Report a false positive if
  it does.
- **Go unit tests in the same package as integration tests** — If
  the package has *any* file that imports `net/http/httptest`, that
  file is classified integration; sibling unit tests in the same
  package are not affected unless they too import httptest.

## How to extend integration-library detection

If your stack uses a library not on the allowlist, the extension
shape is small:

1. Open `internal/testtype/integration_imports.go`.
2. Add an `integrationImportPattern` entry: substring (with quote /
   paren context to avoid matching prose), library name,
   confidence (0.85 default; 0.9 for libraries that are
   integration-only).
3. Add at least one test in `integration_imports_test.go` that
   exercises the new pattern and at least one negative case
   (prose mention should not match).
4. Run `make calibrate` to ensure the addition doesn't shift any
   existing fixture's classification unexpectedly.

The bar for adding a pattern: the library should be either
purpose-built for integration testing (supertest, httptest) or its
presence in a test file should overwhelmingly indicate the test
crosses a real HTTP / database boundary. Conservative is better
than aggressive — false-positive integration claims distort the
test-system inventory more than false negatives do.

## Related reading

- [`internal/testtype/integration_imports.go`](../../internal/testtype/integration_imports.go)
  — the pattern allowlist
- [`docs/product/e2e-attribution.md`](e2e-attribution.md) —
  honest carve-out for e2e-to-code-unit linking
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  which capabilities are publicly claimable
