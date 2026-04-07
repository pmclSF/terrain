package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/models"
	"gopkg.in/yaml.v3"
)

// FrameworkMatrixResult holds device configs and environment info from test
// framework configuration parsing.
type FrameworkMatrixResult struct {
	DeviceConfigs      []models.DeviceConfig
	Environments       []models.Environment
	EnvironmentClasses []models.EnvironmentClass
}

// WireMatrixToTestFiles connects parsed device configs and environment IDs to
// test files based on framework affinity. For example, Playwright devices from
// playwright.config apply to all Playwright test files; BrowserStack/Appium/
// Sauce Labs/Firebase devices apply to all E2E test files since those are
// project-wide CI configs. Similarly, Playwright browser environments
// (chromium, firefox, webkit) are wired to all Playwright test files.
//
// This bridges the gap between config-level parsing and per-file targeting,
// enabling the matrix analysis engine to compute real coverage.
func WireMatrixToTestFiles(testFiles []models.TestFile, result *FrameworkMatrixResult) {
	if result == nil {
		return
	}

	// Group device IDs by provenance prefix.
	var playwrightDeviceIDs []string
	var ciPlatformDeviceIDs []string // BrowserStack, Appium, Sauce Labs, Firebase

	for _, dc := range result.DeviceConfigs {
		switch {
		case strings.HasPrefix(dc.InferredFrom, "playwright"):
			playwrightDeviceIDs = append(playwrightDeviceIDs, dc.DeviceID)
		case strings.HasPrefix(dc.InferredFrom, "browserstack"),
			strings.HasPrefix(dc.InferredFrom, "appium"),
			strings.HasPrefix(dc.InferredFrom, "saucelabs"),
			strings.HasPrefix(dc.InferredFrom, "firebase"):
			ciPlatformDeviceIDs = append(ciPlatformDeviceIDs, dc.DeviceID)
		}
	}

	// Group environment IDs by provenance prefix.
	var playwrightEnvIDs []string
	// pytest-parametrize environments map back to the specific test file that
	// contains the decorator: InferredFrom = "pytest-parametrize:<path>".
	pytestEnvsByFile := map[string][]string{} // file path → environment IDs
	for _, env := range result.Environments {
		switch {
		case strings.HasPrefix(env.InferredFrom, "playwright"):
			playwrightEnvIDs = append(playwrightEnvIDs, env.EnvironmentID)
		case strings.HasPrefix(env.InferredFrom, "pytest-parametrize:"):
			filePath := strings.TrimPrefix(env.InferredFrom, "pytest-parametrize:")
			pytestEnvsByFile[filePath] = append(pytestEnvsByFile[filePath], env.EnvironmentID)
		}
	}

	hasDevices := len(playwrightDeviceIDs) > 0 || len(ciPlatformDeviceIDs) > 0
	hasEnvs := len(playwrightEnvIDs) > 0 || len(pytestEnvsByFile) > 0
	if !hasDevices && !hasEnvs {
		return
	}

	for i := range testFiles {
		tf := &testFiles[i]
		switch tf.Framework {
		case "playwright":
			tf.DeviceIDs = appendUniqueStrings(tf.DeviceIDs, playwrightDeviceIDs)
			tf.EnvironmentIDs = appendUniqueStrings(tf.EnvironmentIDs, playwrightEnvIDs)
		case "cypress", "selenium", "webdriverio", "puppeteer", "testcafe":
			tf.DeviceIDs = appendUniqueStrings(tf.DeviceIDs, ciPlatformDeviceIDs)
		}

		// Playwright tests also get CI platform devices if present.
		if tf.Framework == "playwright" && len(ciPlatformDeviceIDs) > 0 {
			tf.DeviceIDs = appendUniqueStrings(tf.DeviceIDs, ciPlatformDeviceIDs)
		}

		// pytest-parametrize environments are per-file: wire them back to
		// the specific test file that contains the decorator.
		if envIDs, ok := pytestEnvsByFile[tf.Path]; ok {
			tf.EnvironmentIDs = appendUniqueStrings(tf.EnvironmentIDs, envIDs)
		}
	}
}

