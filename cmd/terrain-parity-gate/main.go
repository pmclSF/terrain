// terrain-parity-gate reads the parity rubric + current scores and emits
// a human-readable matrix plus a pass/fail verdict against per-pillar
// floor requirements.
//
// This is the machine-readable enforcement of the parity gate defined in
// `docs/release/0.2.x-maturity-audit.md`. The audit doc is the human-
// readable companion; this tool is what `make pillar-parity` runs and
// what CI uses as a hard gate.
//
// Inputs (defaults; override with --rubric / --scores):
//
//	docs/release/parity/rubric.yaml — pillars / areas / axes / floors / uniformity gates
//	docs/release/parity/scores.yaml — current per-cell scores with evidence
//
// Output modes:
//
//	default       — pretty-print matrix + per-pillar verdict to stdout
//	--json        — emit a single JSON object with the same content
//	--floor-map   — only the per-area / per-pillar floor map (compact)
//
// Exit codes:
//
//	0 — every pillar at or above its floor (release-gate clears)
//	1 — at least one pillar below its hard-gate floor (release blocked)
//	2 — usage error (missing files, malformed YAML)
//
// Soft gates (e.g. "align" in 0.2.0) print a WARN banner but do not fail.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// ── Rubric types (mirror the YAML shape) ─────────────────────────────

type pillar struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	Job             string `yaml:"job"`
	ExternalFraming string `yaml:"external_framing"`
	Priority        int    `yaml:"priority"`
}

type area struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Pillar      string `yaml:"pillar"`
	Tier        int    `yaml:"tier"`
	Surface     string `yaml:"surface"`
	Description string `yaml:"description"`
}

type axis struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Lens        string            `yaml:"lens"` // product | engineering | visual
	Description string            `yaml:"description"`
	Levels      map[string]string `yaml:"levels"`
}

type uniformityGate struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	AppliesTo   string `yaml:"applies_to"`
	VerifiedBy  string `yaml:"verified_by"`
	Blocking    bool   `yaml:"blocking"`
}

type rubric struct {
	SchemaVersion   string           `yaml:"schema_version"`
	Pillars         []pillar         `yaml:"pillars"`
	PillarFloors    map[string]int   `yaml:"pillar_floors"`
	SoftGates       []string         `yaml:"soft_gates"`
	Areas           []area           `yaml:"areas"`
	Axes            []axis           `yaml:"axes"`
	UniformityGates []uniformityGate `yaml:"uniformity_gates"`
}

// ── Score types ──────────────────────────────────────────────────────

type cellScore struct {
	Score    int    `yaml:"score"`
	Evidence string `yaml:"evidence"`
}

type scores struct {
	SchemaVersion         string                          `yaml:"schema_version"`
	CapturedAt            string                          `yaml:"captured_at"`
	CapturedAgainstCommit string                          `yaml:"captured_against_commit"`
	Scores                map[string]map[string]cellScore `yaml:"scores"`
}

// ── Computed verdict ─────────────────────────────────────────────────

type pillarVerdict struct {
	Pillar      string `json:"pillar"`
	Floor       int    `json:"floor"`
	Required    int    `json:"required"`
	Soft        bool   `json:"soft"`
	Status      string `json:"status"` // PASS | FAIL | WARN
	WeakestArea string `json:"weakestArea,omitempty"`
	WeakestAxis string `json:"weakestAxis,omitempty"`
}

type areaVerdict struct {
	Area        string         `json:"area"`
	Pillar      string         `json:"pillar"`
	Floor       int            `json:"floor"`
	Cells       map[string]int `json:"cells"`
	WeakestAxes []string       `json:"weakestAxes,omitempty"`
}

type report struct {
	SchemaVersion         string          `json:"schemaVersion"`
	CapturedAt            string          `json:"capturedAt"`
	CapturedAgainstCommit string          `json:"capturedAgainstCommit"`
	OverallStatus         string          `json:"overallStatus"`
	Pillars               []pillarVerdict `json:"pillars"`
	Areas                 []areaVerdict   `json:"areas"`
}

