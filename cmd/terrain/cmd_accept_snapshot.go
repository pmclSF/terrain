package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pmclSF/terrain/internal/atomicfile"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// runAcceptSnapshotCommand implements `terrain accept-snapshot`.
//
// The command walks the adopter through accepting baseline updates
// deliberately. 0.2.0 ships a focused version:
//   - Reads the current eval-run artifact at .terrain/eval-runs/latest.json
//     (or a path passed as the first positional arg).
//   - Diff against .terrain/baselines/latest.json.
//   - Prompts for confirmation unless --yes is set.
//   - Writes the new baseline.
//
// The diff display is intentionally compact at 0.2.0 — full
// run-vs-run rendering integrates with the eval-adapter package once
// the cmd_ai pipeline routes runs through the unified shape.
func runAcceptSnapshotCommand(root string, yes bool) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	current, baseline, err := loadAcceptSnapshotSources(abs)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("no eval run found at .terrain/eval-runs/latest.json — run `terrain ai run` first")
	}

	fmt.Printf("Accept-snapshot for %s\n\n", abs)

	switch {
	case baseline == nil:
		fmt.Println("No existing baseline. The current run will become the baseline.")
	default:
		fmt.Printf("Replacing baseline recorded at %s\n", baseline.RecordedAt)
		if delta := baseline.CaseCount - current.CaseCount; delta != 0 {
			fmt.Printf("  Case count: %d %s %d (delta %+d)\n", baseline.CaseCount, uitokens.GlyphArrow(), current.CaseCount, -delta)
		}
		if delta := baseline.Successes - current.Successes; delta != 0 {
			fmt.Printf("  Successes:  %d %s %d (delta %+d)\n", baseline.Successes, uitokens.GlyphArrow(), current.Successes, -delta)
		}
		if delta := baseline.Failures - current.Failures; delta != 0 {
			fmt.Printf("  Failures:   %d %s %d (delta %+d)\n", baseline.Failures, uitokens.GlyphArrow(), current.Failures, -delta)
		}
	}
	fmt.Println()

	if !yes {
		fmt.Print("Accept this run as the new baseline? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("Aborted. Baseline unchanged.")
			return nil
		}
	}

	baselinePath := filepath.Join(abs, ".terrain", "baselines", "latest.json")
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
		return fmt.Errorf("create baselines dir: %w", err)
	}

	// Update RecordedAt to now and write.
	current.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := atomicfile.WriteFile(baselinePath, data, 0o644); err != nil {
		return fmt.Errorf("write baseline: %w", err)
	}
	fmt.Printf("Wrote %s\n", baselinePath)
	return nil
}

// acceptSnapshotArtifact is the compact run summary the command
// reads / writes. Shape is intentionally permissive: any extra fields
// in the source files are preserved through copy at the JSON level
// rather than being modeled here.
type acceptSnapshotArtifact struct {
	RecordedAt string `json:"recordedAt,omitempty"`
	CaseCount  int    `json:"caseCount,omitempty"`
	Successes  int    `json:"successes,omitempty"`
	Failures   int    `json:"failures,omitempty"`
}

func loadAcceptSnapshotSources(root string) (current, baseline *acceptSnapshotArtifact, err error) {
	curr, err := loadAcceptArtifact(filepath.Join(root, ".terrain", "eval-runs", "latest.json"))
	if err != nil {
		return nil, nil, err
	}
	base, err := loadAcceptArtifact(filepath.Join(root, ".terrain", "baselines", "latest.json"))
	if err != nil {
		return nil, nil, err
	}
	return curr, base, nil
}

func loadAcceptArtifact(path string) (*acceptSnapshotArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var a acceptSnapshotArtifact
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &a, nil
}
