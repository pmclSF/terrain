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
			cmd := coverageCommand(r.Repository.Languages)
			actions = append(actions, NextAction{
				Title:       "Unlock coverage analysis",
				Command:     cmd,
				Explanation: "Coverage data enables precise structural coverage mapping.",
			})
		case "runtime":
			cmd := runtimeCommand(r.Repository.Languages)
			actions = append(actions, NextAction{
				Title:       "Unlock health signals",
				Command:     cmd,
				Explanation: "Runtime data unlocks flaky, slow, dead, and unstable test detection.",
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

func coverageCommand(languages []string) string {
	for _, lang := range languages {
		switch lang {
		case "javascript", "typescript":
			return "npx jest --coverage --coverageReporters=lcov"
		case "go":
			return "go test -coverprofile=coverage.out ./..."
		case "python":
			return "pytest --cov --cov-report=lcov"
		}
	}
	return "terrain analyze --coverage <path-to-lcov-or-cobertura>"
}

func runtimeCommand(languages []string) string {
	for _, lang := range languages {
		switch lang {
		case "javascript", "typescript":
			return "npx jest --json --outputFile=jest-results.json"
		case "go":
			return "go test -json ./... > test-output.json"
		case "python":
			return "pytest --junitxml=junit.xml"
		}
	}
	return "terrain analyze --runtime <path-to-junit-xml-or-jest-json>"
}
