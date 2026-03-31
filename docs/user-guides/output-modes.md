# Output Modes

Terrain supports multiple output modes across all commands.

## Human-readable (default)

All commands produce formatted terminal output by default. This output is:
- Scannable with clear sections and headings
- Copy-paste friendly (no ANSI color codes)
- Concise — key findings first, details below
- Includes "next steps" hints

```bash
terrain analyze
terrain summary
terrain posture
terrain portfolio
```

## JSON (--json)

Machine-readable commands support `--json` for structured output:

```bash
terrain analyze --json     # AnalyzeReport object (schemaVersion 1)
terrain summary --json     # ExecutiveSummary object
terrain posture --json     # MeasurementSnapshot object
terrain metrics --json     # Metrics Snapshot
terrain portfolio --json   # PortfolioSnapshot object
terrain compare --json     # SnapshotComparison object
```

JSON output is stable and suitable for:
- CI pipeline integration
- Custom dashboards
- Programmatic analysis
- Diff/comparison tooling

`TestSuiteSnapshot` remains the internal engine snapshot and persisted
artifact format used for snapshot storage and comparison. `terrain analyze
--json` emits the user-facing analyze report, not the raw snapshot model.

## Privacy-safe export

```bash
terrain export benchmark
```

Produces a benchmark-safe JSON artifact with only:
- Aggregate counts and ratios
- Qualitative bands (strong/moderate/weak)
- Segmentation tags (language, framework, suite size)
- Posture bands per dimension
- Portfolio intelligence bands (redundancy, overbreadth, leverage)

No raw file paths, symbol names, or source code.
