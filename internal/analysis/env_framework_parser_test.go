package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestParsePlaywrightConfig_Browsers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "playwright.config.ts"), []byte(`
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
    { name: 'Mobile Chrome', use: { ...devices['Pixel 5'] } },
    { name: 'Mobile Safari', use: { ...devices['iPhone 13'] } },
  ],
});
`), 0o644)

	result := &FrameworkMatrixResult{}
	parsePlaywrightConfig(root, result)

	// Should detect browser class with chromium, firefox, webkit, Mobile Chrome, Mobile Safari.
	browserClassFound := false
	for _, cls := range result.EnvironmentClasses {
		if cls.Dimension == "browser" {
			browserClassFound = true
			if len(cls.MemberIDs) < 3 {
				t.Errorf("browser class: want at least 3 members, got %d", len(cls.MemberIDs))
			}
		}
	}
	if !browserClassFound {
		t.Error("expected browser environment class from Playwright config")
	}

	// Should detect device class with Pixel 5, iPhone 13, Desktop Chrome, etc.
	deviceClassFound := false
	for _, cls := range result.EnvironmentClasses {
		if cls.Dimension == "device" {
			deviceClassFound = true
			if len(cls.MemberIDs) < 2 {
				t.Errorf("device class: want at least 2 members, got %d", len(cls.MemberIDs))
			}
		}
	}
	if !deviceClassFound {
		t.Error("expected device environment class from Playwright config")
	}

	// Check device configs.
	devicePlatforms := map[string]string{} // name → platform
	for _, dc := range result.DeviceConfigs {
		devicePlatforms[dc.Name] = dc.Platform
	}
	if p, ok := devicePlatforms["iPhone 13"]; !ok {
		t.Error("expected iPhone 13 device config")
	} else if p != "ios" {
		t.Errorf("iPhone 13: want platform ios, got %s", p)
	}
	if p, ok := devicePlatforms["Pixel 5"]; !ok {
		t.Error("expected Pixel 5 device config")
	} else if p != "android" {
		t.Errorf("Pixel 5: want platform android, got %s", p)
	}
}

func TestParsePlaywrightConfig_BrowserName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "playwright.config.js"), []byte(`
module.exports = {
  projects: [
    { use: { browserName: 'chromium' } },
    { use: { browserName: 'firefox' } },
  ],
};
`), 0o644)

	result := &FrameworkMatrixResult{}
	parsePlaywrightConfig(root, result)

	if len(result.Environments) < 2 {
		t.Fatalf("expected at least 2 browser environments, got %d", len(result.Environments))
	}

	names := map[string]bool{}
	for _, env := range result.Environments {
		names[env.Name] = true
	}
	if !names["chromium"] {
		t.Error("expected chromium environment")
	}
	if !names["firefox"] {
		t.Error("expected firefox environment")
	}
}

func TestParsePytestParametrize_BrowserMatrix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "tests"), 0o755)
	os.WriteFile(filepath.Join(root, "tests", "test_cross_browser.py"), []byte(`
import pytest

@pytest.mark.parametrize("browser", ["chrome", "firefox", "safari", "edge"])
def test_login(browser):
    driver = get_driver(browser)
    driver.get("/login")
    assert driver.title == "Login"

@pytest.mark.parametrize("platform", ["linux", "macos", "windows"])
def test_platform(platform):
    assert True
`), 0o644)

	testFiles := []models.TestFile{
		{Path: "tests/test_cross_browser.py", Framework: "pytest"},
	}

	result := &FrameworkMatrixResult{}
	parsePytestParametrize(root, testFiles, result)

	classIDs := map[string]bool{}
	for _, cls := range result.EnvironmentClasses {
		classIDs[cls.ClassID] = true
	}

	if !classIDs["envclass:pytest-browser"] {
		t.Error("expected pytest browser class")
	}
	if !classIDs["envclass:pytest-platform"] {
		t.Error("expected pytest platform class")
	}

	// Check browser class members.
	for _, cls := range result.EnvironmentClasses {
		if cls.ClassID == "envclass:pytest-browser" {
			if len(cls.MemberIDs) != 4 {
				t.Errorf("browser class: want 4 members, got %d", len(cls.MemberIDs))
			}
			if cls.Dimension != "browser" {
				t.Errorf("browser class: want dimension browser, got %s", cls.Dimension)
			}
		}
		if cls.ClassID == "envclass:pytest-platform" {
			if len(cls.MemberIDs) != 3 {
				t.Errorf("platform class: want 3 members, got %d", len(cls.MemberIDs))
			}
			if cls.Dimension != "os" {
				t.Errorf("platform class: want dimension os, got %s", cls.Dimension)
			}
		}
	}

	// Verify provenance.
	for _, env := range result.Environments {
		if env.InferredFrom == "" {
			t.Errorf("env %s missing InferredFrom", env.EnvironmentID)
		}
	}
}