func appendUniqueStrings(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range src {
		if !seen[s] {
			seen[s] = true
			dst = append(dst, s)
		}
	}
	return dst
}

// ParseFrameworkMatrices scans repository configuration files and test files
// for test framework matrix definitions — Playwright browser configs, mobile
// device matrices, and pytest parametrize markers.
func ParseFrameworkMatrices(root string, testFiles []models.TestFile) *FrameworkMatrixResult {
	result := &FrameworkMatrixResult{}

	parsePlaywrightConfig(root, result)
	parsePytestIni(root, result)
	parsePytestParametrize(root, testFiles, result)
	parseMobileDeviceConfigs(root, result)

	return result
}

// --- Playwright browser configuration ---

// playwrightConfig is a minimal representation of playwright.config.ts/js.
// We parse it with regex since it's JS/TS, not YAML.
var (
	// Match projects: [ { name: 'chromium', use: { ...devices['Desktop Chrome'] } }, ... ]
	pwProjectNamePattern = regexp.MustCompile(`name:\s*['"](\w[\w\s-]*)['"]`)
	// Match use: { browserName: 'chromium' }
	pwBrowserNamePattern = regexp.MustCompile(`browserName:\s*['"](\w+)['"]`)
	// Match ...devices['iPhone 13']
	pwDevicePattern = regexp.MustCompile(`devices\[['"]([^'"]+)['"]\]`)
)

func parsePlaywrightConfig(root string, result *FrameworkMatrixResult) {
	configPaths := []string{
		"playwright.config.ts",
		"playwright.config.mts",
		"playwright.config.js",
		"playwright.config.mjs",
		"playwright.config.cjs",
	}

	var content string
	for _, p := range configPaths {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err == nil {
			content = string(data)
			break
		}
	}
	if content == "" {
		return
	}

	provenance := "playwright-config"

	// Extract project names (browser names).
	browsers := map[string]bool{}
	for _, m := range pwProjectNamePattern.FindAllStringSubmatch(content, -1) {
		name := strings.TrimSpace(m[1])
		if isPWBrowser(name) {
			browsers[name] = true
		}
	}
	for _, m := range pwBrowserNamePattern.FindAllStringSubmatch(content, -1) {
		browsers[m[1]] = true
	}

	// Extract device references, separating desktop browsers from real devices.
	devices := map[string]bool{}
	for _, m := range pwDevicePattern.FindAllStringSubmatch(content, -1) {
		ref := m[1]
		if isPWDesktopBrowserRef(ref) {
			// Desktop browser references like "Desktop Chrome" are environments,
			// not device configs. Add them to the browser set instead.
			browsers[ref] = true
			continue
		}
		devices[ref] = true
	}

	// Create browser environment class + environments.
	if len(browsers) > 0 {
		classID := "envclass:pw-browser"
		memberIDs := make([]string, 0, len(browsers))
		sortedBrowsers := sortedStringSet(browsers)

		for _, browser := range sortedBrowsers {
			envID := "env:pw-" + sanitizeID(browser)
			memberIDs = append(memberIDs, envID)

			result.Environments = append(result.Environments, models.Environment{
				EnvironmentID: envID,
				Name:          browser,
				CIProvider:    "",
				ClassID:       classID,
				InferredFrom:  provenance,
			})
		}

		result.EnvironmentClasses = append(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "Playwright browsers",
			Dimension: "browser",
			MemberIDs: memberIDs,
		})
	}

	// Create device configs from Playwright device references.
	if len(devices) > 0 {
		classID := "envclass:pw-device"
		memberIDs := make([]string, 0, len(devices))
		sortedDevices := sortedStringSet(devices)

		for _, device := range sortedDevices {
			deviceID := "device:pw-" + sanitizeID(device)
			memberIDs = append(memberIDs, deviceID)

			result.DeviceConfigs = append(result.DeviceConfigs, models.DeviceConfig{
				DeviceID:     deviceID,
				Name:         device,
				Platform:     inferDevicePlatform(device),
				FormFactor:   inferFormFactor(device),
				ClassID:      classID,
				InferredFrom: provenance,
			})
		}

		result.EnvironmentClasses = append(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "Playwright devices",
			Dimension: "device",
			MemberIDs: memberIDs,
		})
	}
}

