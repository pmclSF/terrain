# Contributing to Terrain

## Quick Start

```bash
git clone https://github.com/pmclSF/terrain.git
cd terrain
go build -o terrain ./cmd/terrain
go test ./cmd/... ./internal/...
./terrain analyze
```

## Project Structure

```
cmd/terrain/          CLI entry point (10 canonical commands + legacy aliases)
cmd/terrain-bench/    Benchmark harness
internal/             49 Go packages
├── analysis/        Repository scanning and code surface inference
├── convert/         Go-native test conversion (25 directions)
├── depgraph/        Dependency graph with 5 reasoning engines
├── engine/          Pipeline orchestration
├── impact/          Change-scope analysis
├── insights/        Prioritized health report
├── measurement/     5 posture dimensions, 18 measurements
├── reporting/       14 report renderers
└── ...              See README.md for full package list
```

## Development Workflow

```bash
# Build
go build -o terrain ./cmd/terrain

# Test all Go packages
go test ./cmd/... ./internal/...

# Verify formatting
gofmt -l cmd/ internal/

# Run vet
go vet ./...

# Full release verification (Go + npm + VS Code extension)
make release-verify
```

## Adding a New Command

1. Create `cmd/terrain/cmd_<name>.go` with the handler function
2. Add the case to the switch statement in `cmd/terrain/main.go`
3. Add usage text to `printUsage()` in `main.go`
4. Add the command to the CLI spec in `docs/cli-spec.md`
5. Add smoke tests in `cmd/terrain/cli_smoke_test.go`

### Command conventions

- All analysis commands use `defaultPipelineOptionsWithProgress(jsonOutput)` for progress reporting
- JSON output uses `json.NewEncoder(os.Stdout)` with `SetIndent("", "  ")`
- Nil slices should be converted to empty slices before JSON encoding (serialize as `[]` not `null`)
- Error messages go to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Exit codes: 0 = success, 1 = error, 2 = usage error / policy violation
- Commands with positional args use `reorderCLIArgs()` to support flags in any position

## Adding a New Conversion Direction

Extend the conversion runtime under `internal/convert`:

1. Add or update the direction entry in `internal/convert/catalog.go`
2. Implement the source conversion function in `internal/convert/*.go`
3. Wire directory execution in `internal/convert/execute.go`
4. Add config conversion support in `internal/convert/config.go` when needed
5. Add tests alongside the implementation

## Code Style

- Product logic lives in Go under `cmd/` and `internal/`
- Keep the npm wrapper thin; `bin/*.js` and `scripts/*.js` are packaging helpers
- All files must pass `gofmt`
- All packages must pass `go vet`
- Prefer deterministic golden and contract tests over broad integration drift

## Testing Conventions

- Go tests live next to the package they exercise as `*_test.go`
- Prefer table-driven tests for catalog, parser, and command-contract coverage
- Use inline fixtures for focused conversion cases; use `tests/fixtures/` only when repo shape matters
- Test file write helpers must check errors: `if err := os.WriteFile(...); err != nil { t.Fatal(err) }`
- Use `t.TempDir()` for temporary directories, not hardcoded `/tmp` paths
- Keep `go test ./cmd/... ./internal/...` green for product changes

## Commit Messages

```
type(scope): description

# Types: feat, fix, test, docs, refactor, chore, perf, style, ci, build
```

## Architecture

```
Repository scan → Signal detection → Risk scoring → Snapshot → Reporting
```

See [DESIGN.md](DESIGN.md) for the full architecture overview, [docs/architecture/](docs/architecture/) for detailed design documents, and the [CLI spec](docs/cli-spec.md) for the complete command reference.

## Parity gate (lifting maturity uniformly)

Terrain enforces a **parity gate** so no functional area drifts behind
the others. The gate measures every shipping area against a 17-axis
rubric (7 product / 7 engineering / 3 UI/visual). Per-pillar floors
apply:

| Pillar | Floor | Block release? |
|--------|-------|----------------|
| Gate | every cell ≥ 4 | yes |
| Understand | every cell ≥ 3 | yes |
| Align | every cell ≥ 3 | soft (warn-only) |

### How to lift a cell in your PR

1. Find the cell you're improving in `docs/release/parity/scores.yaml`.
2. Update the score (1–5) and replace the evidence line with a
   one-line pointer to the change you're making (file:line, test
   name, or short rationale).
3. If your change touches the audit doc's narrative,
   `docs/release/0.2.x-maturity-audit.md` updates in the same PR.
4. Run `make pillar-parity` locally — your change should move at
   least one cell; CI will compare the diff.

### Source-of-truth split

- **Structural** rubric (areas, axes, level definitions, floors,
  uniformity gates): `docs/release/parity/rubric.yaml`. Changes
  rarely; anything that moves cells around or redefines what "3"
  means lives here.
- **Per-cell scores**: `docs/release/parity/scores.yaml`. Changes
  every parity-lift PR. The shape is `area_id → axis_id → {score,
  evidence}`.
- **Human-readable companion**: `docs/release/0.2.x-maturity-audit.md`.
  Same data, prose form. Update both together.

### Local commands

```bash
make pillar-parity            # full matrix + per-pillar verdict
make pillar-parity-floor      # compact: just the floor map
make pillar-parity-json       # JSON for tooling
```

Exit codes: `0` if every hard-gate pillar is at or above its floor
(soft warns are OK), `1` if any hard-gate pillar is below its floor,
`2` for usage errors (missing files, malformed YAML).

### Uniformity gates

In addition to per-cell floors, the rubric defines seven uniformity
gates that catch *unevenness* across detectors / frameworks /
commands / outputs (e.g. "every detector has the same eight required
fields", "every Tier-1 framework reaches the same axis floor"). These
are tracked as advisory in 0.2.0 and become hard gates in 0.2.x. See
the `uniformity_gates` block in `rubric.yaml`.
