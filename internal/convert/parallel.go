package convert

const defaultConversionConcurrency = 4

func clampWorkerCount(requested, total int) int {
	if total <= 0 {
		return 0
	}
	if requested <= 0 {
		requested = defaultConversionConcurrency
	}
	if requested < 1 {
		requested = 1
	}
	if requested > total {
		requested = total
	}
	return requested
}

func clampBatchSize(requested, total int) int {
	if total <= 0 {
		return 0
	}
	if requested <= 0 || requested > total {
		return total
	}
	return requested
}