func TestParsePytestParametrize_NonEnvParam(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "tests"), 0o755)
	os.WriteFile(filepath.Join(root, "tests", "test_math.py"), []byte(`
import pytest

@pytest.mark.parametrize("x,y,expected", [(1,2,3), (4,5,9)])
def test_add(x, y, expected):
    assert x + y == expected
`), 0o644)

	testFiles := []models.TestFile{
		{Path: "tests/test_math.py", Framework: "pytest"},
	}

	result := &FrameworkMatrixResult{}
	parsePytestParametrize(root, testFiles, result)

	// Non-environment params should be ignored.
	if len(result.EnvironmentClasses) != 0 {
		t.Errorf("expected 0 classes for non-env params, got %d", len(result.EnvironmentClasses))
	}
}

func TestParseBrowserStackConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "browserstack.json"), []byte(`{
  "platforms": [
    {"os": "Windows", "os_version": "11", "browser": "Chrome", "browser_version": "120"},
    {"os": "OS X", "os_version": "Sonoma", "browser": "Safari", "browser_version": "17"},
    {"device": "iPhone 15", "os_version": "17"},
    {"device": "Samsung Galaxy S24", "os_version": "14"}
  ]
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseBrowserStackConfig(root, result)

	if len(result.DeviceConfigs) != 4 {
		t.Fatalf("expected 4 device configs, got %d", len(result.DeviceConfigs))
	}

	platforms := map[string]string{}
	for _, dc := range result.DeviceConfigs {
		platforms[dc.Name] = dc.Platform
	}

	if _, ok := platforms["iPhone 15"]; !ok {
		t.Error("expected iPhone 15 device")
	}
	if _, ok := platforms["Samsung Galaxy S24"]; !ok {
		t.Error("expected Samsung Galaxy S24 device")
	}

	// Browser engine detection.
	for _, dc := range result.DeviceConfigs {
		if dc.Name == "Chrome on Windows" || dc.Name == "Chrome" {
			if dc.BrowserEngine != "chromium" {
				t.Errorf("Chrome: want engine chromium, got %s", dc.BrowserEngine)
			}
		}
	}
}

func TestParseAppiumConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "appium.conf.json"), []byte(`{
  "capabilities": [
    {
      "appium:deviceName": "Pixel 7",
      "platformName": "Android",
      "platformVersion": "13"
    },
    {
      "appium:deviceName": "iPhone 14 Pro",
      "platformName": "iOS",
      "platformVersion": "16.4"
    }
  ]
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseAppiumConfig(root, result)

	if len(result.DeviceConfigs) != 2 {
		t.Fatalf("expected 2 device configs, got %d", len(result.DeviceConfigs))
	}

	names := map[string]bool{}
	for _, dc := range result.DeviceConfigs {
		names[dc.Name] = true
	}
	if !names["Pixel 7"] {
		t.Error("expected Pixel 7 device")
	}
	if !names["iPhone 14 Pro"] {
		t.Error("expected iPhone 14 Pro device")
	}
}

func TestInferDevicePlatform(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"iPhone 15 Pro", "ios"},
		{"iPad Air", "ios"},
		{"Pixel 8", "android"},
		{"Galaxy S24", "android"},
		{"Desktop Chrome", "web-browser"},
		{"Desktop Safari", "web-browser"},
		{"Unknown Device", ""},
	}
	for _, tc := range cases {
		got := inferDevicePlatform(tc.name)
		if got != tc.want {
			t.Errorf("inferDevicePlatform(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestInferFormFactor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"iPhone 15", "phone"},
		{"iPad Air", "tablet"},
		{"Pixel 8", "phone"},
		{"Desktop Chrome", "desktop"},
		{"Galaxy Tab S9", "tablet"},
		{"Unknown", ""},
	}
	for _, tc := range cases {
		got := inferFormFactor(tc.name)
		if got != tc.want {
			t.Errorf("inferFormFactor(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestInferBrowserEngine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		browser string
		want    string
	}{
		{"Chrome", "chromium"},
		{"chromium", "chromium"},
		{"Edge", "chromium"},
		{"Firefox", "gecko"},
		{"Safari", "webkit"},
		{"webkit", "webkit"},
		{"Unknown", ""},
	}
	for _, tc := range cases {
		got := inferBrowserEngine(tc.browser)
		if got != tc.want {
			t.Errorf("inferBrowserEngine(%q) = %q, want %q", tc.browser, got, tc.want)
		}
	}
}

func TestParseSauceLabsConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".sauce"), 0o755)
	os.WriteFile(filepath.Join(root, ".sauce", "config.yml"), []byte(`
apiVersion: v1alpha
suites:
  - name: "Chrome on Windows"
    platformName: "Windows 11"
    browserName: "chrome"
  - name: "Safari on macOS"
    platformName: "macOS 14"
    browserName: "safari"
  - name: "Pixel 7"
    capabilities:
      deviceName: "Google Pixel 7"
      platformName: "Android"
`), 0o644)

	result := &FrameworkMatrixResult{}
	parseSauceLabsConfig(root, result)

	if len(result.DeviceConfigs) != 3 {
		t.Fatalf("expected 3 device configs, got %d", len(result.DeviceConfigs))
	}

	names := map[string]bool{}
	for _, dc := range result.DeviceConfigs {
		names[dc.Name] = true
		if dc.InferredFrom != "saucelabs" {
			t.Errorf("device %q: InferredFrom = %q, want saucelabs", dc.Name, dc.InferredFrom)
		}
	}

	if !names["Google Pixel 7"] {
		t.Error("expected Google Pixel 7 device")
	}
	if !names["chrome on Windows 11"] {
		t.Error("expected chrome on Windows 11 device")
	}

	if len(result.EnvironmentClasses) != 1 || result.EnvironmentClasses[0].ClassID != "envclass:saucelabs-device" {
		t.Error("expected saucelabs-device environment class")
	}
}

func TestParseFirebaseTestLabConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "firebase.json"), []byte(`{
  "testlab": {
    "devices": [
      {"model": "Pixel7", "version": "33"},
      {"model": "Pixel6", "version": "31"},
      {"model": "redfin", "version": "30", "locale": "en_US"}
    ]
  }
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseFirebaseTestLabConfig(root, result)

	if len(result.DeviceConfigs) != 3 {
		t.Fatalf("expected 3 device configs, got %d", len(result.DeviceConfigs))
	}

	found := map[string]bool{}
	for _, dc := range result.DeviceConfigs {
		found[dc.Name] = true
		if dc.InferredFrom != "firebase-testlab" {
			t.Errorf("device %q: InferredFrom = %q, want firebase-testlab", dc.Name, dc.InferredFrom)
		}
	}

	if !found["Pixel7 (API 33)"] {
		t.Error("expected Pixel7 (API 33) device")
	}
	if !found["redfin (API 30)"] {
		t.Error("expected redfin (API 30) device")
	}

	if len(result.EnvironmentClasses) != 1 || result.EnvironmentClasses[0].ClassID != "envclass:firebase-device" {
		t.Error("expected firebase-device environment class")
	}
}

func TestParseFirebaseTestLabConfig_NoTestlab(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// firebase.json without testlab field should be silently ignored.
	os.WriteFile(filepath.Join(root, "firebase.json"), []byte(`{"hosting": {"public": "dist"}}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseFirebaseTestLabConfig(root, result)

	if len(result.DeviceConfigs) != 0 {
		t.Errorf("expected 0 device configs, got %d", len(result.DeviceConfigs))
	}
}

func TestParseSauceLabsConfig_EmptySuites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".sauce"), 0o755)
	os.WriteFile(filepath.Join(root, ".sauce", "config.yml"), []byte(`
apiVersion: v1alpha
suites: []
`), 0o644)

	result := &FrameworkMatrixResult{}
	parseSauceLabsConfig(root, result)

	if len(result.DeviceConfigs) != 0 {
		t.Errorf("expected 0 device configs for empty suites, got %d", len(result.DeviceConfigs))
	}
}

