package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertJUnit4ToJunit5Source_ConvertsLifecycleImportsAndAssertions(t *testing.T) {
	t.Parallel()

	input := `import org.junit.Test;
import org.junit.Before;
import org.junit.Assert;

public class ExampleTest {
    @Before
    public void setUp() {
        // setup
    }

    @Test
    public void testValue() {
        Assert.assertEquals(42, getValue());
    }
}
`

	got, err := ConvertJUnit4ToJunit5Source(input)
	if err != nil {
		t.Fatalf("ConvertJUnit4ToJunit5Source returned error: %v", err)
	}
	for _, want := range []string{
		"import org.junit.jupiter.api.Test;",
		"import org.junit.jupiter.api.BeforeEach;",
		"import org.junit.jupiter.api.Assertions;",
		"@BeforeEach",
		"Assertions.assertEquals(42, getValue())",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertJUnit5ToTestNGSource_ReordersAssertEqualsAndHooks(t *testing.T) {
	t.Parallel()

	input := `import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Assertions;

public class ExampleTest {
    @BeforeEach
    void setUp() {}

    @Test
    void testValue() {
        Assertions.assertEquals(42, getValue());
    }
}
`

	got, err := ConvertJUnit5ToTestNGSource(input)
	if err != nil {
		t.Fatalf("ConvertJUnit5ToTestNGSource returned error: %v", err)
	}
	for _, want := range []string{
		"import org.testng.annotations.BeforeMethod;",
		"import org.testng.annotations.Test;",
		"import org.testng.Assert;",
		"@BeforeMethod",
		"Assert.assertEquals(getValue(), 42)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestConvertTestNGToJunit5Source_ReordersAssertEqualsAndDisabled(t *testing.T) {
	t.Parallel()

	input := `import org.testng.annotations.Test;
import org.testng.Assert;

public class ExampleTest {
    @Test(enabled = false)
    public void testValue() {
        Assert.assertEquals(getValue(), 42);
    }
}
`

	got, err := ConvertTestNGToJunit5Source(input)
	if err != nil {
		t.Fatalf("ConvertTestNGToJunit5Source returned error: %v", err)
	}
	for _, want := range []string{
		"import org.junit.jupiter.api.Test;",
		"import org.junit.jupiter.api.Assertions;",
		"@Disabled",
		"@Test",
		"Assertions.assertEquals(42, getValue())",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExecuteJUnit4ToJunit5Directory_ConvertsJavaFilesAndPreservesHelpers(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "converted")
	testPath := filepath.Join(sourceDir, "ExampleTest.java")
	helperPath := filepath.Join(sourceDir, "Support.java")
	if err := os.WriteFile(testPath, []byte("import org.junit.Test;\npublic class ExampleTest { @Test public void testValue() {} }\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("public class Support {}\n"), 0o644); err != nil {
		t.Fatalf("write helper file: %v", err)
	}

	direction, ok := LookupDirection("junit4", "junit5")
	if !ok {
		t.Fatal("expected junit4 -> junit5 direction to exist")
	}

	result, err := Execute(sourceDir, direction, ExecuteOptions{Output: outputDir})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Mode != "directory" {
		t.Fatalf("mode = %q, want directory", result.Mode)
	}

	convertedTest, err := os.ReadFile(filepath.Join(outputDir, "ExampleTest.java"))
	if err != nil {
		t.Fatalf("read converted test: %v", err)
	}
	if !strings.Contains(string(convertedTest), "org.junit.jupiter.api.Test") {
		t.Fatalf("expected converted junit import, got:\n%s", convertedTest)
	}

	convertedHelper, err := os.ReadFile(filepath.Join(outputDir, "Support.java"))
	if err != nil {
		t.Fatalf("read copied helper: %v", err)
	}
	if string(convertedHelper) != "public class Support {}\n" {
		t.Fatalf("expected helper file to be preserved, got:\n%s", convertedHelper)
	}
}