// ── Entry point ──────────────────────────────────────────────────────

func main() {
	rubricPath := flag.String("rubric", "docs/release/parity/rubric.yaml", "path to rubric.yaml")
	scoresPath := flag.String("scores", "docs/release/parity/scores.yaml", "path to scores.yaml")
	jsonOut := flag.Bool("json", false, "emit JSON instead of human-readable matrix")
	floorMap := flag.Bool("floor-map", false, "emit only the floor map (per-area + per-pillar)")
	flag.Parse()

	r, err := loadRubric(*rubricPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	s, err := loadScores(*scoresPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	if err := validate(r, s); err != nil {
		fmt.Fprintf(os.Stderr, "error: rubric/scores validation failed: %v\n", err)
		os.Exit(2)
	}

	rep := buildReport(r, s)

	switch {
	case *jsonOut:
		emitJSON(os.Stdout, rep)
	case *floorMap:
		emitFloorMap(os.Stdout, rep)
	default:
		emitMatrix(os.Stdout, r, s, rep)
	}

	os.Exit(exitCode(rep))
}

// ── Loading ──────────────────────────────────────────────────────────

func loadRubric(path string) (*rubric, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rubric %q: %w", path, err)
	}
	var r rubric
	if err := yaml.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse rubric %q: %w", path, err)
	}
	return &r, nil
}

func loadScores(path string) (*scores, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scores %q: %w", path, err)
	}
	var s scores
	if err := yaml.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("parse scores %q: %w", path, err)
	}
	return &s, nil
}

// validate enforces structural invariants that the YAML schemas don't
// catch on their own: every area in scores has a corresponding rubric
// entry; every cell is scored; every score is in 1..5; pillar
// references are valid.
func validate(r *rubric, s *scores) error {
	areaIDs := map[string]string{} // areaID → pillarID
	for _, a := range r.Areas {
		areaIDs[a.ID] = a.Pillar
	}
	axisIDs := map[string]bool{}
	for _, a := range r.Axes {
		axisIDs[a.ID] = true
	}
	pillarIDs := map[string]bool{}
	for _, p := range r.Pillars {
		pillarIDs[p.ID] = true
	}

	for _, a := range r.Areas {
		// cross_cutting is allowed even though it isn't a numbered
		// pillar — distribution lives there.
		if a.Pillar != "cross_cutting" && !pillarIDs[a.Pillar] {
			return fmt.Errorf("area %q references unknown pillar %q", a.ID, a.Pillar)
		}
	}

	// Every scored area must be in the rubric.
	for areaID := range s.Scores {
		if _, ok := areaIDs[areaID]; !ok {
			return fmt.Errorf("scored area %q is not in rubric", areaID)
		}
	}
	// Every rubric area must be scored, and every cell must be in 1..5.
	for areaID := range areaIDs {
		areaScores, ok := s.Scores[areaID]
		if !ok {
			return fmt.Errorf("rubric area %q has no scores", areaID)
		}
		for axisID := range axisIDs {
			c, ok := areaScores[axisID]
			if !ok {
				return fmt.Errorf("area %q axis %q is not scored", areaID, axisID)
			}
			if c.Score < 1 || c.Score > 5 {
				return fmt.Errorf("area %q axis %q score %d is out of range [1,5]", areaID, axisID, c.Score)
			}
		}
	}
	return nil
}

// ── Verdict computation ──────────────────────────────────────────────

