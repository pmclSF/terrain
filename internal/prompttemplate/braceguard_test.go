package prompttemplate

import (
	"strings"
	"testing"
)

// TestMustache_MalformedBraceNotPhantomVar: a malformed triple-open sequence
// `{{{x}}` must render the stray brace literally and treat `{{x}}` as the
// variable — not invent a phantom variable named "{x" that can never resolve.
func TestMustache_MalformedBraceNotPhantomVar(t *testing.T) {
	t.Parallel()
	tpl := Template{Kind: KindMustache, Body: "{{{x}}"}

	out, err := tpl.Render(map[string]string{"x": "VAL"})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if out != "{VAL" {
		t.Fatalf("render = %q, want {VAL", out)
	}
	for _, v := range tpl.Vars() {
		if strings.ContainsAny(v, "{}") {
			t.Fatalf("Vars returned a malformed name %q", v)
		}
	}
}
