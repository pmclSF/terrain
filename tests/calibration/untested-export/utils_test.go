package utils

import "testing"

func TestFormatCurrency(t *testing.T) {
	got := FormatCurrency(1.0, "USD")
	if got == "" {
		t.Errorf("FormatCurrency returned empty")
	}
}
