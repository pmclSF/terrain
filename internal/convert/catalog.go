package convert

import "strings"

type GoNativeState string
type CapabilityLevel string

const (
	GoNativeStateCataloged   GoNativeState = "cataloged"
	GoNativeStatePrioritized GoNativeState = "prioritized"
	// GoNativeStateExperimental marks a direction whose Go-native converter
	// runs end-to-end on the smoke fixture but has incomplete coverage of
	// real-world test patterns (assertion variants, custom matchers, lifecycle
	// hooks, framework-specific config). Round 3 review classified these as
	// C-grade. Use with caution; expect manual cleanup post-conversion.
	GoNativeStateExperimental GoNativeState = "experimental"
	GoNativeStateImplemented  GoNativeState = "implemented"

	CapabilityUnsupported CapabilityLevel = "unsupported"
	CapabilityPartial     CapabilityLevel = "partial"
	CapabilitySupported   CapabilityLevel = "supported"
)

type Framework struct {
	Name         string `json:"name"`
	Language     string `json:"language"`
	Category     string `json:"category"`
	Abbreviation string `json:"abbreviation"`
}

type Direction struct {
	From           string                `json:"from"`
	To             string                `json:"to"`
	Language       string                `json:"language"`
	Category       string                `json:"category"`
	Shorthands     []string              `json:"shorthands"`
	LegacyRuntime  string                `json:"legacyRuntime,omitempty"`
	GoNativeState  GoNativeState         `json:"goNativeState"`
	GoNativeReady  bool                  `json:"goNativeReady"`
	Implementation string                `json:"implementation"`
	Capabilities   DirectionCapabilities `json:"capabilities"`
}

type DirectionCapabilities struct {
	TestMigration    CapabilityLevel `json:"testMigration"`
	ConfigMigration  CapabilityLevel `json:"configMigration"`
	ProjectMigration CapabilityLevel `json:"projectMigration"`
	AutoDetect       CapabilityLevel `json:"autoDetect"`
	SyntaxValidation CapabilityLevel `json:"syntaxValidation"`
	ConfidenceReport CapabilityLevel `json:"confidenceReport"`
}

type DirectionCategory struct {
	Name       string      `json:"name"`
	Directions []Direction `json:"directions"`
}

type Shorthand struct {
	Alias         string        `json:"alias"`
	From          string        `json:"from"`
	To            string        `json:"to"`
	Language      string        `json:"language"`
	Category      string        `json:"category"`
	GoNativeState GoNativeState `json:"goNativeState"`
}

var frameworkOrder = []string{
	"cypress",
	"playwright",
	"selenium",
	"jest",
	"vitest",
	"mocha",
	"jasmine",
	"junit4",
	"junit5",
	"testng",
	"pytest",
	"unittest",
	"nose2",
	"webdriverio",
	"puppeteer",
	"testcafe",
}

var categoryOrder = []string{
	"JavaScript E2E / Browser",
	"JavaScript Unit Testing",
	"Java",
	"Python",
}

var frameworks = map[string]Framework{
	"cypress":     {Name: "cypress", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "cy"},
	"playwright":  {Name: "playwright", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "pw"},
	"selenium":    {Name: "selenium", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "sel"},
	"webdriverio": {Name: "webdriverio", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "wdio"},
	"puppeteer":   {Name: "puppeteer", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "pptr"},
	"testcafe":    {Name: "testcafe", Language: "javascript", Category: "JavaScript E2E / Browser", Abbreviation: "tcafe"},
	"jest":        {Name: "jest", Language: "javascript", Category: "JavaScript Unit Testing", Abbreviation: "jest"},
	"vitest":      {Name: "vitest", Language: "javascript", Category: "JavaScript Unit Testing", Abbreviation: "vt"},
	"mocha":       {Name: "mocha", Language: "javascript", Category: "JavaScript Unit Testing", Abbreviation: "mocha"},
	"jasmine":     {Name: "jasmine", Language: "javascript", Category: "JavaScript Unit Testing", Abbreviation: "jas"},
	"junit4":      {Name: "junit4", Language: "java", Category: "Java", Abbreviation: "ju4"},
	"junit5":      {Name: "junit5", Language: "java", Category: "Java", Abbreviation: "ju5"},
	"testng":      {Name: "testng", Language: "java", Category: "Java", Abbreviation: "tng"},
	"pytest":      {Name: "pytest", Language: "python", Category: "Python", Abbreviation: "pyt"},
	"unittest":    {Name: "unittest", Language: "python", Category: "Python", Abbreviation: "ut"},
	"nose2":       {Name: "nose2", Language: "python", Category: "Python", Abbreviation: "nose2"},
}

