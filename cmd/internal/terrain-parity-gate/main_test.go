package main

import (
	"strings"
	"testing"
)

func TestValidate_RejectsMissingArea(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars: []pillar{{ID: "gate"}},
		Areas: []area{
			{ID: "alpha", Pillar: "gate"},
			{ID: "beta", Pillar: "gate"},
		},
		Axes: []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {"P1": {Score: 3}},
			// "beta" missing
		},
	}
	err := validate(r, s)
	if err == nil || !strings.Contains(err.Error(), "beta") {
		t.Errorf("expected error mentioning missing area beta, got: %v", err)
	}
}

func TestValidate_RejectsUnknownArea(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars: []pillar{{ID: "gate"}},
		Areas:   []area{{ID: "alpha", Pillar: "gate"}},
		Axes:    []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha":   {"P1": {Score: 3}},
			"unknown": {"P1": {Score: 3}},
		},
	}
	err := validate(r, s)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected error mentioning unknown area, got: %v", err)
	}
}

func TestValidate_RejectsMissingAxisInArea(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars: []pillar{{ID: "gate"}},
		Areas:   []area{{ID: "alpha", Pillar: "gate"}},
		Axes:    []axis{{ID: "P1"}, {ID: "P2"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {"P1": {Score: 3}}, // P2 missing
		},
	}
	err := validate(r, s)
	if err == nil || !strings.Contains(err.Error(), "P2") {
		t.Errorf("expected error mentioning missing axis P2, got: %v", err)
	}
}

func TestValidate_RejectsOutOfRangeScore(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars: []pillar{{ID: "gate"}},
		Areas:   []area{{ID: "alpha", Pillar: "gate"}},
		Axes:    []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {"P1": {Score: 7}}, // > 5
		},
	}
	err := validate(r, s)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out-of-range error, got: %v", err)
	}
}

func TestValidate_AllowsCrossCuttingPillar(t *testing.T) {
	t.Parallel()
	// The cross_cutting "pillar" isn't a numbered pillar but is allowed
	// for the distribution area.
	r := &rubric{
		Pillars: []pillar{{ID: "gate"}},
		Areas:   []area{{ID: "dist", Pillar: "cross_cutting"}},
		Axes:    []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"dist": {"P1": {Score: 3}},
		},
	}
	if err := validate(r, s); err != nil {
		t.Errorf("expected cross_cutting to validate, got: %v", err)
	}
}

func TestBuildReport_PassWhenAllAtFloor(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars:      []pillar{{ID: "gate", Name: "Gate", Priority: 1}},
		PillarFloors: map[string]int{"gate": 4},
		Areas:        []area{{ID: "alpha", Pillar: "gate"}},
		Axes:         []axis{{ID: "P1"}, {ID: "E1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {
				"P1": {Score: 4},
				"E1": {Score: 5},
			},
		},
	}
	rep := buildReport(r, s)
	if rep.OverallStatus != "PASS" {
		t.Errorf("expected PASS, got %s", rep.OverallStatus)
	}
	if len(rep.Pillars) != 1 || rep.Pillars[0].Status != "PASS" {
		t.Errorf("expected single pillar PASS, got %+v", rep.Pillars)
	}
}

func TestBuildReport_FailWhenBelowHardFloor(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars:      []pillar{{ID: "gate", Name: "Gate", Priority: 1}},
		PillarFloors: map[string]int{"gate": 4},
		Areas:        []area{{ID: "alpha", Pillar: "gate"}},
		Axes:         []axis{{ID: "P1"}, {ID: "E1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {
				"P1": {Score: 4},
				"E1": {Score: 2}, // below floor
			},
		},
	}
	rep := buildReport(r, s)
	if rep.OverallStatus != "FAIL" {
		t.Errorf("expected FAIL, got %s", rep.OverallStatus)
	}
	if rep.Pillars[0].Status != "FAIL" {
		t.Errorf("expected pillar FAIL, got %s", rep.Pillars[0].Status)
	}
	if rep.Pillars[0].WeakestArea != "alpha" || rep.Pillars[0].WeakestAxis != "E1" {
		t.Errorf("weakest pointer wrong: area=%s axis=%s", rep.Pillars[0].WeakestArea, rep.Pillars[0].WeakestAxis)
	}
	if exitCode(rep) != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode(rep))
	}
}

