package reporting

import (
	"fmt"
	"io"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

// WriteHealthGuidance prints actionable guidance when runtime data is absent.
// It is a no-op if runtime-derived signals are present.
func WriteHealthGuidance(w io.Writer, snap *models.TestSuiteSnapshot) {
	if hasRuntimeSignals(snap) {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Static analysis (skip detection, dead test detection) is available without runtime artifacts.")
	fmt.Fprintln(w, "  Additional health signals (flaky, slow, unstable tests) require runtime artifacts.")
	fmt.Fprintln(w, "  Generate with:")
	fmt.Fprintln(w, "    Jest:    npx jest --json --outputFile=jest-results.json")
	fmt.Fprintln(w, "    Pytest:  pytest --junitxml=junit.xml")
	fmt.Fprintln(w, "    Go:      go test -json ./... > test-results.json")
	fmt.Fprintln(w, "    JUnit:   mvn test  (generates target/surefire-reports/*.xml)")
	fmt.Fprintln(w, "  Then re-run with: terrain analyze --runtime <path>")
}

func hasRuntimeSignals(snap *models.TestSuiteSnapshot) bool {
	for _, sig := range snap.Signals {
		switch sig.Type {
		case signals.SignalSlowTest, signals.SignalFlakyTest, signals.SignalSkippedTest,
			signals.SignalDeadTest, signals.SignalUnstableSuite:
			return true
		}
	}
	return false
}
