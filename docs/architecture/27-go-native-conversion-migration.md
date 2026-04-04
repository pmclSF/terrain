# Go-native Conversion Migration

> **Status:** Completed
> **Purpose:** Define how Terrain retires the legacy JavaScript converter runtime and moves the full conversion surface, tests, and release-critical workflows into Go.
> **Key decisions:**
> - `terrain` becomes the only product CLI
> - Migration is phased; we do not attempt a big-bang rewrite of the converter, migration engine, and test suite
> - Low-risk public contract commands land first in Go (`convert`, `list-conversions`, `shorthands`, `detect`) before source-to-source execution
> - Go-native converter fidelity improvements should prefer AST / Tree-sitter-backed rewrites over whole-file regex substitution where practical
> - Performance must never fall below the legacy JS floor; benchmark gates enforce that contract as converters evolve

**See also:** [09-cli-spec.md](09-cli-spec.md), [23-phased-implementation-roadmap.md](23-phased-implementation-roadmap.md), [../legacy/converter-architecture-legacy.md](../legacy/converter-architecture-legacy.md)

## Goal

Terrain already has a strong Go-native analysis engine, and the framework conversion and migration surface now lives alongside it in Go. The migration effort existed to eliminate the product and release friction caused by the old split runtime:

- npm behaves differently from Homebrew and GitHub Releases
- CI and release verification still depend on two implementation stacks for one product story

The target state is simple:

- one canonical CLI: `terrain`
- one primary implementation language for product logic: Go
- one test strategy centered on Go-native contract, golden, and workflow tests
- one release story across GitHub Releases, Homebrew, and npm wrappers

## Scope

Go-native means the following surfaces eventually move into Go:

- conversion catalog and framework metadata
- shorthand alias contract
- framework detection for conversion sources
- source-to-source test conversion
- project migration orchestration
- config conversion
- validation, status, checklist, reset, and doctor workflows
- release-critical verification for the migration surface

Some pieces intentionally stay non-Go:

- GitHub Actions workflow files
- the VS Code extension, which should remain a thin TypeScript client over Go CLI JSON
- npm metadata and binary-wrapper scripts, which are packaging, not product runtime

## Current State

The migration product surface now has two distinct layers:

1. Go-native Terrain analysis in `cmd/terrain` and `internal/*`
2. Go-native conversion and migration runtime in `internal/convert` and `cmd/terrain`

The legacy command contract includes:

- `convert`
- `convert-config`
- `list`
- `list-conversions`
- `shorthands`
- `detect`
- `validate`
- `init`
- `migrate`
- `estimate`
- `status`
- `checklist`
- `reset`
- `doctor`

## Migration Workstreams

### 1. Contract capture

Move the converter surface area into Go as explicit catalog data and CLI wiring so that supported frameworks, directions, shorthands, and public command names are no longer implicit in the JS runtime.

### 2. Detection and metadata

Port low-risk commands first:

- `terrain detect`
- `terrain list-conversions`
- `terrain shorthands`
- `terrain convert --plan`

These commands are useful before full execution exists and give downstream surfaces a stable Go-native contract.

### 3. Conversion runtime

Build the Go-native conversion engine under `internal/convert` in slices:

- direction registry
- parser / rewrite helpers by language
- file-level conversion APIs
- batch execution and output planning

Implementation guidance for this layer:

- use Tree-sitter / AST-backed parsing for structure-sensitive rewrites where correctness or predictable performance matter
- keep regex/string replacement as a fallback or for narrow expression-level substitutions, not as the default whole-file strategy
- verify new parser-backed conversions against the legacy JS runtime floor with the converter benchmark harness

Recommended first execution directions:

- `jest -> vitest`
- `cypress -> playwright`

These are high-value and already familiar from the legacy pipeline-backed paths.

### 4. Migration orchestration

After file conversion works, port the higher-level workflows:

- `migrate`
- `estimate`
- `status`
- `checklist`
- `reset`
- `doctor`
- `convert-config`

This is where resume, retry, dependency ordering, and post-conversion guidance should live.

### 5. Test migration

Replace JS-owned verification with Go-native tests in layers:

- registry / catalog tests
- CLI contract tests
- golden conversion tests
- workflow tests for migrate / status / doctor
- real-world fixture coverage for representative ecosystems

The legacy JS suite is no longer a required parity oracle for release verification.

### 6. Packaging and release simplification

Once the runtime surface is in Go:

- npm becomes a binary installer wrapper, not a product runtime
- Homebrew and GitHub Releases ship the same functional CLI
- release verification no longer needs the legacy JS converter lane

## Milestones

### Milestone 1: Contract foundation

Implemented in this slice:

- Go-native conversion catalog in `internal/convert`
- Go-native `terrain convert` planning surface
- Go-native `terrain list-conversions`
- Go-native `terrain shorthands`
- Go-native `terrain detect`

Still deferred after this milestone:

- migration orchestration
- validation runtime

### Milestone 2: First executable directions

Target outcomes:

