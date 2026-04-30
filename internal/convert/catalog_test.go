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

	implementedCases := [][2]string{
		{"cypress", "webdriverio"},
		{"jasmine", "jest"},
		{"cypress", "selenium"},
		{"jest", "vitest"},
		{"jest", "jasmine"},
		{"jest", "mocha"},
		{"cypress", "playwright"},
		{"mocha", "jest"},
		{"playwright", "cypress"},
		{"playwright", "puppeteer"},
		{"playwright", "selenium"},
		{"playwright", "webdriverio"},
		{"puppeteer", "playwright"},
		{"webdriverio", "cypress"},
		{"webdriverio", "playwright"},
	}

	// experimentalCases are directions that round 3 review classified as
	// C-grade; they dispatch to the Go-native runtime but are not yet
	// production-ready. See internal/convert/catalog.go for promotion criteria.
	experimentalCases := [][2]string{
		{"junit4", "junit5"},
		{"junit5", "testng"},
		{"testng", "junit5"},
		{"nose2", "pytest"},
		{"pytest", "unittest"},
		{"unittest", "pytest"},
		{"selenium", "cypress"},
		{"selenium", "playwright"},
		{"testcafe", "cypress"},
		{"testcafe", "playwright"},
	}

	for _, tc := range implementedCases {
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

	for _, tc := range experimentalCases {
		direction, ok := LookupDirection(tc[0], tc[1])
		if !ok {
			t.Fatalf("expected direction %s -> %s", tc[0], tc[1])
		}
		if direction.GoNativeState != GoNativeStateExperimental {
			t.Fatalf("%s -> %s state = %s, want %s", tc[0], tc[1], direction.GoNativeState, GoNativeStateExperimental)
		}
		if direction.Implementation != "go-native-runtime" {
			t.Fatalf("%s -> %s implementation = %q, want go-native-runtime", tc[0], tc[1], direction.Implementation)
		}
		if !direction.GoNativeReady {
			t.Fatalf("%s -> %s GoNativeReady = false; experimental directions should still dispatch (with warning)", tc[0], tc[1])
		}
	}
}

func TestLookupDirection_ReportsCapabilities(t *testing.T) {
	t.Parallel()

	jestVitest, ok := LookupDirection("jest", "vitest")
	if !ok {
		t.Fatal("expected jest -> vitest direction")
	}
	if jestVitest.Capabilities.TestMigration != CapabilitySupported {
		t.Fatalf("test capability = %s, want %s", jestVitest.Capabilities.TestMigration, CapabilitySupported)
	}
	if jestVitest.Capabilities.ConfigMigration != CapabilitySupported {
		t.Fatalf("config capability = %s, want %s", jestVitest.Capabilities.ConfigMigration, CapabilitySupported)
	}
	if jestVitest.Capabilities.ProjectMigration != CapabilitySupported {
		t.Fatalf("project capability = %s, want %s", jestVitest.Capabilities.ProjectMigration, CapabilitySupported)
	}

	testCafePlaywright, ok := LookupDirection("testcafe", "playwright")
	if !ok {
		t.Fatal("expected testcafe -> playwright direction")
	}
	if testCafePlaywright.Capabilities.ConfigMigration != CapabilitySupported {
		t.Fatalf("config capability = %s, want %s", testCafePlaywright.Capabilities.ConfigMigration, CapabilitySupported)
	}
	if testCafePlaywright.Capabilities.ProjectMigration != CapabilitySupported {
		t.Fatalf("project capability = %s, want %s", testCafePlaywright.Capabilities.ProjectMigration, CapabilitySupported)
	}

	pytestUnittest, ok := LookupDirection("pytest", "unittest")
	if !ok {
		t.Fatal("expected pytest -> unittest direction")
	}
	if pytestUnittest.Capabilities.ConfigMigration != CapabilityUnsupported {
		t.Fatalf("config capability = %s, want %s", pytestUnittest.Capabilities.ConfigMigration, CapabilityUnsupported)
	}
	if pytestUnittest.Capabilities.ProjectMigration != CapabilityPartial {
		t.Fatalf("project capability = %s, want %s", pytestUnittest.Capabilities.ProjectMigration, CapabilityPartial)
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
	if !detection.AutoDetectSafe {
		t.Fatal("expected file detection to be auto-detect safe")
	}
	if detection.Recommendation != "safe" {
		t.Fatalf("recommendation = %q, want safe", detection.Recommendation)
	}
}

func TestDetectSource_DirectoryMarksDominantMixedResultsSafe(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "auth.spec.ts"), []byte("import { test } from '@playwright/test'\n"), 0o644); err != nil {
		t.Fatalf("write playwright file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "checkout.spec.ts"), []byte("import { test } from '@playwright/test'\n"), 0o644); err != nil {
		t.Fatalf("write second playwright file: %v", err)
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
	if len(detection.Candidates) < 2 {
		t.Fatalf("candidate count = %d, want at least 2", len(detection.Candidates))
	}
	if !detection.Mixed {
		t.Fatal("expected mixed directory detection to be marked mixed")
	}
	if detection.Ambiguous {
		t.Fatalf("expected dominant detection, got ambiguous %+v", detection)
	}
	if !detection.AutoDetectSafe {
		t.Fatal("expected dominant mixed detection to be auto-detect safe")
	}
	if detection.Recommendation != "dominant" {
		t.Fatalf("recommendation = %q, want dominant", detection.Recommendation)
	}
	if !detection.Candidates[0].Primary {
		t.Fatal("expected top candidate to be marked primary")
	}
	if detection.Candidates[0].Framework != "playwright" || detection.Candidates[0].FileShare < 0.60 {
		t.Fatalf("unexpected primary candidate: %+v", detection.Candidates[0])
	}
}

func TestDetectSource_DirectoryMarksAmbiguousResults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "playwright.spec.ts"), []byte("import { test } from '@playwright/test'\n"), 0o644); err != nil {
		t.Fatalf("write playwright file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "cypress.spec.ts"), []byte("/// <reference types=\"cypress\" />\ncy.visit('/')\n"), 0o644); err != nil {
		t.Fatalf("write cypress file: %v", err)
	}

	detection, err := DetectSource(root)
	if err != nil {
		t.Fatalf("DetectSource returned error: %v", err)
	}
	if !detection.Mixed {
		t.Fatal("expected mixed repo detection")
	}
	if !detection.Ambiguous {
		t.Fatalf("expected ambiguous detection, got %+v", detection)
	}
	if detection.AutoDetectSafe {
		t.Fatal("expected ambiguous detection not to be auto-detect safe")
	}
	if detection.Recommendation != "ambiguous" {
		t.Fatalf("recommendation = %q, want ambiguous", detection.Recommendation)
	}
	if len(detection.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(detection.Candidates))
	}
}
