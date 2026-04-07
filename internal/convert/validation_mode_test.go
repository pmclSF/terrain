package convert

import (
	"errors"
	"testing"
)

func TestNormalizeValidationMode_EmptyIsStrict(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode(""); got != ValidationModeStrict {
		t.Errorf("normalizeValidationMode(\"\") = %q, want %q", got, ValidationModeStrict)
	}
}

func TestNormalizeValidationMode_ExplicitStrict(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode("strict"); got != ValidationModeStrict {
		t.Errorf("got %q, want %q", got, ValidationModeStrict)
	}
}

func TestNormalizeValidationMode_BestEffort(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode("best-effort"); got != ValidationModeBestEffort {
		t.Errorf("got %q, want %q", got, ValidationModeBestEffort)
	}
}

func TestNormalizeValidationMode_CaseInsensitive(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode("Best-Effort"); got != ValidationModeBestEffort {
		t.Errorf("got %q, want %q", got, ValidationModeBestEffort)
	}
}

func TestNormalizeValidationMode_Whitespace(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode("  strict  "); got != ValidationModeStrict {
		t.Errorf("got %q, want %q", got, ValidationModeStrict)
	}
}

func TestNormalizeValidationMode_UnknownFallsToStrict(t *testing.T) {
	t.Parallel()
	if got := normalizeValidationMode("invalid"); got != ValidationModeStrict {
		t.Errorf("normalizeValidationMode(\"invalid\") = %q, want %q (should fall back to strict)", got, ValidationModeStrict)
	}
}

func TestEffectiveValidationMode_DelegatesNormalize(t *testing.T) {
	t.Parallel()
	if got := effectiveValidationMode("best-effort"); got != ValidationModeBestEffort {
		t.Errorf("got %q, want %q", got, ValidationModeBestEffort)
	}
}

func TestValidationWarningsForError_NilError(t *testing.T) {
	t.Parallel()
	warnings := validationWarningsForError(ValidationModeBestEffort, nil)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil error, got %v", warnings)
	}
}

func TestValidationWarningsForError_StrictMode_NoWarnings(t *testing.T) {
	t.Parallel()
	warnings := validationWarningsForError(ValidationModeStrict, errors.New("syntax error"))
	if len(warnings) != 0 {
		t.Errorf("expected no warnings in strict mode, got %v", warnings)
	}
}

func TestValidationWarningsForError_BestEffort_ReturnsWarning(t *testing.T) {
	t.Parallel()
	err := errors.New("syntax error in output")
	warnings := validationWarningsForError(ValidationModeBestEffort, err)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] == "" {
		t.Error("warning message should not be empty")
	}
}
