package testcase

import "testing"

// TestExtractJava_NestedClasses confirms @Nested classes contribute
// to the suite hierarchy while plain helper inner classes don't.
func TestExtractJava_NestedClasses(t *testing.T) {
	t.Parallel()

	src := `
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Nested;

class CalculatorTest {
    @Test
    void adds() {}

    @Nested
    class WhenNegative {
        @Test
        void rejectsBoth() {}
    }

    // helper class — NOT a JUnit @Nested suite
    static class Helper {
        @Test
        void shouldNotShowUnderHelper() {}
    }
}
`
	cases := extractJavaWithAST(src, "CalculatorTest.java", "junit5")

	byName := map[string]TestCase{}
	for _, c := range cases {
		byName[c.TestName] = c
	}

	adds, ok := byName["adds"]
	if !ok {
		t.Fatal("missing 'adds' case")
	}
	if len(adds.SuiteHierarchy) != 1 || adds.SuiteHierarchy[0] != "CalculatorTest" {
		t.Errorf("adds hierarchy = %v, want [CalculatorTest]", adds.SuiteHierarchy)
	}

	rejects, ok := byName["rejectsBoth"]
	if !ok {
		t.Fatal("missing 'rejectsBoth' case (inside @Nested)")
	}
	want := []string{"CalculatorTest", "WhenNegative"}
	if !slicesEqual(rejects.SuiteHierarchy, want) {
		t.Errorf("rejectsBoth hierarchy = %v, want %v", rejects.SuiteHierarchy, want)
	}

	// Helper class is non-@Nested but contains a @Test-annotated method.
	// The method should be discovered (it's a real test) but should
	// attribute to the OUTER class, not the helper class — helpers
	// don't form suites.
	helper, ok := byName["shouldNotShowUnderHelper"]
	if !ok {
		t.Fatal("missing 'shouldNotShowUnderHelper'")
	}
	if len(helper.SuiteHierarchy) != 1 || helper.SuiteHierarchy[0] != "CalculatorTest" {
		t.Errorf("shouldNotShowUnderHelper hierarchy = %v, want [CalculatorTest]", helper.SuiteHierarchy)
	}
}

// TestExtractJava_DisplayName confirms the @DisplayName annotation
// flows through to TestCase.DisplayName when present, while TestName
// continues to carry the Java method name for stable identity.
func TestExtractJava_DisplayName(t *testing.T) {
	t.Parallel()

	src := `
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

class LoginTest {
    @Test
    @DisplayName("rejects expired credentials")
    void rejectsExpired() {}

    @Test
    void plain() {}
}
`
	cases := extractJavaWithAST(src, "LoginTest.java", "junit5")
	byName := map[string]TestCase{}
	for _, c := range cases {
		byName[c.TestName] = c
	}

	rejects, ok := byName["rejectsExpired"]
	if !ok {
		t.Fatal("missing 'rejectsExpired'")
	}
	if rejects.DisplayName != "rejects expired credentials" {
		t.Errorf("DisplayName = %q, want %q", rejects.DisplayName, "rejects expired credentials")
	}

	plain, ok := byName["plain"]
	if !ok {
		t.Fatal("missing 'plain'")
	}
	if plain.DisplayName != "" {
		t.Errorf("plain.DisplayName = %q, want empty", plain.DisplayName)
	}
}
