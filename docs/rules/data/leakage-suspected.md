# `terrain/data/leakage-suspected`

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

Source-level patterns associated with train/test contamination. Reported metrics may overstate generalization because the model effectively saw test data during training.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high
- **Stable since:** v0.2.0

## 3. What this catches

- A scaler / encoder fit on the full dataset BEFORE `train_test_split` (preprocessing leakage)
- `train_test_split` applied to time-series data (temporal leakage — random shuffling leaks future into past)
- Feature engineering pass over the full dataset before splitting

## 4. Why this matters

Data leakage is the silent producer of inflated metrics. A 0.95 accuracy that drops to 0.65 in production is almost always traced back to a `fit_transform` on the full dataset, or a random split on temporal data, or a target leak through derived features. The rule operates at the source level — it can't detect every leakage shape, but it surfaces the two most common ones with high enough confidence to gate.

Companion rule: `terrain/data/missing-train-test-split` fires when no split exists at all; this rule fires when a split exists but is used incorrectly.

## 5. Detection mechanism

- **Approach:** Python AST + textual heuristics.
- **Path filter:** only training-pathed files (`/train/`, `/training/`, `/models/`, `/notebooks/`, `/experiments/`, `/ml/`, `/pipelines/`).
- **Pattern: preprocessing leakage.** A `.fit_transform()` / `.fit()` call on a preprocessing class (`StandardScaler` / `MinMaxScaler` / `OneHotEncoder` / `TfidfVectorizer` / `SimpleImputer` / `PowerTransformer` / etc., or a variable named `scaler.` / `encoder.` / `vectorizer.` / `imputer.` / `transformer.`) appears BEFORE the first `train_test_split` call.
- **Pattern: temporal leakage.** The file has time-series indicators (`pd.to_datetime`, `DatetimeIndex`, `time_series`, `timestamp`, LSTM imports, `.resample(`, `freq=`) AND uses `train_test_split(` AND does NOT use `TimeSeriesSplit(` or `shuffle=False`.
- **Edge cases NOT handled at 0.2.0:** target leakage (a derived feature column directly mirrors the target), group leakage (rows from the same user split across train/test), nested cross-validation leakage.

## 6. Worked example

```
error[terrain/data/leakage-suspected]: preprocessing-leakage in training/model.py
  --> training/model.py
   = help: Move scaler/encoder fits to AFTER the split. fit on train, transform on test.
   = docs: https://terrain.dev/rules/data/leakage-suspected
```

**Before:**

```python
scaler = StandardScaler()
X_scaled = scaler.fit_transform(X)
X_train, X_test, _, _ = train_test_split(X_scaled, y, test_size=0.2)
```

**After:**

```python
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)
scaler = StandardScaler()
X_train_scaled = scaler.fit_transform(X_train)
X_test_scaled = scaler.transform(X_test)
```

## 7. Configuration

```yaml
rules:
  data/leakage-suspected: warning
ignore:
  rules:
    data/leakage-suspected:
      - "experiments/baseline/**"
```

## 8. False-positive characterization

- **Manual two-step preprocessing** — the rule sees `fit_transform` before `train_test_split` but the adopter's intent was to apply a learned encoder from a separate train file. Mitigation: split first, then transform; OR ignore via path.
- **Notebook files** — the rule path filter includes `/notebooks/` so exploratory notebooks fire. Mitigation: ignore `notebooks/` if the team's policy is "rigor lives in production code, not notebooks."

## 9. Reproducibility

```bash
terrain test --selector data/leakage-suspected
```

## 10. Stability commitment

Rule ID, severity, and the two recognized leakage patterns (preprocessing, temporal) are stable from v0.2.0. New patterns are additive.

## 11. Related rules

- `terrain/data/missing-train-test-split` — sister rule for the no-split-at-all case
- `terrain/reproducibility/no-seed` — adjacent: stochastic evals without seed
- `terrain/regression/performance-regression` — fires when metrics regress; leakage usually masks regression by inflating both runs
