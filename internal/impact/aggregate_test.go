package impact

import "testing"

// TestApplyPrivacySuppression_BelowThresholdZeroesCounts is the
// privacy contract: when fewer than PrivacyThreshold (=3) impacted
// units fall into the change, the protection / confidence breakdowns
// must be zeroed out so an aggregate report can't identify a specific
// team or change-set by triangulating a sparse cell. The aggregate
// keeps the totals and sets IsSparse=true so downstream renderers
// can show a "fewer than N" caption.
func TestApplyPrivacySuppression_BelowThresholdZeroesCounts(t *testing.T) {
	agg := &Aggregate{
		ImpactedUnitCount: 2, // below PrivacyThreshold=3
		ProtectionCounts:  map[string]int{"strong": 1, "weak": 1},
		ConfidenceCounts:  map[string]int{"high": 2},
	}
	applyPrivacySuppression(agg)
	if len(agg.ProtectionCounts) != 0 {
		t.Errorf("ProtectionCounts should be zeroed on sparse aggregate, got %+v", agg.ProtectionCounts)
	}
	if len(agg.ConfidenceCounts) != 0 {
		t.Errorf("ConfidenceCounts should be zeroed on sparse aggregate, got %+v", agg.ConfidenceCounts)
	}
	if !agg.IsSparse {
		t.Error("IsSparse should be true when below threshold")
	}
}

// TestApplyPrivacySuppression_AtThresholdPreservesCounts confirms
// the boundary: exactly PrivacyThreshold (=3) impacted units passes
// the privacy gate and the breakdown is preserved.
func TestApplyPrivacySuppression_AtThresholdPreservesCounts(t *testing.T) {
	agg := &Aggregate{
		ImpactedUnitCount: PrivacyThreshold, // boundary
		ProtectionCounts:  map[string]int{"strong": 2, "weak": 1},
		ConfidenceCounts:  map[string]int{"high": 3},
	}
	applyPrivacySuppression(agg)
	if len(agg.ProtectionCounts) == 0 {
		t.Error("ProtectionCounts should be preserved at threshold")
	}
	if agg.IsSparse {
		t.Error("IsSparse should be false when at threshold (no owner-count concern)")
	}
}

// TestApplyPrivacySuppression_SparseOwnerCountMarked covers the
// second privacy rule: when the owner count is below threshold but
// non-zero, IsSparse is set so downstream renderers can render a
// "few teams" caption rather than a specific number.
func TestApplyPrivacySuppression_SparseOwnerCountMarked(t *testing.T) {
	agg := &Aggregate{
		ImpactedUnitCount: PrivacyThreshold * 10, // not the trigger
		OwnerCount:        2,                     // below threshold
		ProtectionCounts:  map[string]int{"strong": 30},
		ConfidenceCounts:  map[string]int{"high": 30},
	}
	applyPrivacySuppression(agg)
	if !agg.IsSparse {
		t.Error("IsSparse should be true when OwnerCount is below threshold")
	}
	// Counts should NOT be zeroed in this case — only the owner-count
	// is the privacy concern, not the breakdown.
	if len(agg.ProtectionCounts) == 0 {
		t.Error("ProtectionCounts should not be zeroed when only OwnerCount triggers sparsity")
	}
}

// TestApplyPrivacySuppression_ZeroOwnerCountNotSparse: the
// suppression should NOT fire when there are zero owners (no privacy
// concern — there's nothing to identify).
func TestApplyPrivacySuppression_ZeroOwnerCountNotSparse(t *testing.T) {
	agg := &Aggregate{
		ImpactedUnitCount: PrivacyThreshold * 10,
		OwnerCount:        0,
		ProtectionCounts:  map[string]int{"strong": 30},
	}
	applyPrivacySuppression(agg)
	if agg.IsSparse {
		t.Error("IsSparse should be false when OwnerCount is zero (no entity to identify)")
	}
}
