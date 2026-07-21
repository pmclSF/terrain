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
	emailA := "alice" + "@" + "gmail.com"
	emailB := "bob" + "@" + "gmail.com"
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
	email := "alice" + "@" + "gmail.com"
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

// TestDetectPIIInEval_SuppressesSyntheticValues locks the FP classes: reserved/
// example email domains, vendor test card numbers, placeholder SSNs, and (post
// IPv4 removal) version strings must NOT fire a Critical PII finding.
func TestDetectPIIInEval_SuppressesSyntheticValues(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"reserved-email":  "user" + "@" + "example.com\nother" + "@" + "test.local",
		"test-card":       "card: 4111 1111 1111 1111\nvisa: 4242-4242-4242-4242",
		"placeholder-ssn": "ssn: 000-00-0000\nid: 111-11-1111",
		"version-string":  "version: 1.2.3.4\ntimeout: 10.0.0.1\ndep: 192.168.0.1",
	}
	for name, body := range cases {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
			path := filepath.Join(dir, "evals", name+".txt")
			writeFile(t, path, body)
			if sigs := DetectPIIInEval(path); len(sigs) != 0 {
				t.Errorf("%s should be suppressed as synthetic, got %+v", name, sigs)
			}
		})
	}
}

// TestDetectPIIInEval_RealCardStillFires confirms the test-value exclusions
// don't blind the detector to a real-shaped (non-published, high-entropy) PAN.
func TestDetectPIIInEval_RealCardStillFires(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "evals"), 0o755)
	path := filepath.Join(dir, "evals", "leak.csv")
	writeFile(t, path, "pan\n4539578763621486\n") // valid-shaped, not a known test card
	if sigs := DetectPIIInEval(path); len(sigs) != 1 {
		t.Errorf("a real-shaped card should still fire, got %d: %+v", len(sigs), sigs)
	}
}
