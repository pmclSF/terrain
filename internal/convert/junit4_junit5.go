package convert

import (
	"regexp"
	"strings"
)

var (
	reJUnit4WildcardImport     = regexp.MustCompile(`(?m)^import\s+org\.junit\.\*;\s*$`)
	reJUnit4TestWithArgs       = regexp.MustCompile(`(?m)^(\s*)@Test\s*\(([^)]*)\)\s*$`)
	reJUnit4StaticAssertImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.junit\.Assert\.\*;\s*$`)
	reJUnit4StaticAssumeImport = regexp.MustCompile(`(?m)^import\s+static\s+org\.junit\.Assume\.\*;\s*$`)
	reJUnit4ExpectedAttr       = regexp.MustCompile(`\bexpected\s*=\s*([A-Za-z_][A-Za-z0-9_$.]*)\s*\.class\b`)
)

var junit4ToJunit5Replacer = strings.NewReplacer(
	"import org.junit.Test;", "import org.junit.jupiter.api.Test;",
	"import org.junit.Before;", "import org.junit.jupiter.api.BeforeEach;",
	"import org.junit.After;", "import org.junit.jupiter.api.AfterEach;",
	"import org.junit.BeforeClass;", "import org.junit.jupiter.api.BeforeAll;",
	"import org.junit.AfterClass;", "import org.junit.jupiter.api.AfterAll;",
	"import org.junit.Assert;", "import org.junit.jupiter.api.Assertions;",
	"import org.junit.Assume;", "import org.junit.jupiter.api.Assumptions;",
	"import org.junit.Ignore;", "import org.junit.jupiter.api.Disabled;",
	"@BeforeClass", "@BeforeAll",
	"@AfterClass", "@AfterAll",
	"@Before", "@BeforeEach",
	"@After", "@AfterEach",
	"@Ignore", "@Disabled",
	"Assert.", "Assertions.",
	"Assume.", "Assumptions.",
)

// ConvertJUnit4ToJunit5Source rewrites the common JUnit 4 surface into
// high-confidence JUnit 5 output and comments unsupported legacy constructs.
func ConvertJUnit4ToJunit5Source(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}

	result := strings.ReplaceAll(source, "\r\n", "\n")
	result = reJUnit4WildcardImport.ReplaceAllString(result, "import org.junit.jupiter.api.*;")
	result = reJUnit4StaticAssertImport.ReplaceAllString(result, "import static org.junit.jupiter.api.Assertions.*;")
	result = reJUnit4StaticAssumeImport.ReplaceAllString(result, "import static org.junit.jupiter.api.Assumptions.*;")
	result = junit4ToJunit5Replacer.Replace(result)
	result = convertJUnit4ExpectedTests(result)
	result = commentMatchedLines(result, func(line string) bool {
		return strings.Contains(line, "@RunWith(") ||
			strings.Contains(line, "@Rule") ||
			strings.Contains(line, "@ClassRule") ||
			strings.Contains(line, "@Category(") ||
			strings.Contains(line, "@Parameters") ||
			strings.Contains(line, "Parameterized.")
	}, "manual JUnit 4 feature migration required")
	result = collapseBlankLines(result)
	return ensureTrailingNewline(result), nil
}

func convertJUnit4ExpectedTests(source string) string {
	lines := strings.Split(source, "\n")
	output := make([]string, 0, len(lines)+8)

	pendingExpected := ""
	wrapMethod := false
	methodDepth := 0
	wrapperIndent := ""
	usedAssertions := false

	for _, line := range lines {
		if pendingExpected == "" {
			if parts := reJUnit4TestWithArgs.FindStringSubmatch(line); len(parts) == 3 {
				indent := parts[1]
				if expected, ok := extractJUnit4ExpectedException(parts[2]); ok {
					output = append(output, indent+"@Test")
					pendingExpected = expected
					usedAssertions = true
					continue
				}
				output = append(output, indent+"// TERRAIN-TODO: manual JUnit 4 @Test parameter migration required")
				output = append(output, indent+"@Test")
				continue
			}
			output = append(output, line)
			continue
		}

		output = append(output, line)
		if !wrapMethod {
			if strings.Contains(line, "{") {
				wrapperIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))] + "    "
				output = append(output, wrapperIndent+"Assertions.assertThrows("+pendingExpected+".class, () -> {")
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

func extractJUnit4ExpectedException(attrs string) (string, bool) {
	args := splitTopLevelArgs(attrs)
	if len(args) != 1 {
		return "", false
	}
	match := reJUnit4ExpectedAttr.FindStringSubmatch(args[0])
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func ensureJUnit5AssertionsImport(source string) string {
	if strings.Contains(source, "import org.junit.jupiter.api.Assertions;") ||
		strings.Contains(source, "import org.junit.jupiter.api.*;") {
		return source
	}

	lines := strings.Split(source, "\n")
	insertAt := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			insertAt = i + 1
		}
	}
	if insertAt >= 0 {
		lines = append(lines[:insertAt], append([]string{"import org.junit.jupiter.api.Assertions;"}, lines[insertAt:]...)...)
		return strings.Join(lines, "\n")
	}

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			insertAt = i + 1
			lines = append(lines[:insertAt], append([]string{"", "import org.junit.jupiter.api.Assertions;"}, lines[insertAt:]...)...)
			return strings.Join(lines, "\n")
		}
	}

	return "import org.junit.jupiter.api.Assertions;\n" + source
}