func isPWBrowser(name string) bool {
	lower := strings.ToLower(name)
	// "Mobile Chrome" and "Mobile Safari" are device-emulating project names
	// in Playwright (they correspond to devices['Pixel 5'] etc.), not browsers.
	if strings.Contains(lower, "mobile") {
		return false
	}
	return lower == "chromium" || lower == "firefox" || lower == "webkit" ||
		lower == "chrome" || lower == "msedge" || lower == "safari"
}

// isPWDesktopBrowserRef returns true for Playwright devices[...] references
// that represent desktop browsers rather than mobile devices.
// Examples: "Desktop Chrome", "Desktop Firefox", "Desktop Safari".
func isPWDesktopBrowserRef(ref string) bool {
	return strings.HasPrefix(ref, "Desktop ")
}

// --- pytest configuration and parametrize ---

func parsePytestIni(root string, result *FrameworkMatrixResult) {
	// Check pytest.ini, pyproject.toml, setup.cfg for markers or environment config.
	// This is a lightweight check for environment-related markers.
	configPaths := []string{
		filepath.Join(root, "pytest.ini"),
		filepath.Join(root, "pyproject.toml"),
	}

	for _, p := range configPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)

		// Look for env markers in markers section.
		if strings.Contains(content, "[tool.pytest") || strings.Contains(content, "[pytest]") {
			extractPytestEnvMarkers(content, result)
		}
	}
}

var pytestMarkerPattern = regexp.MustCompile(`(?m)^\s*(\w+):\s*(?:mark\s+for\s+)?(?:run(?:ning)?\s+(?:on|in|with)\s+)?(.+)$`)

func extractPytestEnvMarkers(content string, _ *FrameworkMatrixResult) {
	// Scan for environment-related pytest markers. Currently informational only:
	// markers become actionable when pytest.parametrize or conftest fixtures
	// reference them. This function exists as scaffolding for that future wiring.
	lines := strings.Split(content, "\n")
	inMarkers := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "markers") && strings.Contains(trimmed, "=") {
			inMarkers = true
			continue
		}
		if inMarkers {
			if trimmed == "" || (!strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(trimmed, "\"") && !strings.Contains(trimmed, ":")) {
				break
			}
			// Clean up TOML multiline string markers.
			cleaned := strings.Trim(trimmed, "\"',")
			if m := pytestMarkerPattern.FindStringSubmatch(cleaned); m != nil {
				_ = isEnvironmentMarker(m[1]) // validate but don't store yet
			}
		}
	}
}

func isEnvironmentMarker(marker string) bool {
	lower := strings.ToLower(marker)
	return strings.Contains(lower, "linux") || strings.Contains(lower, "macos") ||
		strings.Contains(lower, "windows") || strings.Contains(lower, "chrome") ||
		strings.Contains(lower, "firefox") || strings.Contains(lower, "safari") ||
		strings.Contains(lower, "browser") || strings.Contains(lower, "mobile") ||
		strings.Contains(lower, "device")
}

// parsePytestParametrize scans Python test files for @pytest.mark.parametrize
// decorators that define environment or browser matrices.
var (
	pytestParametrizePattern = regexp.MustCompile(`@pytest\.mark\.parametrize\s*\(\s*['"](\w+)['"]\s*,\s*\[([^\]]+)\]`)
	pytestParamValuePattern  = regexp.MustCompile(`['"]([^'"]+)['"]`)
)