func buildReport(r *rubric, s *scores) *report {
	rep := &report{
		SchemaVersion:         s.SchemaVersion,
		CapturedAt:            s.CapturedAt,
		CapturedAgainstCommit: s.CapturedAgainstCommit,
	}

	// Per-area floor map.
	areasByPillar := map[string][]areaVerdict{}
	for _, a := range r.Areas {
		areaScores := s.Scores[a.ID]
		floor, weakest := lowestCell(areaScores)
		cells := map[string]int{}
		for axisID, cs := range areaScores {
			cells[axisID] = cs.Score
		}
		av := areaVerdict{
			Area:        a.ID,
			Pillar:      a.Pillar,
			Floor:       floor,
			Cells:       cells,
			WeakestAxes: weakest,
		}
		rep.Areas = append(rep.Areas, av)
		areasByPillar[a.Pillar] = append(areasByPillar[a.Pillar], av)
	}
	sort.Slice(rep.Areas, func(i, j int) bool { return rep.Areas[i].Area < rep.Areas[j].Area })

	// Per-pillar verdict.
	soft := map[string]bool{}
	for _, p := range r.SoftGates {
		soft[p] = true
	}
	overall := "PASS"
	for _, p := range r.Pillars {
		areas := areasByPillar[p.ID]
		if len(areas) == 0 {
			continue
		}
		floor := 5
		var weakestArea, weakestAxis string
		for _, av := range areas {
			if av.Floor < floor {
				floor = av.Floor
				weakestArea = av.Area
				if len(av.WeakestAxes) > 0 {
					weakestAxis = av.WeakestAxes[0]
				}
			}
		}
		required := r.PillarFloors[p.ID]
		status := "PASS"
		if floor < required {
			if soft[p.ID] {
				status = "WARN"
				if overall == "PASS" {
					overall = "WARN"
				}
			} else {
				status = "FAIL"
				overall = "FAIL"
			}
		}
		rep.Pillars = append(rep.Pillars, pillarVerdict{
			Pillar:      p.ID,
			Floor:       floor,
			Required:    required,
			Soft:        soft[p.ID],
			Status:      status,
			WeakestArea: weakestArea,
			WeakestAxis: weakestAxis,
		})
	}
	rep.OverallStatus = overall
	return rep
}

func lowestCell(scores map[string]cellScore) (int, []string) {
	floor := 5
	for _, c := range scores {
		if c.Score < floor {
			floor = c.Score
		}
	}
	var weakest []string
	for axisID, c := range scores {
		if c.Score == floor {
			weakest = append(weakest, axisID)
		}
	}
	sort.Strings(weakest)
	return floor, weakest
}

// ── Output ───────────────────────────────────────────────────────────

func emitJSON(w io.Writer, rep *report) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

