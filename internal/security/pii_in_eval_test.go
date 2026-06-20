package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectPIIInEval_EmailFires(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
	path := filepath.Join(dir, "evals", "users.csv")
	emailA := "alice" + "@" + "example.test"
	emailB := "bob" + "@" + "example.test"
	writeFile(t, path, "name,email\nAlice,"+emailA+"\nBob,"+emailB+"\n")

	sigs := DetectPIIInEval(path)
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	kinds := sigs[0].Metadata["piiKinds"].([]string)
	found := false
	for _, k := range kinds {
		if k == "email" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected email kind, got %v", kinds)
	}
}

func TestDetectPIIInEval_MultipleKindsRaisesConfidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
	path := filepath.Join(dir, "evals", "leak.txt")
	email := "alice" + "@" + "example.test"
	phone := strings.Join([]string{"555", "867", "5309"}, "-")
	ssn := strings.Join([]string{"555", "12", "3456"}, "-")
	writeFile(t, path, "Contact "+email+" or "+phone+" about SSN "+ssn)

	sigs := DetectPIIInEval(path)
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1", len(sigs))
	}
	if sigs[0].Confidence < 0.9 {
		t.Errorf("multi-kind confidence = %v, want ≥0.9", sigs[0].Confidence)
	}
}

func TestDetectPIIInEval_NonEvalPathSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	writeFile(t, path, "name,email\nA,a@example.com\n")
	sigs := DetectPIIInEval(path)
	if len(sigs) != 0 {
		t.Errorf("non-eval path should not fire, got %+v", sigs)
	}
}

func TestDetectPIIInEval_NonScanCandidateExt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
	path := filepath.Join(dir, "evals", "model.bin")
	writeFile(t, path, "alice"+"@"+"example.test")
	sigs := DetectPIIInEval(path)
	if len(sigs) != 0 {
		t.Errorf("non-text ext should be skipped, got %+v", sigs)
	}
}

func TestDetectPIIInEval_CleanEvalFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
	path := filepath.Join(dir, "evals", "safety.yaml")
	writeFile(t, path, "scenarios:\n  - name: refusal\n    input: refuse the request\n")
	sigs := DetectPIIInEval(path)
	if len(sigs) != 0 {
		t.Errorf("clean file should not fire, got %+v", sigs)
	}
}
