package convert

import (
	"regexp"
	"strings"
)

var (
	reTestNGWildcardImport     = regexp.MustCompile(`(?m)^import\s+org\.testng\.annotations\.\*;\s*$`)
	reTestNGStaticAssertImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.testng\.Assert\.\*;\s*$`)
	reTestNGDisabledTest       = regexp.MustCompile(`(?m)^(\s*)@Test\s*\(\s*enabled\s*=\s*false\s*\)\s*$`)
	reTestNGExpectedAttr       = regexp.MustCompile(`\bexpectedExceptions\s*=\s*([A-Za-z_][A-Za-z0-9_$.]*\.class)\b`)
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
	result = convertTestNGExpectedExceptions(result)
	result = reTestNGDisabledTest.ReplaceAllString(result, "${1}@Disabled\n${1}@Test")
	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "@DataProvider") ||
			strings.Contains(line, "@Factory") ||
			strings.Contains(line, "@Parameters") ||
			strings.Contains(line, "dataProvider") ||
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

func convertTestNGExpectedExceptions(source string) string {
	lines := strings.Split(source, "\n")
	output := make([]string, 0, len(lines)+8)

	pendingExpected := ""
	wrapMethod := false
	methodDepth := 0
	wrapperIndent := ""
	usedAssertions := false

	for _, line := range lines {
		if pendingExpected == "" {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "@Test(") && strings.Contains(trimmed, "expectedExceptions") {
				if converted, expected, ok := convertTestNGExpectedAnnotation(line); ok {
					output = append(output, converted)
					pendingExpected = expected
					usedAssertions = true
					continue
				}
			}
			output = append(output, line)
			continue
		}

		output = append(output, line)
		if !wrapMethod {
			if strings.Contains(line, "{") {
				wrapperIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))] + "    "
				output = append(output, wrapperIndent+"Assertions.assertThrows("+pendingExpected+", () -> {")
				open, close := countJavaBraces(line)
				methodDepth = open - close
				if methodDepth <= 0 {
					methodDepth = 1
				}
				wrapMethod = true
			}
			continue
		}

		open, close := countJavaBraces(line)
		if methodDepth+open-close <= 0 {
			output = output[:len(output)-1]
			output = append(output, wrapperIndent+"});")
			output = append(output, line)
			pendingExpected = ""
			wrapMethod = false
			methodDepth = 0
			wrapperIndent = ""
			continue
		}
		methodDepth += open - close
	}

	result := strings.Join(output, "\n")
	if usedAssertions {
		result = ensureJUnit5AssertionsImport(result)
	}
	return result
}

func convertTestNGExpectedAnnotation(line string) (string, string, bool) {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	trimmed := strings.TrimSpace(line)
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return "", "", false
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	remaining := make([]string, 0, len(args))
	expected := ""
	for _, arg := range args {
		match := reTestNGExpectedAttr.FindStringSubmatch(strings.TrimSpace(arg))
		if len(match) == 2 {
			expected = match[1]
			continue
		}
		remaining = append(remaining, strings.TrimSpace(arg))
	}
	if expected == "" {
		return "", "", false
	}
	if len(remaining) == 0 {
		return indent + "@Test", expected, true
	}
	return indent + "@Test(" + strings.Join(remaining, ", ") + ")", expected, true
}
