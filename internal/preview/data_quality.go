package preview

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// DetectTargetLeakage fires when a Python training file derives a
// feature column from the target column. Implements
// terrain/data-quality/target-leakage.
//
// The detection is heuristic: looking for `X["target_*"] = y` patterns
// or feature columns whose name strongly mirrors the target.
func DetectTargetLeakage(sourceFiles map[string][]byte) []models.Signal {
	var out []models.Signal
	for path, content := range sourceFiles {
		s := string(content)
		if !looksLikeTrainingFile(path) {
			continue
		}
		// Find target variable assignments: y = ..., target = ...
		hasTargetIdent := strings.Contains(s, "y =") || strings.Contains(s, "target =") ||
			strings.Contains(s, "y_train") || strings.Contains(s, "y_test")
		if !hasTargetIdent {
			continue
		}
		// Pattern: X["xxxxx"] = y.<something> OR X["target_xxx"] = ...
		if containsTargetDerivedFeature(s) {
			out = append(out, signal(
				signals.SignalTargetLeakage, models.SeverityHigh,
				"terrain/data-quality/target-leakage",
				"docs/rules/data-quality/target-leakage.md",
				models.SignalLocation{File: path},
				"Feature column derived from the target column.",
				"Compute features only from non-target inputs. A feature that mirrors the target leaks the answer into training.",
				map[string]any{},
			))
		}
	}
	return out
}

func looksLikeTrainingFile(path string) bool {
	lower := strings.ToLower(path)
	lower = strings.ReplaceAll(lower, "\\", "/")
	if !strings.HasPrefix(lower, "/") {
		lower = "/" + lower
	}
	for _, m := range []string{"/train/", "/training/", "/models/", "/notebooks/", "/experiments/", "/ml/", "/pipelines/"} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func containsTargetDerivedFeature(s string) bool {
	// Heuristic patterns.
	patterns := []string{
		`= y.shift`,
		`= y_train`,
		`= y_test`,
		`"target_`,
		`'target_`,
		`["y"]`,
		`['y']`,
		`y.rolling`,
		`y.expanding`,
	}
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// DetectDuplicateEvalRows fires when an eval data file has >5% duplicate
// input rows. Implements terrain/data-quality/duplicate-rows.
func DetectDuplicateEvalRows(evalFiles []string, threshold float64) []models.Signal {
	if threshold <= 0 {
		threshold = 0.05
	}
	var out []models.Signal
	for _, path := range evalFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		dupRate, total := lineDuplicateRate(data)
		if total < 10 || dupRate < threshold {
			continue
		}
		out = append(out, signal(
			signals.SignalDuplicateEvalRows, models.SeverityMedium,
			"terrain/data-quality/duplicate-rows",
			"docs/rules/data-quality/duplicate-rows.md",
			models.SignalLocation{File: path},
			fmt.Sprintf("Eval dataset has %.1f%% duplicate input rows (%d rows total).", dupRate*100, total),
			"Dedupe the eval dataset. Duplicate rows inflate coverage without adding distinct test cases and can mask regressions on edge cases.",
			map[string]any{"duplicate_rate": dupRate, "total_rows": total},
		))
	}
	return out
}

// lineDuplicateRate computes the collision rate among non-empty lines
// in data, using SHA-1 of the canonicalized line as the collision key.
func lineDuplicateRate(data []byte) (float64, int) {
	seen := map[string]int{}
	total := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		total++
		h := sha1.Sum(line)
		key := hex.EncodeToString(h[:])
		seen[key]++
	}
	if total == 0 {
		return 0, 0
	}
	dupes := 0
	for _, count := range seen {
		if count > 1 {
			dupes += count - 1
		}
	}
	return float64(dupes) / float64(total), total
}

// DetectSchemaDrift fires when the pipeline output's column set differs
// from the baseline. Implements terrain/data-quality/schema-drift.
//
// At 0.2.0 this is a stub that consumes pre-computed column sets from
// the baseline / current artifacts; the runtime hook lands when the
// pipelines detection package surfaces them.
func DetectSchemaDrift(baselineColumns, currentColumns map[string][]string) []models.Signal {
	var out []models.Signal
	for table, baseCols := range baselineColumns {
		curCols, ok := currentColumns[table]
		if !ok {
			out = append(out, signal(
				signals.SignalSchemaDrift, models.SeverityHigh,
				"terrain/data-quality/schema-drift",
				"docs/rules/data-quality/schema-drift.md",
				models.SignalLocation{File: table},
				"Table "+table+" missing in current run.",
				"Confirm the pipeline that produces this table still runs; if intentionally retired, update the baseline.",
				map[string]any{"table": table},
			))
			continue
		}
		added, removed := diffStringSets(baseCols, curCols)
		if len(added) == 0 && len(removed) == 0 {
			continue
		}
		out = append(out, signal(
			signals.SignalSchemaDrift, models.SeverityHigh,
			"terrain/data-quality/schema-drift",
			"docs/rules/data-quality/schema-drift.md",
			models.SignalLocation{File: table},
			fmt.Sprintf("Table %s schema drifted: +%d -%d columns.", table, len(added), len(removed)),
			"Reconcile the schema change with downstream consumers. If intentional, refresh the baseline.",
			map[string]any{"table": table, "added": added, "removed": removed},
		))
	}
	return out
}

func diffStringSets(base, cur []string) (added, removed []string) {
	baseSet := map[string]bool{}
	for _, b := range base {
		baseSet[b] = true
	}
	curSet := map[string]bool{}
	for _, c := range cur {
		curSet[c] = true
	}
	for _, c := range cur {
		if !baseSet[c] {
			added = append(added, c)
		}
	}
	for _, b := range base {
		if !curSet[b] {
			removed = append(removed, b)
		}
	}
	return added, removed
}
