package fixscaffold

// RegistryAdapter is a small shim that lets a Registry satisfy
// aipipeline.ScaffoldGenerator without aipipeline depending on
// fixscaffold (which would be an import cycle if the dependency went
// the other way).
type RegistryAdapter struct {
	*Registry
}

// NewRegistryAdapter wraps a Registry. Use this when constructing a
// Composer: composer.Scaffolds = fixscaffold.NewRegistryAdapter(reg).
func NewRegistryAdapter(r *Registry) *RegistryAdapter {
	return &RegistryAdapter{Registry: r}
}

// GenerateScaffold returns the scaffold body, target path, and
// description for a given (rule, surface, language). Returns empty
// strings when no scaffold is registered for the rule.
func (a *RegistryAdapter) GenerateScaffold(ruleID, surfacePath, lang string) (string, string, string) {
	if a == nil || a.Registry == nil {
		return "", "", ""
	}
	s := a.Generate(ruleID, surfacePath, lang)
	if s == nil {
		return "", "", ""
	}
	return s.Body, s.TargetPath, s.Description
}
