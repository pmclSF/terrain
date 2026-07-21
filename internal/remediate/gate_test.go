package remediate

import (
	"testing"

	"github.com/pmclSF/terrain/internal/findings"
)

func errFinding(rule string, fix *findings.Fix) findings.Finding {
	f := findings.Finding{
		Version: findings.SchemaVersion, RuleID: rule, Severity: findings.SeverityError,
		PrimaryLoc: findings.Location{Path: "x"}, ShortMessage: "m", DocsURL: "d",
	}
	if fix != nil {
		f.Suggestions = []findings.Suggestion{{Text: "fix", Fix: fix}}
	}
	return f
}

// TestEnforceGate_ValidatedRemediationStaysGating: a gate-blocking finding
// whose remediation is closed-loop validated keeps its error severity.
func TestEnforceGate_ValidatedRemediationStaysGating(t *testing.T) {
	t.Parallel()
	reg := DefaultValidityRegistry()
	fs := []findings.Finding{
		errFinding("terrain/ai/surface-missing-eval", &findings.Fix{Kind: findings.FixNewFile, Path: "e.yaml"}),
		errFinding("terrain/deps/drift-risk", &findings.Fix{Kind: findings.FixEditInPlace, Path: "package.json"}),
	}
	if n := EnforceGate(fs, reg); n != 0 {
		t.Errorf("demoted %d, want 0 (both remediations validated)", n)
	}
	for _, f := range fs {
		if f.Severity != findings.SeverityError {
			t.Errorf("%s demoted, want error", f.RuleID)
		}
	}
}

// TestEnforceGate_JudgeOnlyDemoted: a gate-blocking finding with no
// structured Fix (judge-only) cannot block CI — it demotes to observability.
func TestEnforceGate_JudgeOnlyDemoted(t *testing.T) {
	t.Parallel()
	reg := DefaultValidityRegistry()
	fs := []findings.Finding{errFinding("terrain/deps/drift-risk", nil)} // strict-pin: no fix
	if n := EnforceGate(fs, reg); n != 1 {
		t.Fatalf("demoted %d, want 1", n)
	}
	if fs[0].Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want warning", fs[0].Severity)
	}
	if fs[0].Metadata[GateMetadataKey] != true {
		t.Errorf("missing %s annotation", GateMetadataKey)
	}
}

// TestEnforceGate_UnvalidatedKindDemoted: a fix whose (rule, kind) is not in
// the registry demotes — covers rules whose remediation hasn't been proven.
func TestEnforceGate_UnvalidatedKindDemoted(t *testing.T) {
	t.Parallel()
	reg := DefaultValidityRegistry()
	// A rule absent from the registry, even carrying a structured fix.
	fs := []findings.Finding{errFinding("terrain/quality/untested-export", &findings.Fix{Kind: findings.FixNewFile, Path: "t.go"})}
	if n := EnforceGate(fs, reg); n != 1 {
		t.Errorf("demoted %d, want 1 (rule not validated)", n)
	}
}

// TestEnforceGate_NonGatingUntouched: warning/notice findings are not gate-
// blocking and must be left alone.
func TestEnforceGate_NonGatingUntouched(t *testing.T) {
	t.Parallel()
	reg := DefaultValidityRegistry()
	fs := []findings.Finding{{
		Version: findings.SchemaVersion, RuleID: "terrain/x/y", Severity: findings.SeverityWarning,
		PrimaryLoc: findings.Location{Path: "x"}, ShortMessage: "m", DocsURL: "d",
	}}
	if n := EnforceGate(fs, reg); n != 0 {
		t.Errorf("demoted %d, want 0 (warning is not gate-blocking)", n)
	}
	if fs[0].Severity != findings.SeverityWarning {
		t.Error("warning finding must be untouched")
	}
}
