package convert

import "testing"

func TestClampWorkerCount_ZeroTotal(t *testing.T) {
	t.Parallel()
	if got := clampWorkerCount(4, 0); got != 0 {
		t.Errorf("clampWorkerCount(4, 0) = %d, want 0", got)
	}
}

func TestClampWorkerCount_NegativeTotal(t *testing.T) {
	t.Parallel()
	if got := clampWorkerCount(4, -1); got != 0 {
		t.Errorf("clampWorkerCount(4, -1) = %d, want 0", got)
	}
}

func TestClampWorkerCount_ZeroRequested_DefaultsTo4(t *testing.T) {
	t.Parallel()
	got := clampWorkerCount(0, 10)
	if got != defaultConversionConcurrency {
		t.Errorf("clampWorkerCount(0, 10) = %d, want %d", got, defaultConversionConcurrency)
	}
}

func TestClampWorkerCount_NegativeRequested_DefaultsTo4(t *testing.T) {
	t.Parallel()
	got := clampWorkerCount(-5, 10)
	if got != defaultConversionConcurrency {
		t.Errorf("clampWorkerCount(-5, 10) = %d, want %d", got, defaultConversionConcurrency)
	}
}

func TestClampWorkerCount_RequestedExceedsTotal(t *testing.T) {
	t.Parallel()
	got := clampWorkerCount(100, 3)
	if got != 3 {
		t.Errorf("clampWorkerCount(100, 3) = %d, want 3", got)
	}
}

func TestClampWorkerCount_RequestedWithinBounds(t *testing.T) {
	t.Parallel()
	got := clampWorkerCount(2, 10)
	if got != 2 {
		t.Errorf("clampWorkerCount(2, 10) = %d, want 2", got)
	}
}

func TestClampWorkerCount_OneFileOneWorker(t *testing.T) {
	t.Parallel()
	got := clampWorkerCount(4, 1)
	if got != 1 {
		t.Errorf("clampWorkerCount(4, 1) = %d, want 1", got)
	}
}

func TestClampBatchSize_ZeroTotal(t *testing.T) {
	t.Parallel()
	if got := clampBatchSize(5, 0); got != 0 {
		t.Errorf("clampBatchSize(5, 0) = %d, want 0", got)
	}
}

func TestClampBatchSize_NegativeTotal(t *testing.T) {
	t.Parallel()
	if got := clampBatchSize(5, -1); got != 0 {
		t.Errorf("clampBatchSize(5, -1) = %d, want 0", got)
	}
}

func TestClampBatchSize_ZeroRequested_DefaultsToTotal(t *testing.T) {
	t.Parallel()
	got := clampBatchSize(0, 10)
	if got != 10 {
		t.Errorf("clampBatchSize(0, 10) = %d, want 10", got)
	}
}

func TestClampBatchSize_NegativeRequested_DefaultsToTotal(t *testing.T) {
	t.Parallel()
	got := clampBatchSize(-3, 10)
	if got != 10 {
		t.Errorf("clampBatchSize(-3, 10) = %d, want 10", got)
	}
}

func TestClampBatchSize_RequestedExceedsTotal(t *testing.T) {
	t.Parallel()
	got := clampBatchSize(100, 5)
	if got != 5 {
		t.Errorf("clampBatchSize(100, 5) = %d, want 5", got)
	}
}

func TestClampBatchSize_RequestedWithinBounds(t *testing.T) {
	t.Parallel()
	got := clampBatchSize(3, 10)
	if got != 3 {
		t.Errorf("clampBatchSize(3, 10) = %d, want 3", got)
	}
}