func emitFloorMap(w io.Writer, rep *report) {
	fmt.Fprintln(w, "Per-pillar floor:")
	for _, pv := range rep.Pillars {
		marker := pv.Status
		fmt.Fprintf(w, "  %-12s floor=%d required=%d  %s", pv.Pillar, pv.Floor, pv.Required, marker)
		if pv.Soft && pv.Status == "WARN" {
			fmt.Fprint(w, " (soft)")
		}
		if pv.Status != "PASS" && pv.WeakestArea != "" {
			fmt.Fprintf(w, "  weakest=%s/%s", pv.WeakestArea, pv.WeakestAxis)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Overall: %s\n", rep.OverallStatus)
}

func emitMatrix(w io.Writer, r *rubric, s *scores, rep *report) {
	fmt.Fprintln(w, "Terrain parity gate")
	fmt.Fprintln(w, "===================")
	fmt.Fprintf(w, "Captured: %s (commit %s)\n", rep.CapturedAt, rep.CapturedAgainstCommit)
	fmt.Fprintln(w)

	// Order axes for the column header: P1..P7, E1..E7, V1..V3.
	var axisIDs []string
	for _, a := range r.Axes {
		axisIDs = append(axisIDs, a.ID)
	}
	sort.Slice(axisIDs, func(i, j int) bool {
		return axisOrderKey(axisIDs[i]) < axisOrderKey(axisIDs[j])
	})

	// Group areas by pillar, and order pillars by priority.
	pillarOrder := make([]string, len(r.Pillars))
	for i, p := range r.Pillars {
		pillarOrder[i] = p.ID
	}
	sort.SliceStable(pillarOrder, func(i, j int) bool {
		var pi, pj int
		for _, p := range r.Pillars {
			if p.ID == pillarOrder[i] {
				pi = p.Priority
			}
			if p.ID == pillarOrder[j] {
				pj = p.Priority
			}
		}
		return pi < pj
	})

	areaByID := map[string]area{}
	for _, a := range r.Areas {
		areaByID[a.ID] = a
	}

	// Header row.
	fmt.Fprintf(w, "%-32s ", "Area")
	for _, id := range axisIDs {
		fmt.Fprintf(w, "%-3s ", id)
	}
	fmt.Fprintln(w, " floor")
	fmt.Fprintln(w, repeatStr("-", 32+len(axisIDs)*4+8))

	// Rows grouped by pillar.
	for _, pillarID := range append(pillarOrder, "cross_cutting") {
		var rowsInPillar []area
		for _, a := range r.Areas {
			if a.Pillar == pillarID {
				rowsInPillar = append(rowsInPillar, a)
			}
		}
		if len(rowsInPillar) == 0 {
			continue
		}
		sort.Slice(rowsInPillar, func(i, j int) bool { return rowsInPillar[i].ID < rowsInPillar[j].ID })

		// Pillar separator.
		fmt.Fprintf(w, "[ %s ]\n", pillarLabel(r, pillarID))
		for _, a := range rowsInPillar {
			areaScores := s.Scores[a.ID]
			fmt.Fprintf(w, "  %-30s ", truncate(a.Name, 30))
			for _, axisID := range axisIDs {
				c := areaScores[axisID]
				marker := scoreMarker(c.Score)
				fmt.Fprintf(w, "%s%d  ", marker, c.Score)
			}
			floor, _ := lowestCell(areaScores)
			fmt.Fprintf(w, "  %d\n", floor)
		}
	}
	fmt.Fprintln(w)

	// Per-pillar verdict.
	fmt.Fprintln(w, "Pillar verdict")
	fmt.Fprintln(w, "--------------")
	for _, pv := range rep.Pillars {
		marker := pv.Status
		if pv.Soft && pv.Status == "WARN" {
			marker = "WARN (soft — does not block release)"
		}
		fmt.Fprintf(w, "  %-12s floor=%d / required=%d   %s\n", pv.Pillar, pv.Floor, pv.Required, marker)
		if pv.Status != "PASS" && pv.WeakestArea != "" {
			fmt.Fprintf(w, "                 weakest cell: %s / %s\n", pv.WeakestArea, pv.WeakestAxis)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Overall: %s\n", rep.OverallStatus)
}

func exitCode(rep *report) int {
	if rep.OverallStatus == "FAIL" {
		return 1
	}
	return 0
}

// ── Small helpers ────────────────────────────────────────────────────

func axisOrderKey(id string) int {
	// P1..P7 → 100..107; E1..E7 → 200..207; V1..V3 → 300..302.
	if len(id) < 2 {
		return 999
	}
	base := 0
	switch id[0] {
	case 'P':
		base = 100
	case 'E':
		base = 200
	case 'V':
		base = 300
	default:
		base = 900
	}
	n := 0
	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return base + n
}

func pillarLabel(r *rubric, id string) string {
	for _, p := range r.Pillars {
		if p.ID == id {
			return p.Name
		}
	}
	if id == "cross_cutting" {
		return "Cross-cutting"
	}
	return id
}

func scoreMarker(score int) string {
	// Three-state marker: ≥4 = strong, 3 = workable, ≤2 = below floor.
	// When the design tokens land (Track 10.1), this becomes a token
	// reference rather than ad-hoc characters.
	switch {
	case score >= 4:
		return " "
	case score == 3:
		return "·"
	default:
		return "!"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func repeatStr(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
