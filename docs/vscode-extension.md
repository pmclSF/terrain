# VS Code / Cursor Extension

## Purpose

The extension makes Terrain intelligence explorable inside the editor.

It is not a separate engine.

The extension should remain a thin client over CLI JSON output.

## Current architecture

Flow:
1. extension invokes `terrain analyze --json`
2. extension invokes `terrain insights --json`
3. extension invokes `terrain migration readiness --json`
4. extension renders sidebar views from that report bundle

## Sidebar views

### Overview
Repository-level summary:
- framework inventory
- validation counts
- headline summary
- top findings
- risk posture

### Health
Reliability-oriented findings from `terrain insights`, including skipped-test
burden and runtime-readiness guidance when runtime artifacts are absent.

### Quality
Coverage and architecture findings from `terrain insights`, such as structural
coverage debt and high-fanout architecture pressure.

### Migration
- migration readiness level and explanation
- blocker groups by type
- area assessments (safe/caution/risky)
- framework summary
- coverage guidance for risky areas

### Review
Grouped triage for:
- prioritized findings by category
- prioritized findings by severity
- prioritized findings by directory/scope

## Current interactions

- sidebar views fed by `terrain analyze --json`, `terrain insights --json`, and
  `terrain migration readiness --json`
- refresh command to rerun analysis on demand
- optional auto-refresh on relevant file changes
- terminal shortcuts for summary and migration blockers
- file reveal from findings

Future editor enrichments like diagnostics, hovers, and diff previews can build
on the same CLI report contracts without moving business logic into the
extension.

## Implementation

Source files under `extension/vscode/src/`:

| File | Purpose |
|------|---------|
| `extension.ts` | Entry point: TreeDataProviders, commands, CLI integration, state management |
| `types.ts` | TypeScript types aligned with analyze/insights/migration JSON contracts |
| `signal_renderer.ts` | Grouping/filtering helpers for report findings and severity/risk display |
| `views.ts` | View data builders for the report bundle consumed by the extension |

### Commands

| Command | Description |
|---------|-------------|
| `terrain.refresh` | Re-run analysis and refresh all views |
| `terrain.openSummary` | Open executive summary in terminal |
| `terrain.openMigrationBlockers` | Open migration blockers in terminal |
| `terrain.revealFile` | Open file associated with a finding |

### States

All views handle empty, loading, error, and loaded states gracefully.

## Rules

- business logic stays in the CLI/core
- avoid duplicating detector logic
- prefer tree views and standard UI surfaces
- webviews should be minimal or deferred
