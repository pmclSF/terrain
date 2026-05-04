# Getting Started with Terrain

## Prerequisites

### Node 22 (npm path only)

The npm install path requires **Node 22 or later**. The postinstall
script uses `fetch`, top-level await, and modern stream primitives
that landed in Node 22 — earlier versions fail at install time, not
run time.

```bash
node --version    # expect v22.x or higher
```

If your CI image is pinned to Node 20 LTS, two recommended
alternatives keep working without a Node bump:

```bash
# Homebrew (macOS / Linux)
brew install pmclSF/terrain/mapterrain

# Go install (any platform with Go 1.23+)
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

Node-20 compat for the npm path is on the 0.3 roadmap.

### Cosign (npm path only)

The npm install path verifies signed binaries with cosign before
extracting them. Cosign needs to be on `PATH` before you run `npm
install`:

```bash
# macOS / Linux
brew install cosign
# Linux (Debian/Ubuntu)
apt-get install cosign     # 22.04+; otherwise use the Sigstore release
# Windows
scoop install cosign
```

If you can't or don't want to install cosign, two opt-out env vars
are recognized by the npm installer:

| Env var | Effect |
|---------|--------|
| `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` | Falls back to checksum-only verification |
| `TERRAIN_INSTALLER_SKIP_VERIFY=1` | Skips verification entirely (not recommended) |

Homebrew and `go install` paths handle their own verification and
do not need cosign on `PATH`.

See [`docs/release/supply-chain.md`](../release/supply-chain.md) for
the full signing / attestation story.

## Install

```bash
brew install pmclSF/terrain/mapterrain
# or
npm install -g mapterrain          # see Prerequisites above re: cosign
# or
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

## First run

Navigate to any repository with tests and run:

```bash
terrain analyze
```

Terrain will discover test files, detect frameworks, emit signals, compute risk surfaces, and produce a posture assessment — all from static analysis.

## Understanding the output

The analyze report shows:

- **Repository** — languages, frameworks, CI systems detected
- **Frameworks** — which test frameworks and how many files each
- **Posture** — five dimensions (health, coverage depth, coverage diversity, structural risk, operational risk) rated strong/moderate/weak
- **Signals** — categorized findings: health, quality, migration, governance
- **Risk** — where risk concentrates by directory or owner

## Next commands

After `analyze`, try:

```bash
terrain summary     # leadership-ready overview
terrain posture     # detailed posture with evidence per measurement
terrain portfolio   # see which tests provide the most value and which waste resources
terrain metrics     # aggregate scorecard
```

## Saving snapshots for trend tracking

```bash
terrain analyze --write-snapshot
```

This saves the snapshot to `.terrain/snapshots/`. After multiple snapshots, compare them:

```bash
terrain compare
```

## JSON output

All commands support `--json` for machine-readable output:

```bash
terrain analyze --json
terrain summary --json
terrain posture --json
```

## Policy enforcement

Create `.terrain/policy.yaml` to define rules, then:

```bash
terrain policy check
```

Returns exit code 2 if violations are found — useful in CI gates.
