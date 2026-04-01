# Telemetry

Terrain includes opt-in, local-only telemetry. It is **disabled by default**.

## What is collected

When enabled, each command invocation appends one JSON line to `~/.terrain/telemetry.jsonl`:

| Field | Example | Purpose |
|-------|---------|---------|
| `ts` | `2026-03-31T12:00:00Z` | When the command ran |
| `version` | `3.1.0` | Terrain version |
| `command` | `analyze` | Which command was run |
| `sizeBand` | `medium` | Test file count band (small/medium/large) |
| `languages` | `["js","go"]` | Detected languages |
| `signals` | `12` | Number of signals detected |
| `durationMs` | `1450` | Execution time |

## What is NOT collected

- File paths, file names, or directory structure
- Repository URLs, git remotes, or branch names
- User names, emails, or any personally identifiable information
- Signal details, finding text, or report content
- Source code or test content

## Where data goes

Nowhere. Events are written to a local file (`~/.terrain/telemetry.jsonl`) on your machine. Terrain does not phone home, does not send data to any server, and does not make any network requests for telemetry.

The local file exists so you (or your team) can optionally analyze usage patterns. You can delete it at any time.

## How to enable/disable

```bash
# Check current status
terrain telemetry

# Enable
terrain telemetry --on

# Disable
terrain telemetry --off
```

Or set the environment variable (overrides file config):

```bash
export TERRAIN_TELEMETRY=on   # enable
export TERRAIN_TELEMETRY=off  # disable
```
