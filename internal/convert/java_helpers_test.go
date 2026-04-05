package convert

import (
	"reflect"
	"testing"
)

func TestSplitTopLevelArgs_HandlesNestedCallsAndQuotedCommas(t *testing.T) {
	t.Parallel()

	got := splitTopLevelArgs(`expected, call(1, 2), "a,b", mapOf("x", listOf(1, 2))`)
	want := []string{
		"expected",
		"call(1, 2)",
		`"a,b"`,
		`mapOf("x", listOf(1, 2))`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTopLevelArgs() = %#v, want %#v", got, want)
	}
}

func TestSplitTopLevelArgs_HandlesTemplateLiteralCommas(t *testing.T) {
	t.Parallel()

	got := splitTopLevelArgs("selector, `value,${count}`, { timeout: 5000 }")
	want := []string{
		"selector",
		"`value,${count}`",
		"{ timeout: 5000 }",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTopLevelArgs() = %#v, want %#v", got, want)
	}
}

func TestSplitTopLevelArgs_IgnoresTrailingComma(t *testing.T) {
	t.Parallel()

	got := splitTopLevelArgs("selector, { timeout: 5000 },")
	want := []string{
		"selector",
		"{ timeout: 5000 }",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTopLevelArgs() = %#v, want %#v", got, want)
	}
}

func TestFindMatchingParenSameLine_HandlesQuotedParens(t *testing.T) {
	t.Parallel()

	line := `assertEquals("(", actual)`
	open := len("assertEquals")
	got := findMatchingParenSameLine(line, open)
	if got != len(line)-1 {
		t.Fatalf("findMatchingParenSameLine() = %d, want %d", got, len(line)-1)
	}
}

func TestFindMatchingParenSameLine_HandlesTemplateLiteralParens(t *testing.T) {
	t.Parallel()

	line := "fill(`name,${user})`, value)"
	open := len("fill")
	got := findMatchingParenSameLine(line, open)
	if got != len(line)-1 {
		t.Fatalf("findMatchingParenSameLine() = %d, want %d", got, len(line)-1)
	}
}

func TestSwapFirstTwoArgsOnLine_SwapsStandaloneCallOnly(t *testing.T) {
	t.Parallel()

	line := `assertEquals(expectedValue(), actualValue()); obj.assertEquals(keep, order);`
	got := swapFirstTwoArgsOnLine(line, "assertEquals")
	want := `assertEquals(actualValue(), expectedValue()); obj.assertEquals(keep, order);`
	if got != want {
		t.Fatalf("swapFirstTwoArgsOnLine() = %q, want %q", got, want)
	}
}

func TestSwapFirstTwoArgsOnCallLines_SkipsComments(t *testing.T) {
	t.Parallel()

	source := "// assertEquals(expected, actual)\nassertEquals(expected, actual)\n"
	got := swapFirstTwoArgsOnCallLines(source, "assertEquals")
	want := "// assertEquals(expected, actual)\nassertEquals(actual, expected)\n"
	if got != want {
		t.Fatalf("swapFirstTwoArgsOnCallLines() = %q, want %q", got, want)
	}
}

func TestCountJavaBraces_IgnoresQuotedBraces(t *testing.T) {
	t.Parallel()

	open, close := countJavaBraces(`String s = "{"; if (ready) { log("}"); }`)
	if open != 1 || close != 1 {
		t.Fatalf("countJavaBraces() = (%d, %d), want (1, 1)", open, close)
	}
}
