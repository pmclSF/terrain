package convert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupportedDirections_CountAndOrder(t *testing.T) {
	t.Parallel()

	directions := SupportedDirections()
	if got := len(directions); got != 25 {
		t.Fatalf("supported direction count = %d, want 25", got)
	}
	if directions[0].From != "cypress" || directions[0].To != "playwright" {
		t.Fatalf("first direction = %s -> %s, want cypress -> playwright", directions[0].From, directions[0].To)
	}
	if directions[len(directions)-1].From != "testcafe" || directions[len(directions)-1].To != "cypress" {
		t.Fatalf("last direction = %s -> %s, want testcafe -> cypress", directions[len(directions)-1].From, directions[len(directions)-1].To)
	}
}

func TestLookupShorthand(t *testing.T) {
	t.Parallel()

	direction, ok := LookupShorthand("cy2pw")
	if !ok {
		t.Fatal("expected shorthand cy2pw to resolve")
	}
	if direction.From != "cypress" || direction.To != "playwright" {
		t.Fatalf("cy2pw resolved to %s -> %s", direction.From, direction.To)
	}

	direction, ok = LookupShorthand("jesttovt")
	if !ok {
		t.Fatal("expected shorthand jesttovt to resolve")
	}
	if direction.From != "jest" || direction.To != "vitest" {
		t.Fatalf("jesttovt resolved to %s -> %s", direction.From, direction.To)
	}
	if direction.GoNativeState != GoNativeStateImplemented {
		t.Fatalf("jesttovt state = %s, want %s", direction.GoNativeState, GoNativeStateImplemented)
	}
}

func TestLookupDirection_ImplementedDirectionsReportGoNativeRuntime(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"cypress", "webdriverio"},
		{"jasmine", "jest"},
		{"cypress", "selenium"},
		{"jest", "vitest"},
		{"jest", "jasmine"},
		{"jest", "mocha"},
		{"junit4", "junit5"},
		{"junit5", "testng"},
		{"cypress", "playwright"},
		{"mocha", "jest"},
		{"nose2", "pytest"},
		{"playwright", "cypress"},
		{"playwright", "puppeteer"},
		{"playwright", "selenium"},
		{"playwright", "webdriverio"},
		{"puppeteer", "playwright"},
		{"pytest", "unittest"},
		{"selenium", "cypress"},
		{"selenium", "playwright"},
		{"testng", "junit5"},
		{"testcafe", "cypress"},
		{"testcafe", "playwright"},
		{"unittest", "pytest"},
		{"webdriverio", "cypress"},
		{"webdriverio", "playwright"},
	}

	for _, tc := range cases {
		direction, ok := LookupDirection(tc[0], tc[1])
		if !ok {
			t.Fatalf("expected direction %s -> %s", tc[0], tc[1])
		}
		if direction.GoNativeState != GoNativeStateImplemented {
			t.Fatalf("%s -> %s state = %s, want %s", tc[0], tc[1], direction.GoNativeState, GoNativeStateImplemented)
		}
		if direction.Implementation != "go-native-runtime" {
			t.Fatalf("%s -> %s implementation = %q, want go-native-runtime", tc[0], tc[1], direction.Implementation)
		}
	}
}

func TestCategories_PreserveLegacyGroupingOrder(t *testing.T) {
	t.Parallel()

	categories := Categories()
	want := []string{
		"JavaScript E2E / Browser",
		"JavaScript Unit Testing",
		"Java",
		"Python",
	}
	if len(categories) != len(want) {
		t.Fatalf("category count = %d, want %d", len(categories), len(want))
	}
	for i, name := range want {
		if categories[i].Name != name {
			t.Fatalf("category[%d] = %q, want %q", i, categories[i].Name, name)
		}
	}
}

func TestDetectSource_File(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "example.spec.ts")
	content := "import { test, expect } from '@playwright/test';\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	detection, err := DetectSource(path)
	if err != nil {
		t.Fatalf("DetectSource returned error: %v", err)
	}
	if detection.Framework != "playwright" {
		t.Fatalf("framework = %q, want playwright", detection.Framework)
	}
	if detection.Mode != "file" {
		t.Fatalf("mode = %q, want file", detection.Mode)
	}
	if detection.Language != "javascript" {
		t.Fatalf("language = %q, want javascript", detection.Language)
	}
}

func TestDetectSource_DirectoryChoosesStrongestMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "auth.spec.ts"), []byte("import { test } from '@playwright/test'\n"), 0o644); err != nil {
		t.Fatalf("write playwright file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "legacy.test.js"), []byte("describe('x', () => { expect(true).toBe(true) })\n"), 0o644); err != nil {
		t.Fatalf("write jest file: %v", err)
	}

	detection, err := DetectSource(root)
	if err != nil {
		t.Fatalf("DetectSource returned error: %v", err)
	}
	if detection.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", detection.Mode)
	}
	if detection.Framework != "playwright" {
		t.Fatalf("framework = %q, want playwright", detection.Framework)
	}
	if detection.FilesScanned < 2 {
		t.Fatalf("files scanned = %d, want at least 2", detection.FilesScanned)
	}
}