func TestParseSauceLabsConfig_MalformedYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, ".sauce"), 0o755)
	os.WriteFile(filepath.Join(root, ".sauce", "config.yml"), []byte(`{{{not yaml`), 0o644)

	result := &FrameworkMatrixResult{}
	parseSauceLabsConfig(root, result) // Should not panic.

	if len(result.DeviceConfigs) != 0 {
		t.Errorf("expected 0 device configs for malformed YAML, got %d", len(result.DeviceConfigs))
	}
}

func TestParseSauceLabsConfig_AlternatePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// .sauce.yml in root (alternate location).
	os.WriteFile(filepath.Join(root, ".sauce.yml"), []byte(`
apiVersion: v1alpha
suites:
  - name: "Firefox"
    browserName: "firefox"
    platformName: "Windows 10"
`), 0o644)

	result := &FrameworkMatrixResult{}
	parseSauceLabsConfig(root, result)

	if len(result.DeviceConfigs) != 1 {
		t.Fatalf("expected 1 device config from .sauce.yml, got %d", len(result.DeviceConfigs))
	}
	if result.DeviceConfigs[0].InferredFrom != "saucelabs" {
		t.Errorf("InferredFrom = %q, want saucelabs", result.DeviceConfigs[0].InferredFrom)
	}
}

func TestParseFrameworkMatrices_NoConfigs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := ParseFrameworkMatrices(root, nil)
	if len(result.DeviceConfigs) != 0 {
		t.Errorf("expected 0 device configs, got %d", len(result.DeviceConfigs))
	}
	if len(result.Environments) != 0 {
		t.Errorf("expected 0 environments, got %d", len(result.Environments))
	}
}

func TestParsePlaywrightConfig_DeviceFormFactors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "playwright.config.ts"), []byte(`
export default {
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'Mobile Safari', use: { ...devices['iPhone 13'] } },
    { name: 'Tablet', use: { ...devices['iPad Pro 11'] } },
  ],
};
`), 0o644)

	result := &FrameworkMatrixResult{}
	parsePlaywrightConfig(root, result)

	formFactors := map[string]string{}
	for _, dc := range result.DeviceConfigs {
		formFactors[dc.Name] = dc.FormFactor
	}

	if ff, ok := formFactors["iPhone 13"]; !ok {
		t.Error("expected iPhone 13")
	} else if ff != "phone" {
		t.Errorf("iPhone 13: want form_factor phone, got %s", ff)
	}

	if ff, ok := formFactors["iPad Pro 11"]; !ok {
		t.Error("expected iPad Pro 11")
	} else if ff != "tablet" {
		t.Errorf("iPad Pro 11: want form_factor tablet, got %s", ff)
	}
}

// ---------------------------------------------------------------------------
// Extended device inference tests
// ---------------------------------------------------------------------------

