package quality

import (
	"fmt"

	"github.com/pmclSF/terrain/internal/models"
)

// MockHeavyDetector identifies test files with high mock usage relative
// to direct assertions.
//
// Heuristic:
//   - If mock count exceeds assertion count, the test is mock-heavy.
//   - If mock count is >= 5 and assertion count is <= mock count, flag it.
//   - Higher ratios get higher severity.
//
// Limitations:
//   - Regex-based mock counting may miss some mock patterns or
//     over-count mock-related helper calls.
//   - Does not distinguish between necessary isolation mocks and
//     excessive mocking of internals.
//   - This is not saying mocking is bad — it signals increased risk
//     of false confidence when mocks dominate.
type MockHeavyDetector struct{}

// Detect scans test files for high mock usage patterns.
func (d *MockHeavyDetector) Detect(snap *models.TestSuiteSnapshot) []models.Signal {
	var signals []models.Signal

	for _, tf := range snap.TestFiles {
		if tf.MockCount == 0 || tf.TestCount == 0 {
			continue
		}

		// Only flag files with meaningful mock usage (>= 3 mocks)
		if tf.MockCount < 3 {
			continue
		}

		// Tests-only-mocks: file has mocks but zero assertions — the most
		// severe form of mock overuse. These tests verify wiring, not behavior.
		if tf.AssertionCount == 0 {
			signals = append(signals, models.Signal{
				Type:             "testsOnlyMocks",
				Category:         models.CategoryQuality,
				Severity:         models.SeverityHigh,
				Confidence:       0.8,
				EvidenceStrength: models.EvidenceModerate,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Explanation: fmt.Sprintf("Test file contains %d mock(s) but zero assertions. Tests verify wiring only, not behavior.", tf.MockCount),
				SuggestedAction: "Add assertions on outputs, state changes, or side effects to validate real behavior.",
			})
			continue
		}

		// Mock-heavy: mocks outnumber assertions, suggesting over-isolation.
		//
		// Severity is Low and confidence is 0.35 because the signal has
		// not held up as a reliable regression predictor. It remains in
		// experimental status pending redesign; Low severity + low
		// confidence keeps it visible in extended reports without
		// dominating the gate.
		if tf.MockCount > tf.AssertionCount {
			sev := models.SeverityLow
			conf := 0.35
			signals = append(signals, models.Signal{
				Type:             "mockHeavyTest",
				Category:         models.CategoryQuality,
				Severity:         sev,
				Confidence:       conf,
				EvidenceStrength: models.EvidenceWeak,
				EvidenceSource:   models.SourceStructuralPattern,
				Location:         models.SignalLocation{File: tf.Path},
				Explanation: fmt.Sprintf("High mock usage detected: %d mock(s) vs %d assertion(s).", tf.MockCount, tf.AssertionCount),
				SuggestedAction: "Consider adding assertions on real outputs or supplementing with integration coverage. Detector is experimental.",
			})
		}
	}

	return signals
}
