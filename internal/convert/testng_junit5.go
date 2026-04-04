package convert

import (
	"regexp"
	"strings"
)

var (
	reTestNGWildcardImport     = regexp.MustCompile(`(?m)^import\s+org\.testng\.annotations\.\*;\s*$`)
	reTestNGStaticAssertImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.testng\.Assert\.\*;\s*$`)
	reTestNGDisabledTest       = regexp.MustCompile(`(?m)^(\s*)@Test\s*\(\s*enabled\s*=\s*false\s*\)\s*$`)
)

var testNGToJunit5Replacer = strings.NewReplacer(
	"import org.testng.annotations.Test;", "import org.junit.jupiter.api.Test;",
	"import org.testng.annotations.BeforeMethod;", "import org.junit.jupiter.api.BeforeEach;",
	"import org.testng.annotations.AfterMethod;", "import org.junit.jupiter.api.AfterEach;",
	"import org.testng.annotations.BeforeClass;", "import org.junit.jupiter.api.BeforeAll;",
	"import org.testng.annotations.AfterClass;", "import org.junit.jupiter.api.AfterAll;",
	"import org.testng.Assert;", "import org.junit.jupiter.api.Assertions;",
	"@BeforeMethod", "@BeforeEach",
	"@AfterMethod", "@AfterEach",
	"@BeforeClass", "@BeforeAll",
	"@AfterClass", "@AfterAll",
	"Assert.", "Assertions.",
)

// ConvertTestNGToJunit5Source rewrites the common TestNG surface into
// high-confidence JUnit 5 output and comments TestNG-only workflow constructs.
func ConvertTestNGToJunit5Source(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reTestNGWildcardImport.ReplaceAllString(result, "import org.junit.jupiter.api.*;")
	result = reTestNGStaticAssertImport.ReplaceAllString(result, "import static org.junit.jupiter.api.Assertions.*;")
	result = testNGToJunit5Replacer.Replace(result)
	result = swapFirstTwoArgsOnCallLines(result,
		"Assertions.assertEquals",
		"Assertions.assertNotEquals",
		"Assertions.assertSame",
		"Assertions.assertNotSame",
		"Assertions.assertArrayEquals",
		"assertEquals",
		"assertNotEquals",
		"assertSame",
		"assertNotSame",
		"assertArrayEquals",
	)
	result = reTestNGDisabledTest.ReplaceAllString(result, "${1}@Disabled\n${1}@Test")
	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "@DataProvider") ||
			strings.Contains(line, "@Factory") ||
			strings.Contains(line, "@Parameters") ||
			strings.Contains(line, "dataProvider") ||
			strings.Contains(line, "expectedExceptions") ||
			strings.Contains(line, "timeOut") ||
			strings.Contains(line, "dependsOn") ||
			strings.Contains(line, "@BeforeSuite") ||
			strings.Contains(line, "@AfterSuite") ||
			strings.Contains(line, "@BeforeTest") ||
			strings.Contains(line, "@AfterTest")
	}, "manual TestNG feature migration required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}