func parsePytestParametrize(root string, testFiles []models.TestFile, result *FrameworkMatrixResult) {
	for _, tf := range testFiles {
		if !strings.HasSuffix(tf.Path, ".py") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, tf.Path))
		if err != nil {
			continue
		}
		src := string(data)

		matches := pytestParametrizePattern.FindAllStringSubmatch(src, -1)
		for _, m := range matches {
			paramName := m[1]
			paramValues := m[2]

			if !isEnvironmentParam(paramName) {
				continue
			}

			values := pytestParamValuePattern.FindAllStringSubmatch(paramValues, -1)
			if len(values) == 0 {
				continue
			}

			dimension := inferDimension(paramName)
			classID := "envclass:pytest-" + sanitizeID(paramName)
			memberIDs := make([]string, 0, len(values))

			for _, v := range values {
				val := v[1]
				envID := "env:pytest-" + sanitizeID(paramName) + "-" + sanitizeID(val)
				memberIDs = append(memberIDs, envID)

				result.Environments = appendEnvIfNew(result.Environments, models.Environment{
					EnvironmentID: envID,
					Name:          paramName + " " + val,
					ClassID:       classID,
					InferredFrom:  "pytest-parametrize:" + tf.Path,
				})
			}

			result.EnvironmentClasses = appendClassIfNew(result.EnvironmentClasses, models.EnvironmentClass{
				ClassID:   classID,
				Name:      paramName,
				Dimension: dimension,
				MemberIDs: memberIDs,
			})
		}
	}
}

func isEnvironmentParam(name string) bool {
	lower := strings.ToLower(name)
	return lower == "browser" || lower == "browsers" ||
		lower == "os" || lower == "platform" ||
		lower == "device" || lower == "devices" ||
		lower == "runtime" || lower == "version" ||
		lower == "env" || lower == "environment" ||
		lower == "python_version" || lower == "node_version"
}

// --- Mobile device matrices ---

// parseMobileDeviceConfigs looks for device matrix definitions in config files
// commonly used by mobile test frameworks.
func parseMobileDeviceConfigs(root string, result *FrameworkMatrixResult) {
	// BrowserStack config (browserstack.json or .browserstack.yml)
	parseBrowserStackConfig(root, result)
	// Appium capabilities in JSON
	parseAppiumConfig(root, result)
	// Sauce Labs config (.sauce/config.yml or .sauce.yml)
	parseSauceLabsConfig(root, result)
	// Firebase Test Lab config (firebase.json with testlab field)
	parseFirebaseTestLabConfig(root, result)
}

func parseBrowserStackConfig(root string, result *FrameworkMatrixResult) {
	paths := []string{
		filepath.Join(root, "browserstack.json"),
		filepath.Join(root, ".browserstack.yml"),
		filepath.Join(root, "browserstack.yml"),
	}

	var data []byte
	for _, p := range paths {
		d, err := os.ReadFile(p)
		if err == nil {
			data = d
			break
		}
	}
	if data == nil {
		return
	}

	provenance := "browserstack"

	// yaml.Unmarshal handles both YAML and JSON.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}

	// Extract platforms array: [{os, os_version, browser, browser_version, device}, ...]
	platforms := extractInterfaceSlice(raw, "platforms")
	if platforms == nil {
		platforms = extractInterfaceSlice(raw, "browsers")
	}

	classID := "envclass:browserstack-device"
	var memberIDs []string

	for _, p := range platforms {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		device := stringFromMap(pMap, "device")
		browser := stringFromMap(pMap, "browser")
		osName := stringFromMap(pMap, "os")
		osVersion := stringFromMap(pMap, "os_version")

		var name string
		if device != "" {
			name = device
		} else if browser != "" {
			name = browser
			if osName != "" {
				name += " on " + osName
			}
		} else {
			continue
		}

		deviceID := "device:bs-" + sanitizeID(name)
		memberIDs = append(memberIDs, deviceID)

		platform := inferDevicePlatform(name)
		if platform == "" && osName != "" {
			// Fall back to the explicit OS field when the device/browser
			// name alone doesn't reveal the platform.
			platform = inferDevicePlatform(osName)
		}

		dc := models.DeviceConfig{
			DeviceID:     deviceID,
			Name:         name,
			Platform:     platform,
			FormFactor:   inferFormFactor(name),
			OSVersion:    osVersion,
			ClassID:      classID,
			InferredFrom: provenance,
		}
		if browser != "" {
			dc.BrowserEngine = inferBrowserEngine(browser)
		}

		result.DeviceConfigs = appendDeviceIfNew(result.DeviceConfigs, dc)
	}

	if len(memberIDs) > 0 {
		result.EnvironmentClasses = appendClassIfNew(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "BrowserStack devices",
			Dimension: "device",
			MemberIDs: memberIDs,
		})
	}
}

