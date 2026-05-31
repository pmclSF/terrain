# terrain/data/missing-train-test-split — Missing Train/Test Split

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `missingTrainTestSplit`  
**Domain:** ai  
**Default severity:** high  
**Lifecycle status:** stable  
**Gating tier:** gate

## Summary

A training call (.fit() / .train() / .partial_fit()) appears in a training file without any preceding split helper (train_test_split, KFold, TimeSeriesSplit, cross_val_score, etc.). The model is fit on the full dataset; evaluation against the same data measures memorization, not generalization.

## Remediation

Split the dataset before training (sklearn.model_selection.train_test_split, KFold for general use, TimeSeriesSplit for temporal data).

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.75–0.90.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

A training call (`.fit()` / `.train()` / `.partial_fit()`) appears in a training file without any preceding train/test split or cross-validation helper. The model is fit on the full dataset; evaluation against the same data measures memorization, not generalization.

## 2. Severity & status

- **Tier:** stable
- **Default severity:** high
- **Stable since:** v0.2.0
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- `clf.fit(X, y)` in `training/model.py` with no preceding `train_test_split` import or call
- A research notebook that runs `model.fit(full_data, full_labels)` and reports accuracy on the same data
- An XGBoost training script that doesn't use any of `KFold` / `StratifiedKFold` / `cross_val_score` / `GridSearchCV`
- A TimeSeries forecasting file that fits on the full series with no `TimeSeriesSplit`

## 4. Why this matters

The single most common cause of inflated model metrics is training-on-test contamination — typically because no split exists at all. The model memorizes the training set and reports near-perfect numbers; production accuracy then drops to chance. The rule is a structural safeguard: any training file that has `.fit()` calls must demonstrate (somewhere) that it understands the train/test boundary. Either a `train_test_split` call, a k-fold helper, or `cross_val_score` qualifies.

The rule is a pre-merge guard. It doesn't measure leakage at runtime (that's `terrain/data/leakage-suspected`'s job); it measures whether the structural discipline is in place at the source level.

## 5. Detection mechanism

- **Approach:** file-level scan + AST walk. The file is considered when its path includes a training-shaped segment (`/train/`, `/training/`, `/models/`, `/notebooks/`, `/experiments/`, `/ml/`, `/pipelines/`). Within the file we look for any of the split-vocabulary primitives; if any are present, the rule is suppressed. Otherwise, the first `.fit()` / `.train()` / `.partial_fit()` / `.fit_transform()` / `.fit_one_cycle()` call fires the signal.
- **Split-vocabulary recognized:** `train_test_split`, `StratifiedKFold`, `KFold`, `GroupKFold`, `TimeSeriesSplit`, `StratifiedShuffleSplit`, `ShuffleSplit`, `LeaveOneOut`, `LeavePOut`, `.cv_results_`, `cross_val_score`, `cross_validate`, `GridSearchCV`, `RandomizedSearchCV`.
- **Edge cases handled:** notebook files (`.ipynb` exported to `.py`); files using cross_val_score without a manual split (suppressed correctly).
- **Edge cases NOT handled at 0.2.0:** holdout sets computed manually (e.g., `X_train = X[:8000]`) without using any sklearn helper. The rule flags this as a false positive; mitigation is to use `train_test_split` or accept and ignore.

## 6. Worked example

```
error[terrain/data/missing-train-test-split]: training call without preceding split helper
  --> training/model.py:5
   = help: Split the dataset before training (sklearn.model_selection.train_test_split, KFold, or TimeSeriesSplit for temporal data).
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/data/missing-train-test-split
```

**Before:**

```python
from sklearn.ensemble import RandomForestClassifier

def train(X, y):
    clf = RandomForestClassifier()
    clf.fit(X, y)
    return clf
```

**After:**

```python
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split

def train(X, y):
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)
    clf = RandomForestClassifier()
    clf.fit(X_train, y_train)
    return clf
```

## 7. Configuration

```yaml
rules:
  data/missing-train-test-split: warning
ignore:
  rules:
    data/missing-train-test-split:
      - "notebooks/exploratory/**"
```

## 8. False-positive characterization

- **Manual slicing** (`X[:8000]` for train, `X[8000:]` for test) — not recognized. Mitigation: use `train_test_split` or accept.
- **Cross-language pipelines** where the split happens upstream (in a separate Python file) — the rule fires on the file with `.fit()` regardless of whether a sibling file splits. Mitigation: include a no-op `from sklearn.model_selection import train_test_split` for the rule's benefit (ugly), or split in the training file itself, or ignore.
- **Measured FP rate at last validation:** see the per-rule readiness card.

## 9. Reproducibility

```bash
terrain test --selector data/missing-train-test-split
```

## 10. Stability commitment

Rule ID, severity, path conventions, and split / fit vocabulary are stable from v0.2.0.

## 11. Related rules

- `terrain/data/leakage-suspected` — runtime-level detection of train/test contamination
- `terrain/reproducibility/no-seed` — sister discipline rule (seed before training)
- `terrain/hygiene/eval-no-assertion` — same family of "rigor missing" structural patterns
