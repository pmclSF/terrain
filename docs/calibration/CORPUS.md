# Calibration corpus

The calibration corpus is Terrain's ground truth for measuring detector
precision and recall. Each fixture is a small repository tree with a
`labels.yaml` declaring which signals the detector suite SHOULD fire and
which it should NOT fire (false-positive guards).

## Status

**0.2 ships the infrastructure.** The corpus today has ~1 fixture and
the integration gate runs in advisory mode (misses log warnings rather
than failing CI). Per `docs/release/0.2.md`:

- Target by 0.2 close: 50 labelled fixtures.
- Once the corpus reaches ~25 fixtures, flip the gate from advisory
  (`t.Logf`) to hard-fail (`t.Errorf`) in
  `internal/engine/calibration_integration_test.go`.
- Release gate: ≥ 90% precision per active detector.

## Layout

```
tests/calibration/
├── <fixture-name>/
│   ├── labels.yaml          ← ground truth
│   ├── package.json         ← (or pyproject.toml, go.mod, etc.)
│   ├── src/...              ← source under test
│   └── tests/...            ← test files
└── ...
```

## Adding a fixture

1. Create the directory under `tests/calibration/<fixture-name>/`.
2. Drop in real-world-shaped source + test files (small but realistic;
   ~5–20 files is the sweet spot).
3. Hand-label `labels.yaml`:

```yaml
schemaVersion: 1
fixture: my-fixture
description: |
  One-paragraph context: where this fixture comes from, what it
  exercises, why these particular labels.
expected:
  - type: weakAssertion
    file: src/auth/login.test.js
    notes: uses toBeTruthy on a string return value
  - type: flakyTest
    file: test/queue.test.js
    notes: PR #123 documented intermittent failures
expectedAbsent:
  - type: aiHardcodedAPIKey
    file: tests/fixtures/keys.js
    notes: placeholder string, not a real key
```

4. Run `make calibrate` and check the precision/recall numbers in
   `t.Logf` output.
5. Commit fixture + labels in the same PR.

## Matching

The runner matches on `(Type, File)` only. Line numbers and symbol
names from `labels.yaml` are advisory — they're shown in mismatch
reports but not used for matching. This trades some precision for
fixture maintainability: small edits don't break the labels.

## Outcomes

- **TP (true positive)** — detector fired, label expected it.
- **FP (false positive)** — detector fired, `expectedAbsent` flagged it.
- **FN (false negative)** — label expected, detector silent.
- **Out-of-scope** — detector fired, no label either way; silent. The
  corpus only measures what it claims; unclaimed signals neither help
  nor hurt the score.

## Per-detector metrics

`calibration.CorpusResult.PrecisionByType()` and `RecallByType()` skip
detectors with empty denominators, so an under-tested detector shows up
as "no precision yet" rather than 0.0. The 90% gate only applies to
detectors that have at least one TP+FP (precision) or TP+FN (recall) in
the corpus.

## Reproducibility

The runner is deterministic given the same fixture set. The engine's
analysis pipeline is locked behind the determinism gate (`make
test-determinism`), so calibration drift in CI is real drift, not flake.
