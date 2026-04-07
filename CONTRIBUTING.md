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
cmd/terrain/          CLI entry point (30+ commands)
cmd/terrain-bench/    Benchmark harness
internal/             47 Go packages (83k lines)
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

# Test all Go packages (48 packages)
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
