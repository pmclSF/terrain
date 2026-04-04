package convert

import (
	"regexp"
	"strings"
)

var (
	reJUnit5WildcardImport     = regexp.MustCompile(`(?m)^import\s+org\.junit\.jupiter\.api\.\*;\s*$`)
	reJUnit5StaticAssertImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.junit\.jupiter\.api\.Assertions\.\*;\s*$`)
)

var junit5ToTestNGReplacer = strings.NewReplacer(
	"import org.junit.jupiter.api.Test;", "import org.testng.annotations.Test;",
	"import org.junit.jupiter.api.BeforeEach;", "import org.testng.annotations.BeforeMethod;",
	"import org.junit.jupiter.api.AfterEach;", "import org.testng.annotations.AfterMethod;",
	"import org.junit.jupiter.api.BeforeAll;", "import org.testng.annotations.BeforeClass;",
	"import org.junit.jupiter.api.AfterAll;", "import org.testng.annotations.AfterClass;",
	"import org.junit.jupiter.api.Assertions;", "import org.testng.Assert;",
	"@BeforeEach", "@BeforeMethod",
	"@AfterEach", "@AfterMethod",
	"@BeforeAll", "@BeforeClass",
	"@AfterAll", "@AfterClass",
	"Assertions.", "Assert.",
)

// ConvertJUnit5ToTestNGSource rewrites the common JUnit 5 surface into
// high-confidence TestNG output and annotates unsupported Jupiter-only features.
func ConvertJUnit5ToTestNGSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reJUnit5WildcardImport.ReplaceAllString(result, "import org.testng.annotations.*;")
	result = reJUnit5StaticAssertImport.ReplaceAllString(result, "import static org.testng.Assert.*;")
	result = junit5ToTestNGReplacer.Replace(result)
	result = swapFirstTwoArgsOnCallLines(result,
		"Assert.assertEquals",
		"Assert.assertNotEquals",
		"Assert.assertSame",
		"Assert.assertNotSame",
		"Assert.assertArrayEquals",
		"assertEquals",
		"assertNotEquals",
		"assertSame",
		"assertNotSame",
		"assertArrayEquals",
	)
	result = collapseDisabledJUnit5TestPair(result)
	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "@Disabled") ||
			strings.Contains(line, "@ParameterizedTest") ||
			strings.Contains(line, "org.junit.jupiter.params") ||
			strings.Contains(line, "@ValueSource") ||
			strings.Contains(line, "@CsvSource") ||
			strings.Contains(line, "@MethodSource") ||
			strings.Contains(line, "@EnumSource") ||
			strings.Contains(line, "@NullAndEmptySource") ||
			strings.Contains(line, "@DisplayName") ||
			strings.Contains(line, "@Nested") ||
			strings.Contains(line, "@ExtendWith") ||
			strings.Contains(line, "@Tag(") ||
			strings.Contains(line, "assertThrows(") ||
			strings.Contains(line, "assertTimeout(") ||
			strings.Contains(line, "assertAll(")
	}, "manual JUnit 5 feature migration required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}

func collapseDisabledJUnit5TestPair(source string) string {
	lines := strings.Split(source, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "@Disabled" && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "@Test" {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
			out = append(out, indent+"@Test(enabled = false)")
			i++
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}