- `terrain convert` writes output for at least one JavaScript unit path and one browser migration path
- first golden fixtures live in Go tests
- CLI output stays stable across npm/Homebrew/GitHub releases

Current status:

- `jest -> vitest` is now executable in the Go CLI for the high-confidence core surface
- `terrain convert-config` is now Go-native for the legacy config conversion directions
- `cypress -> playwright` is now executable in the Go CLI for the high-confidence browser migration surface
- `cypress -> selenium` is now executable in the Go CLI for the first direct Selenium migration surface from Cypress
- `cypress -> webdriverio` is now executable in the Go CLI for the first direct Cypress-to-WebdriverIO migration surface
- `jasmine -> jest` is now executable in the Go CLI for the Jasmine modernization surface
- `jest -> jasmine` is now executable in the Go CLI for the reverse Jasmine migration surface
- `jest -> mocha` is now executable in the Go CLI for the reverse Mocha migration surface
- `mocha -> jest` is now executable in the Go CLI for the Mocha modernization surface
- `playwright -> cypress` is now executable in the Go CLI for the reverse browser migration surface
- `playwright -> puppeteer` is now executable in the Go CLI for the reverse Puppeteer migration surface
- `playwright -> selenium` is now executable in the Go CLI for the reverse Selenium migration surface
- `playwright -> webdriverio` is now executable in the Go CLI for the first reverse WebdriverIO migration surface
- `puppeteer -> playwright` is now executable in the Go CLI for the first Puppeteer modernization surface
- `selenium -> cypress` is now executable in the Go CLI for the reverse Cypress migration surface from Selenium
- `selenium -> playwright` is now executable in the Go CLI for the Selenium modernization surface
- `junit4 -> junit5` is now executable in the Go CLI for the first Java modernization surface
- `junit5 -> testng` is now executable in the Go CLI for the first JUnit-to-TestNG migration surface
- `testng -> junit5` is now executable in the Go CLI for the reverse TestNG modernization surface
- `pytest -> unittest` is now executable in the Go CLI for the first pytest-to-class-based migration surface
- `unittest -> pytest` is now executable in the Go CLI for the primary Python modernization surface
- `nose2 -> pytest` is now executable in the Go CLI for the nose2 retirement surface
- `testcafe -> cypress` is now executable in the Go CLI for the first TestCafe migration surface into Cypress
- `testcafe -> playwright` is now executable in the Go CLI for the first TestCafe modernization surface
- `webdriverio -> cypress` is now executable in the Go CLI for the reverse Cypress migration surface
- `webdriverio -> playwright` is now executable in the Go CLI for the first WebdriverIO migration surface

Milestone result:

- every cataloged `terrain convert` direction is now executable with the Go-native runtime
- directory conversion now covers JavaScript, Java, and Python sources through the same execution path

### Milestone 3: Workflow parity

Target outcomes:

- `terrain migrate`, `estimate`, `status`, `checklist`, and `doctor` are Go-native
- release verification can stop depending on the legacy JS runtime for product behavior

Current status:

- `terrain estimate` is now Go-native and runs the conversion runtime in-memory to produce confidence bands, blockers, and effort estimates
- `terrain migrate` is now Go-native and adds project scanning, config conversion, state tracking, resume/retry support, and checklist generation on top of the direct converters
- `terrain status` is now Go-native and reads conversion workflow state from `.terrain/migration/`
- `terrain checklist` is now Go-native and renders the saved migration checklist from workflow state
- `terrain doctor` is now Go-native and validates path access, writable output, project config, test inventory, supported directions, and saved migration state
- `terrain reset` is now Go-native and clears only conversion migration state rather than deleting the full `.terrain/` directory

Milestone result:

- the end-to-end conversion workflow no longer depends on the legacy JS runtime for product behavior
- direct conversion and project-wide migration now share the same Go-native execution core

### Milestone 4: Legacy retirement

Completed outcomes:

- removed `src/*` legacy product runtime from the supported release path
- removed `bin/terrain.js` as a supported implementation
- reduced npm package responsibilities to install/bootstrap only
- moved CI and release verification onto the Go-native product runtime

## Risks

1. **Big-bang rewrite risk.** The JS converter is large and already covers many directions. Porting everything at once would hide regressions until late. We avoid this by keeping each milestone independently useful.

2. **Command drift risk.** If Go and JS expose different names or semantics during migration, the product gets harder to explain. The contract catalog exists specifically to reduce that drift.

3. **Test coverage illusion.** Porting runtime code without porting the test strategy would only move risk. The migration must include tests as a first-class workstream.

## Immediate Next Slice

Post-migration follow-up:

1. Keep improving conversion fidelity for advanced Java, Python, and browser patterns that still emit `TERRAIN-TODO`
2. Archive or trim legacy converter docs once they are no longer useful historical references
3. Keep the npm wrapper intentionally thin so the product runtime stays Go-native
4. Continue replacing regex-heavy JavaScript conversion paths with AST-backed rewrite passes, starting with the highest-traffic directions
