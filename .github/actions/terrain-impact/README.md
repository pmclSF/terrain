# Terrain Impact Analysis

Run [Terrain](https://github.com/pmclSF/terrain) impact analysis on pull requests. Identifies impacted tests, computes change risk, and posts a PR comment with findings.

## Quick Start

```yaml
- uses: pmclSF/terrain/.github/actions/terrain-impact@main
  with:
    post-comment: 'true'
```

## What It Does

1. Builds the Terrain CLI from source
2. Runs `terrain pr --json` against the PR diff
3. Parses impact results (impacted tests, risk level, protection gaps)
4. Posts or updates a PR comment with findings
5. Uploads `terrain-impact.json` as a build artifact

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `go-version-file` | Path to go.mod for Go version detection | No | `go.mod` |
| `base-sha` | Base commit SHA for impact diff | No | Auto-detected from PR |
| `head-sha` | Head commit SHA for impact diff | No | Auto-detected from PR |
| `comment-header` | HTML comment used to find/update existing PR comment | No | `<!-- terrain-impact -->` |
| `post-comment` | Whether to post/update a PR comment | No | `true` |

## Outputs

| Output | Description |
|--------|-------------|
| `artifact-path` | Path to the generated `terrain-impact.json` artifact |
| `test-count` | Number of impacted tests |
| `risk-level` | Overall PR risk level: `high`, `medium`, `low`, or `none` |

## Full Example

```yaml
name: Terrain PR Analysis
on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  pull-requests: write

jobs:
  terrain:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Required for git diff

      - uses: pmclSF/terrain/.github/actions/terrain-impact@main
        id: terrain
        with:
          post-comment: 'true'

      - name: Use risk level in workflow
        run: echo "Risk level is ${{ steps.terrain.outputs.risk-level }}"
```

## Requirements

- Repository must be checked out with `fetch-depth: 0` (full history needed for diff)
- `GITHUB_TOKEN` must have `pull-requests: write` permission for PR comments
- Go version is auto-detected from `go.mod`

## How It Works

Terrain analyzes your repository's test structure, import graph, and code surfaces to determine which tests are impacted by the PR's changes. It computes a risk posture based on protection gaps (untested exports, weak coverage) and recommends a targeted test set.

The PR comment includes:
- Changed file count and impacted unit count
- Protection gaps with severity ratings
- Recommended tests to run (with package grouping)
- Pre-existing issues on changed files
- Limitations when data is incomplete
