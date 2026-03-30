package analyze

import "fmt"

// NextAction is a prioritized recommendation with a runnable command.
type NextAction struct {
	// Title is a short description of what to do.
	Title string `json:"title"`

	// Command is a copy-pasteable terrain command.
	Command string `json:"command"`

	// Explanation provides context on why this matters.
	Explanation string `json:"explanation"`
}

// deriveNextActions returns up to 3 actions, prioritizing data-completeness
// actions first (they unlock more findings), then finding-based actions.
func deriveNextActions(r *Report) []NextAction {
	var actions []NextAction

	// Data-completeness actions first — they unlock more signals.
	for _, ds := range r.DataCompleteness {
		if len(actions) >= 3 {
			break
		}
		if ds.Available {
			continue
		}
		switch ds.Name {
		case "coverage":
			actions = append(actions, NextAction{
				Title:       "Unlock coverage analysis",
				Command:     "terrain analyze --coverage <path-to-lcov-or-cobertura>",
				Explanation: "Coverage data enables precise structural coverage mapping.",
			})
		case "runtime":
			actions = append(actions, NextAction{
				Title:       "Unlock health signals",
				Command:     "terrain analyze --runtime <path-to-junit-xml-or-jest-json>",
				Explanation: "Runtime data reveals flaky, slow, and dead tests.",
			})
		}
	}

	// Finding-based actions.
	if len(actions) < 3 && r.DuplicateClusters.ClusterCount > 0 {
		actions = append(actions, NextAction{
			Title: fmt.Sprintf("Investigate %d duplicate test clusters",
				r.DuplicateClusters.ClusterCount),
			Command:     "terrain insights",
			Explanation: fmt.Sprintf("%d tests are structurally similar.", r.DuplicateClusters.RedundantTestCount),
		})
	}

	if len(actions) < 3 && len(r.WeakCoverageAreas) > 0 {
		actions = append(actions, NextAction{
			Title: fmt.Sprintf("Review %d weak coverage areas",
				len(r.WeakCoverageAreas)),
			Command:     "terrain insights",
			Explanation: "Source areas with low structural test coverage.",
		})
	}

	if len(actions) < 3 && r.HighFanout.FlaggedCount > 0 {
		actions = append(actions, NextAction{
			Title: fmt.Sprintf("Examine %d high-fanout fixtures",
				r.HighFanout.FlaggedCount),
			Command:     "terrain debug fanout",
			Explanation: "Shared fixtures that affect many tests on any change.",
		})
	}

	// Always suggest impact if nothing else matches.
	if len(actions) == 0 {
		actions = append(actions, NextAction{
			Title:       "See what your next change affects",
			Command:     "terrain impact --base main",
			Explanation: "Traces a diff through the dependency graph to find impacted tests.",
		})
	}

	return actions
}
