# Severity

Every Terrain finding carries a severity. Two label sets exist for two audiences:

## In the PR-comment surface and IDE diagnostics

Engineer-facing labels — what the engineer does about the finding:

| Label | What it means |
|---|---|
| **BLOCK** | Don't merge. Production reach. |
| **GATE** | Fix in this PR or write a dismiss reason. |
| **WATCH** | Visible, doesn't block; track. |
| **NOTE** | Informational; collapsed footer. |

## In the JSON output

CVSS-style labels — `Critical / High / Medium / Low / Info` — stable for SOC, audit, and CI-gate consumers. The mapping to the engineer labels is BLOCK→Critical, GATE→High, WATCH→Medium, NOTE→Low; Info is JSON-only.

The label vocabulary in JSON output is stable from 0.2.0 forward (one-cycle deprecation on changes per the [versioning contract](versioning.md)).

## Severity vs actionability

Severity describes the finding's class of risk. Actionability describes whether the engineer should act on this PR. A Critical-severity finding in a deprecated module may still be advisory; a Medium finding blocking a release may be immediate. The `actionability` field on each finding handles that axis separately.

## Configuring severity in your repo

Override severity per rule in `terrain.yaml`:

```yaml
rules:
  ai.train.missing_tracker:
    severity: WATCH  # default is GATE
```

The override is binding even when the project graduates the rule to a different default in a later release.
