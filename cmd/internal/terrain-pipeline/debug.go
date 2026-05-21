package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipipeline/stages"
)

// runDebug emits one structured line per labeled row showing the
// atoms collected, the composer's log-odds, the sigmoid confidence,
// and whether the row would be emitted at the active threshold.
//
// Output channels (controlled by --filter):
//
//	missed-tps   — rows labeled TP but suppressed
//	emitted-fps  — rows labeled FP but emitted
//	all          — everything (large)
//
// Use this to discover which atom weights need tuning. A TP getting
// suppressed because base_rate (-3.5) + atoms (+1.8) = -1.7 → conf 0.15
// is a calibration problem the validate report can't show on its own.
func runDebug(args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	var f validateFlags
	var filter string
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv", "labels TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files", "v1 cache root")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2", "v2 cache root")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3", "v3 cache root")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval", "rule ID")
	fs.StringVar(&f.posture, "posture", "observability", "posture")
	fs.IntVar(&f.maxRows, "max", 0, "max rows to evaluate")
	fs.StringVar(&filter, "filter", "missed-tps",
		"one of: missed-tps | emitted-fps | all")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	rows, err := loadLabels(f.labelsPath, f.filesV1Dir, f.filesV2Dir, f.filesV3Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if f.maxRows > 0 && len(rows) > f.maxRows {
		rows = rows[:f.maxRows]
	}

	posture := aipipeline.PostureObservability
	if f.posture == "gate" {
		posture = aipipeline.PostureGate
	}
	cal := aipipeline.DefaultCalibration()
	comp := aipipeline.NewComposer(cal, posture)
	pipeline := aipipeline.NewPipeline(comp,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(nil),
		stages.NewChangeScope(),
	)

	threshold := comp.ThresholdFor(f.rule)
	fmt.Printf("# rule: %s | posture: %s | threshold: %.3f\n", f.rule, posture, threshold)
	fmt.Printf("# filter: %s | rows: %d\n\n", filter, len(rows))

	for _, row := range rows {
		effectiveRule := f.rule
		if row.detector == "train" {
			effectiveRule = "ai.train.missing_tracker"
		} else if row.detector == "surface" {
			effectiveRule = "ai.surface.missing_eval"
		}
		cand := &aipipeline.Candidate{
			Path:   row.path,
			Lang:   string(aipipeline.LanguageFromPath(row.path)),
			RuleID: effectiveRule,
			Cohort: "unknown",
			Src:    row.src,
		}
		dropped := false
		_, ok := pipeline.Run(context.Background(), cand)
		if !ok {
			dropped = true
		}
		fin := comp.Compose(cand)
		emitted := !dropped && comp.ShouldEmit(fin)

		isTP := row.label == "TP"
		show := false
		switch filter {
		case "missed-tps":
			show = isTP && !emitted
		case "emitted-fps":
			show = !isTP && emitted
		case "all":
			show = true
		}
		if !show {
			continue
		}

		atomStr := formatAtoms(cand.Atoms)
		fmt.Printf("[%s] %s/%s\n  path: %s\n  lang: %s\n  atoms: %s\n  logOdds: %+.2f  conf: %.3f  emit: %v  dropped: %v\n\n",
			row.label, row.detector, row.idx,
			row.path, cand.Lang,
			atomStr, fin.LogOdds, fin.Confidence, emitted, dropped)
	}
}

func formatAtoms(atoms []aipipeline.EvidenceAtom) string {
	if len(atoms) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(atoms))
	for _, a := range atoms {
		parts = append(parts, fmt.Sprintf("%s(%+0.1f)", a.RuleID, a.Weight))
	}
	return strings.Join(parts, " ")
}

// ── tune subcommand ─────────────────────────────────────────────────

