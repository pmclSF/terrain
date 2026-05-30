package plugin

import (
	"strings"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		SchemaVersion: 1,
		ID:            "acme/example-detector",
		Name:          "Acme Example Detector",
		Version:       "0.1.0",
		Author:        "Acme",
		Description:   "Demonstrates the plugin manifest contract.",
		Detectors: []DetectorSpec{
			{
				RuleID:          "acme/example/sample-rule",
				SignalType:      "acmeSampleRule",
				MechanismClass:  "structural-ast",
				DefaultSeverity: "medium",
				Tier:            "observability",
				Description:     "Sample detector for the contract test.",
			},
		},
	}
}

func TestValidate_AcceptsCanonicalManifest(t *testing.T) {
	if err := Validate(validManifest()); err != nil {
		t.Errorf("expected canonical manifest to validate, got: %v", err)
	}
}

func TestValidate_RejectsUnsupportedSchemaVersion(t *testing.T) {
	m := validManifest()
	m.SchemaVersion = 999
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("expected schema_version rejection, got: %v", err)
	}
}

func TestValidate_RejectsIDWithoutSlash(t *testing.T) {
	m := validManifest()
	m.ID = "loose-name"
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "<author>/<name>") {
		t.Errorf("expected id-shape rejection, got: %v", err)
	}
}

func TestValidate_RequiresAtLeastOneDetector(t *testing.T) {
	m := validManifest()
	m.Detectors = nil
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "at least one detector") {
		t.Errorf("expected detector-required rejection, got: %v", err)
	}
}

func TestValidate_RejectsForbiddenMechanismClass(t *testing.T) {
	cases := []string{"literal-string", "regex", "curated-allowlist", "rando"}
	for _, mc := range cases {
		t.Run(mc, func(t *testing.T) {
			m := validManifest()
			m.Detectors[0].MechanismClass = mc
			err := Validate(m)
			if err == nil || !strings.Contains(err.Error(), "mechanism_class") {
				t.Errorf("expected mechanism_class rejection for %q, got: %v", mc, err)
			}
		})
	}
}

func TestValidate_AcceptsEachAllowedMechanismClass(t *testing.T) {
	for _, mc := range AllowedMechanismClasses {
		t.Run(mc, func(t *testing.T) {
			m := validManifest()
			m.Detectors[0].MechanismClass = mc
			if err := Validate(m); err != nil {
				t.Errorf("expected %q to validate, got: %v", mc, err)
			}
		})
	}
}

func TestValidate_RejectsDuplicateRuleID(t *testing.T) {
	m := validManifest()
	m.Detectors = append(m.Detectors, DetectorSpec{
		RuleID:          "acme/example/sample-rule", // duplicate
		SignalType:      "acmeOther",
		MechanismClass:  "structural-ast",
		DefaultSeverity: "low",
	})
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "duplicate rule_id") {
		t.Errorf("expected duplicate rule_id rejection, got: %v", err)
	}
}

func TestValidate_RejectsDuplicateSignalType(t *testing.T) {
	m := validManifest()
	m.Detectors = append(m.Detectors, DetectorSpec{
		RuleID:          "acme/example/other",
		SignalType:      "acmeSampleRule", // duplicate
		MechanismClass:  "structural-ast",
		DefaultSeverity: "low",
	})
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "duplicate signal_type") {
		t.Errorf("expected duplicate signal_type rejection, got: %v", err)
	}
}

func TestValidate_RejectsInvalidTier(t *testing.T) {
	m := validManifest()
	m.Detectors[0].Tier = "stable"
	err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Errorf("expected tier rejection, got: %v", err)
	}
}

func TestParseManifest_ReadsYAML(t *testing.T) {
	body := []byte(`
schema_version: 1
id: lakera/prompt-injection
name: Lakera Prompt Injection Detector
version: 1.0.0
author: Lakera
description: Cross-checks prompts against the Lakera prompt-injection corpus.
requires_network: true
requires_api_key: [lakera]
detectors:
  - rule_id: lakera/prompt-injection
    signal_type: lakeraPromptInjection
    mechanism_class: structural-ast
    default_severity: high
    tier: observability
    description: Flags prompt templates likely vulnerable to injection.
`)
	m, err := ParseManifest(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.ID != "lakera/prompt-injection" {
		t.Errorf("ID = %q", m.ID)
	}
	if !m.RequiresNetwork {
		t.Errorf("RequiresNetwork should be true")
	}
	if len(m.RequiresAPIKey) != 1 || m.RequiresAPIKey[0] != "lakera" {
		t.Errorf("RequiresAPIKey = %v", m.RequiresAPIKey)
	}
}
