package impact

import "strings"

// FilterByOwner returns a new ImpactResult filtered to only include
// units and related data for the specified owner.
func FilterByOwner(result *ImpactResult, owner string) *ImpactResult {
	ownerLower := strings.ToLower(owner)

	filtered := &ImpactResult{
		ChangeSet:          result.ChangeSet,
		Scope:              result.Scope,
		ChangedAreas:       result.ChangedAreas,
		AffectedBehaviors:  result.AffectedBehaviors,
		ImpactedScenarios:  result.ImpactedScenarios,
		Graph:              result.Graph,
		Posture:            result.Posture,
		CoverageConfidence: result.CoverageConfidence,
		ReasonCategories:   result.ReasonCategories,
		Fallback:           result.Fallback,
		TotalTestCount:     result.TotalTestCount,
		HasCoverageData:    result.HasCoverageData,
		PolicyApplied:      result.PolicyApplied,
		PolicyNotes:        result.PolicyNotes,
	}

	// Filter impacted units.
	coveredPaths := map[string]bool{}
	for _, iu := range result.ImpactedUnits {
		if strings.ToLower(iu.Owner) == ownerLower {
			filtered.ImpactedUnits = append(filtered.ImpactedUnits, iu)
			for _, tp := range iu.CoveringTests {
				coveredPaths[tp] = true
			}
		}
	}

	// Filter tests to those covering the owner's units.
	for _, t := range result.ImpactedTests {
		if coveredPaths[t.Path] {
			filtered.ImpactedTests = append(filtered.ImpactedTests, t)
		}
	}

	// Filter gaps to the owner's units.
	ownerUnitIDs := map[string]bool{}
	for _, iu := range filtered.ImpactedUnits {
		ownerUnitIDs[iu.UnitID] = true
	}
	for _, gap := range result.ProtectionGaps {
		if ownerUnitIDs[gap.CodeUnitID] {
			filtered.ProtectionGaps = append(filtered.ProtectionGaps, gap)
		}
	}

	filtered.SelectedTests = selectProtectiveTests(filtered.ImpactedTests, filtered.ImpactedUnits)
	filtered.ProtectiveSet = buildProtectiveSet(filtered)
	filtered.ImpactedOwners = []string{owner} // preserve original casing
	filtered.Summary = buildImpactSummary(filtered)
	filtered.Limitations = result.Limitations

	return filtered
}
