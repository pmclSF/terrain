package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipipeline/stages"
)

// runCV reports k-fold cross-validation precision/recall/F1 for the
// current calibration table. The calibration weights are NOT refit on
// each fold — the goal is to estimate sample variance in the reported
// metrics, not to measure overfit-from-fitting. A high variance means
// the headline 13% precision number is noisy; a low variance means
// the calibration's behavior is stable across random corpus subsets.
//
// For each fold:
//
//	test    — held-out fold (1/k of rows)
//	report  — precision, recall, F1 against current calibration
//
// Final block reports mean ± stddev across folds, plus min/max, plus
// a 95% confidence interval (Wilson interval on precision).
//
// Limitations:
//
//   - This is NOT a true overfit check. The hand-tuned langchain
//     down-weight was informed by the whole corpus's marginal lift,
//     so per-fold marginal lift would shift. A real overfit check
//     would refit on each training fold and evaluate on test.
//   - The corpus has only 52 TPs total. Across 5 folds, mean ~10 TPs
//     per fold. Per-fold precision is noisy at this sample size.
func runCV(args []string) {
	fs := flag.NewFlagSet("cv", flag.ExitOnError)
	var f validateFlags
	var k int
	var seed int64
	var threshold float64
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv", "labels TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files", "v1 cache root")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2", "v2 cache root")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3", "v3 cache root")
	fs.StringVar(&f.appShapeTxt, "app-shape", "",
		"path to app-shape filter file; empty disables cohort labels")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval", "rule ID")
	fs.IntVar(&f.maxRows, "max", 0, "max rows")
	fs.IntVar(&k, "k", 5, "number of folds")
	fs.Int64Var(&seed, "seed", 42, "shuffle seed")
	fs.Float64Var(&threshold, "threshold", 0.40, "emission threshold")
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

	// Score every row once — composer output is identical regardless of
	// which fold the row lands in, since weights don't change.
	cal := aipipeline.DefaultCalibration()
	comp := aipipeline.NewComposer(cal, aipipeline.PostureObservability)
	pipeline := aipipeline.NewPipeline(comp,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(nil),
		stages.NewChangeScope(),
	)
	type scored struct {
		conf  float64
		label string
	}
	scoredRows := make([]scored, 0, len(rows))
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
			Cohort: cohortForRow(row),
			Src:    row.src,
		}
		if _, ok := pipeline.Run(context.Background(), cand); !ok {
			scoredRows = append(scoredRows, scored{0.0, row.label})
			continue
		}
		fin := comp.Compose(cand)
		scoredRows = append(scoredRows, scored{fin.Confidence, row.label})
	}

	// Shuffle, then assign each row a fold index.
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(scoredRows), func(i, j int) {
		scoredRows[i], scoredRows[j] = scoredRows[j], scoredRows[i]
	})

	type foldStats struct {
		emitted   int
		tps       int
		totalTPs  int
		precision float64
		recall    float64
		f1        float64
	}
	folds := make([]foldStats, k)
	for i, s := range scoredRows {
		fold := i % k
		if s.label == "TP" {
			folds[fold].totalTPs++
		}
		if s.conf >= threshold {
			folds[fold].emitted++
			if s.label == "TP" {
				folds[fold].tps++
			}
		}
	}
	for i := range folds {
		if folds[i].emitted > 0 {
			folds[i].precision = 100 * float64(folds[i].tps) / float64(folds[i].emitted)
		}
		if folds[i].totalTPs > 0 {
			folds[i].recall = 100 * float64(folds[i].tps) / float64(folds[i].totalTPs)
		}
		if folds[i].precision+folds[i].recall > 0 {
			folds[i].f1 = 2 * folds[i].precision * folds[i].recall /
				(folds[i].precision + folds[i].recall)
		}
	}

	fmt.Printf("# k=%d | seed=%d | threshold=%.3f | rows=%d\n\n",
		k, seed, threshold, len(scoredRows))
	fmt.Printf("%-6s %-9s %-6s %-7s %-11s %-9s %s\n",
		"fold", "emitted", "tps", "totalT", "precision%", "recall%", "f1")
	fmt.Println("─────────────────────────────────────────────────────────────────")
	for i, fs := range folds {
		fmt.Printf("%-6d %-9d %-6d %-7d %-11.2f %-9.2f %.2f\n",
			i, fs.emitted, fs.tps, fs.totalTPs, fs.precision, fs.recall, fs.f1)
	}

	mean := func(g func(foldStats) float64) float64 {
		s := 0.0
		for _, f := range folds {
			s += g(f)
		}
		return s / float64(len(folds))
	}
	std := func(g func(foldStats) float64, m float64) float64 {
		s := 0.0
		for _, f := range folds {
			d := g(f) - m
			s += d * d
		}
		return math.Sqrt(s / float64(len(folds)))
	}
	minMax := func(g func(foldStats) float64) (float64, float64) {
		mn, mx := math.Inf(1), math.Inf(-1)
		for _, f := range folds {
			v := g(f)
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
		return mn, mx
	}
	gp := func(f foldStats) float64 { return f.precision }
	gr := func(f foldStats) float64 { return f.recall }
	gf := func(f foldStats) float64 { return f.f1 }
	pMean, rMean, fMean := mean(gp), mean(gr), mean(gf)
	pStd, rStd, fStd := std(gp, pMean), std(gr, rMean), std(gf, fMean)
	pMin, pMax := minMax(gp)
	rMin, rMax := minMax(gr)

	fmt.Println()
	fmt.Println("cross-fold summary (mean ± stddev, min … max)")
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Printf("  precision: %5.2f%% ± %4.2f   (%5.2f … %5.2f)\n",
		pMean, pStd, pMin, pMax)
	fmt.Printf("  recall:    %5.2f%% ± %4.2f   (%5.2f … %5.2f)\n",
		rMean, rStd, rMin, rMax)
	fmt.Printf("  F1:        %5.2f   ± %4.2f\n", fMean, fStd)

	// Aggregate full-corpus emit/tp counts and apply Wilson 95% CI on
	// precision. This is the honest confidence interval to cite next
	// to the headline number.
	totalEmit, totalTP := 0, 0
	for _, f := range folds {
		totalEmit += f.emitted
		totalTP += f.tps
	}
	if totalEmit > 0 {
		p := float64(totalTP) / float64(totalEmit)
		n := float64(totalEmit)
		z := 1.96
		denom := 1 + z*z/n
		center := (p + z*z/(2*n)) / denom
		spread := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denom
		fmt.Printf("\naggregate precision: %.2f%% (Wilson 95%% CI: %.2f%%–%.2f%%, emitted=%d, TPs=%d)\n",
			100*p, 100*math.Max(0, center-spread), 100*(center+spread), totalEmit, totalTP)
	}

	// Sanity: per-fold ranges shouldn't span more than ~2× the mean if
	// the calibration is stable. Print a warning otherwise.
	sort.Float64s([]float64{pMin, pMax})
	if pMean > 0 && (pMax-pMin)/pMean > 1.0 {
		fmt.Println("\n⚠ per-fold precision spread > 100% of the mean — small sample noise dominates")
	}
}