func TestBuildReport_SoftGateWarns(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars:      []pillar{{ID: "align", Name: "Align", Priority: 3}},
		PillarFloors: map[string]int{"align": 3},
		SoftGates:    []string{"align"},
		Areas:        []area{{ID: "alpha", Pillar: "align"}},
		Axes:         []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {"P1": {Score: 2}}, // below soft floor
		},
	}
	rep := buildReport(r, s)
	if rep.OverallStatus != "WARN" {
		t.Errorf("expected WARN, got %s", rep.OverallStatus)
	}
	if rep.Pillars[0].Status != "WARN" {
		t.Errorf("expected pillar WARN, got %s", rep.Pillars[0].Status)
	}
	// Soft warn should not produce a non-zero exit code.
	if exitCode(rep) != 0 {
		t.Errorf("soft WARN should exit 0; got %d", exitCode(rep))
	}
}

func TestBuildReport_MixedHardAndSoft(t *testing.T) {
	t.Parallel()
	r := &rubric{
		Pillars: []pillar{
			{ID: "gate", Name: "Gate", Priority: 1},
			{ID: "align", Name: "Align", Priority: 3},
		},
		PillarFloors: map[string]int{"gate": 4, "align": 3},
		SoftGates:    []string{"align"},
		Areas: []area{
			{ID: "alpha", Pillar: "gate"},
			{ID: "beta", Pillar: "align"},
		},
		Axes: []axis{{ID: "P1"}},
	}
	s := &scores{
		Scores: map[string]map[string]cellScore{
			"alpha": {"P1": {Score: 2}}, // hard FAIL
			"beta":  {"P1": {Score: 2}}, // soft WARN
		},
	}
	rep := buildReport(r, s)
	if rep.OverallStatus != "FAIL" {
		t.Errorf("any FAIL should make overall FAIL; got %s", rep.OverallStatus)
	}
	if exitCode(rep) != 1 {
		t.Errorf("expected exit 1 for hard FAIL, got %d", exitCode(rep))
	}
}

func TestLowestCell(t *testing.T) {
	t.Parallel()
	scores := map[string]cellScore{
		"P1": {Score: 4},
		"P2": {Score: 2},
		"P3": {Score: 3},
		"E1": {Score: 2},
	}
	floor, weakest := lowestCell(scores)
	if floor != 2 {
		t.Errorf("expected floor=2, got %d", floor)
	}
	if len(weakest) != 2 {
		t.Errorf("expected 2 weakest axes, got %v", weakest)
	}
	// Sorted ascending.
	if weakest[0] != "E1" || weakest[1] != "P2" {
		t.Errorf("weakest not sorted: %v", weakest)
	}
}

func TestAxisOrderKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   string
		want int
	}{
		{"P1", 101},
		{"P7", 107},
		{"E1", 201},
		{"E7", 207},
		{"V1", 301},
		{"V3", 303},
	}
	for _, tc := range cases {
		got := axisOrderKey(tc.id)
		if got != tc.want {
			t.Errorf("axisOrderKey(%q) = %d, want %d", tc.id, got, tc.want)
		}
	}
}

// TestRealRubricLoads verifies the shipped rubric.yaml + scores.yaml
// in the repo parse, validate, and produce a plausible report. This
// catches structural drift between the YAML and the Go types.
func TestRealRubricLoads(t *testing.T) {
	t.Parallel()
	r, err := loadRubric("../../docs/release/parity/rubric.yaml")
	if err != nil {
		t.Fatalf("load rubric: %v", err)
	}
	s, err := loadScores("../../docs/release/parity/scores.yaml")
	if err != nil {
		t.Fatalf("load scores: %v", err)
	}
	if err := validate(r, s); err != nil {
		t.Fatalf("validate real rubric/scores: %v", err)
	}
	rep := buildReport(r, s)
	if len(rep.Pillars) == 0 {
		t.Fatal("expected at least one pillar verdict")
	}
	// The shipped baseline is FAIL — that's the honest starting point.
	// If this assertion ever passes (PASS), the rubric is being lifted
	// without scores keeping up; investigate.
	if rep.OverallStatus == "PASS" {
		t.Logf("note: parity gate is now PASSing — confirm rubric and scores moved together")
	}
}
