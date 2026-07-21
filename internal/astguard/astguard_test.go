package astguard

import (
	"strings"
	"testing"
)

func TestLooksPathological_RealCodeIsFine(t *testing.T) {
	t.Parallel()
	samples := []string{
		"import openai\nclient = openai.OpenAI()\n",
		"def f(x):\n    return g(h(i(j(x))))\n", // ordinary nesting
		strings.Repeat("if True:\n    x = [1, 2, {3: (4, 5)}]\n", 500),
		"", // empty
	}
	for i, s := range samples {
		if LooksPathological([]byte(s)) {
			t.Errorf("sample %d flagged as pathological but is ordinary code", i)
		}
	}
}

func TestLooksPathological_DeepNestingFlagged(t *testing.T) {
	t.Parallel()
	// The exact crafted vector: thousands of unclosed opening brackets.
	if !LooksPathological([]byte("x = " + strings.Repeat("(", 50000))) {
		t.Fatal("deeply nested unclosed brackets must be flagged")
	}
	// Deeply nested but balanced is just as pathological for the parser.
	deep := strings.Repeat("[", 5000) + strings.Repeat("]", 5000)
	if !LooksPathological([]byte(deep)) {
		t.Fatal("deeply nested balanced brackets must be flagged")
	}
}

func TestLooksPathological_WideNotDeepIsFine(t *testing.T) {
	t.Parallel()
	// Many shallow bracket pairs in sequence (wide, not nested) is ordinary —
	// e.g. a big list of tuples — and must NOT be flagged.
	if LooksPathological([]byte(strings.Repeat("(1,2),", 100000))) {
		t.Fatal("wide-but-shallow bracket use must not be flagged")
	}
}