func TestInferDevicePlatform_ExtendedBrands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"OnePlus 12", "android"},
		{"Xiaomi 14 Pro", "android"},
		{"Redmi Note 13", "android"},
		{"Huawei P60", "android"},
		{"Motorola Edge 40", "android"},
		{"Moto G Power", "android"},
		{"Oppo Find X7", "android"},
		{"Vivo X100", "android"},
		{"Realme GT 5", "android"},
		{"Sony Xperia 1 V", "android"},
		{"Nokia G42", "android"},
		// Firebase Test Lab codenames still fall through to "" because
		// the Firebase parser applies its own "android" default.
		{"redfin", ""},
		{"oriole", ""},
	}
	for _, tc := range cases {
		got := inferDevicePlatform(tc.name)
		if got != tc.want {
			t.Errorf("inferDevicePlatform(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestInferFormFactor_ExtendedPatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"Galaxy S24 Ultra", "phone"},
		{"Galaxy A54", "phone"},
		{"Galaxy Z Flip5", "phone"},
		{"Galaxy Tab S9", "tablet"},
		{"Surface Pro 9", "tablet"},
		{"OnePlus 12", "phone"},
		{"Xiaomi 14", "phone"},
		{"Redmi Note 13", "phone"},
		{"Huawei P60", "phone"},
		{"Motorola Edge 40", "phone"},
		{"Moto G Power", "phone"},
		{"Nokia G42", "phone"},
	}
	for _, tc := range cases {
		got := inferFormFactor(tc.name)
		if got != tc.want {
			t.Errorf("inferFormFactor(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseFirebaseTestLabConfig_CodenamePlatformDefaultsToAndroid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Firebase Test Lab uses Android device codenames like "redfin" or "oriole"
	// that don't contain "pixel" or "android" in their names.
	os.WriteFile(filepath.Join(root, "firebase.json"), []byte(`{
  "testlab": {
    "devices": [
      {"model": "redfin", "version": "30"},
      {"model": "oriole", "version": "33"}
    ]
  }
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseFirebaseTestLabConfig(root, result)

	if len(result.DeviceConfigs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(result.DeviceConfigs))
	}
	for _, dc := range result.DeviceConfigs {
		if dc.Platform != "android" {
			t.Errorf("device %q: platform = %q, want 'android' (Firebase Test Lab default)", dc.Name, dc.Platform)
		}
	}
}

func TestParseBrowserStackConfig_OSFieldFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// BrowserStack config with OS field but no device name — the platform
	// should be inferred from the OS field.
	os.WriteFile(filepath.Join(root, "browserstack.json"), []byte(`{
  "platforms": [
    {"os": "iOS", "os_version": "17", "browser": "Safari", "device": ""},
    {"os": "android", "os_version": "14", "browser": "Chrome"}
  ]
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseBrowserStackConfig(root, result)

	// The first entry has empty device, so name falls through to browser "Safari".
	// inferDevicePlatform("Safari") = "web-browser", but the OS is "iOS".
	// With the fix, the OS field fallback should detect "ios".
	for _, dc := range result.DeviceConfigs {
		if dc.Platform == "" {
			t.Errorf("device %q: platform should not be empty when os field is available", dc.Name)
		}
	}
}

func TestParseAppiumConfig_ProximityAlignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Config where platformName declarations don't align by array index
	// with deviceName declarations — the proximity-based matching should
	// pick the nearest platform for each device.
	os.WriteFile(filepath.Join(root, "appium.conf.json"), []byte(`{
  "capabilities": [
    {
      "appium:deviceName": "Pixel 7",
      "platformName": "Android",
      "platformVersion": "13"
    },
    {
      "appium:deviceName": "iPhone 14 Pro",
      "platformName": "iOS",
      "platformVersion": "16.4"
    }
  ]
}`), 0o644)

	result := &FrameworkMatrixResult{}
	parseAppiumConfig(root, result)

	if len(result.DeviceConfigs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(result.DeviceConfigs))
	}

	platforms := map[string]string{}
	versions := map[string]string{}
	for _, dc := range result.DeviceConfigs {
		platforms[dc.Name] = dc.Platform
		versions[dc.Name] = dc.OSVersion
	}

	if platforms["Pixel 7"] != "android" {
		t.Errorf("Pixel 7 platform = %q, want 'android'", platforms["Pixel 7"])
	}
	if platforms["iPhone 14 Pro"] != "ios" {
		t.Errorf("iPhone 14 Pro platform = %q, want 'ios'", platforms["iPhone 14 Pro"])
	}
	if versions["Pixel 7"] != "13" {
		t.Errorf("Pixel 7 version = %q, want '13'", versions["Pixel 7"])
	}
	if versions["iPhone 14 Pro"] != "16.4" {
		t.Errorf("iPhone 14 Pro version = %q, want '16.4'", versions["iPhone 14 Pro"])
	}
}

func TestNearestMatch_FindsClosest(t *testing.T) {
	t.Parallel()
	// Simulate submatch indices: each []int has [start, end, groupStart, groupEnd]
	matches := [][]int{
		{10, 30, 15, 25},
		{100, 120, 105, 115},
		{200, 220, 205, 215},
	}

	// Position 95 should find match at 100.
	got := nearestMatch(matches, 95, 500)
	if got == nil || got[0] != 100 {
		t.Errorf("expected match at 100, got %v", got)
	}

	// Position 205 should find match at 200.
	got = nearestMatch(matches, 205, 500)
	if got == nil || got[0] != 200 {
		t.Errorf("expected match at 200, got %v", got)
	}

	// Position 500 with maxDist 50 should find nothing.
	got = nearestMatch(matches, 500, 50)
	if got != nil {
		t.Errorf("expected nil for distant position, got %v", got)
	}
}

func TestNearestMatch_EmptyMatches(t *testing.T) {
	t.Parallel()
	got := nearestMatch(nil, 50, 500)
	if got != nil {
		t.Errorf("expected nil for empty matches, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// WireMatrixToTestFiles
// ---------------------------------------------------------------------------

func TestWireMatrixToTestFiles_PlaywrightDevices(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/login.spec.ts", Framework: "playwright"},
		{Path: "tests/checkout.spec.ts", Framework: "playwright"},
		{Path: "tests/utils.test.ts", Framework: "jest"}, // Not E2E — should not get devices.
	}
	result := &FrameworkMatrixResult{
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:pw-iphone-13", InferredFrom: "playwright-config"},
			{DeviceID: "device:pw-pixel-5", InferredFrom: "playwright-config"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	if len(testFiles[0].DeviceIDs) != 2 {
		t.Errorf("playwright file[0] DeviceIDs = %d, want 2", len(testFiles[0].DeviceIDs))
	}
	if len(testFiles[1].DeviceIDs) != 2 {
		t.Errorf("playwright file[1] DeviceIDs = %d, want 2", len(testFiles[1].DeviceIDs))
	}
	if len(testFiles[2].DeviceIDs) != 0 {
		t.Errorf("jest file DeviceIDs = %d, want 0 (non-E2E)", len(testFiles[2].DeviceIDs))
	}
}

func TestWireMatrixToTestFiles_CIPlatformDevices(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/e2e/login.cy.ts", Framework: "cypress"},
		{Path: "tests/e2e/nav.test.ts", Framework: "selenium"},
		{Path: "tests/unit/math.test.ts", Framework: "jest"},
	}
	result := &FrameworkMatrixResult{
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:bs-iphone-15", InferredFrom: "browserstack"},
			{DeviceID: "device:bs-galaxy-s24", InferredFrom: "browserstack"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	if len(testFiles[0].DeviceIDs) != 2 {
		t.Errorf("cypress file DeviceIDs = %d, want 2", len(testFiles[0].DeviceIDs))
	}
	if len(testFiles[1].DeviceIDs) != 2 {
		t.Errorf("selenium file DeviceIDs = %d, want 2", len(testFiles[1].DeviceIDs))
	}
	if len(testFiles[2].DeviceIDs) != 0 {
		t.Errorf("jest file DeviceIDs = %d, want 0", len(testFiles[2].DeviceIDs))
	}
}

func TestWireMatrixToTestFiles_PlaywrightGetsBothSources(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/e2e.spec.ts", Framework: "playwright"},
	}
	result := &FrameworkMatrixResult{
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:pw-pixel-5", InferredFrom: "playwright-config"},
			{DeviceID: "device:bs-galaxy-s24", InferredFrom: "browserstack"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	// Playwright files should get both Playwright devices AND CI platform devices.
	if len(testFiles[0].DeviceIDs) != 2 {
		t.Errorf("DeviceIDs = %v, want 2 (Playwright + BrowserStack)", testFiles[0].DeviceIDs)
	}
}

func TestWireMatrixToTestFiles_NilResult(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/login.spec.ts", Framework: "playwright"},
	}
	WireMatrixToTestFiles(testFiles, nil) // Should not panic.
	if len(testFiles[0].DeviceIDs) != 0 {
		t.Error("expected no change with nil result")
	}
}

func TestWireMatrixToTestFiles_NoDuplicates(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/e2e.spec.ts", Framework: "playwright", DeviceIDs: []string{"device:pw-pixel-5"}},
	}
	result := &FrameworkMatrixResult{
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:pw-pixel-5", InferredFrom: "playwright-config"},
			{DeviceID: "device:pw-iphone-13", InferredFrom: "playwright-config"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	// Should not duplicate the pre-existing "device:pw-pixel-5".
	if len(testFiles[0].DeviceIDs) != 2 {
		t.Errorf("DeviceIDs = %v, want 2 (no duplicates)", testFiles[0].DeviceIDs)
	}
}

func TestWireMatrixToTestFiles_AppiumAndFirebase(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/e2e/login.wdio.ts", Framework: "webdriverio"},
		{Path: "tests/e2e/home.test.ts", Framework: "testcafe"},
	}
	result := &FrameworkMatrixResult{
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:appium-pixel-7", InferredFrom: "appium:wdio.conf.js"},
			{DeviceID: "device:ftl-redfin", InferredFrom: "firebase-testlab"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	if len(testFiles[0].DeviceIDs) != 2 {
		t.Errorf("wdio file DeviceIDs = %d, want 2", len(testFiles[0].DeviceIDs))
	}
	if len(testFiles[1].DeviceIDs) != 2 {
		t.Errorf("testcafe file DeviceIDs = %d, want 2", len(testFiles[1].DeviceIDs))
	}
}

func TestWireMatrixToTestFiles_PlaywrightEnvironments(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/login.spec.ts", Framework: "playwright"},
		{Path: "tests/utils.test.ts", Framework: "jest"},
	}
	result := &FrameworkMatrixResult{
		Environments: []models.Environment{
			{EnvironmentID: "env:pw-chromium", InferredFrom: "playwright-config"},
			{EnvironmentID: "env:pw-firefox", InferredFrom: "playwright-config"},
			{EnvironmentID: "env:pw-webkit", InferredFrom: "playwright-config"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	if len(testFiles[0].EnvironmentIDs) != 3 {
		t.Errorf("playwright file EnvironmentIDs = %d, want 3", len(testFiles[0].EnvironmentIDs))
	}
	if len(testFiles[1].EnvironmentIDs) != 0 {
		t.Errorf("jest file EnvironmentIDs = %d, want 0", len(testFiles[1].EnvironmentIDs))
	}
}

func TestIsPWDesktopBrowserRef(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ref  string
		want bool
	}{
		{"Desktop Chrome", true},
		{"Desktop Firefox", true},
		{"Desktop Safari", true},
		{"iPhone 13", false},
		{"Pixel 5", false},
		{"Galaxy S24", false},
	}
	for _, tc := range cases {
		got := isPWDesktopBrowserRef(tc.ref)
		if got != tc.want {
			t.Errorf("isPWDesktopBrowserRef(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

func TestIsPWBrowser_MobileProjectNamesAreNotBrowsers(t *testing.T) {
	t.Parallel()
	// "Mobile Chrome" and "Mobile Safari" are Playwright project names that
	// correspond to device emulations, not browser identifiers.
	cases := []struct {
		name string
		want bool
	}{
		{"chromium", true},
		{"firefox", true},
		{"webkit", true},
		{"chrome", true},
		{"msedge", true},
		{"safari", true},
		{"Mobile Chrome", false},
		{"Mobile Safari", false},
		{"mobile firefox", false},
	}
	for _, tc := range cases {
		got := isPWBrowser(tc.name)
		if got != tc.want {
			t.Errorf("isPWBrowser(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestInferFormFactor_GalaxyTabWithoutTrailingSpace(t *testing.T) {
	t.Parallel()
	// "Galaxy Tab" without a trailing space (e.g., at end of string)
	// should still be recognized as a tablet, not fall through to phone.
	cases := []struct {
		name string
		want string
	}{
		{"Galaxy Tab", "tablet"},
		{"Galaxy Tab S9", "tablet"},
		{"Samsung Galaxy Tab", "tablet"},
		{"Galaxy S24", "phone"},
	}
	for _, tc := range cases {
		got := inferFormFactor(tc.name)
		if got != tc.want {
			t.Errorf("inferFormFactor(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestWireMatrixToTestFiles_PytestParametrize(t *testing.T) {
	t.Parallel()
	testFiles := []models.TestFile{
		{Path: "tests/test_login.py", Framework: "pytest"},
		{Path: "tests/test_other.py", Framework: "pytest"},
	}
	result := &FrameworkMatrixResult{
		Environments: []models.Environment{
			{EnvironmentID: "env:pytest-browser-chrome", InferredFrom: "pytest-parametrize:tests/test_login.py"},
			{EnvironmentID: "env:pytest-browser-firefox", InferredFrom: "pytest-parametrize:tests/test_login.py"},
		},
	}

	WireMatrixToTestFiles(testFiles, result)

	// test_login.py should get the environments from its parametrize decorator.
	if len(testFiles[0].EnvironmentIDs) != 2 {
		t.Errorf("test_login.py EnvironmentIDs = %d, want 2", len(testFiles[0].EnvironmentIDs))
	}
	// test_other.py should NOT get them — they belong to a different file.
	if len(testFiles[1].EnvironmentIDs) != 0 {
		t.Errorf("test_other.py EnvironmentIDs = %d, want 0", len(testFiles[1].EnvironmentIDs))
	}
}
