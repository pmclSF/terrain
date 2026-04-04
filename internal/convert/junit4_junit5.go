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
	result = reJUnit4TestWithArgs.ReplaceAllStringFunc(result, func(match string) string {
		parts := reJUnit4TestWithArgs.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		indent := parts[1]
		return indent + "// TERRAIN-TODO: manual JUnit 4 @Test parameter migration required\n" + indent + "@Test"
	})
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
