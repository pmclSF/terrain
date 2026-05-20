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

// runFit performs a k-fold logistic-regression refit. For each fold, it
// trains weights on the 4/5 training rows via batch gradient descent
// with L2 regularization, then evaluates the held-out fold using those
// learned weights. The mean test-fold precision is an honest
// out-of-sample number; comparing it to the in-sample precision tells
// us whether the hand-tuned calibration is overfit.
//
// Method:
//
//	1. Run the full pipeline (stages 1-5) over every row to collect the
//	   set of atoms each row emits.
//	2. Build a binary feature matrix X (n_rows × n_atoms) using the
//	   union of all atom IDs observed across the corpus.
//	3. For each fold:
//	   a. Split into train (4/5) and test (1/5).
//	   b. Fit weights w by minimizing
//	        -Σ [y log σ(z) + (1-y) log(1-σ(z))] + λ/2 ||w||²
//	      via batch gradient descent.
//	   c. Pick a threshold that maximizes F1 on the training set.
//	   d. Apply that threshold to the test set; record precision/
//	      recall/F1.
//	4. Report mean ± stddev of per-fold test metrics.
//
// Compared to runCV (which uses the fixed hand-tuned calibration), this
// command answers the question "is the hand-tuning corpus-specific?"
func runFit(args []string) {
	fs := flag.NewFlagSet("fit", flag.ExitOnError)
	var f validateFlags
	var k int
	var seed int64
	var lambda, lr float64
	var iters int
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv", "labels TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files", "v1 cache root")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2", "v2 cache root")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3", "v3 cache root")
	fs.StringVar(&f.appShapeTxt, "app-shape",
		"/Users/pzachary/terrain/tier-4/app-shaped-repos.txt", "app-shape filter")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval", "rule ID")
	fs.IntVar(&f.maxRows, "max", 0, "max rows")
	fs.IntVar(&k, "k", 5, "number of folds")
	fs.Int64Var(&seed, "seed", 42, "shuffle seed")
	fs.Float64Var(&lambda, "lambda", 0.1, "L2 regularization strength")
	fs.Float64Var(&lr, "lr", 0.1, "learning rate")
	fs.IntVar(&iters, "iters", 500, "max gradient descent iterations")
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

	// Stage 1: collect atom set per row.
	cal := aipipeline.DefaultCalibration()
	comp := aipipeline.NewComposer(cal, aipipeline.PostureObservability)
	pipeline := aipipeline.NewPipeline(comp,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(nil),
		stages.NewChangeScope(),
	)
	type rowFeatures struct {
		atoms   map[string]bool
		label   string
		dropped bool
	}
	all := make([]rowFeatures, len(rows))
	atomSet := map[string]struct{}{}
	for i, row := range rows {
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
		_, ok := pipeline.Run(context.Background(), cand)
		all[i].label = row.label
		all[i].dropped = !ok
		all[i].atoms = map[string]bool{}
		for _, a := range cand.Atoms {
			all[i].atoms[a.RuleID] = true
			atomSet[a.RuleID] = struct{}{}
		}
	}

	// Stage 2: stable atom index — sorted so output is reproducible.
	atomIDs := make([]string, 0, len(atomSet))
	for id := range atomSet {
		atomIDs = append(atomIDs, id)
	}
	sort.Strings(atomIDs)
	atomIdx := map[string]int{}
	for i, id := range atomIDs {
		atomIdx[id] = i
	}

	// Build dense feature matrix X (n×d) and label vector y.
	n := len(all)
	d := len(atomIDs)
	X := make([][]float64, n)
	y := make([]float64, n)
	for i, rf := range all {
		row := make([]float64, d)
		for atom := range rf.atoms {
			row[atomIdx[atom]] = 1
		}
		X[i] = row
		if rf.label == "TP" {
			y[i] = 1
		}
	}

	// Stage 3: shuffle and fold-assign.
	idx := rand.New(rand.NewSource(seed)).Perm(n)

	type foldMetrics struct {
		emitted, tps, totalTPs int
		precision, recall, f1  float64
		threshold              float64
	}
	folds := make([]foldMetrics, k)

	fmt.Printf("# k=%d seed=%d lambda=%.3f lr=%.3f iters=%d atoms=%d rows=%d\n\n",
		k, seed, lambda, lr, iters, d, n)

	// For each fold: train on rest, evaluate on this fold.
	for fold := 0; fold < k; fold++ {
		var trainX [][]float64
		var trainY []float64
		var testIdx []int
		for pos, j := range idx {
			if pos%k == fold {
				testIdx = append(testIdx, j)
				continue
			}
			trainX = append(trainX, X[j])
			trainY = append(trainY, y[j])
		}

		w, b := fitLogistic(trainX, trainY, lambda, lr, iters)

		// Choose threshold via training-set F1 (no test-leak).
		trainScores := make([]float64, len(trainX))
		for i := range trainX {
			trainScores[i] = sigmoid(dot(trainX[i], w) + b)
		}
		tThresh := bestF1Threshold(trainScores, trainY)
		folds[fold].threshold = tThresh

		// Evaluate on test fold.
		em, tp, total := 0, 0, 0
		for _, j := range testIdx {
			s := sigmoid(dot(X[j], w) + b)
			if y[j] == 1 {
				total++
			}
			if s >= tThresh {
				em++
				if y[j] == 1 {
					tp++
				}
			}
		}
		folds[fold].emitted = em
		folds[fold].tps = tp
		folds[fold].totalTPs = total
		if em > 0 {
			folds[fold].precision = 100 * float64(tp) / float64(em)
		}
		if total > 0 {
			folds[fold].recall = 100 * float64(tp) / float64(total)
		}
		if folds[fold].precision+folds[fold].recall > 0 {
			folds[fold].f1 = 2 * folds[fold].precision * folds[fold].recall /
				(folds[fold].precision + folds[fold].recall)
		}
	}

	fmt.Printf("%-6s %-9s %-9s %-6s %-7s %-11s %-9s %s\n",
		"fold", "threshold", "emitted", "tps", "totalT", "precision%", "recall%", "f1")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	for i, fm := range folds {
		fmt.Printf("%-6d %-9.3f %-9d %-9d %-7d %-11.2f %-9.2f %.2f\n",
			i, fm.threshold, fm.emitted, fm.tps, fm.totalTPs,
			fm.precision, fm.recall, fm.f1)
	}

	// Summary stats.
	meanG := func(g func(foldMetrics) float64) float64 {
		s := 0.0
		for _, f := range folds {
			s += g(f)
		}
		return s / float64(len(folds))
	}
	stdG := func(g func(foldMetrics) float64, m float64) float64 {
		s := 0.0
		for _, f := range folds {
			d := g(f) - m
			s += d * d
		}
		return math.Sqrt(s / float64(len(folds)))
	}
	pM := meanG(func(f foldMetrics) float64 { return f.precision })
	rM := meanG(func(f foldMetrics) float64 { return f.recall })
	fM := meanG(func(f foldMetrics) float64 { return f.f1 })
	fmt.Println()
	fmt.Println("test-fold summary (mean ± stddev)")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Printf("  precision: %5.2f%% ± %4.2f\n", pM, stdG(func(f foldMetrics) float64 { return f.precision }, pM))
	fmt.Printf("  recall:    %5.2f%% ± %4.2f\n", rM, stdG(func(f foldMetrics) float64 { return f.recall }, rM))
	fmt.Printf("  F1:        %5.2f   ± %4.2f\n", fM, stdG(func(f foldMetrics) float64 { return f.f1 }, fM))

	// Aggregate Wilson CI on test-fold precision.
	totalEmit, totalTP := 0, 0
	for _, fm := range folds {
		totalEmit += fm.emitted
		totalTP += fm.tps
	}
	if totalEmit > 0 {
		p := float64(totalTP) / float64(totalEmit)
		nF := float64(totalEmit)
		z := 1.96
		denom := 1 + z*z/nF
		center := (p + z*z/(2*nF)) / denom
		spread := z * math.Sqrt(p*(1-p)/nF+z*z/(4*nF*nF)) / denom
		fmt.Printf("\nfitted CV precision: %.2f%% (Wilson 95%% CI: %.2f%%–%.2f%%, emitted=%d, TPs=%d)\n",
			100*p, 100*math.Max(0, center-spread), 100*(center+spread), totalEmit, totalTP)
		fmt.Println()
		fmt.Println("compare to hand-tuned CV (cv subcommand): 13.00% (9.21%–18.05%)")
	}

	// Print final-pass weights fit on ALL rows for inspection.
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Println("final-pass weights (fit on all rows, for inspection only)")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	w, b := fitLogistic(X, y, lambda, lr, iters)
	type aw struct {
		id     string
		weight float64
	}
	weights := make([]aw, 0, d)
	for i, id := range atomIDs {
		weights = append(weights, aw{id: id, weight: w[i]})
	}
	sort.Slice(weights, func(i, j int) bool {
		return math.Abs(weights[i].weight) > math.Abs(weights[j].weight)
	})
	fmt.Printf("  intercept: %+.3f\n", b)
	for _, w := range weights {
		if math.Abs(w.weight) < 0.05 {
			continue
		}
		fmt.Printf("  %-35s %+.3f\n", w.id, w.weight)
	}
}

