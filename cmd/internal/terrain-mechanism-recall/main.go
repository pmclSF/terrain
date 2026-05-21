// Command terrain-mechanism-recall computes per-mechanism recall
// accounting against the v2 validation corpus. For each Phase 2
// mechanism it applies the gate's predicate to every row in
// tier-4/detector-validation-v2-combined-good.jsonl and reports:
//
//   - TP-loss: true positives the mechanism would suppress / demote
//   - FP-gain: false positives the mechanism would suppress / demote
//   - Net precision change per detector
//
// This is the harness behind R3.8 (per-mechanism recall budgets) and
// is the gating evidence required to graduate a shadow mechanism to
// live state. Run via `make mechanism-recall` (added separately).
//
// Output: a markdown table to stdout plus a JSON detail dump to
// --out (default tier-4/mechanism-recall-report.json). The markdown
// table is the human surface; the JSON is the input to per-mechanism
// stacking analysis (R3.8 stacking).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/aliases"
	"github.com/pmclSF/terrain/internal/ascg"
	"github.com/pmclSF/terrain/internal/deffollowing"
	"github.com/pmclSF/terrain/internal/looppredicate"
	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/surfacelit"
	"github.com/pmclSF/terrain/internal/triggergate"
)

// v2Row is one rated row from
// tier-4/detector-validation-v2-combined-good.jsonl.
type v2Row struct {
	Repo        string  `json:"repo"`
	RuleID      string  `json:"rule_id"`
	File        string  `json:"file"`
	Symbol      string  `json:"symbol"`
	Line        int     `json:"line,omitempty"`
	Confidence  float64 `json:"confidence"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Evidence    string  `json:"evidence"`
	FileExcerpt string  `json:"_file_excerpt"`
	Verdict     struct {
		Verdict string `json:"verdict"` // TP or FP
		Reason  string `json:"reason"`
	} `json:"_verdict"`
}

// mechanism is a thin record describing one Phase 2 mechanism's
// applicability + predicate against a v2 row. Each predicate returns
// true when the mechanism's action would apply.
//
// action semantics:
//   - "suppress": the mechanism would drop the finding entirely.
//     TP-loss is bad (legitimate finding lost), FP-gain is good.
//     R3.8 graduation rule: FP-gain >= TP-loss AND TP-loss/n <= 5%.
//   - "demote": the mechanism would reduce severity one tier.
//     TP-loss = legitimate finding downgraded (somewhat bad);
//     FP-gain = noise downgraded (good). Same R3.8 rule.
//   - "add": the mechanism would lift an over-broad suppression
//     OR switch the signal Type (split mechanisms). For type-switch
//     splits, precision is preserved (both halves stay at gate
//     tier), so the R3.8 rule doesn't apply — verdict is reported
//     as TYPE-SWITCH instead of HOLD/GRADUATE.
type mechanism struct {
	name       string
	action     string // "suppress" | "demote" | "add"
	typeSwitch bool   // true when action=add and the mechanism preserves precision (split mechanisms)
	consumers  map[string]bool
	predicate  func(v2Row) bool
}

// counts is the per-detector recall accounting for one mechanism.
type counts struct {
	TotalRows  int
	TPTotal    int // TPs in the row pool
	FPTotal    int // FPs in the row pool
	UnkTotal   int
	Changed    int
	TPChanged  int // TP-loss when action=suppress, TP-demote when action=demote
	FPChanged  int // FP-gain when action=suppress, FP-demote when action=demote
	UnkChanged int
}

func main() {
	in := flag.String("in", "tier-4/detector-validation-v2-combined-good.jsonl", "v2 validation rows (JSONL)")
	out := flag.String("out", "tier-4/mechanism-recall-report.json", "JSON detail output")
	flag.Parse()

	rows, err := loadRows(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}

	mechs := buildMechanisms()
	report := map[string]map[string]counts{}
	for _, m := range mechs {
		perDet := map[string]counts{}
		for _, r := range rows {
			if !m.consumers[r.RuleID] {
				continue
			}
			c := perDet[r.RuleID]
			c.TotalRows++
			switch r.Verdict.Verdict {
			case "TP":
				c.TPTotal++
			case "FP":
				c.FPTotal++
			default:
				c.UnkTotal++
			}
			if m.predicate(r) {
				c.Changed++
				switch r.Verdict.Verdict {
				case "TP":
					c.TPChanged++
				case "FP":
					c.FPChanged++
				default:
					c.UnkChanged++
				}
			}
			perDet[r.RuleID] = c
		}
		report[m.name] = perDet
	}

	// JSON detail dump.
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		_ = os.WriteFile(*out, data, 0o644)
	}

	// Markdown table to stdout.
	printMarkdown(mechs, report)
}

func loadRows(path string) ([]v2Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var out []v2Row
	for {
		var r v2Row
		if err := dec.Decode(&r); err != nil {
			if err.Error() == "EOF" {
				break
			}
			// jsonl: one object per line; if Decode hits a non-EOF
			// terminator it means the file is malformed.
			break
		}
		out = append(out, r)
	}
	return out, nil
}

func buildMechanisms() []mechanism {
	// Build a registry with every mechanism flipped to On so the
	// predicates actually fire.
	reg, err := mechanisms.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load mechanisms:", err)
		os.Exit(1)
	}
	overrides := []string{}
	for _, m := range reg.All() {
		overrides = append(overrides, m.Name+"=on")
	}
	if err := reg.ApplyCLIOverrides(overrides); err != nil {
		fmt.Fprintln(os.Stderr, "override:", err)
		os.Exit(1)
	}
	mechanisms.SetDefault(reg)

	// Alias registry just to confirm it loads without error; not
	// directly used in the harness predicates but Phase 2 expects it.
	if _, err := aliases.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "alias load:", err)
	}

	return []mechanism{
		{
			name:   "surface_literal_presence_gate",
			action: "suppress",
			consumers: map[string]bool{
				"aiModelDeprecationRisk": true,
				"aiPromptVersioning":     true,
				"aiEmbeddingModelChange": true,
				"aiSafetyEvalMissing":    true,
				"aiToolWithoutSandbox":   true,
			},
			predicate: func(r v2Row) bool {
				name := strings.TrimSpace(r.Symbol)
				if name == "" {
					return false
				}
				// Match production: skip the presence check when the
				// surface name is a synthetic constructor-derived
				// label (handled by isSyntheticIdentifier in the
				// consumer wirings).
				if isSyntheticIdentifier(name) {
					return false
				}
				// Run the in-memory CheckBytes against the excerpt
				// so the predicate has the same shape as production.
				res := surfacelit.CheckBytes(name, []byte(r.FileExcerpt))
				return res == surfacelit.Absent
			},
		},
		{
			name:   "deprecated_test_pattern_trigger_gate",
			action: "suppress",
			consumers: map[string]bool{
				"deprecatedTestPattern": true,
			},
			predicate: func(r v2Row) bool {
				// Only the enzyme-usage sub-rule is gated. We can't
				// recover the sub-rule from the row, so approximate
				// by looking for "enzyme" in the title/description.
				if !strings.Contains(strings.ToLower(r.Title+" "+r.Description), "enzyme") {
					return false
				}
				return !triggergate.ImportsFromBytes([]byte(r.FileExcerpt), []string{"enzyme", "enzyme-adapter-*"})
			},
		},
		{
			name:   "ascg_live_vs_catalog",
			action: "demote",
			consumers: map[string]bool{
				"aiNonDeterministicEval": true,
			},
			predicate: func(r v2Row) bool {
				cls := ascg.Classify(ascg.Location{
					Path:     r.File,
					Line:     r.Line,
					FileBody: r.FileExcerpt,
				})
				return cls.Class == ascg.CatalogOrExample
			},
		},
		{
			name:   "a3_loop_predicate",
			action: "suppress",
			consumers: map[string]bool{
				"dynamicTestGeneration": true,
			},
			predicate: func(r v2Row) bool {
				if r.Line == 0 {
					return false
				}
				inLoop := looppredicate.IsTestBuilderInLoopBytes([]byte(r.FileExcerpt), r.Line)
				return !inLoop
			},
		},
		{
			name:   "a1_def_following",
			action: "demote",
			consumers: map[string]bool{
				"assertionFreeImport": true,
				"assertionFreeTest":   true,
				"weakAssertion":       true,
			},
			predicate: func(r v2Row) bool {
				// a1 lifts the assertion count via transitive in-repo
				// helper bodies. We don't have the full repo from the
				// excerpt, so we measure only the within-excerpt
				// transitive count — understates production effect.
				c := deffollowing.NewCounter("/dev/null")
				return c.CountTransitive(r.FileExcerpt, deffollowing.MaxDepth) > 0
			},
		},
		{
			// a7_barrel_resolver: resolves Jest path aliases, dist-
			// path indirection, and Python namespace re-exports. The
			// predicate needs a full-repo import-graph; v2 excerpts
			// don't carry that. Harness reports NO SIGNAL — this
			// mechanism graduates via a different validation path
			// (frozen regression suite on the 141 v2 TPs of
			// untestedExport, per master plan R3.4).
			name:   "a7_barrel_resolver",
			action: "add",
			consumers: map[string]bool{
				"untestedExport":   true,
				"orphanedTestFile": true,
			},
			predicate: func(r v2Row) bool { return false },
		},
		{
			// ehr_surfaces_covered: needs structured eval-config
			// parsing of the actual eval files in the repo (Report
			// shape). v2 excerpts of the FLAGGED file don't give
			// us those. Harness reports NO SIGNAL — graduates via
			// a dedicated ehr-against-eval-configs harness.
			name:   "ehr_surfaces_covered",
			action: "add",
			consumers: map[string]bool{
				"aiSafetyEvalMissing": true,
			},
			predicate: func(r v2Row) bool { return false },
		},
		{
			// runtime_config_recognizer: structural recognizer of
			// runtime config files (YAML/properties with loader
			// reachability). Needs the repo-side loader-reach graph
			// to fire. Excerpt-only harness has no signal.
			name:   "runtime_config_recognizer",
			action: "demote",
			consumers: map[string]bool{
				"aiNonDeterministicEval": true,
			},
			predicate: func(r v2Row) bool { return false },
		},
		{
			// static_skipped_test_split: changes the emitted Type
			// rather than precision. The v2 corpus has legacy
			// `staticSkippedTest` rows only; the split-half
			// allocation is observable by checking each row's file
			// excerpt for the gate-predicate pattern (env var,
			// pytest.mark.skipif, etc.). Both split halves remain
			// at gate tier, so precision is preserved; "Changed"
			// rows here just emit a different Type, not a lost TP.
			name:       "static_skipped_test_split",
			action:     "add",
			typeSwitch: true,
			consumers: map[string]bool{
				"staticSkippedTest": true,
			},
			predicate: func(r v2Row) bool {
				// "Changed" here means: this row would emit the
				// conditional-gate variant (file has a gate
				// predicate). Otherwise it emits the unconditional
				// variant. Either way both halves remain at gate
				// tier, so the split is precision-neutral.
				body := r.FileExcerpt
				if body == "" {
					return false
				}
				return staticSkipGatePredicateRe.MatchString(body)
			},
		},
		{
			// deps_drift_risk_split: ditto — emits caret-policy vs
			// strict-pin based on which moving-target class dominates.
			// Precision-neutral; both halves stay at gate tier.
			name:       "deps_drift_risk_split",
			action:     "add",
			typeSwitch: true,
			consumers: map[string]bool{
				"depsDriftRisk": true,
			},
			predicate: func(r v2Row) bool {
				// "Changed" = this row would emit caret-policy.
				body := r.FileExcerpt
				if body == "" {
					return false
				}
				return strings.Contains(body, "^") && strings.Contains(body, ".")
			},
		},
	}
}

// staticSkipGatePredicateRe mirrors the StaticSkipDetector
// gate-predicate regex (internal/quality/static_skip.go) so the
// harness can model the split mechanism's allocation.
var staticSkipGatePredicateRe = regexp.MustCompile(
	`(?i)` +
		`process\.env\.[A-Z_]+|` +
		`os\.environ\b|os\.getenv\(|` +
		`@\s*pytest\.mark\.skipif\b|` +
		`@\s*unittest\.skipIf\b|` +
		`@\s*Skip[A-Z]\w*\b|` +
		`if\s+__name__\b|` +
		`platform\.(system|machine|python_implementation)\b|` +
		`feature[_]?flag\b|featureFlag\b|` +
		`os\.Getenv\(`)

func printMarkdown(mechs []mechanism, report map[string]map[string]counts) {
	fmt.Println("# Per-mechanism recall accounting (v2 corpus)")
	fmt.Println()
	fmt.Println("Each row: applying the mechanism's predicate at state=on against the v2 baseline (`tier-4/detector-validation-v2-combined-good.jsonl`).")
	fmt.Println()
	fmt.Println("| Mechanism | Action | Detector | n | Changed | TP-loss | FP-gain | UNK | Prec before | Prec after |")
	fmt.Println("|---|---|---|---:|---:|---:|---:|---:|---:|---:|")

	for _, m := range mechs {
		perDet := report[m.name]
		// Stable order: alphabetical by detector.
		var dets []string
		for k := range perDet {
			dets = append(dets, k)
		}
		sort.Strings(dets)
		for _, det := range dets {
			c := perDet[det]
			if c.TotalRows == 0 {
				continue
			}
			// Precision before is over the full row pool. Precision
			// after removes the Changed rows (suppress: drop both
			// TPs and FPs that fire; demote: model the same for
			// precision-comparison purposes — operationally a demote
			// keeps the finding but the detector's gate-tier
			// precision is computed from the kept-at-tier subset).
			tpAfter := c.TPTotal - c.TPChanged
			fpAfter := c.FPTotal - c.FPChanged
			precBefore := safeDiv(c.TPTotal, c.TPTotal+c.FPTotal)
			precAfter := safeDiv(tpAfter, tpAfter+fpAfter)
			fmt.Printf("| %s | %s | %s | %d | %d | %d | %d | %d | %.1f%% | %.1f%% |\n",
				m.name, m.action, det,
				c.TotalRows, c.Changed, c.TPChanged, c.FPChanged, c.UnkChanged,
				precBefore*100, precAfter*100,
			)
		}
	}

	fmt.Println()
	fmt.Println("**Reading the table:**")
	fmt.Println("- `n` = rows in the v2 baseline tagged with the detector AND in the mechanism's consumer list.")
	fmt.Println("- `Changed` = rows where the mechanism's predicate fires (action would apply at state=on).")
	fmt.Println("- `TP-loss` = changed rows whose verdict was TP. These are legitimate findings the mechanism would suppress / demote.")
	fmt.Println("- `FP-gain` = changed rows whose verdict was FP. These are the noise the mechanism removes.")
	fmt.Println("- `Prec after` = precision on the cohort after applying the mechanism (TPs and FPs that stayed).")
	fmt.Println()

	// Graduation summary: per-mechanism verdict against R3.8 rule.
	fmt.Println("## Graduation rule check (R3.8)")
	fmt.Println()
	fmt.Println("A mechanism graduates to `state: on` when, for every consumer detector with v2 data:")
	fmt.Println("- `FP-gain >= TP-loss` (net precision improvement), AND")
	fmt.Println("- `TP-loss / n <= 0.05` (recall budget — no more than 5% of the pool is sacrificed).")
	fmt.Println()
	fmt.Println("| Mechanism | Verdict | Failing detectors |")
	fmt.Println("|---|---|---|")
	for _, m := range mechs {
		perDet := report[m.name]
		failing := []string{}
		anyData := false
		anyChange := false
		for det, c := range perDet {
			if c.TotalRows == 0 {
				continue
			}
			anyData = true
			if c.Changed > 0 {
				anyChange = true
			}
			// Type-switch mechanisms preserve precision (both halves
			// stay at gate tier), so the R3.8 FP-gain >= TP-loss rule
			// doesn't apply. Skip the failure check for them.
			if m.typeSwitch {
				continue
			}
			recallLoss := float64(c.TPChanged) / float64(c.TotalRows)
			if c.FPChanged < c.TPChanged || recallLoss > 0.05 {
				failing = append(failing, fmt.Sprintf("%s (loss %.1f%%, TP=%d FP=%d)", det, recallLoss*100, c.TPChanged, c.FPChanged))
			}
		}
		sort.Strings(failing)
		var verdict, fails string
		switch {
		case !anyData:
			verdict = "NO V2 DATA"
			fails = "no v2 rows for any consumer"
		case m.typeSwitch && anyChange:
			verdict = "TYPE-SWITCH OK"
			fails = "split halves stay at gate tier — precision preserved"
		case len(failing) > 0:
			verdict = "HOLD"
			fails = strings.Join(failing, "; ")
		case !anyChange:
			// All consumers have v2 rows but the predicate never
			// fires — the harness has no signal to validate
			// graduation. Common when the v2 excerpt is too short or
			// the row lacks line numbers the gate needs.
			verdict = "INSUFFICIENT EVIDENCE"
			fails = "predicate never fires on v2 rows (excerpt too narrow or missing line numbers)"
		default:
			verdict = "GRADUATE"
		}
		fmt.Printf("| %s | %s | %s |\n", m.name, verdict, fails)
	}
}

// syntheticPrefixes / syntheticExactMatches / syntheticLineSuffixRe
// mirror the production isSyntheticIdentifier check used by the AI
// detectors. Keep these in sync with
// internal/aidetect/embedding_model_change.go.
var syntheticPrefixes = []string{
	"sdk_client_", "llm_call_", "framework_msg_",
	"template_prompt_", "api_prompt_", "system_prompt_",
	"message_array_", "message_list_",
	"few_shot_", "prompt_const_", "dspy_",
}
var syntheticExactMatches = map[string]bool{
	"structured_output":   true,
	"message_slice":       true,
	"message_array":       true,
	"template_file":       true,
	"system_message":      true,
	"user_message":        true,
	"assistant_message":   true,
	"vector_store":        true,
	"vector_store_chroma": true,
	"vector_store_faiss":  true,
	"vector_store_config": true,
	"embedding_model":     true,
	"retriever_config":    true,
	"prompt_template":     true,
	"system_prompt":       true,
	"user_prompt":         true,
	"rag_pipeline":        true,
	"langchain_message":   true,
	"llamaindex_message":  true,
	"chunking_config":     true,
	"reranker_config":     true,
	"retrieval_query":     true,
	"rag_component":       true,
}

func isSyntheticIdentifier(id string) bool {
	if id == "" || strings.ContainsAny(id, "( ") {
		return true
	}
	// "_L<line>" suffix is synthesized for unnamed surfaces.
	for i := len(id) - 1; i >= 0; i-- {
		c := id[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c == 'L' && i > 0 && id[i-1] == '_' && i+1 < len(id) {
			return true
		}
		break
	}
	if syntheticExactMatches[id] {
		return true
	}
	for _, p := range syntheticPrefixes {
		if strings.HasPrefix(id, p) {
			return true
		}
	}
	return false
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
