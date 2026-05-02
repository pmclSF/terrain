# Release Checklist

Pre-tag checklist for cutting a Terrain release. Run from a clean
checkout of the release branch. All boxes must be checked before tag.

## Build & test gates
- [ ] `make build` (or `go build ./cmd/terrain/`) succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./...` all pass
- [ ] `make calibrate` (calibration corpus regression gate) green
- [ ] `make docs-verify` zero-diff (manifest, severity rubric, rule docs in sync)
- [ ] `make bench-gate` perf regression gate green
- [ ] `make release-verify` (Go + npm + VS Code extension end-to-end) passes

## CLI surface — canonical commands
Run each canonical command on a real repo and confirm it doesn't
panic, produces useful output, and lists "Next steps" where the
command's design promises them:

- [ ] `terrain analyze`
- [ ] `terrain analyze --json` (valid JSON, stable shape)
- [ ] `terrain analyze --write-snapshot` (writes `.terrain/snapshots/latest.json` + timestamped archive)
- [ ] `terrain report summary`, `terrain report insights`, `terrain report explain <target>`, `terrain report posture`, `terrain report portfolio`, `terrain report metrics`, `terrain report focus`, `terrain report show <kind> <id>`, `terrain report pr`
- [ ] `terrain migrate run <pair>`, `terrain migrate config <pair>`, `terrain migrate readiness`, `terrain migrate list`
- [ ] `terrain convert <file> --from <fw> --to <fw>`
- [ ] `terrain posture`
- [ ] `terrain doctor` exits 0 on a clean repo
- [ ] `terrain ai list`, `terrain ai run` (with eval data)
- [ ] `terrain serve` launches and serves /api/health on 127.0.0.1
- [ ] `terrain version --json` includes version, commit, date, schemaVersion

## CLI surface — legacy aliases
Smoke-check that every legacy command still works in this release
(removal targets 0.3) and emits the deprecation hint when
`TERRAIN_LEGACY_HINT=1`:

- [ ] `terrain summary`, `terrain insights`, `terrain explain`, `terrain compare`, `terrain impact`, `terrain show`, `terrain pr`, `terrain export benchmark`, `terrain init`
- [ ] Legacy `terrain migrate <dir> --from --to` and `terrain convert <file> --from --to`

## Determinism
- [ ] `SOURCE_DATE_EPOCH=1700000000 terrain analyze --json` produces byte-identical output across two invocations
- [ ] `terrain analyze --write-snapshot` twice in succession yields identical content for the same SHA
- [ ] `make test-determinism` (if present) green

## Snapshot schema
- [ ] `terrain analyze --json | jq .meta.schemaVersion` matches `models.SnapshotSchemaVersion`
- [ ] A 0.1.x snapshot loads correctly via `terrain compare --baseline <old.json>` (schema migration verified)

## Policy
- [ ] `terrain policy check` exits 0 when no policy file present
- [ ] `terrain policy check` against a violating policy exits 2

## Supply chain
- [ ] `make sbom` produces both CycloneDX and SPDX
- [ ] `make release-dry-run` (`goreleaser release --snapshot --clean --skip=publish,sign`) succeeds
- [ ] `package.json` `version` matches the release tag
- [ ] `extension/vscode/package.json` `version` matches the release tag
- [ ] `CHANGELOG.md` has an `## [<version>]` heading dated correctly
- [ ] `docs/release/<version>.md` exists for major/minor releases
- [ ] `docs/release/feature-status.md` reflects this release

## Output quality
- [ ] No command dumps raw data without summary
- [ ] All human output is scannable (no walls of text)
- [ ] No ANSI codes in JSON output
- [ ] Errors are specific and actionable
- [ ] Exit codes follow the documented convention (0/1/2/4/5)

## Documentation
- [ ] `README.md` reflects current commands and current canonical surface
- [ ] `docs/release/feature-status.md` rewritten if any detector promoted/demoted
- [ ] `docs/release/<version>-known-gaps.md` lists honest carryovers
- [ ] No doc references stale flag names, version strings, or removed commands
