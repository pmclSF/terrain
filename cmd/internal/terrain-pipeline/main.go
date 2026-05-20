// Command terrain-pipeline is a validation harness for the
// internal/aipipeline package. It runs the full pipeline against a
// labeled corpus (GPT-rated detector hits) and reports precision,
// effective usefulness, atom volume, and per-category breakdown.
//
// Usage:
//
//	terrain-pipeline validate \
//	    --labels /tmp/gpt-labels.tsv \
//	    --files-v2 /tmp/sample-files-v2 \
//	    --files-v3 /tmp/sample-files-v3
//
// The harness is the regression test that ratifies pipeline changes.
// Every modification to a stage, atom weight, or calibration row should
// be compared against the baseline precision (2.72% path-only, 10.29%
// regex-v2 simulator) before merging.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipipeline/fixscaffold"
	"github.com/pmclSF/terrain/internal/aipipeline/stages"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "validate":
		runValidate(os.Args[2:])
	case "debug":
		runDebug(os.Args[2:])
	case "tune":
		runTune(os.Args[2:])
	case "atoms":
		runAtoms(os.Args[2:])
	case "cv":
		runCV(os.Args[2:])
	case "fit":
		runFit(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `terrain-pipeline — validation harness for internal/aipipeline

Subcommands:
  validate    Run the pipeline against a labeled corpus and report metrics.
  debug       Print per-row evaluation (atoms, log-odds, confidence, emit-decision).
              Useful for inspecting missing TPs and emitted FPs.
  tune        Sweep composer threshold values and report precision at each
              cut. Helps locate the right operating point against the corpus.
  atoms       Per-atom marginal TP rate + calibration weight check. Surfaces
              atoms whose hand-tuned weight is misaligned with empirical lift.
  cv          k-fold cross-validation against the current calibration. Reports
              per-fold precision/recall/F1 + Wilson CI on aggregate precision.
  fit         k-fold logistic-regression refit. Trains weights per fold and
              evaluates on the held-out fold — the proper out-of-sample check.

Run "terrain-pipeline <subcommand> --help" for subcommand flags.
`)
}

// ── validate subcommand ─────────────────────────────────────────────

type validateFlags struct {
	labelsPath  string
	filesV1Dir  string
	filesV2Dir  string
	filesV3Dir  string
	appShapeTxt string
	rule        string
	posture     string
	maxRows     int
}

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	var f validateFlags
	fs.StringVar(&f.labelsPath, "labels", "/tmp/gpt-labels.tsv",
		"path to the labeled corpus TSV")
	fs.StringVar(&f.filesV1Dir, "files-v1", "/tmp/sample-files",
		"root of the v1 cached-file tree")
	fs.StringVar(&f.filesV2Dir, "files-v2", "/tmp/sample-files-v2",
		"root of the v2 cached-file tree")
	fs.StringVar(&f.filesV3Dir, "files-v3", "/tmp/sample-files-v3",
		"root of the v3 cached-file tree")
	fs.StringVar(&f.appShapeTxt, "app-shape",
		"/Users/pzachary/terrain/tier-4/app-shaped-repos.txt",
		"one repo per line; rows in these repos get cohort=ai-feature-in-app, others cohort=library-sdk")
	fs.StringVar(&f.rule, "rule", "ai.surface.missing_eval",
		"rule ID to evaluate against")
	fs.StringVar(&f.posture, "posture", "observability",
		"posture: observability | gate")
	fs.IntVar(&f.maxRows, "max", 0,
		"max rows to process (0 = all)")
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

	posture := aipipeline.PostureObservability
	if f.posture == "gate" {
		posture = aipipeline.PostureGate
	}

	cal := aipipeline.DefaultCalibration()
	scaffolds := fixscaffold.NewRegistryAdapter(fixscaffold.NewRegistry())
	composer := aipipeline.NewComposer(cal, posture)
	composer.Scaffolds = scaffolds

	pipeline := aipipeline.NewPipeline(composer,
		stages.NewPathPrefilter(),
		stages.NewRegexFastscan(),
		stages.NewASTConfirm(),
		stages.NewCrossFileScope(nil), // corpus has no sibling files
		stages.NewChangeScope(),
	)

	report := evaluate(pipeline, composer, rows, f.rule)
	printReport(report, f)
}

// ── corpus loader ───────────────────────────────────────────────────

type labelRow struct {
	detector  string
	idx       string
	repo      string
	path      string
	label     string
	conf      string
	reason    string
	rawJSON   string
	sampleSet string
	src       []byte
	cohort    string
}