var supportedDirectionKeys = []string{
	"cypress-playwright",
	"cypress-selenium",
	"playwright-cypress",
	"playwright-selenium",
	"selenium-cypress",
	"selenium-playwright",
	"jest-vitest",
	"mocha-jest",
	"jasmine-jest",
	"jest-mocha",
	"jest-jasmine",
	"junit4-junit5",
	"junit5-testng",
	"testng-junit5",
	"pytest-unittest",
	"unittest-pytest",
	"nose2-pytest",
	"webdriverio-playwright",
	"webdriverio-cypress",
	"playwright-webdriverio",
	"cypress-webdriverio",
	"puppeteer-playwright",
	"playwright-puppeteer",
	"testcafe-playwright",
	"testcafe-cypress",
}

var prioritizedDirections = map[string]struct{}{
	"cypress-playwright": {},
}

// experimentalDirections names directions that run end-to-end on the smoke
// fixture but were rated C-grade (<70% coverage of real-world conversion
// patterns) by the round 3 review. They remain available behind the same
// CLI surface; users are warned via `terrain convert list`.
//
// Tagging policy: any direction calling out to runtime that the round 3 audit
// flagged as <70% complete on a representative real-world fixture. Promoting
// out of experimental requires:
//   - 95%+ post-conversion test pass rate on a labeled fixture corpus
//     (planned: 0.2 stage corpus per direction)
//   - documented coverage of assertion variants, lifecycle hooks, and
//     framework-specific config keys
var experimentalDirections = map[string]struct{}{
	"junit4-junit5":       {},
	"junit5-testng":       {},
	"testng-junit5":       {},
	"pytest-unittest":     {},
	"unittest-pytest":     {},
	"nose2-pytest":        {},
	"selenium-cypress":    {},
	"selenium-playwright": {},
	"testcafe-cypress":    {},
	"testcafe-playwright": {},
}

var implementedDirections = map[string]struct{}{
	"cypress-selenium":       {},
	"cypress-webdriverio":    {},
	"jasmine-jest":           {},
	"jest-vitest":            {},
	"jest-jasmine":           {},
	"jest-mocha":             {},
	"junit4-junit5":          {},
	"junit5-testng":          {},
	"cypress-playwright":     {},
	"mocha-jest":             {},
	"nose2-pytest":           {},
	"playwright-cypress":     {},
	"playwright-puppeteer":   {},
	"playwright-selenium":    {},
	"playwright-webdriverio": {},
	"puppeteer-playwright":   {},
	"pytest-unittest":        {},
	"selenium-cypress":       {},
	"selenium-playwright":    {},
	"testng-junit5":          {},
	"testcafe-cypress":       {},
	"testcafe-playwright":    {},
	"unittest-pytest":        {},
	"webdriverio-cypress":    {},
	"webdriverio-playwright": {},
}

