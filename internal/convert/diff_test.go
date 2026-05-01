package convert

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_NoChange(t *testing.T) {
	t.Parallel()
	got := UnifiedDiff("a.js", "b.js", "console.log(1);\n", "console.log(1);\n")
	if !strings.Contains(got, "no changes") {
		t.Errorf("identical inputs should produce a no-changes marker, got:\n%s", got)
	}
}

func TestUnifiedDiff_AddedLine(t *testing.T) {
	t.Parallel()
	got := UnifiedDiff("a", "b", "x\n", "x\ny\n")
	if !strings.Contains(got, " x") {
		t.Errorf("expected context line for x, got:\n%s", got)
	}
	if !strings.Contains(got, "+y") {
		t.Errorf("expected addition for y, got:\n%s", got)
	}
}

func TestUnifiedDiff_DeletedLine(t *testing.T) {
	t.Parallel()
	got := UnifiedDiff("a", "b", "x\ny\n", "x\n")
	if !strings.Contains(got, " x") {
		t.Errorf("expected context line for x, got:\n%s", got)
	}
	if !strings.Contains(got, "-y") {
		t.Errorf("expected deletion for y, got:\n%s", got)
	}
}

func TestUnifiedDiff_ChangedLine(t *testing.T) {
	t.Parallel()
	got := UnifiedDiff("a", "b", "x\nz\n", "x\ny\n")
	if !strings.Contains(got, "-z") {
		t.Errorf("expected deletion of z, got:\n%s", got)
	}
	if !strings.Contains(got, "+y") {
		t.Errorf("expected addition of y, got:\n%s", got)
	}
}

func TestUnifiedDiff_HasHeader(t *testing.T) {
	t.Parallel()
	got := UnifiedDiff("src/old.js", "src/new.js", "x\n", "y\n")
	if !strings.HasPrefix(got, "--- src/old.js\n+++ src/new.js\n") {
		t.Errorf("expected diff header, got:\n%s", got[:60])
	}
}

func TestUnifiedDiff_LongerExample(t *testing.T) {
	t.Parallel()
	old := `import { test, expect } from '@jest/globals';
test('login', () => {
  expect(login('alice')).toBe('alice');
});
`
	new := `import { test, expect } from 'vitest';
test('login', () => {
  expect(login('alice')).toBe('alice');
});
`
	got := UnifiedDiff("a.test.js", "a.test.js", old, new)
	if !strings.Contains(got, "-import { test, expect } from '@jest/globals';") {
		t.Errorf("missing deletion of jest import")
	}
	if !strings.Contains(got, "+import { test, expect } from 'vitest';") {
		t.Errorf("missing addition of vitest import")
	}
	if !strings.Contains(got, " test('login', () => {") {
		t.Errorf("missing context line for unchanged test signature")
	}
}