func parseAppiumConfig(root string, result *FrameworkMatrixResult) {
	paths := []string{
		filepath.Join(root, "appium.conf.json"),
		filepath.Join(root, "wdio.conf.js"),
		filepath.Join(root, "wdio.conf.ts"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		extractAppiumDevices(content, filepath.Base(p), result)
	}
}

var (
	appiumDeviceNamePattern  = regexp.MustCompile(`['"]appium:deviceName['"]\s*:\s*['"]([^'"]+)['"]`)
	appiumPlatformPattern    = regexp.MustCompile(`['"](?:appium:)?platformName['"]\s*:\s*['"]([^'"]+)['"]`)
	appiumPlatformVerPattern = regexp.MustCompile(`['"](?:appium:)?platformVersion['"]\s*:\s*['"]([^'"]+)['"]`)
)

func extractAppiumDevices(content, filename string, result *FrameworkMatrixResult) {
	deviceMatches := appiumDeviceNamePattern.FindAllStringSubmatchIndex(content, -1)
	if len(deviceMatches) == 0 {
		return
	}

	provenance := "appium:" + filename
	classID := "envclass:appium-device"
	var memberIDs []string

	platformMatches := appiumPlatformPattern.FindAllStringSubmatchIndex(content, -1)
	versionMatches := appiumPlatformVerPattern.FindAllStringSubmatchIndex(content, -1)

	for _, dm := range deviceMatches {
		name := content[dm[2]:dm[3]]
		deviceID := "device:appium-" + sanitizeID(name)
		memberIDs = append(memberIDs, deviceID)

		dc := models.DeviceConfig{
			DeviceID:     deviceID,
			Name:         name,
			Platform:     inferDevicePlatform(name),
			FormFactor:   inferFormFactor(name),
			ClassID:      classID,
			InferredFrom: provenance,
		}

		// Find the nearest platformName and platformVersion within 500 chars
		// of this deviceName match, rather than relying on array-index alignment.
		if pm := nearestMatch(platformMatches, dm[0], 500); pm != nil {
			dc.Platform = strings.ToLower(content[pm[2]:pm[3]])
		}
		if vm := nearestMatch(versionMatches, dm[0], 500); vm != nil {
			dc.OSVersion = content[vm[2]:vm[3]]
		}

		result.DeviceConfigs = appendDeviceIfNew(result.DeviceConfigs, dc)
	}

	if len(memberIDs) > 0 {
		result.EnvironmentClasses = appendClassIfNew(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "Appium devices",
			Dimension: "device",
			MemberIDs: memberIDs,
		})
	}
}

// --- Sauce Labs ---

