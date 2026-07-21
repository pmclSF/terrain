// Command gate-validate answers the gate-promotion safety question for the
// schema->prompt drift detector: for every drift fire in a repo, does it carry
// a closed-loop-validated fix? Under the default trust floor, only fix-carrying
// findings block CI — so this reports exactly which fires would gate.
//
// Operator/validation tool. Prints JSON: per fire, the location, message,
// whether a fix was produced, and the corrected line (for hand-verification).
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/promptcontract"
)

type fireReport struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Variable  string `json:"variable"`
	Message   string `json:"message"`
	HasFix    bool   `json:"hasFix"`
	FixKind   string `json:"fixKind,omitempty"`
	FixedPath string `json:"fixedPath,omitempty"`
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	drift, err := promptcontract.AnalyzeInRepo(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sigs := promptcontract.ToSignals(drift)
	fires := make([]fireReport, 0, len(sigs))
	withFix := 0
	for i, s := range sigs {
		f := findings.FromSignal(s, "terrain/ai/prompt-schema-drift")
		fr := fireReport{
			Path:     f.PrimaryLoc.Path,
			Line:     f.PrimaryLoc.Line,
			Variable: drift[i].Variable,
			Message:  drift[i].Message,
		}
		if fix := promptcontract.DriftFix(root, f); fix != nil {
			fr.HasFix = true
			fr.FixKind = string(fix.Kind)
			fr.FixedPath = fix.Path
			withFix++
		}
		fires = append(fires, fr)
	}

	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"root":    root,
		"fires":   len(fires),
		"withFix": withFix, // == count that would BLOCK CI under the trust floor
		"detail":  fires,
	})
}