func loadLabels(labelsPath, filesV1, filesV2, filesV3 string) ([]labelRow, error) {
	f, err := os.Open(labelsPath)
	if err != nil {
		return nil, fmt.Errorf("open labels: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1 // tolerate variable widths
	r.LazyQuotes = true

	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	var rows []labelRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Soft-fail on malformed lines so a single bad row doesn't
			// poison the whole validation pass.
			continue
		}
		if len(rec) < 9 {
			continue
		}
		if rec[4] == "API_ERROR" {
			continue
		}
		row := labelRow{
			detector:  rec[0],
			idx:       rec[1],
			repo:      rec[2],
			path:      rec[3],
			label:     rec[4],
			conf:      rec[5],
			reason:    rec[6],
			rawJSON:   rec[7],
			sampleSet: rec[8],
		}
		row.src = readCached(row, filesV1, filesV2, filesV3)
		rows = append(rows, row)
	}
	return rows, nil
}

func readCached(row labelRow, v1, v2, v3 string) []byte {
	cleanIdx := strings.TrimLeft(row.idx, "0")
	if cleanIdx == "" {
		cleanIdx = "0"
	}
	idx, err := strconv.Atoi(cleanIdx)
	if err != nil {
		return nil
	}
	// v1 uses 3-digit zero-padded filenames; v2 and v3 use 4-digit.
	var root, name string
	switch row.sampleSet {
	case "v1":
		root, name = v1, fmt.Sprintf("%03d.txt", idx)
	case "v2":
		root, name = v2, fmt.Sprintf("%04d.txt", idx)
	case "v3":
		root, name = v3, fmt.Sprintf("%04d.txt", idx)
	default:
		// Fall back to v2 path shape for unknown sample sets.
		root, name = v2, fmt.Sprintf("%04d.txt", idx)
	}
	path := filepath.Join(root, row.detector, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if string(data) == "FETCH_FAILED" {
		return nil
	}
	return data
}

// ── evaluation ──────────────────────────────────────────────────────

type report struct {
	posture    aipipeline.Posture
	rule       string
	totalRows  int
	emitted    int
	tps        int
	suppressed int
	noContent  int
	byLabel    map[string]int

	// "Effective usefulness" per the verdict-quality review:
	// TPs + informative FPs. Approximated here by counting any label
	// in the informative set as "useful".
	effectiveUseful int

	// Per-cohort breakdown — populated when rows carry cohort labels.
	byCohort map[string]*cohortStats

	// Per-rule breakdown — surface (LLM eval gaps) vs train (ML
	// tracker gaps) have different signal profiles and different
	// product implications; splitting them is useful for tuning.
	byRule map[string]*ruleStats
}

type cohortStats struct {
	totalRows int
	emitted   int
	tps       int
}

type ruleStats struct {
	totalRows int
	emitted   int
	tps       int
}

func evaluate(p *aipipeline.Pipeline, comp *aipipeline.Composer, rows []labelRow, rule string) report {
	rep := report{
		posture:  comp.Posture,
		rule:     rule,
		byLabel:  map[string]int{},
		byCohort: map[string]*cohortStats{},
		byRule:   map[string]*ruleStats{},
	}
	getCohort := func(c string) *cohortStats {
		if rep.byCohort[c] == nil {
			rep.byCohort[c] = &cohortStats{}
		}
		return rep.byCohort[c]
	}
	getRule := func(r string) *ruleStats {
		if rep.byRule[r] == nil {
			rep.byRule[r] = &ruleStats{}
		}
		return rep.byRule[r]
	}
	for _, row := range rows {
		// Route the candidate to the rule that matches the row's detector.
		// Train rows want ai.train.missing_tracker; surface rows want
		// ai.surface.missing_eval. Forcing every row through the same
		// rule wrongly cross-applies AST verification.
		effectiveRule := rule
		if row.detector == "train" {
			effectiveRule = "ai.train.missing_tracker"
		} else if row.detector == "surface" {
			effectiveRule = "ai.surface.missing_eval"
		}
		cohort := cohortForRow(row)
		cs := getCohort(cohort)
		cs.totalRows++
		rs := getRule(effectiveRule)
		rs.totalRows++
		cand := &aipipeline.Candidate{
			Path:   row.path,
			Lang:   string(aipipeline.LanguageFromPath(row.path)),
			RuleID: effectiveRule,
			Cohort: cohort,
			Src:    row.src,
		}
		if len(row.src) == 0 {
			rep.noContent++
		}
		_, ok := p.Run(context.Background(), cand)
		if !ok {
			// dropped before composition (e.g. hard-dropped by path filter)
			rep.suppressed++
			continue
		}
		f := comp.Compose(cand)
		if !comp.ShouldEmit(f) {
			rep.suppressed++
			continue
		}
		rep.emitted++
		cs.emitted++
		rs.emitted++
		rep.byLabel[row.label]++
		if row.label == "TP" {
			rep.tps++
			cs.tps++
			rs.tps++
		}
		if isInformativeLabel(row.label) {
			rep.effectiveUseful++
		}
	}
	rep.totalRows = len(rows) // overall denominator
	return rep
}

func cohortForRow(row labelRow) string {
	if row.cohort != "" {
		return row.cohort
	}
	return "unknown"
}

// assignCohorts annotates each row with a cohort label using the
// app-shape filter output from the internal calibration corpus. Repos
// in that filter get cohort=ai-feature-in-app (the dominant app cohort);
// others get cohort=library-sdk. This approximates the binary
// production-vs-framework split that the calibration table cares
// about.
//
// Future iterations should run DetectCohortFromDir on cloned repos to
// produce the full per-cohort labels (rag-app, agent-app, ml-pipeline,
// notebook-heavy, ai-feature-in-app, library-sdk).
func assignCohorts(rows []labelRow, appShapePath string) error {
	appShape := make(map[string]bool)
	if appShapePath != "" {
		data, err := os.ReadFile(appShapePath)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					appShape[line] = true
				}
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for i := range rows {
		if appShape[rows[i].repo] {
			rows[i].cohort = string(aipipeline.CohortAIFeatureInApp)
		} else {
			rows[i].cohort = string(aipipeline.CohortLibrarySDK)
		}
	}
	return nil
}

// isInformativeLabel approximates the verdict-quality "informative FP"
// classification. A label is treated as informative when a developer
// would plausibly take useful action even though the finding isn't a
// strict TP. The list below mirrors the categories in the FP study.
func isInformativeLabel(label string) bool {
	switch label {
	case "TP":
		return true
	case "FP-not-actually":
		// these are wrappers/factories where awareness is still useful
		return false
	}
	return false
}

// ── reporting ───────────────────────────────────────────────────────

func printReport(r report, f validateFlags) {
	fmt.Printf("\nterrain-pipeline validation report\n")
	fmt.Printf("==================================\n")
	fmt.Printf("rule:        %s\n", f.rule)
	fmt.Printf("posture:     %s\n", r.posture)
	fmt.Printf("rows total:  %d\n", r.totalRows)
	fmt.Printf("no content:  %d\n", r.noContent)
	fmt.Printf("suppressed:  %d (dropped or below threshold)\n", r.suppressed)
	fmt.Printf("emitted:     %d\n", r.emitted)
	prec := 0.0
	if r.emitted > 0 {
		prec = 100.0 * float64(r.tps) / float64(r.emitted)
	}
	fmt.Printf("TPs emitted: %d\n", r.tps)
	fmt.Printf("precision:   %.2f%% (TPs / emitted)\n", prec)
	euRate := 0.0
	if r.emitted > 0 {
		euRate = 100.0 * float64(r.effectiveUseful) / float64(r.emitted)
	}
	fmt.Printf("effective usefulness: %.2f%% (approximate; informative-label fraction)\n", euRate)

	fmt.Printf("\nemitted-row label breakdown\n")
	fmt.Printf("---------------------------\n")
	labels := make([]string, 0, len(r.byLabel))
	for l := range r.byLabel {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool { return r.byLabel[labels[i]] > r.byLabel[labels[j]] })
	for _, l := range labels {
		n := r.byLabel[l]
		pct := 0.0
		if r.emitted > 0 {
			pct = 100.0 * float64(n) / float64(r.emitted)
		}
		fmt.Printf("  %4d (%5.1f%%) %s\n", n, pct, l)
	}

	if len(r.byCohort) > 0 {
		fmt.Printf("\nper-cohort precision\n")
		fmt.Printf("--------------------\n")
		cohorts := make([]string, 0, len(r.byCohort))
		for c := range r.byCohort {
			cohorts = append(cohorts, c)
		}
		sort.Strings(cohorts)
		fmt.Printf("  %-22s %8s %8s %8s %10s\n", "cohort", "rows", "emitted", "TPs", "precision")
		for _, c := range cohorts {
			cs := r.byCohort[c]
			p := 0.0
			if cs.emitted > 0 {
				p = 100.0 * float64(cs.tps) / float64(cs.emitted)
			}
			fmt.Printf("  %-22s %8d %8d %8d %9.2f%%\n",
				c, cs.totalRows, cs.emitted, cs.tps, p)
		}
	}

	if len(r.byRule) > 0 {
		fmt.Printf("\nper-rule precision\n")
		fmt.Printf("------------------\n")
		rules := make([]string, 0, len(r.byRule))
		for rl := range r.byRule {
			rules = append(rules, rl)
		}
		sort.Strings(rules)
		fmt.Printf("  %-32s %8s %8s %8s %10s\n", "rule", "rows", "emitted", "TPs", "precision")
		for _, rl := range rules {
			rs := r.byRule[rl]
			p := 0.0
			if rs.emitted > 0 {
				p = 100.0 * float64(rs.tps) / float64(rs.emitted)
			}
			fmt.Printf("  %-32s %8d %8d %8d %9.2f%%\n",
				rl, rs.totalRows, rs.emitted, rs.tps, p)
		}
	}

	fmt.Printf("\nbaselines (for reference)\n")
	fmt.Printf("-------------------------\n")
	fmt.Printf("  path-only (combined-v3):    2.72%%\n")
	fmt.Printf("  AST safe (data-agent):      4.74%%\n")
	fmt.Printf("  AST aggressive (data-agent):7.82%%\n")
	fmt.Printf("  regex-v2 (ctx-loose+neg):  10.29%%  ← Python sim\n")
}
