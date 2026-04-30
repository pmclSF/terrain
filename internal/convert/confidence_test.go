package convert

import "testing"

func TestComputeFileConfidence_CleanConversion(t *testing.T) {
	t.Parallel()

	src := `
import { test, expect } from '@jest/globals';
test('login works', () => {
  expect(login('alice')).toEqual({name: 'alice'});
});
test('logout works', () => {
  expect(logout()).toBe(true);
});
`
	dst := `
import { test, expect } from 'vitest';
test('login works', () => {
  expect(login('alice')).toEqual({name: 'alice'});
});
test('logout works', () => {
  expect(logout()).toBe(true);
});
`
	covered, lossy, confidence := computeFileConfidence(src, dst)
	if lossy != 0 {
		t.Errorf("lossy = %d, want 0 for a 1:1 conversion", lossy)
	}
	if confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0", confidence)
	}
	if covered == 0 {
		t.Errorf("covered = 0, want > 0")
	}
}

func TestComputeFileConfidence_LossyConversion(t *testing.T) {
	t.Parallel()

	src := `
test('one', () => { expect(1).toBe(1); });
test('two', () => { expect(2).toBe(2); });
test('three', () => { expect(3).toBe(3); });
test('four', () => { expect(4).toBe(4); });
`
	// Output dropped two of the four tests.
	dst := `
test('one', () => { expect(1).toBe(1); });
test('two', () => { expect(2).toBe(2); });
`
	covered, lossy, confidence := computeFileConfidence(src, dst)
	if lossy == 0 {
		t.Errorf("expected non-zero lossy on a partial conversion")
	}
	if covered == 0 {
		t.Errorf("expected non-zero covered")
	}
	if confidence >= 1.0 {
		t.Errorf("confidence should be < 1.0 on lossy conversion, got %v", confidence)
	}
	if confidence <= 0 {
		t.Errorf("confidence should be > 0 (some items survived), got %v", confidence)
	}
}

func TestComputeFileConfidence_EmptyFile(t *testing.T) {
	t.Parallel()

	covered, lossy, confidence := computeFileConfidence("", "")
	if covered != 0 || lossy != 0 || confidence != 1.0 {
		t.Errorf("(0, 0, 1.0) expected for empty/empty, got (%d, %d, %v)",
			covered, lossy, confidence)
	}
}

func TestComputeFileConfidence_PytestStyle(t *testing.T) {
	t.Parallel()

	src := `
import pytest

@pytest.fixture
def db():
    return Database()

def test_one():
    assert one() == 1

def test_two():
    assert two() == 2
`
	// Same content "converted" — should be high confidence.
	covered, lossy, confidence := computeFileConfidence(src, src)
	if covered == 0 {
		t.Errorf("expected non-zero covered for pytest-style, got 0")
	}
	if lossy != 0 {
		t.Errorf("identical src/dst should have 0 lossy, got %d", lossy)
	}
	if confidence != 1.0 {
		t.Errorf("identical src/dst confidence = %v, want 1.0", confidence)
	}
}
