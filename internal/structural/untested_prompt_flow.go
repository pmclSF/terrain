package structural

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// UntestedPromptFlowDetector finds prompts that flow through multiple source
// files via import chains with zero test coverage at any point in the chain.
type UntestedPromptFlowDetector struct{}

func (d *UntestedPromptFlowDetector) Detect(_ *models.TestSuiteSnapshot) []models.Signal {
	return nil
}

func (d *UntestedPromptFlowDetector) DetectWithGraph(snap *models.TestSuiteSnapshot, g *depgraph.Graph) []models.Signal {
	var out []models.Signal

	for _, n := range g.NodesByType(depgraph.NodePrompt) {
		// Find the source file where this prompt is defined.
		var definingFile string
		for _, e := range g.Outgoing(n.ID) {
			if e.Type == depgraph.EdgeAIDefinedInFile {
				target := g.Node(e.To)
				if target != nil {
					definingFile = e.To
				}
				break
			}
		}
		if definingFile == "" {
			continue
		}

		// Trace forward through source imports to build the flow chain.
		flowChain := traceForwardFlow(g, definingFile)
		if len(flowChain) == 0 {
			continue
		}

		// Check if any file in the flow chain has a test file importing it.
		allFiles := append([]string{definingFile}, flowChain...)
		anyTested := false
		for _, fileID := range allFiles {
			incoming := g.Incoming(fileID)
			for _, e := range incoming {
				if e.Type == depgraph.EdgeImportsModule {
					source := g.Node(e.From)
					if source != nil && source.Type == depgraph.NodeTestFile {
						anyTested = true
						break
					}
				}
			}
			if anyTested {
				break
			}
		}

		if anyTested {
			continue
		}

		name := n.Name
		if name == "" {
			name = n.ID
		}
		promptPath := n.Path

		severity := models.SeverityHigh
		confidence := 0.80
		if len(flowChain) >= 3 {
			severity = models.SeverityCritical
			confidence = 0.75 // lower confidence for long chains
		}

		out = append(out, models.Signal{
			Type:             signals.SignalUntestedPromptFlow,
			Category:         models.CategoryAI,
			Severity:         severity,
			Confidence:       confidence,
			EvidenceStrength: models.EvidenceStrong,
			EvidenceSource:   models.SourceGraphTraversal,
			Location:         models.SignalLocation{File: promptPath, Symbol: name},
			Explanation: fmt.Sprintf(
				"Prompt '%s' flows through %d source files with zero test coverage anywhere in the chain. Behavioral regressions will be invisible.",
				name, len(allFiles)),
			SuggestedAction: "Add integration tests at the prompt's consumption points.",
			Metadata: map[string]any{
				"promptName": name,
				"promptPath": promptPath,
				"flowLength": len(allFiles),
				"flowChain":  allFiles,
			},
		})
	}

	return out
}

// traceForwardFlow follows EdgeSourceImportsSource edges forward from a file
// to find all files that transitively consume it.
func traceForwardFlow(g *depgraph.Graph, startFileID string) []string {
	var chain []string
	visited := map[string]bool{startFileID: true}
	queue := []string{startFileID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		incoming := g.Incoming(current)
		for _, e := range incoming {
			if e.Type != depgraph.EdgeSourceImportsSource {
				continue
			}
			if visited[e.From] {
				continue
			}
			visited[e.From] = true
			chain = append(chain, e.From)
			queue = append(queue, e.From)
		}
	}

	return chain
}
