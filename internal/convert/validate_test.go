package convert

import "testing"

func TestValidateSyntax_AcceptsValidOutputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		language string
		source   string
	}{
		{
			name:     "javascript",
			path:     "login.test.ts",
			language: "javascript",
			source:   "import { test } from '@playwright/test';\ntest('ok', async ({ page }) => { await page.goto('/'); });\n",
		},
		{
			name:     "python",
			path:     "test_example.py",
			language: "python",
			source:   "import unittest\n\nclass TestExample(unittest.TestCase):\n    def test_ok(self):\n        self.assertTrue(True)\n",
		},
		{
			name:     "java",
			path:     "ExampleTest.java",
			language: "java",
			source:   "import org.junit.jupiter.api.Test;\nclass ExampleTest { @Test void testOk() {} }\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateSyntax(tc.path, tc.language, tc.source); err != nil {
				t.Fatalf("ValidateSyntax returned error: %v", err)
			}
		})
	}
}

func TestValidateSyntax_RejectsInvalidOutputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		language string
		source   string
	}{
		{
			name:     "javascript",
			path:     "broken.test.js",
			language: "javascript",
			source:   "describe('broken', () => {\n",
		},
		{
			name:     "python",
			path:     "broken_test.py",
			language: "python",
			source:   "def test_broken(:\n    pass\n",
		},
		{
			name:     "java",
			path:     "BrokenTest.java",
			language: "java",
			source:   "class BrokenTest { void testBroken( { }",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateSyntax(tc.path, tc.language, tc.source); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}