// runTune sweeps confidence thresholds against the corpus and prints
// precision / recall / emit-count at each cut. This is the operating
// table you'd consult before changing the production threshold.
func runTune(args []string) {
	fs := flag.NewFlagSet("tune", flag.ExitOnError)
	var f validateFlags
	var minThresh, maxThresh, step float64
	var byCohort bool
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv", "labels TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files", "v1 cache root")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2", "v2 cache root")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3", "v3 cache root")
	fs.StringVar(&f.appShapeTxt, "app-shape", "",
		"path to app-shape filter file; empty disables cohort calibration")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval", "rule ID")
	fs.IntVar(&f.maxRows, "max", 0, "max rows")
	fs.Float64Var(&minThresh, "min", 0.10, "lowest threshold to sweep")
	fs.Float64Var(&maxThresh, "max-thresh", 0.80, "highest threshold")
	fs.Float64Var(&step, "step", 0.05, "step size between thresholds")
	fs.BoolVar(&byCohort, "by-cohort", false,
		"also report per-cohort precision/recall at each threshold")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	rows, err := loadLabels(f.labelsPath, f.filesV1Dir, f.filesV2Dir, f.filesV3Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := assignCohorts(rows, f.appShapeTxt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cohort assignment failed: %v\n", err)
	}
	if f.maxRows > 0 && len(rows) > f.maxRows {
		rows = rows[:f.maxRows]
	}

	cal := aipipeline.DefaultCalibration()
	comp := aipipeline.NewComposer(cal, aipipeline.PostureObservability)
	pipeline := aipipeline.NewPipeline(comp,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(nil),
		stages.NewChangeScope(),
	)

	// Score every row once. We compute the finding for surviving
	// candidates and store (confidence, label, cohort) tuples.
	type scored struct {
		conf   float64
		label  string
		cohort string
	}
	var scoredRows []scored
	totalTPs := 0
	totalTPsByCohort := map[string]int{}
	for _, row := range rows {
		cohort := cohortForRow(row)
		if row.label == "TP" {
			totalTPs++
			totalTPsByCohort[cohort]++
		}
		effectiveRule := f.rule
		if row.detector == "train" {
			effectiveRule = "ai.train.missing_tracker"
		} else if row.detector == "surface" {
			effectiveRule = "ai.surface.missing_eval"
		}
		cand := &aipipeline.Candidate{
			Path:   row.path,
			Lang:   string(aipipeline.LanguageFromPath(row.path)),
			RuleID: effectiveRule,
			Cohort: cohort,
			Src:    row.src,
		}
		if _, ok := pipeline.Run(context.Background(), cand); !ok {
			continue
		}
		fin := comp.Compose(cand)
		scoredRows = append(scoredRows, scored{fin.Confidence, row.label, cohort})
	}

	// Threshold sweep.
	fmt.Printf("%-9s %-9s %-9s %-9s %-9s %-9s\n",
		"threshold", "emitted", "tps", "precision%", "recall%", "f1")
	fmt.Println(strings.Repeat("-", 60))
	for t := minThresh; t <= maxThresh+1e-9; t += step {
		emitted, tps := 0, 0
		cohortE := map[string]int{}
		cohortT := map[string]int{}
		for _, s := range scoredRows {
			if s.conf >= t {
				emitted++
				cohortE[s.cohort]++
				if s.label == "TP" {
					tps++
					cohortT[s.cohort]++
				}
			}
		}
		precision := 0.0
		if emitted > 0 {
			precision = 100 * float64(tps) / float64(emitted)
		}
		recall := 0.0
		if totalTPs > 0 {
			recall = 100 * float64(tps) / float64(totalTPs)
		}
		f1 := 0.0
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}
		fmt.Printf("%-9.2f %-9d %-9d %-9.2f %-9.2f %-9.2f\n",
			t, emitted, tps, precision, recall, f1)
		if byCohort {
			cohorts := make([]string, 0, len(cohortE))
			for c := range cohortE {
				cohorts = append(cohorts, c)
			}
			sort.Strings(cohorts)
			for _, c := range cohorts {
				e := cohortE[c]
				tp := cohortT[c]
				p := 0.0
				if e > 0 {
					p = 100 * float64(tp) / float64(e)
				}
				r := 0.0
				if totalTPsByCohort[c] > 0 {
					r = 100 * float64(tp) / float64(totalTPsByCohort[c])
				}
				fmt.Printf("    └─ %-18s emit=%-4d tp=%-3d  prec=%5.2f%%  recall=%5.2f%%\n",
					c, e, tp, p, r)
			}
		}
	}

	// Also report the maximum-F1 cut.
	sort.Slice(scoredRows, func(i, j int) bool {
		return scoredRows[i].conf > scoredRows[j].conf
	})
	bestF1, bestT, bestP, bestR, bestE := 0.0, 0.0, 0.0, 0.0, 0
	for i := range scoredRows {
		t := scoredRows[i].conf
		emitted, tps := 0, 0
		for _, s := range scoredRows {
			if s.conf >= t {
				emitted++
				if s.label == "TP" {
					tps++
				}
			}
		}
		p := 100 * float64(tps) / float64(max(emitted, 1))
		r := 100 * float64(tps) / float64(max(totalTPs, 1))
		f1 := 0.0
		if p+r > 0 {
			f1 = 2 * p * r / (p + r)
		}
		if f1 > bestF1 {
			bestF1 = f1
			bestT = t
			bestP = p
			bestR = r
			bestE = emitted
		}
	}
	fmt.Println()
	fmt.Printf("best F1: %.2f at threshold=%.3f (emitted=%d, precision=%.2f%%, recall=%.2f%%)\n",
		bestF1, bestT, bestE, bestP, bestR)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