// fitLogistic runs batch gradient descent for L2-regularized logistic
// regression. Returns (weights, bias).
func fitLogistic(X [][]float64, y []float64, lambda, lr float64, iters int) ([]float64, float64) {
	if len(X) == 0 {
		return nil, 0
	}
	n := len(X)
	d := len(X[0])
	w := make([]float64, d)
	b := 0.0
	for it := 0; it < iters; it++ {
		gw := make([]float64, d)
		gb := 0.0
		for i := 0; i < n; i++ {
			s := sigmoid(dot(X[i], w) + b)
			r := s - y[i]
			for j := 0; j < d; j++ {
				if X[i][j] != 0 {
					gw[j] += r * X[i][j]
				}
			}
			gb += r
		}
		inv := 1.0 / float64(n)
		for j := 0; j < d; j++ {
			gw[j] = gw[j]*inv + lambda*w[j]
			w[j] -= lr * gw[j]
		}
		b -= lr * gb * inv
	}
	return w, b
}

func dot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		e := math.Exp(-z)
		return 1 / (1 + e)
	}
	e := math.Exp(z)
	return e / (1 + e)
}

// bestF1Threshold scans candidate thresholds and returns the one that
// maximizes F1 on (scores, labels).
func bestF1Threshold(scores, y []float64) float64 {
	cands := append([]float64(nil), scores...)
	sort.Float64s(cands)
	best, bestT := -1.0, 0.5
	for _, t := range cands {
		em, tp, total := 0, 0, 0
		for i, s := range scores {
			if y[i] == 1 {
				total++
			}
			if s >= t {
				em++
				if y[i] == 1 {
					tp++
				}
			}
		}
		if em == 0 || total == 0 {
			continue
		}
		p := float64(tp) / float64(em)
		r := float64(tp) / float64(total)
		if p+r == 0 {
			continue
		}
		f1 := 2 * p * r / (p + r)
		if f1 > best {
			best, bestT = f1, t
		}
	}
	return bestT
}
