package convert

import (
	"strings"
	"testing"
)

func TestExecute_UnsupportedDirection_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := Direction{From: "nonexistent", To: "framework"}
	_, err := Execute(".", dir, ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported direction, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("error = %q, want message containing 'not implemented'", err.Error())
	}
}

func TestExecute_UnsupportedDirection_IncludesNames(t *testing.T) {
	t.Parallel()
	dir := Direction{From: "alpha", To: "beta"}
	_, err := Execute(".", dir, ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("error should mention both framework names: %q", err.Error())
	}
}

func TestConvertSource_UnsupportedDirection_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := Direction{From: "foo", To: "bar"}
	_, err := ConvertSource(dir, "some test source")
	if err == nil {
		t.Fatal("expected error for unsupported direction, got nil")
	}
}

func TestConvertSource_EmptyInput_AllDirections(t *testing.T) {
	t.Parallel()
	// Every supported direction should handle empty input gracefully.
	for _, d := range SupportedDirections() {
		d := d
		t.Run(d.From+"-"+d.To, func(t *testing.T) {
			t.Parallel()
			out, err := ConvertSource(d, "")
			if err != nil {
				t.Errorf("ConvertSource(%s->%s, \"\") returned error: %v", d.From, d.To, err)
			}
			if strings.TrimSpace(out) != "" {
				t.Errorf("ConvertSource(%s->%s, \"\") returned non-empty: %q", d.From, d.To, out)
			}
		})
	}
}
