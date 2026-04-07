package convert

import "strings"

type ValidationMode string

const (
	ValidationModeStrict     ValidationMode = "strict"
	ValidationModeBestEffort ValidationMode = "best-effort"
)

func normalizeValidationMode(mode string) ValidationMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", string(ValidationModeStrict):
		return ValidationModeStrict
	case string(ValidationModeBestEffort):
		return ValidationModeBestEffort
	default:
		return ValidationModeStrict
	}
}

func validationWarningsForError(mode ValidationMode, err error) []string {
	if err == nil || mode != ValidationModeBestEffort {
		return nil
	}
	return []string{"best-effort mode kept output despite validation failure: " + err.Error()}
}

func effectiveValidationMode(mode string) ValidationMode {
	return normalizeValidationMode(mode)
}
