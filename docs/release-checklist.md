# Release Checklist

Status checklist for Terrain release readiness.

## Product

- [x] Core commands work: analyze, summary, insights, explain, focus, portfolio, posture, metrics, impact, select-tests, pr, show, compare, policy check, export benchmark, init
- [x] Migration commands: readiness, blockers, preview (file + scope)
- [x] Debug/inspection commands: debug (graph, coverage, fanout, duplicates), depgraph
- [x] Human-readable output across user-facing commands and JSON output where documented
- [x] Executive summary with posture, trends, focus, benchmark readiness
- [x] Structured recommendations with what/why/where/evidence-strength
- [x] Blind spots / known-limitations section in summaries
- [x] Sample outputs updated in examples/sample-reports/
- [x] Demo walkthrough documented (docs/demo.md)

## Engineering

- [x] All Go packages build cleanly (`go build ./cmd/... ./internal/...`)
- [x] All Go tests pass (`go test ./cmd/... ./internal/...`)
- [x] 25 internal packages with test coverage
- [x] Analyze report contract (`terrain analyze --json`, schema v1) and persisted snapshot contract are documented distinctly
- [x] Evidence model: EvidenceStrength, EvidenceSource, Confidence on all signals
- [x] Registry-based detector architecture with 10 detectors (quality, migration, governance)
- [x] Runtime ingestion: JUnit XML, Jest JSON parsers
- [x] Coverage ingestion: LCOV, Istanbul JSON parsers with attribution
- [x] Extension report-bundle types and view builders

## UX

- [x] `terrain --help` output is accurate and readable
- [x] Each subcommand has clear flag descriptions
- [x] Summary command includes trend + benchmark readiness
- [x] Graceful degradation when no snapshot history exists
- [x] Graceful degradation when no policy file exists
- [x] Graceful degradation when no runtime/coverage artifacts provided
- [x] Extension TreeDataProvider implementations with empty/loading/error states

## Docs

- [x] README accurate with quick start, commands, architecture
- [x] docs/README.md provides navigation index with product evolution
- [x] docs/demo.md walkthrough complete (includes migration workflow)
- [x] docs/cli-spec.md covers all shipped commands
- [x] docs/roadmap.md milestones A through O documented
- [x] docs/architecture.md layered architecture documented
- [x] docs/engineering/detector-architecture.md documented
- [x] Contributor architecture map (docs/engineering/architecture-map.md)

## Packaging

- [x] `go install` path documented
- [x] Build-from-source instructions in README
- [x] Makefile with build, test, demo, install targets
- [x] goreleaser config for multi-platform binaries
- [x] Version command with build metadata (`terrain version`)
- [x] Checksum generation in release artifacts
- [ ] Homebrew formula (not yet created)

## Hardening

- [x] signal-first identity consistent across all top-level files
- [x] Legacy converter material clearly marked as historical
- [x] CLI help text aligned with actual behavior
- [x] Doc comment in main.go matches CLI surface
- [x] Test expectations updated for output changes
- [x] Empty states reviewed across all reports
- [x] Evidence strength reflected in report language

## Honest Gaps

These are intentionally not yet shipped:

- **Benchmark comparison**: Export model ready, but no hosted comparison service exists
- **Cross-repo aggregation**: Metrics model supports it, but aggregation is not implemented
- **Organization features**: No auth, accounts, or portfolio management
- **Hosted dashboard**: Not built — product is local-only
- **Homebrew formula**: Not yet created; install via `go install` or binary releases

## Stability

The user-facing analyze JSON contract is the structured `AnalyzeReport`
(schema versioned). The persisted engine snapshot (`TestSuiteSnapshot`) is a
separate internal/storage contract.

Core concepts are stable:
- Signal types and categories
- Risk bands (low, medium, high, critical)
- Analyze report structure (`terrain analyze --json`)
- Persisted snapshot structure
- Policy rule format
- Metrics aggregate model
- Benchmark export schema v1
- Evidence strength model

## Recommended Next Steps

1. Homebrew tap or formula
2. First public release communication
3. Runtime artifact ingestion for additional formats
4. Cross-repo aggregation foundation
