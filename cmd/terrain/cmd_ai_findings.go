package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipiperun"
)

// runAIFindings is the user-facing entry point for the calibrated
// aipipeline. It walks the repo root through the full pipeline
// (path-prefilter → regex-fastscan → ast-confirm → cross-file-scope →
// change-scope → composer) and renders the surviving findings.
//
// This is the production-side surface that exposes the calibration
// work done in internal/aipipeline. On the labeled corpus
// (2,651 rows, 2026-05-15) the same pipeline reaches 16.83% precision
// on app-shaped repos and 13.00% overall at observability threshold.
//
// Posture defaults to "observability" — emit anything that clears the
// 0.40 confidence threshold. Pass --posture=gate for the stricter
// 0.80 cut used by CI gate decisions.
func runAIFindings(root string, jsonOutput bool, posture string, rule string) error {
	rules := []string{"ai.surface.missing_eval"}
	if rule != "" {
		rules = []string{rule}
	}

	post := aipipeline.PostureObservability
	if posture == "gate" {
		post = aipipeline.PostureGate
	}

	findings, err := aipiperun.RunRepo(context.Background(), root, rules, post)
	if err != nil {
		return fmt.Errorf("aipipeline run: %w", err)
	}

	// Sort by confidence descending, then by path for stable output.
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Confidence != findings[j].Confidence {
			return findings[i].Confidence > findings[j].Confidence
		}
		return findings[i].Path < findings[j].Path
	})

	if jsonOutput {
		return renderFindingsJSON(findings)
	}
	return renderFindingsText(findings, post)
}

type findingJSON struct {
	Path        string     `json:"path"`
	Rule        string     `json:"rule"`
	Cohort      string     `json:"cohort,omitempty"`
	Confidence  float64    `json:"confidence"`
	Severity    string     `json:"severity"`
	Preview     bool       `json:"preview,omitempty"`
	Evidence    []atomJSON `json:"evidence"`
	FixScaffold string     `json:"fixScaffold,omitempty"`
}

type atomJSON struct {
	Kind   string  `json:"kind"`
	RuleID string  `json:"ruleId"`
	Weight float64 `json:"weight"`
	Source string  `json:"source"`
	Line   int     `json:"line,omitempty"`
	Span   string  `json:"span,omitempty"`
}

func renderFindingsJSON(findings []aipipeline.Finding) error {
	cal := aipipeline.DefaultCalibration()
	out := make([]findingJSON, 0, len(findings))
	for _, f := range findings {
		fj := findingJSON{
			Path:        f.Path,
			Rule:        f.RuleID,
			Cohort:      f.Cohort,
			Confidence:  f.Confidence,
			Severity:    string(f.Severity),
			Preview:     cal.IsPreview(f.RuleID),
			FixScaffold: f.FixScaffold,
		}
		for _, a := range f.Atoms {
			fj.Evidence = append(fj.Evidence, atomJSON{
				Kind:   string(a.Kind),
				RuleID: a.RuleID,
				Weight: a.Weight,
				Source: a.Source,
				Line:   a.Span.Line,
				Span:   a.Span.Snippet,
			})
		}
		out = append(out, fj)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderFindingsText(findings []aipipeline.Finding, posture aipipeline.Posture) error {
	if len(findings) == 0 {
		fmt.Printf("No findings at %s posture.\n", posture)
		return nil
	}
	cal := aipipeline.DefaultCalibration()
	fmt.Printf("%d %s at %s posture:\n\n",
		len(findings), pluralizeFindings(len(findings)), posture)
	for _, f := range findings {
		previewTag := ""
		if cal.IsPreview(f.RuleID) {
			previewTag = " [preview]"
		}
		fmt.Printf("  %s%s\n", f.Path, previewTag)
		fmt.Printf("    rule:       %s\n", f.RuleID)
		fmt.Printf("    severity:   %s\n", f.Severity)
		fmt.Printf("    confidence: %.2f\n", f.Confidence)
		if f.Cohort != "" {
			fmt.Printf("    cohort:     %s\n", f.Cohort)
		}
		if previewTag != "" {
			fmt.Printf("    NOTE:       this rule is in preview — confidence number is not yet corpus-validated\n")
		}
		if len(f.Atoms) > 0 {
			fmt.Printf("    evidence:\n")
			for _, a := range f.Atoms {
				line := ""
				if a.Span.Line > 0 {
					line = fmt.Sprintf(" L%d", a.Span.Line)
				}
				fmt.Printf("      [%s] %s%s  (w=%+0.2f)\n",
					a.Kind, a.RuleID, line, a.Weight)
			}
		}
		if f.FixScaffold != "" {
			fmt.Printf("    fix-scaffold available\n")
		}
		fmt.Println()
	}
	return nil
}

func pluralizeFindings(n int) string {
	if n == 1 {
		return "finding"
	}
	return "findings"
}
