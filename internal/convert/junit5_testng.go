package convert

import (
	"regexp"
	"strings"
)

var (
	reJUnit5WildcardImport     = regexp.MustCompile(`(?m)^import\s+org\.junit\.jupiter\.api\.\*;\s*$`)
	reJUnit5StaticAssertImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.junit\.jupiter\.api\.Assertions\.\*;\s*$`)
	reJUnit5AssertThrowsLine   = regexp.MustCompile(`^(\s*)(?:(?:Assertions|Assert)\.)?assertThrows\(\s*([A-Za-z_][A-Za-z0-9_$.]*\.class)\s*,\s*\(\)\s*->\s*(.+)\)\s*;\s*$`)
	reJUnit5AssertThrowsBlock  = regexp.MustCompile(`^(\s*)(?:(?:Assertions|Assert)\.)?assertThrows\(\s*([A-Za-z_][A-Za-z0-9_$.]*\.class)\s*,\s*\(\)\s*->\s*\{\s*$`)
	reJUnit5AssertThrowsEnd    = regexp.MustCompile(`^\s*}\)\s*;\s*$`)
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
	result = convertJUnit5AssertThrowsToTestNG(result)
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

func convertJUnit5AssertThrowsToTestNG(source string) string {
	lines := strings.Split(source, "\n")
	out := make([]string, 0, len(lines))
	skipWrapper := false
	wrapperIndent := ""

	for _, line := range lines {
		if skipWrapper {
			if reJUnit5AssertThrowsEnd.MatchString(strings.TrimSpace(line)) {
				skipWrapper = false
				wrapperIndent = ""
				continue
			}
			if wrapperIndent != "" && strings.HasPrefix(line, wrapperIndent+"    ") {
				out = append(out, strings.TrimPrefix(line, wrapperIndent+"    "))
				continue
			}
			out = append(out, line)
			continue
		}

		if parts := reJUnit5AssertThrowsLine.FindStringSubmatch(line); len(parts) == 4 {
			if addExpectedExceptionsToLastTestAnnotation(&out, parts[2]) {
				out = append(out, parts[1]+strings.TrimSpace(parts[3]))
				continue
			}
		}
		if parts := reJUnit5AssertThrowsBlock.FindStringSubmatch(line); len(parts) == 3 {
			if addExpectedExceptionsToLastTestAnnotation(&out, parts[2]) {
				skipWrapper = true
				wrapperIndent = parts[1]
				continue
			}
		}
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func addExpectedExceptionsToLastTestAnnotation(lines *[]string, exceptionClass string) bool {
	for i := len(*lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace((*lines)[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.HasPrefix(trimmed, "@Test") {
			(*lines)[i] = mergeJavaTestAnnotationAttribute((*lines)[i], "expectedExceptions", exceptionClass)
			return true
		}
		if strings.HasPrefix(trimmed, "public ") || strings.HasPrefix(trimmed, "private ") || strings.HasPrefix(trimmed, "protected ") || strings.Contains(trimmed, " void ") {
			return false
		}
	}
	return false
}

func mergeJavaTestAnnotationAttribute(line, key, value string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	trimmed := strings.TrimSpace(line)
	if trimmed == "@Test" {
		return indent + "@Test(" + key + " = " + value + ")"
	}
	open := strings.IndexByte(trimmed, '(')
	close := strings.LastIndexByte(trimmed, ')')
	if open < 0 || close <= open {
		return line
	}
	args := splitTopLevelArgs(trimmed[open+1 : close])
	for _, arg := range args {
		if strings.HasPrefix(strings.TrimSpace(arg), key+" ") || strings.HasPrefix(strings.TrimSpace(arg), key+"=") {
			return line
		}
	}
	args = append([]string{key + " = " + value}, args...)
	return indent + "@Test(" + strings.Join(args, ", ") + ")"
}
