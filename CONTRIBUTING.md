# Contributing to Terrain

## Quick Start

```bash
git clone https://github.com/pmclSF/terrain.git
cd terrain
go build -o terrain ./cmd/terrain
go test ./cmd/... ./internal/...
./terrain analyze
```

## Project structure

```
cmd/terrain/        CLI entry point (14 canonical commands + legacy aliases)
cmd/internal/       Maintainer-only tooling (benchmarks, doc gen, release gates)
internal/           Core libraries — see DESIGN.md for the full package map
├── analysis/       Repository scanning + code-surface inference
├── convert/        Go-native test conversion (25 directions)
├── depgraph/       Typed dependency graph (21 node types, 18 edge types)
├── engine/         Pipeline orchestration
├── impact/         Change-scope analysis
├── injection/      Prompt-injection pattern library + test-input emitter
├── insights/       Prioritized health report
├── measurement/    Posture-band computation
├── plugin/         Third-party plugin manifest schema + validator
├── reporting/      Diagnostic format renderers
├── scaffold/       Mutation-test scaffold generator
└── signals/        Signal manifest (rule registry)
```

## Development Workflow

```bash
# Build
go build -o terrain ./cmd/terrain

# Test all Go packages
go test ./cmd/... ./internal/...

# Format (release-gate runs gofmt -l first and fails if any file needs reformatting)
gofmt -w .

# Run vet
go vet ./...

# Verify generated docs are in sync with the manifest
make docs-verify

# Full release verification: gofmt check → vet → tests → snapshot tests → regression gate
make release-gate

# Broader release verification (Go + npm + VS Code extension)
make release-verify
```

`make release-gate` is the single command CI runs to validate a PR. It exits 0 only when:

1. `gofmt -l .` produces no output (run `gofmt -w .` to fix).
2. `go vet ./cmd/... ./internal/...` is clean.
3. `go test ./cmd/... ./internal/...` passes.
4. The CLI snapshot tests (`-run TestSnapshot`) pass.
5. The regression-suite + recall-harness loaders validate every YAML file under `harness/`.

If `make release-gate` fails, fix the underlying issue rather than skipping the gate.

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
- Exit codes: 0 = success, 1 = error, 2 = usage error / policy violation, 4 = AI gate block, 5 = entity not found, 6 = severity-gate block (`--fail-on`). See `docs/cli-spec.md` for the full table.
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

See [DESIGN.md](DESIGN.md) for the full architecture and the [CLI spec](docs/cli-spec.md) for the complete command reference.

## Maintainer gates

Beyond `make release-gate` (which all PRs must pass), maintainers run uniformity gates that catch unevenness across detectors / frameworks / commands / outputs — e.g. "every detector has the same required fields", "every Tier-1 framework reaches the same coverage axis floor":

```bash
make pillar-parity            # full matrix + per-pillar verdict
make pillar-parity-floor      # compact floor map
make pillar-parity-json       # JSON for tooling
```

Exit codes: `0` every hard-gate pillar at or above floor (soft warns allowed); `1` any hard-gate pillar below floor; `2` usage error.
