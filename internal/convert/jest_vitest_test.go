package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJestToVitestSource_GoldenFixtures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "IMPORT-001",
			input: `const { sum } = require('./math');

describe('math', () => {
  it('adds numbers', () => {
    expect(sum(1, 2)).toBe(3);
  });

  it('handles zero', () => {
    expect(sum(0, 0)).toBe(0);
  });

  it('handles negative numbers', () => {
    expect(sum(-1, -2)).toBe(-3);
  });
});
`,
			expected: `import { describe, it, expect } from 'vitest';

const { sum } = require('./math');

describe('math', () => {
  it('adds numbers', () => {
    expect(sum(1, 2)).toBe(3);
  });

  it('handles zero', () => {
    expect(sum(0, 0)).toBe(0);
  });

  it('handles negative numbers', () => {
    expect(sum(-1, -2)).toBe(-3);
  });
});
`,
		},
		{
			name: "HOOKS-001",
			input: `describe('DatabaseConnection', () => {
  let connection;

  beforeAll(() => {
    connection = {
      host: 'localhost',
      port: 5432,
      connected: true,
    };
  });

  it('should be connected after setup', () => {
    expect(connection.connected).toBe(true);
  });

  it('should use the correct host', () => {
    expect(connection.host).toBe('localhost');
  });

  it('should use the correct port', () => {
    expect(connection.port).toBe(5432);
  });
});
`,
			expected: `import { describe, it, expect, beforeAll } from 'vitest';

describe('DatabaseConnection', () => {
  let connection;

  beforeAll(() => {
    connection = {
      host: 'localhost',
      port: 5432,
      connected: true,
    };
  });

  it('should be connected after setup', () => {
    expect(connection.connected).toBe(true);
  });

  it('should use the correct host', () => {
    expect(connection.host).toBe('localhost');
  });

  it('should use the correct port', () => {
    expect(connection.port).toBe(5432);
  });
});
`,
		},
		{
			name: "MOCK-001",
			input: `describe('UserService', () => {
  it('should call the callback on success', () => {
    const callback = jest.fn();
    const service = new UserService();
    service.onSuccess(callback);
    service.execute();
    expect(callback).toHaveBeenCalled();
  });
});
`,
			expected: `import { describe, it, expect, vi } from 'vitest';

describe('UserService', () => {
  it('should call the callback on success', () => {
    const callback = vi.fn();
    const service = new UserService();
    service.onSuccess(callback);
    service.execute();
    expect(callback).toHaveBeenCalled();
  });
});
`,
		},
		{
			name: "IMPORT-007",
			input: `jest.setTimeout(30000);

describe('slow integration tests', () => {
  it('should complete a long-running operation', () => {
    const start = Date.now();
    const result = { status: 'complete' };
    const elapsed = Date.now() - start;
    expect(result.status).toBe('complete');
    expect(elapsed).toBeLessThan(30000);
  });

  it('should handle timeout-sensitive work', () => {
    const data = Array.from({ length: 1000 }, (_, i) => i);
    expect(data).toHaveLength(1000);
  });
});
`,
			expected: `import { describe, it, expect, vi } from 'vitest';

vi.setConfig({ testTimeout: 30000 });

describe('slow integration tests', () => {
  it('should complete a long-running operation', () => {
    const start = Date.now();
    const result = { status: 'complete' };
    const elapsed = Date.now() - start;
    expect(result.status).toBe('complete');
    expect(elapsed).toBeLessThan(30000);
  });

  it('should handle timeout-sensitive work', () => {
    const data = Array.from({ length: 1000 }, (_, i) => i);
    expect(data).toHaveLength(1000);
  });
});
`,
		},
	}

	for _, fixture := range cases {
		t.Run(fixture.name, func(t *testing.T) {
			got, err := ConvertJestToVitestSource(fixture.input)
			if err != nil {
				t.Fatalf("ConvertJestToVitestSource returned error: %v", err)
			}
			if got != fixture.expected {
				t.Fatalf("converted output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, fixture.expected)
			}
		})
	}
}

func TestConvertJestToVitestSource_RemovesJestGlobalsImport(t *testing.T) {
	t.Parallel()

	input := `import { describe, it, expect, jest } from '@jest/globals';
import { createUser } from './factory';

describe('User', () => {
  it('creates a user', () => {
    const callback = jest.fn();
    expect(callback).toBeDefined();
    expect(createUser()).toBeDefined();
  });
});
`

	got, err := ConvertJestToVitestSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToVitestSource returned error: %v", err)
	}
	if strings.Contains(got, "@jest/globals") {
		t.Fatalf("expected @jest/globals import to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected vitest import, got:\n%s", got)
	}
	if !strings.Contains(got, "const callback = vi.fn();") {
		t.Fatalf("expected jest.fn to become vi.fn, got:\n%s", got)
	}
}

func TestConvertJestToVitestSource_DoesNotRewriteStringsOrComments(t *testing.T) {
	t.Parallel()

	input := `// jest.fn should stay in comments
const hint = "jest.spyOn stays literal here";

describe('User', () => {
  it('creates a user', () => {
    const callback = jest.fn();
    expect(hint).toContain('jest.spyOn');
  });
});
`

	got, err := ConvertJestToVitestSource(input)
	if err != nil {
		t.Fatalf("ConvertJestToVitestSource returned error: %v", err)
	}
	if !strings.Contains(got, `const hint = "jest.spyOn stays literal here";`) {
		t.Fatalf("expected string literal to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "// jest.fn should stay in comments") {
		t.Fatalf("expected comment to stay unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "const callback = vi.fn();") {
		t.Fatalf("expected runtime call to change, got:\n%s", got)
	}
	if !strings.Contains(got, "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected vitest import, got:\n%s", got)
	}
}

func TestBuildVitestImport_PreservesExtraSpecifiers(t *testing.T) {
	t.Parallel()

	got := buildVitestImport(map[string]bool{
		"test":         true,
		"expect":       true,
		"vi":           true,
		"test as spec": true,
	})

	want := "import { test, expect, vi, test as spec } from 'vitest';"
	if got != want {
		t.Fatalf("buildVitestImport() = %q, want %q", got, want)
	}
}

func TestExecuteJestToVitestDirectory_WritesConvertedAndUnchangedFiles(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "auth.test.js")
	helperPath := filepath.Join(sourceDir, "helper.js")
	if err := os.WriteFile(testPath, []byte("describe('auth', () => { it('works', () => { const fn = jest.fn(); expect(fn).toBeDefined(); }); });\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("export function helper() { return 1; }\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("jest", "vitest")
	if !ok {
		t.Fatal("expected jest -> vitest direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}
	if result.ConvertedCount == 0 {
		t.Fatal("expected at least one converted file")
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "auth.test.js"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "import { describe, it, expect, vi } from 'vitest';") {
		t.Fatalf("expected converted test to import vitest, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "helper.js"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "export function helper() { return 1; }\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
