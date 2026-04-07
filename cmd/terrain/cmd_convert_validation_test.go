package main

import (
	"testing"

	conv "github.com/pmclSF/terrain/internal/convert"
)

func TestResolveConvertValidationMode_DefaultSkip_IsStrict(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "skip"}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeStrict {
		t.Errorf("got %q, want %q", got, conv.ValidationModeStrict)
	}
}

func TestResolveConvertValidationMode_Fail_IsStrict(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "fail"}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeStrict {
		t.Errorf("got %q, want %q", got, conv.ValidationModeStrict)
	}
}

func TestResolveConvertValidationMode_BestEffort(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "best-effort"}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeBestEffort {
		t.Errorf("got %q, want %q", got, conv.ValidationModeBestEffort)
	}
}

func TestResolveConvertValidationMode_BestEffort_OverriddenByStrictValidate(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "best-effort", StrictValidate: true}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeStrict {
		t.Errorf("--strict-validate should override best-effort: got %q, want %q", got, conv.ValidationModeStrict)
	}
}

func TestResolveConvertValidationMode_Empty_IsStrict(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: ""}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeStrict {
		t.Errorf("got %q, want %q", got, conv.ValidationModeStrict)
	}
}

func TestResolveConvertValidationMode_Invalid_ReturnsError(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "garbage"}
	_, err := resolveConvertValidationMode(opts)
	if err == nil {
		t.Fatal("expected error for invalid --on-error value")
	}
}

func TestResolveConvertValidationMode_CaseInsensitive(t *testing.T) {
	t.Parallel()
	opts := convertCommandOptions{OnError: "Best-Effort"}
	got, err := resolveConvertValidationMode(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != conv.ValidationModeBestEffort {
		t.Errorf("got %q, want %q", got, conv.ValidationModeBestEffort)
	}
}