func NormalizeFramework(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func FrameworkNames() []string {
	names := make([]string, 0, len(frameworkOrder))
	for _, name := range frameworkOrder {
		names = append(names, name)
	}
	return names
}

func LookupFramework(name string) (Framework, bool) {
	fw, ok := frameworks[NormalizeFramework(name)]
	return fw, ok
}

func Frameworks() []Framework {
	items := make([]Framework, 0, len(frameworkOrder))
	for _, name := range frameworkOrder {
		items = append(items, frameworks[name])
	}
	return items
}

func SupportedDirections() []Direction {
	items := make([]Direction, 0, len(supportedDirectionKeys))
	for _, key := range supportedDirectionKeys {
		items = append(items, directionFromKey(key))
	}
	return items
}

var supportedDirectionSet = func() map[string]bool {
	m := make(map[string]bool, len(supportedDirectionKeys))
	for _, key := range supportedDirectionKeys {
		m[key] = true
	}
	return m
}()

func LookupDirection(from, to string) (Direction, bool) {
	key := NormalizeFramework(from) + "-" + NormalizeFramework(to)
	if !supportedDirectionSet[key] {
		return Direction{}, false
	}
	return directionFromKey(key), true
}

func IsSupported(from, to string) bool {
	_, ok := LookupDirection(from, to)
	return ok
}

func SupportedTargets(from string) []string {
	from = NormalizeFramework(from)
	targets := make([]string, 0, 4)
	for _, key := range supportedDirectionKeys {
		parts := strings.SplitN(key, "-", 2)
		if len(parts) != 2 || parts[0] != from {
			continue
		}
		targets = append(targets, parts[1])
	}
	return targets
}

func Categories() []DirectionCategory {
	grouped := make(map[string][]Direction)
	for _, direction := range SupportedDirections() {
		grouped[direction.Category] = append(grouped[direction.Category], direction)
	}

	categories := make([]DirectionCategory, 0, len(categoryOrder))
	for _, name := range categoryOrder {
		directions := grouped[name]
		if len(directions) == 0 {
			continue
		}
		categories = append(categories, DirectionCategory{
			Name:       name,
			Directions: directions,
		})
	}
	return categories
}

func LookupShorthand(alias string) (Direction, bool) {
	alias = strings.ToLower(strings.TrimSpace(alias))
	for _, direction := range SupportedDirections() {
		for _, shorthand := range direction.Shorthands {
			if shorthand == alias {
				return direction, true
			}
		}
	}
	return Direction{}, false
}

func Shorthands() []Shorthand {
	items := make([]Shorthand, 0, len(supportedDirectionKeys)*2)
	for _, direction := range SupportedDirections() {
		for _, alias := range direction.Shorthands {
			items = append(items, Shorthand{
				Alias:         alias,
				From:          direction.From,
				To:            direction.To,
				Language:      direction.Language,
				Category:      direction.Category,
				GoNativeState: direction.GoNativeState,
			})
		}
	}
	return items
}

func directionFromKey(key string) Direction {
	parts := strings.SplitN(key, "-", 2)
	if len(parts) < 2 {
		return Direction{}
	}
	from := parts[0]
	to := parts[1]
	framework := frameworks[from]
	state := GoNativeStateCataloged
	if _, ok := implementedDirections[key]; ok {
		state = GoNativeStateImplemented
		// Experimental classification overrides "implemented": the converter
		// runs but coverage of real-world patterns is too partial for
		// production use. We keep the entry in implementedDirections so the
		// CLI dispatcher still routes to the Go-native runtime.
		if _, ok := experimentalDirections[key]; ok {
			state = GoNativeStateExperimental
		}
	} else if _, ok := prioritizedDirections[key]; ok {
		state = GoNativeStatePrioritized
	}
	implementation := "legacy-js-runtime"
	if state == GoNativeStateImplemented || state == GoNativeStateExperimental {
		implementation = "go-native-runtime"
	}

	return Direction{
		From:           from,
		To:             to,
		Language:       framework.Language,
		Category:       framework.Category,
		Shorthands:     buildAliases(from, to),
		LegacyRuntime:  "javascript",
		GoNativeState: state,
		// Experimental directions also dispatch to the Go-native runtime;
		// callers gating on GoNativeReady should additionally check
		// state != GoNativeStateExperimental if they want stable-only.
		GoNativeReady:  state == GoNativeStateImplemented || state == GoNativeStateExperimental,
		Implementation: implementation,
		Capabilities: DirectionCapabilities{
			TestMigration:    CapabilitySupported,
			ConfigMigration:  configCapabilityLevel(from, to),
			ProjectMigration: projectCapabilityLevel(from, to),
			AutoDetect:       CapabilitySupported,
			SyntaxValidation: CapabilitySupported,
			ConfidenceReport: CapabilitySupported,
		},
	}
}

func configCapabilityLevel(from, to string) CapabilityLevel {
	if SupportsConfigConversion(from, to) {
		return CapabilitySupported
	}
	return CapabilityUnsupported
}

func projectCapabilityLevel(from, to string) CapabilityLevel {
	if !SupportsConfigConversion(from, to) {
		return CapabilityPartial
	}
	return CapabilitySupported
}

func buildAliases(from, to string) []string {
	fromAbbrev := frameworks[from].Abbreviation
	toAbbrev := frameworks[to].Abbreviation
	numeric := fromAbbrev + "2" + toAbbrev
	long := fromAbbrev + "to" + toAbbrev
	if numeric == long {
		return []string{numeric}
	}
	return []string{numeric, long}
}