func parseSauceLabsConfig(root string, result *FrameworkMatrixResult) {
	paths := []string{
		filepath.Join(root, ".sauce", "config.yml"),
		filepath.Join(root, ".sauce.yml"),
		filepath.Join(root, ".sauce", "config.yaml"),
	}

	var data []byte
	for _, p := range paths {
		d, err := os.ReadFile(p)
		if err == nil {
			data = d
			break
		}
	}
	if data == nil {
		return
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}

	classID := "envclass:saucelabs-device"
	var memberIDs []string

	// Sauce Labs config: suites[].capabilities or suites[].platformName/browserName
	suites := extractInterfaceSlice(raw, "suites")
	for _, s := range suites {
		sMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		// Direct fields on suite.
		platformName := stringFromMap(sMap, "platformName")
		browserName := stringFromMap(sMap, "browserName")
		deviceName := stringFromMap(sMap, "deviceName")

		// Or nested in capabilities.
		if caps, ok := sMap["capabilities"].(map[string]interface{}); ok {
			if platformName == "" {
				platformName = stringFromMap(caps, "platformName")
			}
			if browserName == "" {
				browserName = stringFromMap(caps, "browserName")
			}
			if deviceName == "" {
				deviceName = stringFromMap(caps, "deviceName")
			}
		}

		var name string
		if deviceName != "" {
			name = deviceName
		} else if browserName != "" {
			name = browserName
			if platformName != "" {
				name += " on " + platformName
			}
		} else if platformName != "" {
			name = platformName
		} else {
			continue
		}

		deviceID := "device:sl-" + sanitizeID(name)
		memberIDs = append(memberIDs, deviceID)

		dc := models.DeviceConfig{
			DeviceID:     deviceID,
			Name:         name,
			Platform:     inferDevicePlatform(name),
			FormFactor:   inferFormFactor(name),
			ClassID:      classID,
			InferredFrom: "saucelabs",
		}
		if browserName != "" {
			dc.BrowserEngine = inferBrowserEngine(browserName)
		}

		result.DeviceConfigs = appendDeviceIfNew(result.DeviceConfigs, dc)
	}

	if len(memberIDs) > 0 {
		result.EnvironmentClasses = appendClassIfNew(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "Sauce Labs devices",
			Dimension: "device",
			MemberIDs: memberIDs,
		})
	}
}

// --- Firebase Test Lab ---

func parseFirebaseTestLabConfig(root string, result *FrameworkMatrixResult) {
	data, err := os.ReadFile(filepath.Join(root, "firebase.json"))
	if err != nil {
		return
	}

	// firebase.json is standard JSON; yaml.Unmarshal handles JSON too.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}

	// Look for testlab configuration.
	testlab, ok := raw["testlab"].(map[string]interface{})
	if !ok {
		// Also check emulators.testlab or hosting.testlab as fallback.
		if emu, ok := raw["emulators"].(map[string]interface{}); ok {
			testlab, _ = emu["testlab"].(map[string]interface{})
		}
	}
	if testlab == nil {
		return
	}

	classID := "envclass:firebase-device"
	var memberIDs []string

	// Firebase Test Lab: devices[] array with model, version, locale, orientation.
	devices := extractInterfaceSlice(testlab, "devices")
	if devices == nil {
		// Also check device as a flat key.
		devices = extractInterfaceSlice(testlab, "device")
	}

	for _, d := range devices {
		dMap, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		model := stringFromMap(dMap, "model")
		version := stringFromMap(dMap, "version")

		if model == "" {
			continue
		}

		name := model
		if version != "" {
			name += " (API " + version + ")"
		}

		deviceID := "device:ftl-" + sanitizeID(model)
		memberIDs = append(memberIDs, deviceID)

		platform := inferDevicePlatform(model)
		if platform == "" {
			// Firebase Test Lab devices are Android; codenames like
			// "redfin" or "oriole" won't match inferDevicePlatform.
			platform = "android"
		}

		dc := models.DeviceConfig{
			DeviceID:     deviceID,
			Name:         name,
			Platform:     platform,
			FormFactor:   inferFormFactor(model),
			OSVersion:    version,
			ClassID:      classID,
			InferredFrom: "firebase-testlab",
		}

		result.DeviceConfigs = appendDeviceIfNew(result.DeviceConfigs, dc)
	}

	if len(memberIDs) > 0 {
		result.EnvironmentClasses = appendClassIfNew(result.EnvironmentClasses, models.EnvironmentClass{
			ClassID:   classID,
			Name:      "Firebase Test Lab devices",
			Dimension: "device",
			MemberIDs: memberIDs,
		})
	}
}

// --- Helpers ---

