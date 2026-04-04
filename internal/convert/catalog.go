package convert

import "strings"

type GoNativeState string

const (
	GoNativeStateCataloged   GoNativeState = "cataloged"
	GoNativeStatePrioritized GoNativeState = "prioritized"
	GoNativeStateImplemented GoNativeState = "implemented"
)

type Framework struct {
	Name         string `json:"name"`
	Language     string `json:"language"`
	Category     string `json:"category"`
	Abbreviation string `json:"abbreviation"`
}

type Direction struct {
	From           string        `json:"from"`
	To             string        `json:"to"`
	Language       string        `json:"language"`
	Category       string        `json:"category"`
	Shorthands     []string      `json:"shorthands"`
	LegacyRuntime  string        `json:"legacyRuntime"`
	GoNativeState  GoNativeState `json:"goNativeState"`
	GoNativeReady  bool          `json:"goNativeReady"`
	Implementation string        `json:"implementation"`
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

var implementedDirections = map[string]struct{}{
	"cypress-selenium":       {},
	"cypress-webdriverio":    {},
	"jasmine-jest":           {},
	"jest-vitest":            {},
	"jest-jasmine":           {},
	"jest-mocha":             {},
	"cypress-playwright":     {},
	"mocha-jest":             {},
	"playwright-cypress":     {},
	"playwright-puppeteer":   {},
	"playwright-selenium":    {},
	"playwright-webdriverio": {},
	"puppeteer-playwright":   {},
	"selenium-cypress":       {},
	"selenium-playwright":    {},
	"testcafe-cypress":       {},
	"testcafe-playwright":    {},
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

func LookupDirection(from, to string) (Direction, bool) {
	key := NormalizeFramework(from) + "-" + NormalizeFramework(to)
	for _, item := range supportedDirectionKeys {
		if item == key {
			return directionFromKey(key), true
		}
	}
	return Direction{}, false
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
	from := parts[0]
	to := parts[1]
	framework := frameworks[from]
	state := GoNativeStateCataloged
	if _, ok := implementedDirections[key]; ok {
		state = GoNativeStateImplemented
	} else if _, ok := prioritizedDirections[key]; ok {
		state = GoNativeStatePrioritized
	}
	implementation := "legacy-js-runtime"
	if state == GoNativeStateImplemented {
		implementation = "go-native-runtime"
	}

	return Direction{
		From:           from,
		To:             to,
		Language:       framework.Language,
		Category:       framework.Category,
		Shorthands:     buildAliases(from, to),
		LegacyRuntime:  "javascript",
		GoNativeState:  state,
		GoNativeReady:  state == GoNativeStateImplemented,
		Implementation: implementation,
	}
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
