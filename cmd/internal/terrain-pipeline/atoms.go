package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipipeline/stages"
)

// runAtoms computes per-atom marginal TP rates against the labeled
// corpus. For each atom that appears in at least minSupport rows, the
// report shows:
//
//	support     — rows that emitted the atom
//	tps         — TPs among those rows
//	prec %      — TPs / support
//	lift        — prec divided by base TP rate of all corpus rows
//	calibration — current weight in DefaultCalibration (for comparison)
//
// This is descriptive statistics, not model fitting. The point is to
// see which atoms are pulling weight in the right direction. Atoms
// with positive calibration weight but precision below the corpus
// base rate are mis-signed; atoms with strong empirical lift but
// near-zero weight are under-used.
func runAtoms(args []string) {
	fs := flag.NewFlagSet("atoms", flag.ExitOnError)
	var f validateFlags
	var minSupport int
	var sortBy string
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv", "labels TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files", "v1 cache root")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2", "v2 cache root")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3", "v3 cache root")
	fs.StringVar(&f.appShapeTxt, "app-shape", "",
		"path to app-shape filter file; empty disables cohort labels")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval", "rule ID")
	fs.IntVar(&f.maxRows, "max", 0, "max rows")
	fs.IntVar(&minSupport, "min-support", 10,
		"hide atoms with fewer than this many emissions")
	fs.StringVar(&sortBy, "sort", "lift",
		"sort by: lift | precision | support | weight")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	rows, err := loadLabels(f.labelsPath, f.filesV1Dir, f.filesV2Dir, f.filesV3Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := assignCohorts(rows, f.appShapeTxt); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
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

	type atomStats struct {
		support int
		tps     int
	}
	byAtom := map[string]*atomStats{}
	totalTPs := 0

	for _, row := range rows {
		if row.label == "TP" {
			totalTPs++
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
			Cohort: cohortForRow(row),
			Src:    row.src,
		}
		pipeline.Run(context.Background(), cand)
		seen := map[string]bool{}
		for _, a := range cand.Atoms {
			if seen[a.RuleID] {
				continue
			}
			seen[a.RuleID] = true
			if byAtom[a.RuleID] == nil {
				byAtom[a.RuleID] = &atomStats{}
			}
			byAtom[a.RuleID].support++
			if row.label == "TP" {
				byAtom[a.RuleID].tps++
			}
		}
	}

	baseRate := float64(totalTPs) / float64(len(rows))

	type atomRow struct {
		id          string
		support     int
		tps         int
		precision   float64
		lift        float64
		calibration float64
	}
	var atomRows []atomRow
	for id, s := range byAtom {
		if s.support < minSupport {
			continue
		}
		prec := float64(s.tps) / float64(s.support)
		lift := math.Inf(1)
		if baseRate > 0 {
			lift = prec / baseRate
		}
		w, _ := cal.AtomWeight("*", "*", id)
		atomRows = append(atomRows, atomRow{
			id: id, support: s.support, tps: s.tps,
			precision: prec, lift: lift, calibration: w,
		})
	}

	switch sortBy {
	case "precision":
		sort.Slice(atomRows, func(i, j int) bool { return atomRows[i].precision > atomRows[j].precision })
	case "support":
		sort.Slice(atomRows, func(i, j int) bool { return atomRows[i].support > atomRows[j].support })
	case "weight":
		sort.Slice(atomRows, func(i, j int) bool { return atomRows[i].calibration > atomRows[j].calibration })
	default:
		sort.Slice(atomRows, func(i, j int) bool { return atomRows[i].lift > atomRows[j].lift })
	}

	fmt.Printf("# corpus rows: %d | TPs: %d | base TP rate: %.4f\n",
		len(rows), totalTPs, baseRate)
	fmt.Printf("# min-support: %d | sort by: %s\n\n", minSupport, sortBy)
	fmt.Printf("%-35s %-8s %-6s %-9s %-7s %-9s %s\n",
		"atom", "support", "tps", "prec %", "lift", "weight", "signal")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")
	for _, r := range atomRows {
		signal := "·"
		if r.calibration > 0 && r.lift > 1.5 {
			signal = "POS confirmed"
		} else if r.calibration < 0 && r.lift < 0.5 {
			signal = "NEG confirmed"
		} else if r.calibration > 0 && r.lift < 0.8 {
			signal = "POS MISALIGNED"
		} else if r.calibration < 0 && r.lift > 1.5 {
			signal = "NEG MISALIGNED"
		}
		fmt.Printf("%-35s %-8d %-6d %-9.2f %-7.2f %+9.2f  %s\n",
			r.id, r.support, r.tps, 100*r.precision, r.lift, r.calibration, signal)
	}
}