func inferDevicePlatform(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad") || strings.Contains(lower, "ios"):
		return "ios"
	case strings.Contains(lower, "pixel") || strings.Contains(lower, "galaxy") ||
		strings.Contains(lower, "android") || strings.Contains(lower, "oneplus") ||
		strings.Contains(lower, "xiaomi") || strings.Contains(lower, "redmi") ||
		strings.Contains(lower, "huawei") || strings.Contains(lower, "motorola") ||
		strings.Contains(lower, "moto ") || lower == "moto" || strings.Contains(lower, "oppo") ||
		strings.Contains(lower, "vivo") || strings.Contains(lower, "realme") ||
		strings.Contains(lower, "xperia") || strings.Contains(lower, "nokia"):
		return "android"
	case strings.Contains(lower, "chrome") || strings.Contains(lower, "firefox") ||
		strings.Contains(lower, "safari") || strings.Contains(lower, "edge") ||
		strings.Contains(lower, "webkit") || strings.Contains(lower, "desktop"):
		return "web-browser"
	default:
		return ""
	}
}

func inferFormFactor(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") ||
		strings.Contains(lower, "tab ") || strings.HasSuffix(lower, " tab") ||
		strings.Contains(lower, "surface"):
		return "tablet"
	case strings.Contains(lower, "desktop"):
		return "desktop"
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "pixel") ||
		strings.Contains(lower, "phone") || strings.Contains(lower, "mobile") ||
		strings.Contains(lower, "oneplus") || strings.Contains(lower, "xiaomi") ||
		strings.Contains(lower, "redmi") || strings.Contains(lower, "huawei") ||
		strings.Contains(lower, "motorola") || strings.Contains(lower, "moto ") || lower == "moto" ||
		strings.Contains(lower, "oppo") || strings.Contains(lower, "vivo") ||
		strings.Contains(lower, "realme") || strings.Contains(lower, "xperia") ||
		strings.Contains(lower, "nokia"):
		return "phone"
	// Galaxy without "tab" is a phone (Galaxy S, A, Z series).
	case strings.Contains(lower, "galaxy"):
		return "phone"
	default:
		return ""
	}
}

func inferBrowserEngine(browser string) string {
	lower := strings.ToLower(browser)
	switch {
	case strings.Contains(lower, "chrome") || strings.Contains(lower, "chromium") || strings.Contains(lower, "edge"):
		return "chromium"
	case strings.Contains(lower, "firefox"):
		return "gecko"
	case strings.Contains(lower, "safari") || strings.Contains(lower, "webkit"):
		return "webkit"
	default:
		return ""
	}
}

func extractInterfaceSlice(m map[string]interface{}, key string) []interface{} {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	return arr
}

func stringFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

// nearestMatch finds the regex submatch index closest to pos within maxDist.
// Each entry in matches is a []int from FindAllStringSubmatchIndex.
// It prefers the nearest forward match (same config block) and falls back
// to the nearest match in either direction.
// Returns nil if no match is within range.
func nearestMatch(matches [][]int, pos, maxDist int) []int {
	var bestForward, bestAny []int
	bestForwardDist := maxDist + 1
	bestAnyDist := maxDist + 1
	for _, m := range matches {
		d := m[0] - pos
		absDist := d
		if absDist < 0 {
			absDist = -absDist
		}
		if absDist < bestAnyDist {
			bestAnyDist = absDist
			bestAny = m
		}
		if d >= 0 && d < bestForwardDist {
			bestForwardDist = d
			bestForward = m
		}
	}
	if bestForward != nil && bestForwardDist <= maxDist {
		return bestForward
	}
	if bestAnyDist <= maxDist {
		return bestAny
	}
	return nil
}

func appendDeviceIfNew(devices []models.DeviceConfig, dc models.DeviceConfig) []models.DeviceConfig {
	for _, d := range devices {
		if d.DeviceID == dc.DeviceID {
			return devices
		}
	}
	return append(devices, dc)
}

func sortedStringSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
