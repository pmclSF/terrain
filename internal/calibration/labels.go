// Package calibration provides ground-truth labels and a runner that
// measures detector precision/recall against a corpus of fixtures.
//
// The corpus lives under `tests/calibration/`. Each fixture is a directory
// containing a real-world-shaped repository tree plus a `labels.yaml`
// declaring which signals are expected to fire and which are explicitly
// expected to NOT fire (false-positive guards).
//
// 0.2 ships the infrastructure plus a starter corpus. Scaling to the
// 50-fixture target documented in `docs/release/0.2.md` is a content
// effort that runs in parallel with the rest of the milestone.
package calibration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"gopkg.in/yaml.v3"
)

// FixtureLabels is the schema of `labels.yaml` for a single corpus fixture.
type FixtureLabels struct {
	// Fixture is the human-readable name (also the directory name by
	// convention). Used in reports.
	Fixture string `yaml:"fixture"`

	// SchemaVersion locks the labels.yaml shape. 1 ships in 0.2; bump
	// only when fields are removed or repurposed.
	SchemaVersion int `yaml:"schemaVersion"`

	// Description: one-line context for reviewers ("Real-world Express
	// app with known flakiness in test/db/").
	Description string `yaml:"description,omitempty"`

	// Expected lists signals the detector suite SHOULD emit on this
	// fixture. Missing entries count as false negatives (recall hit).
	Expected []ExpectedSignal `yaml:"expected"`

	// ExpectedAbsent lists signals the detector suite should explicitly
	// NOT emit on this fixture (false-positive guards). E.g. an API-key
	// pattern that's actually a placeholder.
	ExpectedAbsent []ExpectedSignal `yaml:"expectedAbsent,omitempty"`
}

// ExpectedSignal is a single ground-truth label. Matching against emitted
// signals is intentionally fuzzy on Line so labels survive small edits
// to the fixture (we match on Type + File; Line/Symbol are advisory).
type ExpectedSignal struct {
	Type   models.SignalType `yaml:"type"`
	File   string            `yaml:"file,omitempty"`
	Symbol string            `yaml:"symbol,omitempty"`
	Line   int               `yaml:"line,omitempty"`

	// Notes is a free-form explanation for human reviewers ("PR #123
	// documented this test as intermittently failing"). Ignored by the
	// runner; rendered in mismatch reports.
	Notes string `yaml:"notes,omitempty"`
}

// LoadLabels reads and validates a `labels.yaml` from a fixture directory.
// Returns a clear error if the file is missing or malformed; the runner
// surfaces this directly in the calibration report.
func LoadLabels(fixtureDir string) (*FixtureLabels, error) {
	path := filepath.Join(fixtureDir, "labels.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load labels %s: %w", path, err)
	}

	var labels FixtureLabels
	if err := yaml.Unmarshal(raw, &labels); err != nil {
		return nil, fmt.Errorf("parse labels %s: %w", path, err)
	}

	if labels.SchemaVersion != 1 {
		return nil, fmt.Errorf("labels %s: schemaVersion = %d, want 1", path, labels.SchemaVersion)
	}
	if strings.TrimSpace(labels.Fixture) == "" {
		return nil, fmt.Errorf("labels %s: empty fixture name", path)
	}

	return &labels, nil
}
