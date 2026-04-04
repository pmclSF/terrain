# Contributing to Terrain

## Quick Start (Go Engine)

```bash
git clone https://github.com/pmclSF/terrain.git
cd terrain
make build
make test
./terrain analyze
```

## Quick Start (Conversion Workflow)

Test framework migration is now part of the main Go CLI:

```bash
go build -o terrain ./cmd/terrain
./terrain list-conversions
./terrain convert tests/ --from jest --to vitest -o converted/
```

## Adding a New Conversion Direction

### 1. Add runtime support in Go

Extend the conversion runtime under `internal/convert`:

- add or update the direction entry in `internal/convert/catalog.go`
- implement the source conversion function in `internal/convert/*.go`
- wire directory execution in `internal/convert/execute.go`
- add config conversion support in `internal/convert/config.go` when needed

### 2. Wire the CLI

Update `cmd/terrain` when the public contract changes:

- `cmd/terrain/cmd_convert.go`
- `cmd/terrain/cmd_convert_config.go`
- `cmd/terrain/cmd_workflow.go`
- `cmd/terrain/main.go`

## Code Style

- Product logic lives in Go under `cmd/` and `internal/`
- Keep the npm wrapper thin; `bin/*.js` and `scripts/*.js` are packaging helpers, not product runtime
- Prefer deterministic golden and contract tests over broad integration drift

## Testing Conventions

- Test file naming: `ClassName.test.js` in matching `test/` subdirectory
- Use `beforeEach` for fresh instances, never share mutable state
- Jest `expect()` assertions only
- Async tests use `async/await`
- Every new public class/function must have test coverage

## Commit Messages

```
type(scope): description

# Types: feat, fix, test, docs, refactor, chore, perf, style, ci, build
```

## Architecture

### Go Engine (current product direction)

```
Repository scan → Signal detection → Risk scoring → Snapshot → Reporting
```

See [DESIGN.md](DESIGN.md) for the full architecture overview and [docs/architecture.md](docs/architecture.md) for the layered design.

### Go-native conversion runtime

```
Source File/Project → internal/convert runtime → Migration state/checklist → Target Code
```

See [docs/architecture/27-go-native-conversion-migration.md](docs/architecture/27-go-native-conversion-migration.md) for the migration plan and end state.
