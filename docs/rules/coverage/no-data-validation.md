# `terrain/coverage/no-data-validation`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A data pipeline file (ETL / dbt model / training data loader) has no data-validation library import (Great Expectations, pandera, dbt-expectations, soda). Pipeline outputs may drift in schema or distribution without anyone noticing.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** medium
- **Stable since:** v0.2.0

## 3. What this catches

- A Python ETL file under `pipelines/` that reads from a source and writes to a warehouse with no schema check on the output
- A dbt project's `models/` SQL files paired with no `.yml` declaring tests / expectations
- A Spark / pandas DataFrame transformation with no shape / dtype assertion

## 4. Why this matters

Data validation is to data pipelines what assertions are to code: the only way a downstream consumer can trust the input is by knowing the upstream producer asserted on its output. Pipelines without validation drift silently — schema changes upstream propagate, distribution shifts go undetected, and ML models trained on the result regress weeks later for "no reason."

## 5. Detection mechanism

- **Approach:** path filter (pipeline-shaped paths) + import-presence check.
- **Recognized validation libraries:** Great Expectations (`great_expectations`, `ge`), pandera, dbt-expectations (via schema.yml), soda, pydantic (when used on DataFrame rows), data-diff.
- **Pipeline-path markers:** `/pipelines/`, `/etl/`, `/dbt/`, `/training/`, `/dags/` (Airflow), `/flows/` (Prefect).

## 6. Worked example

```
warning[terrain/coverage/no-data-validation]: pipeline file has no data-validation import
  --> pipelines/load_users.py
   = help: Add Great Expectations / pandera / dbt-expectations to validate pipeline output.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/coverage/no-data-validation
```

## 7. Configuration

```yaml
rules:
  coverage/no-data-validation:
    severity: low
ignore:
  rules:
    coverage/no-data-validation:
      - "pipelines/legacy/**"
```

## 9. Reproducibility

```bash
terrain test --selector coverage/no-data-validation
```

## 10. Stability commitment

Rule ID, severity, recognized-validation-library set, and pipeline-path markers are stable from v0.2.0.

## 11. Related rules

- `terrain/data/leakage-suspected` — adjacent: runtime detection of train/test contamination
- `terrain/data/missing-train-test-split` — sister discipline rule
