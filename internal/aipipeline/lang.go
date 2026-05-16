package aipipeline

import "strings"

// Language is a canonical tag for source-file language. The pipeline
// is multi-language by construction: every Stage either operates on a
// language-aware basis (RegexFastscan, ASTConfirm) or is
// language-agnostic (PathPrefilter, ChangeScope).
type Language string

const (
	LangPython     Language = "python"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangGo         Language = "go"
	LangJava       Language = "java"
	LangKotlin     Language = "kotlin"
	LangRuby       Language = "ruby"
	LangRust       Language = "rust"
	LangCSharp     Language = "csharp"
	LangUnknown    Language = ""
)

// LanguageFromPath returns the canonical Language for a file path
// based on its extension. Returns LangUnknown when no extension
// matches.
//
// Multi-language by construction is the right shape for a tool that
// scans monorepos. The regex stage emits language-tagged atoms, the
// AST stage dispatches per language, and the calibration table can
// hold per-language overrides when warranted.
func LanguageFromPath(path string) Language {
	p := strings.ToLower(path)
	for ext, lang := range extToLang {
		if strings.HasSuffix(p, ext) {
			return lang
		}
	}
	return LangUnknown
}

var extToLang = map[string]Language{
	".py":   LangPython,
	".pyi":  LangPython,
	".ts":   LangTypeScript,
	".tsx":  LangTypeScript,
	".js":   LangJavaScript,
	".jsx":  LangJavaScript,
	".mjs":  LangJavaScript,
	".cjs":  LangJavaScript,
	".go":   LangGo,
	".java": LangJava,
	".kt":   LangKotlin,
	".kts":  LangKotlin,
	".rb":   LangRuby,
	".rs":   LangRust,
	".cs":   LangCSharp,
}

// SupportedForAST reports whether the language currently has an AST
// detector wired up. Languages without an AST detector still get the
// regex / path / scope stages.
func (l Language) SupportedForAST() bool {
	switch l {
	case LangPython, LangJavaScript, LangTypeScript, LangGo, LangJava:
		return true
	}
	return false
}
