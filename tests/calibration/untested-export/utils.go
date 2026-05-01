package utils

// FormatCurrency renders an amount in the given ISO currency code.
func FormatCurrency(amount float64, currency string) string {
	return currency + " " + formatAmount(amount)
}

func formatAmount(amount float64) string {
	return "0"
}
