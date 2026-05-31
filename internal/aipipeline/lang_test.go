package aipipeline

import "testing"

func TestLanguageFromPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want Language
	}{
		{"app/handlers/chat.py", LangPython},
		{"src/components/Chat.tsx", LangTypeScript},
		{"src/app/page.ts", LangTypeScript},
		{"public/main.js", LangJavaScript},
		{"public/main.mjs", LangJavaScript},
		{"cmd/server/main.go", LangGo},
		{"app/src/main/java/com/example/Foo.java", LangJava},
		{"app/MainActivity.kt", LangKotlin},
		{"lib/handler.rb", LangRuby},
		{"src/lib.rs", LangRust},
		{"Program.cs", LangCSharp},
		{"README.md", LangUnknown},
	}
	for _, c := range cases {
		got := LanguageFromPath(c.path)
		if got != c.want {
			t.Errorf("LanguageFromPath(%q) = %q; want %q", c.path, got, c.want)
		}
	}
}

func TestLanguageSupportedForAST(t *testing.T) {
	t.Parallel()
	supported := []Language{LangPython, LangJavaScript, LangTypeScript, LangGo, LangJava}
	unsupported := []Language{LangKotlin, LangRuby, LangRust, LangCSharp, LangUnknown}
	for _, l := range supported {
		if !l.SupportedForAST() {
			t.Errorf("%q should be SupportedForAST", l)
		}
	}
	for _, l := range unsupported {
		if l.SupportedForAST() {
			t.Errorf("%q should NOT be SupportedForAST", l)
		}
	}
}
